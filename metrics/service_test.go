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
	assert.Equal(t, service.pruneInterval, 24*time.Hour)
	assert.Equal(t, service.retentionTime, 90*24*time.Hour) // 3 months
}

func TestRecordCounter(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Record a counter metric (writes directly to database)
	service.RecordCounter(store.MetricTypeAutomationTriggered, "test.entity", "test automation")
	
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
	
	// Record a timer metric (writes directly to database)
	service.RecordTimer(store.MetricTypeTickProcessingTime, duration, "test.entity", "")
	
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
	
	// Record multiple metrics (writes directly to database)
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity1", "automation1")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity2", "automation2")
	service.RecordCounter(store.MetricTypeAutomationTriggered, "entity3", "automation3")
	
	// Verify all metrics were recorded
	var count int64
	db.Model(&store.Metric{}).Count(&count)
	assert.Equal(t, count, int64(3))
}

func TestPruneOldMetrics(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	
	// Create old and new metrics (100 days old vs 1 hour old)
	oldTime := time.Now().Add(-100 * 24 * time.Hour) // 100 days old (older than 90 day retention)
	newTime := time.Now().Add(-1 * time.Hour)        // 1 hour old
	
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
	
	// Manually trigger pruning with the default retention time (90 days)
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
	
	// Brief pause to let pruning goroutine start
	time.Sleep(10 * time.Millisecond)
	
	service.Stop()
	
	// Verify no errors occurred (success is just clean start/stop)
}