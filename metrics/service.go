package metrics

import (
	"errors"
	"time"

	"github.com/dansimau/hal/logger"
	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// rollupInterval is how often completed minutes are aggregated into rollups.
	rollupInterval = time.Minute
	// rawRetention is how long individual raw metric points are kept. Raw points
	// are only needed for the high-resolution "last minute" view; everything
	// longer is served from rollups, so this can be short.
	rawRetention = 24 * time.Hour
	// rollupRetention is how long per-minute rollups are kept.
	rollupRetention = 90 * 24 * time.Hour
	// pruneInterval is how often old raw points and rollups are deleted.
	pruneInterval = time.Hour
	// backfillChunk is the raw-data window processed per iteration when catching
	// up. Chunking bounds memory when rolling up a large historical backlog.
	backfillChunk = 24 * time.Hour
)

// Service handles metrics collection, rollup and pruning.
// Raw points are written via the async write queue for non-blocking performance,
// then periodically aggregated into per-minute rollups for fast long-range queries.
type Service struct {
	db       *store.Store
	stopChan chan struct{}
}

// NewService creates a new metrics service
func NewService(db *store.Store) *Service {
	return &Service{
		db:       db,
		stopChan: make(chan struct{}),
	}
}

// Start begins the rollup and pruning goroutines.
func (s *Service) Start() {
	go s.rollupLoop()
	go s.pruneLoop()
	logger.Info("Metrics service started", "")
}

// Stop stops the metrics service
func (s *Service) Stop() {
	close(s.stopChan)
	logger.Info("Metrics service stopped", "")
}

// RecordCounter records a counter metric (value = 1)
func (s *Service) RecordCounter(metricType store.MetricType, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          1,
		EntityID:       entityID,
		AutomationName: automationName,
	}

	s.db.EnqueueWrite(func(db *gorm.DB) error {
		return db.Create(&metric).Error
	})
}

// RecordTimer records a timer metric (value = duration in nanoseconds)
func (s *Service) RecordTimer(metricType store.MetricType, duration time.Duration, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          duration.Nanoseconds(),
		EntityID:       entityID,
		AutomationName: automationName,
	}

	s.db.EnqueueWrite(func(db *gorm.DB) error {
		return db.Create(&metric).Error
	})
}

// rollupLoop rolls up completed minutes from raw data. The first pass catches up
// all historical raw (the initial backfill); subsequent passes roll up each newly
// completed minute. A single mechanism handles backfill, gap-filling after
// downtime, and steady state (see rollupPending).
func (s *Service) rollupLoop() {
	ticker := time.NewTicker(rollupInterval)
	defer ticker.Stop()

	for {
		if err := s.rollupPending(time.Now()); err != nil {
			logger.Error("Failed to roll up metrics", "", "error", err)
		}

		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
		}
	}
}

// pruneLoop periodically deletes old raw points and old rollups.
func (s *Service) pruneLoop() {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			if err := pruneOldData(s.db.DB, time.Now()); err != nil {
				logger.Error("Failed to prune metrics", "", "error", err)
			}
		}
	}
}

// rollupPending rolls up every completed minute that has not been rolled up yet:
// from just after the latest existing rollup (or the earliest raw point if none
// exist) up to the current minute, in day-sized chunks. This one mechanism does
// the initial historical backfill, closes gaps left by downtime or a slow
// backfill, and performs steady-state per-minute rollups. Because the resume
// point is derived from the rollups themselves, a failed pass changes nothing and
// is simply retried on the next tick.
func (s *Service) rollupPending(now time.Time) error {
	upper := now.Truncate(time.Minute) // exclude the in-progress minute

	lower, ok, err := rollupResumePoint(s.db.DB)
	if err != nil {
		return err
	}
	if !ok {
		return nil // no raw data to roll up
	}

	for chunkStart := lower; chunkStart.Before(upper); chunkStart = chunkStart.Add(backfillChunk) {
		chunkEnd := chunkStart.Add(backfillChunk)
		if chunkEnd.After(upper) {
			chunkEnd = upper
		}

		if err := rollupWindow(s.db.DB, chunkStart, chunkEnd); err != nil {
			return err
		}
	}

	return nil
}

