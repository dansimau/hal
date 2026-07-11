package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

func setupTestDB(t *testing.T) *store.Store {
	t.Helper()

	// Use in-memory database for testing
	db, err := store.Open(":memory:")
	assert.NilError(t, err)

	// Ensure the Metric table is created (store.Open should handle this via AutoMigrate)
	// But let's be explicit for tests
	err = db.AutoMigrate(&store.Metric{})
	assert.NilError(t, err)

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	assert.Assert(t, service != nil)
}

func TestRecordCounter(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	// Record a counter metric (writes asynchronously to database)
	service.RecordCounter(store.MetricTypeAutomationTriggered, "test.entity", "test automation")

	// Wait for async write to complete
	db.WaitForWrites()

	// Verify metric was recorded
	var metrics []store.Metric
	result := db.Find(&metrics)
	assert.NilError(t, result.Error)
	assert.Equal(t, len(metrics), 1)

	savedMetric := metrics[0]
	assert.Equal(t, savedMetric.MetricType, store.MetricTypeAutomationTriggered)
	assert.Equal(t, savedMetric.Value, int64(1))
	assert.Equal(t, savedMetric.EntityID, "test.entity")
	assert.Equal(t, savedMetric.AutomationName, "test automation")
	assert.Assert(t, !savedMetric.Timestamp.IsZero())
}

func TestRecordTimer(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	duration := 150 * time.Millisecond

	// Record a timer metric (writes asynchronously to database)
	service.RecordTimer(store.MetricTypeTickProcessingTime, duration, "test.entity", "")

	// Wait for async write to complete
	db.WaitForWrites()

	// Verify metric was recorded
	var metrics []store.Metric
	result := db.Find(&metrics)
	assert.NilError(t, result.Error)
	assert.Equal(t, len(metrics), 1)

	metric := metrics[0]
	assert.Equal(t, metric.MetricType, store.MetricTypeTickProcessingTime)
	assert.Equal(t, metric.Value, duration.Nanoseconds())
	assert.Equal(t, metric.EntityID, "test.entity")
	assert.Equal(t, metric.AutomationName, "")
}

func TestMultipleMetrics(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	// Record multiple metrics (writes asynchronously to database)
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity1", "automation1")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity2", "automation2")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity3", "automation3")

	// Wait for async writes to complete
	db.WaitForWrites()

	// Verify all metrics were recorded
	var count int64
	db.Model(&store.Metric{}).Count(&count)
	assert.Equal(t, count, int64(3))
}

func TestPruneOldData(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()

	// Raw metrics: one older than rawRetention (24h), one within it.
	assert.NilError(t, db.Create(&store.Metric{
		Timestamp:  now.Add(-48 * time.Hour),
		MetricType: store.MetricTypeAutomationTriggered,
		Value:      1,
	}).Error)
	assert.NilError(t, db.Create(&store.Metric{
		Timestamp:  now.Add(-1 * time.Hour),
		MetricType: store.MetricTypeAutomationTriggered,
		Value:      1,
	}).Error)

	// Rollups: one older than rollupRetention (90d), one within it.
	assert.NilError(t, db.Create(&store.MetricRollup{
		MetricType:  store.MetricTypeAutomationTriggered,
		BucketStart: now.Add(-100 * 24 * time.Hour).Truncate(time.Minute).Unix(),
		Count:       1, Sum: 1, Histogram: map[int32]int64{0: 1},
	}).Error)
	assert.NilError(t, db.Create(&store.MetricRollup{
		MetricType:  store.MetricTypeAutomationTriggered,
		BucketStart: now.Add(-1 * time.Hour).Truncate(time.Minute).Unix(),
		Count:       1, Sum: 1, Histogram: map[int32]int64{0: 1},
	}).Error)

	assert.NilError(t, pruneOldData(db.DB, now, true))

	var rawCount, rollupCount int64
	db.Model(&store.Metric{}).Count(&rawCount)
	db.Model(&store.MetricRollup{}).Count(&rollupCount)

	// Only the recent raw point and recent rollup should remain.
	assert.Equal(t, rawCount, int64(1))
	assert.Equal(t, rollupCount, int64(1))
}

