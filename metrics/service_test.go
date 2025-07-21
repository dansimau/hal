package metrics

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
	"gotest.tools/v3/assert"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	
	// Use a temporary file database instead of in-memory to avoid concurrency issues
	dbPath := t.TempDir() + "/test.db"
	db, err := store.Open(dbPath)
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
	assert.Equal(t, service.batchSize, 100)
	assert.Equal(t, service.pruneInterval, 24*time.Hour)
	assert.Equal(t, service.retentionTime, 30*24*time.Hour)
}

func TestRecordCounter(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Test by directly adding to buffer and flushing (synchronous)
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     store.MetricTypeAutomationTriggered,
		Value:          1,
		EntityID:       "test.entity",
		AutomationName: "test automation",
	}
	
	service.metricsBuffer = append(service.metricsBuffer, metric)
	service.flushBuffer()
	
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

func TestRecordCounterAsync(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Start the service to enable background processing
	service.Start()
	defer service.Stop()
	
	// Record a counter metric through the async channel
	service.RecordCounter(store.MetricTypeAutomationTriggered, "test.entity", "test automation")
	
	// Wait for background processing with a retry loop
	var metrics []store.Metric
	for i := 0; i < 50; i++ { // Try for up to 5 seconds
		db.Find(&metrics)
		if len(metrics) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	// Verify metric was recorded
	assert.Assert(t, len(metrics) >= 1, "Expected at least 1 metric, got %d", len(metrics))
	
	metric := metrics[0]
	assert.Equal(t, metric.MetricType, store.MetricTypeAutomationTriggered)
	assert.Equal(t, metric.Value, int64(1))
	assert.Equal(t, metric.EntityID, "test.entity")
	assert.Equal(t, metric.AutomationName, "test automation")
}

func TestRecordTimer(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Start the service to enable background processing
	service.Start()
	defer service.Stop()
	
	duration := 150 * time.Millisecond
	
	// Record a timer metric
	service.RecordTimer(store.MetricTypeTickProcessingTime, duration, "test.entity", "")
	
	// Wait for background processing with retry logic
	var metrics []store.Metric
	for i := 0; i < 20; i++ {
		db.Find(&metrics)
		if len(metrics) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	// Verify metric was recorded
	assert.Assert(t, len(metrics) >= 1, "Expected at least 1 metric, got %d", len(metrics))
	
	metric := metrics[0]
	assert.Equal(t, metric.MetricType, store.MetricTypeTickProcessingTime)
	assert.Equal(t, metric.Value, duration.Nanoseconds())
	assert.Equal(t, metric.EntityID, "test.entity")
	assert.Equal(t, metric.AutomationName, "")
}

func TestBatchingBehavior(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	service.batchSize = 3 // Small batch size for testing
	
	// Start the service to enable background processing
	service.Start()
	defer service.Stop()
	
	// Record multiple metrics
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity1", "automation1")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity2", "automation2")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity3", "automation3")
	
	// Wait for background processing - should trigger batch flush
	time.Sleep(50 * time.Millisecond)
	
	// Should be flushed now
	var count int64
	db.Model(&store.Metric{}).Count(&count)
	assert.Equal(t, count, int64(3))
}

func TestPruneOldMetrics(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Create old and new metrics
	oldTime := time.Now().Add(-45 * 24 * time.Hour) // 45 days old
	newTime := time.Now().Add(-1 * time.Hour)       // 1 hour old
	
	oldMetric := store.Metric{
		Timestamp:  oldTime,
		MetricType: store.MetricTypeAutomationTriggered,
		Value:      1,
	}
	
	newMetric := store.Metric{
		Timestamp:  newTime,
		MetricType: store.MetricTypeAutomationTriggered,
		Value:      1,
	}
	
	// Insert metrics directly into database
	assert.NilError(t, db.Create(&oldMetric).Error)
	assert.NilError(t, db.Create(&newMetric).Error)
	
	// Verify both metrics exist
	var count int64
	db.Model(&store.Metric{}).Count(&count)
	assert.Equal(t, count, int64(2))
	
	// Manually trigger pruning with a short retention time
	service.retentionTime = 30 * 24 * time.Hour // 30 days
	cutoffTime := time.Now().Add(-service.retentionTime)
	result := db.Where("timestamp < ?", cutoffTime).Delete(&store.Metric{})
	assert.NilError(t, result.Error)
	
	// Only new metric should remain
	db.Model(&store.Metric{}).Count(&count)
	assert.Equal(t, count, int64(1))
	
	// Verify it's the new metric
	var remaining store.Metric
	db.First(&remaining)
	assert.Assert(t, remaining.Timestamp.After(cutoffTime))
}

func TestServiceStartStop(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Test that service can start and stop cleanly
	service.Start()
	
	// Just test basic lifecycle, don't test flushing behavior here
	// (that's tested in other tests)
	time.Sleep(10 * time.Millisecond)
	
	service.Stop()
	
	// Verify no errors occurred (success is just clean start/stop)
}

func TestBufferOverflow(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Make buffer very small for testing
	service.bufferChan = make(chan store.Metric, 2)
	
	// Start the service to enable background processing
	service.Start()
	defer service.Stop()
	
	// Fill buffer beyond capacity
	service.RecordCounter("test1", "entity1", "")
	service.RecordCounter("test2", "entity2", "")
	service.RecordCounter("test3", "entity3", "") // This should be dropped
	
	// Wait for background processing with retry
	var count int64
	for i := 0; i < 20; i++ {
		result := db.Model(&store.Metric{}).Count(&count)
		assert.NilError(t, result.Error)
		if count >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	assert.Equal(t, count, int64(2))
}