// rollupResumePoint returns the minute from which rolling up should resume: just
// after the latest existing rollup, or the earliest raw point if none exist. The
// bool is false when there is no raw data at all.
func rollupResumePoint(db *gorm.DB) (time.Time, bool, error) {
	watermark, ok, err := latestRolledMinute(db)
	if err != nil {
		return time.Time{}, false, err
	}
	if ok {
		// Resume at the minute after the latest rolled-up one.
		return time.Unix(watermark, 0).Add(time.Minute), true, nil
	}

	// No rollups yet: start from the earliest raw point. Fetch it via the ORM so
	// the timestamp deserialises correctly (a raw MIN() aggregate comes back as a
	// string).
	var earliest store.Metric
	if err := db.Select("timestamp").Order("timestamp ASC").First(&earliest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}

		return time.Time{}, false, err
	}

	return earliest.Timestamp.Truncate(time.Minute), true, nil
}

// latestRolledMinute returns the BucketStart (Unix seconds) of the most recent
// rollup, or ok=false when there are no rollups.
func latestRolledMinute(db *gorm.DB) (int64, bool, error) {
	var result struct {
		Max *int64
	}
	if err := db.Model(&store.MetricRollup{}).Select("MAX(bucket_start) as max").Scan(&result).Error; err != nil {
		return 0, false, err
	}
	if result.Max == nil {
		return 0, false, nil
	}

	return *result.Max, true, nil
}

// rollupWindow aggregates raw metrics in [lower, upper) into per-minute rollups
// and upserts them, overwriting any existing rollup for the same bucket. Since
// rollups are recomputed contiguously from the watermark, overwriting is
// idempotent.
func rollupWindow(db *gorm.DB, lower, upper time.Time) error {
	var raw []store.Metric
	if err := db.
		Select("metric_type", "timestamp", "value").
		Where("timestamp >= ? AND timestamp < ?", lower, upper).
		Find(&raw).Error; err != nil {
		return err
	}

	rollups := aggregate(raw)
	if len(rollups) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "metric_type"}, {Name: "bucket_start"}},
		DoUpdates: clause.AssignmentColumns([]string{"count", "sum", "histogram"}),
	}).Create(&rollups).Error
}

// aggregate groups raw metrics into per-minute rollups, building a count, sum and
// log-scale histogram for each bucket. The histogram lets true quantiles be
// computed later over any range of merged rollups.
func aggregate(raw []store.Metric) []store.MetricRollup {
	type key struct {
		metricType store.MetricType
		bucket     int64
	}

	type accumulator struct {
		count     int64
		sum       int64
		histogram map[int32]int64
	}

	accs := make(map[key]*accumulator)
	for _, m := range raw {
		k := key{m.MetricType, m.Timestamp.Truncate(time.Minute).Unix()}

		acc := accs[k]
		if acc == nil {
			acc = &accumulator{histogram: make(map[int32]int64)}
			accs[k] = acc
		}

		acc.count++
		acc.sum += m.Value
		acc.histogram[store.HistogramBucket(m.Value)]++
	}

	rollups := make([]store.MetricRollup, 0, len(accs))
	for k, acc := range accs {
		rollups = append(rollups, store.MetricRollup{
			MetricType:  k.metricType,
			BucketStart: k.bucket,
			Count:       acc.count,
			Sum:         acc.sum,
			Histogram:   acc.histogram,
		})
	}

	return rollups
}

// pruneOldData deletes rollups older than rollupRetention and raw points older
// than rawRetention. Raw is only pruned once it has been captured in rollups: the
// rollup watermark must have advanced past the raw cut-off. This means an
// incomplete or failed catch-up simply defers raw pruning (no data is lost), and
// pruning resumes automatically once the rollups catch up.
func pruneOldData(db *gorm.DB, now time.Time) error {
	rawCutoff := now.Add(-rawRetention)

	watermark, ok, err := latestRolledMinute(db)
	if err != nil {
		return err
	}

	switch {
	case ok && watermark >= rawCutoff.Unix():
		if result := db.Where("timestamp < ?", rawCutoff).Delete(&store.Metric{}); result.Error != nil {
			logger.Error("Failed to prune old raw metrics", "", "error", result.Error)
			return result.Error
		} else if result.RowsAffected > 0 {
			logger.Info("Pruned old raw metrics", "", "count", result.RowsAffected, "cutoff", rawCutoff)
		}
	default:
		logger.Info("Deferring raw metric prune until rollups catch up", "", "cutoff", rawCutoff)
	}

	rollupCutoff := now.Add(-rollupRetention).Unix()
	if result := db.Where("bucket_start < ?", rollupCutoff).Delete(&store.MetricRollup{}); result.Error != nil {
		logger.Error("Failed to prune old metric rollups", "", "error", result.Error)
		return result.Error
	} else if result.RowsAffected > 0 {
		logger.Info("Pruned old metric rollups", "", "count", result.RowsAffected, "cutoff", rollupCutoff)
	}

	return nil
}