func TestPruneOldDataSkipsRawUntilBackfilled(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	assert.NilError(t, db.Create(&store.Metric{
		Timestamp:  now.Add(-48 * time.Hour),
		MetricType: store.MetricTypeAutomationTriggered,
		Value:      1,
	}).Error)

	// With pruneRaw=false, old raw points must be retained.
	assert.NilError(t, pruneOldData(db.DB, now, false))

	var rawCount int64
	db.Model(&store.Metric{}).Count(&rawCount)
	assert.Equal(t, rawCount, int64(1))
}

func TestAggregate(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 30, 0, 0, time.UTC)

	var raw []store.Metric
	// 100 samples in one minute with values 1..100 (ns). True p99 (nearest-rank,
	// ceil(100*0.99)=99, index 98) is 99.
	for i := int64(1); i <= 100; i++ {
		raw = append(raw, store.Metric{
			Timestamp:  base.Add(time.Duration(i) * time.Millisecond),
			MetricType: store.MetricTypeTickProcessingTime,
			Value:      i,
		})
	}

	rollups := aggregate(raw)
	assert.Equal(t, len(rollups), 1)

	r := rollups[0]
	assert.Equal(t, r.BucketStart, base.Unix())
	assert.Equal(t, r.Count, int64(100))
	assert.Equal(t, r.Sum, int64(5050))

	// The histogram-derived p99 should be within the bucketing's relative accuracy
	// of the true value (99).
	p99 := store.HistogramQuantile(r.Histogram, 0.99)
	assert.Assert(t, math.Abs(float64(p99)-99)/99 <= 0.05, "p99 = %d, want ~99", p99)
}

func TestBackfillRollups(t *testing.T) {
	db := setupTestDB(t)

	base := time.Date(2026, 7, 11, 12, 30, 0, 0, time.UTC)

	// Two samples in minute 12:30, one in 12:31.
	for _, ts := range []time.Time{base, base.Add(30 * time.Second), base.Add(90 * time.Second)} {
		assert.NilError(t, db.Create(&store.Metric{
			Timestamp:  ts,
			MetricType: store.MetricTypeAutomationTriggered,
			Value:      1,
		}).Error)
	}

	// Backfill as if "now" is well after the samples so both minutes are complete.
	assert.NilError(t, backfillRollups(db.DB, base.Add(10*time.Minute)))

	var rollups []store.MetricRollup
	assert.NilError(t, db.Order("bucket_start ASC").Find(&rollups).Error)
	assert.Equal(t, len(rollups), 2)
	assert.Equal(t, rollups[0].BucketStart, base.Unix())
	assert.Equal(t, rollups[0].Count, int64(2))
	assert.Equal(t, rollups[1].BucketStart, base.Add(time.Minute).Unix())
	assert.Equal(t, rollups[1].Count, int64(1))

	// Backfill must not overwrite existing rollups (ON CONFLICT DO NOTHING).
	assert.NilError(t, backfillRollups(db.DB, base.Add(10*time.Minute)))
	var count int64
	db.Model(&store.MetricRollup{}).Count(&count)
	assert.Equal(t, count, int64(2))
}

func TestRollupRecent(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now().Truncate(time.Minute)
	// Put samples in the previous, now-completed minute.
	bucket := now.Add(-time.Minute)
	for i := int64(1); i <= 10; i++ {
		assert.NilError(t, db.Create(&store.Metric{
			Timestamp:  bucket.Add(time.Duration(i) * time.Second),
			MetricType: store.MetricTypeTickProcessingTime,
			Value:      i * 1000,
		}).Error)
	}

	assert.NilError(t, rollupRecent(db.DB, now))

	var r store.MetricRollup
	assert.NilError(t, db.Where("bucket_start = ?", bucket.Unix()).First(&r).Error)
	assert.Equal(t, r.Count, int64(10))
	assert.Equal(t, r.Sum, int64(55000))

	// Histogram must survive the round-trip through the JSON serializer.
	var histTotal int64
	for _, c := range r.Histogram {
		histTotal += c
	}
	assert.Equal(t, histTotal, int64(10))
}

func TestServiceStartStop(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	// Test that service can start and stop cleanly
	service.Start()

	// Brief pause to let pruning goroutine start
	time.Sleep(10 * time.Millisecond)

	service.Stop()

	// Verify no errors occurred (success is just clean start/stop)
}
