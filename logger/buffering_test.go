package logger

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
)

func TestBuffering(t *testing.T) {
	// Create logging service without database
	service := NewService()

	// Test logging without database (should buffer)
	service.Info("Buffered message 1", "entity.test")
	service.Error("Buffered message 2", "")
	service.Debug("Buffered message 3", "entity.another")

	// Verify messages are buffered
	service.mu.RLock()
	if service.bufferCount != 3 {
		t.Errorf("Expected 3 buffered messages, got %d", service.bufferCount)
	}
	service.mu.RUnlock()

	// Create database and set it
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Set database (should flush buffer)
	service.SetDatabase(db)

	// Verify buffer is empty
	service.mu.RLock()
	bufferCount := service.bufferCount
	service.mu.RUnlock()

	if bufferCount != 0 {
		t.Errorf("Expected buffer to be empty after setting database, got %d items", bufferCount)
	}

	// Verify logs were written to database
	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("Expected 3 logs in database, got %d", len(logs))
	}

	// Verify log contents
	expectedMessages := []string{"Buffered message 1", "Buffered message 2", "Buffered message 3"}
	expectedEntityIDs := []string{"entity.test", "", "entity.another"}

	for i, log := range logs {
		if log.LogText != expectedMessages[i] {
			t.Errorf("Expected log message '%s', got '%s'", expectedMessages[i], log.LogText)
		}
		if log.EntityID != expectedEntityIDs[i] {
			t.Errorf("Expected entity ID '%s', got '%s'", expectedEntityIDs[i], log.EntityID)
		}
	}
}

func TestCircularBuffer(t *testing.T) {
	// Create service with small buffer for testing
	service := NewService()
	service.bufferSize = 3
	service.buffer = make([]BufferedLog, 3)

	// Fill buffer beyond capacity
	for i := 0; i < 5; i++ {
		service.Info("Message %d", "", "index", i)
	}

	// Check that buffer contains only last 3 messages
	service.mu.RLock()
	if service.bufferCount != 3 {
		t.Errorf("Expected buffer count to be 3, got %d", service.bufferCount)
	}

	// Check that the oldest messages were overwritten
	// Buffer should contain messages 2, 3, 4
	for i := 0; i < 3; i++ {
		idx := (service.bufferHead - service.bufferCount + i + service.bufferSize) % service.bufferSize
		bufferedLog := service.buffer[idx]
		expectedText := "Message %d"
		if bufferedLog.LogText != expectedText {
			t.Errorf("Expected buffered message '%s', got '%s'", expectedText, bufferedLog.LogText)
		}
	}
	service.mu.RUnlock()
}

func TestGlobalFunctions(t *testing.T) {
	// Create temporary database
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Set database on global logger
	GetDefaultLogger().SetDatabase(db)

	// Test global functions
	Info("Global info message", "global.entity")
	Error("Global error message", "")
	Debug("Global debug message", "global.debug")
	Warn("Global warn message", "global.warn")

	// Give a moment for database writes
	time.Sleep(10 * time.Millisecond)

	// Verify logs were written
	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 4 {
		t.Errorf("Expected 4 logs from global functions, got %d", len(logs))
	}

	expectedMessages := []string{"Global info message", "Global error message", "Global debug message", "Global warn message"}
	expectedEntityIDs := []string{"global.entity", "", "global.debug", "global.warn"}

	for i, log := range logs {
		if log.LogText != expectedMessages[i] {
			t.Errorf("Expected log message '%s', got '%s'", expectedMessages[i], log.LogText)
		}
		if log.EntityID != expectedEntityIDs[i] {
			t.Errorf("Expected entity ID '%s', got '%s'", expectedEntityIDs[i], log.EntityID)
		}
	}
}
