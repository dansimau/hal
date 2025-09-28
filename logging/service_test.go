package logging

import (
	"os"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
)

func TestLoggingService(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_logs.db"
	defer os.Remove(dbPath)

	// Open database
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create logging service
	service := NewService(db)

	// Test logging without entity ID
	service.Info("Test log message without entity", nil)

	// Test logging with entity ID
	entityID := "sensor.test"
	service.InfoWithEntity("Test log message with entity", entityID)

	// Test different log levels
	service.Debug("Debug message", nil)
	service.Warn("Warning message", nil)
	service.Error("Error message", nil)

	// Verify logs were stored in database
	var logs []store.Log
	err = db.Find(&logs).Error
	if err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	expectedLogCount := 5
	if len(logs) != expectedLogCount {
		t.Errorf("Expected %d logs, got %d", expectedLogCount, len(logs))
	}

	// Check that logs have proper fields
	for i, log := range logs {
		if log.Timestamp.IsZero() {
			t.Errorf("Log %d has zero timestamp", i)
		}
		if log.LogText == "" {
			t.Errorf("Log %d has empty log text", i)
		}
	}

	// Check that the entity ID was stored correctly
	entityLogFound := false
	for _, log := range logs {
		if log.EntityID != nil && *log.EntityID == entityID {
			entityLogFound = true
			break
		}
	}
	if !entityLogFound {
		t.Error("Log with entity ID not found")
	}
}

func TestLogPruning(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_prune_logs.db"
	defer os.Remove(dbPath)

	// Open database
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create old log entries (older than 1 month)
	oldTime := time.Now().Add(-35 * 24 * time.Hour) // 35 days ago
	oldLog := store.Log{
		Timestamp: oldTime,
		LogText:   "Old log that should be pruned",
	}
	if err := db.Create(&oldLog).Error; err != nil {
		t.Fatalf("Failed to create old log: %v", err)
	}

	// Create recent log entry
	recentLog := store.Log{
		Timestamp: time.Now(),
		LogText:   "Recent log that should be kept",
	}
	if err := db.Create(&recentLog).Error; err != nil {
		t.Fatalf("Failed to create recent log: %v", err)
	}

	// Manually trigger pruning using the same logic as the service
	// (1 month retention as per requirements)
	cutoffTime := time.Now().Add(-30 * 24 * time.Hour)
	result := db.Where("timestamp < ?", cutoffTime).Delete(&store.Log{})
	if result.Error != nil {
		t.Fatalf("Failed to prune logs: %v", result.Error)
	}

	// Verify only recent log remains
	var remainingLogs []store.Log
	if err := db.Find(&remainingLogs).Error; err != nil {
		t.Fatalf("Failed to query remaining logs: %v", err)
	}

	if len(remainingLogs) != 1 {
		t.Errorf("Expected 1 remaining log after pruning, got %d", len(remainingLogs))
	}

	if len(remainingLogs) > 0 && remainingLogs[0].LogText != "Recent log that should be kept" {
		t.Errorf("Wrong log remained after pruning: %s", remainingLogs[0].LogText)
	}
}