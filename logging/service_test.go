package logging

import (
	"os"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
)

func TestLoggingService(t *testing.T) {
	// Create temporary database
	tmpDB := "test_logs.db"
	defer os.Remove(tmpDB)

	// Open database
	db, err := store.Open(tmpDB)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create logging service
	service := NewService(db)

	// Test logging with entity ID
	entityID := "light.kitchen"
	service.Info("Light turned on", &entityID)
	service.Error("Failed to turn off light", &entityID)
	
	// Test logging without entity ID
	service.Info("System started", nil)
	service.Debug("Debug message", nil)

	// Verify logs were written to database
	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	// Check first log with entity ID
	if logs[0].LogText != "Light turned on" {
		t.Errorf("Expected log text 'Light turned on', got '%s'", logs[0].LogText)
	}
	if logs[0].EntityID == nil || *logs[0].EntityID != "light.kitchen" {
		t.Errorf("Expected entity ID 'light.kitchen', got %v", logs[0].EntityID)
	}

	// Check log without entity ID
	if logs[2].LogText != "System started" {
		t.Errorf("Expected log text 'System started', got '%s'", logs[2].LogText)
	}
	if logs[2].EntityID != nil {
		t.Errorf("Expected entity ID to be nil, got %v", logs[2].EntityID)
	}
}

func TestLogPruning(t *testing.T) {
	// Create temporary database
	tmpDB := "test_prune_logs.db"
	defer os.Remove(tmpDB)

	// Open database
	db, err := store.Open(tmpDB)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create logging service with short retention for testing
	service := &Service{
		db:            db,
		retentionTime: 1 * time.Second, // Very short retention for testing
	}

	// Add some logs
	service.Info("Old log", nil)
	
	// Wait for retention period to pass
	time.Sleep(2 * time.Second)
	
	// Add a new log
	service.Info("New log", nil)

	// Manually trigger pruning
	cutoffTime := time.Now().Add(-service.retentionTime)
	result := db.Where("timestamp < ?", cutoffTime).Delete(&store.Log{})
	if result.Error != nil {
		t.Fatalf("Failed to prune logs: %v", result.Error)
	}

	// Verify that only the new log remains
	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log after pruning, got %d", len(logs))
	}

	if logs[0].LogText != "New log" {
		t.Errorf("Expected remaining log to be 'New log', got '%s'", logs[0].LogText)
	}
}