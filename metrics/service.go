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
	// rollupLookback is how far back each incremental rollup pass reprocesses
	// completed minutes. Reprocessing is idempotent (upsert) and guards against
	// gaps if a pass is delayed or samples arrive slightly late.
	rollupLookback = 5 * time.Minute
)

// Service handles metrics collection, rollup and pruning.
// Raw points are written via the async write queue for non-blocking performance,
// then periodically aggregated into per-minute rollups for fast long-range queries.
type Service struct {
	db       *store.Store
	stopChan chan struct{}
	// backfillDone is closed once the historical raw data has been rolled up. Raw
	// pruning is held off until then so that un-rolled history is never deleted.
	backfillDone chan struct{}
}

// NewService creates a new metrics service
func NewService(db *store.Store) *Service {
	return &Service{
		db:           db,
		stopChan:     make(chan struct{}),
		backfillDone: make(chan struct{}),
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

// rollupLoop performs a one-time backfill of any historical raw data, then
// incrementally rolls up completed minutes.
func (s *Service) rollupLoop() {
	// Roll up all historical raw data so long-range queries have data immediately
	// (rather than waiting for it to accumulate minute by minute). Runs once at
	// startup. Only on success do we allow raw pruning to proceed, so that raw
	// history is never deleted before it has been captured in rollups.
	if err := backfillRollups(s.db.DB, time.Now()); err != nil {
		logger.Error("Failed to backfill metric rollups", "", "error", err)
	} else {
		close(s.backfillDone)
	}

	ticker := time.NewTicker(rollupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.db.EnqueueWrite(func(db *gorm.DB) error {
				return rollupRecent(db, time.Now())
			})
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
			now := time.Now()

			// Only prune raw once the historical backfill has completed.
			pruneRaw := false
			select {
			case <-s.backfillDone:
				pruneRaw = true
			default:
			}

			s.db.EnqueueWrite(func(db *gorm.DB) error {
				return pruneOldData(db, now, pruneRaw)
			})
		}
	}
}

// backfillHistogram is the raw-data chunk size processed per iteration during
// backfill. Chunking bounds memory when rolling up a large historical backlog.
const backfillChunk = 24 * time.Hour

// backfillRollups rolls up all historical raw metrics (older than the current
// minute) into per-minute rollups. Histograms cannot be built in SQL, so raw
// data is read in day-sized chunks and aggregated in Go. Existing rollups are
// left untouched (ON CONFLICT DO NOTHING), so freshly computed incremental
// rollups always take precedence over the backfill.
func backfillRollups(db *gorm.DB, now time.Time) error {
	upper := now.Truncate(time.Minute) // exclude the in-progress minute

	// Find the earliest raw point via the ORM so its timestamp deserialises
	// correctly (a raw MIN() aggregate comes back as a string).
	var earliest store.Metric
	if err := db.Select("timestamp").Order("timestamp ASC").First(&earliest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // no raw data to back-fill
		}

		return err
	}

	for chunkStart := earliest.Timestamp.Truncate(time.Minute); chunkStart.Before(upper); chunkStart = chunkStart.Add(backfillChunk) {
		chunkEnd := chunkStart.Add(backfillChunk)
		if chunkEnd.After(upper) {
			chunkEnd = upper
		}

		if err := rollupWindow(db, chunkStart, chunkEnd, true); err != nil {
			return err
		}
	}

	return nil
}

// rollupRecent recomputes rollups for recently completed minutes from raw data.
// It is idempotent: existing rollups for the same bucket are overwritten with
// the freshly computed values.
func rollupRecent(db *gorm.DB, now time.Time) error {
	upper := now.Truncate(time.Minute) // exclude the in-progress minute
	return rollupWindow(db, upper.Add(-rollupLookback), upper, false)
}

// rollupWindow aggregates raw metrics in [lower, upper) into per-minute rollups
// and upserts them. When keepExisting is true, existing rollups are preserved
// (used by backfill); otherwise they are overwritten (used by incremental
// rollups, which are authoritative for recent minutes).
func rollupWindow(db *gorm.DB, lower, upper time.Time, keepExisting bool) error {
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

	onConflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "metric_type"}, {Name: "bucket_start"}},
		DoUpdates: clause.AssignmentColumns([]string{"count", "sum", "histogram"}),
	}
	if keepExisting {
		onConflict = clause.OnConflict{
			Columns:   []clause.Column{{Name: "metric_type"}, {Name: "bucket_start"}},
			DoNothing: true,
		}
	}

	return db.Clauses(onConflict).Create(&rollups).Error
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

// pruneOldData deletes rollups older than rollupRetention and, when pruneRaw is
// true, raw points older than rawRetention. Raw pruning is gated on the caller
// having confirmed the historical backfill completed, so that raw history is
// never deleted before it has been captured in rollups.
func pruneOldData(db *gorm.DB, now time.Time, pruneRaw bool) error {
	if pruneRaw {
		rawCutoff := now.Add(-rawRetention)
		if result := db.Where("timestamp < ?", rawCutoff).Delete(&store.Metric{}); result.Error != nil {
			logger.Error("Failed to prune old raw metrics", "", "error", result.Error)
			return result.Error
		} else if result.RowsAffected > 0 {
			logger.Info("Pruned old raw metrics", "", "count", result.RowsAffected, "cutoff", rawCutoff)
		}
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
