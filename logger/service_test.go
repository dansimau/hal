package logger

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
)

func TestLoggingService(t *testing.T) {
	// Create temporary database
	// Open database
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create logging service
	service := NewServiceWithDB(db)

	// Test logging with entity ID
	entityID := "light.kitchen"
	service.Info("Light turned on", entityID)
	service.Error("Failed to turn off light", entityID)
	
	// Test logging without entity ID
	service.Info("System started", "")
	service.Debug("Debug message", "")

	// Verify logs were written to database
	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	// Check first log with entity ID
	if logs[0].LogText != "Light turned on entity_id=light.kitchen" {
		t.Errorf("Expected log text 'Light turned on entity_id=light.kitchen', got '%s'", logs[0].LogText)
	}
	if logs[0].EntityID != "light.kitchen" {
		t.Errorf("Expected entity ID 'light.kitchen', got '%s'", logs[0].EntityID)
	}

	// Check log without entity ID
	if logs[2].LogText != "System started" {
		t.Errorf("Expected log text 'System started', got '%s'", logs[2].LogText)
	}
	if logs[2].EntityID != "" {
		t.Errorf("Expected entity ID to be empty, got '%s'", logs[2].EntityID)
	}
}

func TestLogPruning(t *testing.T) {
	// Create temporary database
	// Open database
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create logging service with short retention for testing
	service := NewServiceWithDB(db)
	service.retentionTime = 1 * time.Second // Very short retention for testing

	// Add some logs
	service.Info("Old log", "")
	
	// Wait for retention period to pass
	time.Sleep(2 * time.Second)
	
	// Add a new log
	service.Info("New log", "")

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