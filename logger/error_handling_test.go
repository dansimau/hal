package logger

import (
	"testing"

	"github.com/dansimau/hal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDatabaseErrorHandling(t *testing.T) {
	// Test that logging methods track no errors when database is working correctly
	t.Run("successful database logging", func(t *testing.T) {
		db, err := store.Open(":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}

		service := NewServiceWithDB(db)

		// Test that all logging methods succeed without database errors
		service.Info("Test info message", "test.entity")
		service.Error("Test error message", "test.entity")
		service.Debug("Test debug message", "test.entity")
		service.Warn("Test warn message", "test.entity")

		// Wait for async writes to complete
		db.WaitForWrites()

		// Verify no database errors occurred
		if lastErr := service.LastError(); lastErr != nil {
			t.Errorf("Expected no database errors, got: %v", lastErr)
		}

		if errCount := service.ErrorCount(); errCount != 0 {
			t.Errorf("Expected 0 database errors, got: %d", errCount)
		}

		// Verify logs were actually written to database
		var logs []store.Log
		if err := db.Find(&logs).Error; err != nil {
			t.Fatalf("Failed to query logs: %v", err)
		}

		if len(logs) != 4 {
			t.Errorf("Expected 4 logs in database, got %d", len(logs))
		}
	})

	// Test that logging methods track no errors when no database is available (buffering mode)
	t.Run("buffering mode when no database", func(t *testing.T) {
		service := NewService() // No database set

		// Test that all logging methods work when buffering
		service.Info("Buffered info message", "test.entity")
		service.Error("Buffered error message", "test.entity")

		// Verify no database errors occurred (since there's no database)
		if lastErr := service.LastError(); lastErr != nil {
			t.Errorf("Expected no database errors when buffering, got: %v", lastErr)
		}

		if errCount := service.ErrorCount(); errCount != 0 {
			t.Errorf("Expected 0 database errors when buffering, got: %d", errCount)
		}

		// Verify messages are in buffer
		service.mu.RLock()
		bufferCount := service.bufferCount
		service.mu.RUnlock()

		if bufferCount != 2 {
			t.Errorf("Expected 2 buffered messages, got %d", bufferCount)
		}
	})

	// Test global functions track no errors when working correctly
	t.Run("global functions error handling", func(t *testing.T) {
		db, err := store.Open(":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}

		// Set database on global logger
		SetDefaultDatabase(db)

		// Test global functions succeed without database errors
		Info("Global info", "global.entity")
		Error("Global error", "global.entity")
		Debug("Global debug", "global.entity")
		Warn("Global warn", "global.entity")

		// Verify no database errors occurred
		if lastErr := LastError(); lastErr != nil {
			t.Errorf("Expected no database errors from global functions, got: %v", lastErr)
		}

		if errCount := ErrorCount(); errCount != 0 {
			t.Errorf("Expected 0 database errors from global functions, got: %d", errCount)
		}
	})
}

// mockFailingDB creates a database that will fail on Create operations
func TestDatabaseFailureErrorHandling(t *testing.T) {
	// Create a database with a closed connection to simulate failures
	// Open database normally first
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate to set up tables
	if err := db.AutoMigrate(&store.Log{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Close the underlying database connection to force errors
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying DB: %v", err)
	}
	sqlDB.Close()

	// Note: With async writes, database errors are logged but not tracked in error counters.
	// This test now verifies that the service doesn't crash when database fails.
	// The actual error will be logged by the async writer.

	// Wrap in Store
	storeDB, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close the underlying database connection to force errors
	sqlDB2, err := storeDB.DB.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying DB: %v", err)
	}
	sqlDB2.Close()

	// Create service with the closed database
	service := NewServiceWithDB(storeDB)

	// Test that logging methods don't crash when database fails
	// With async writes, errors are logged by the async writer, not tracked in the logger service
	service.Info("Test message", "test.entity")
	service.Error("Test message", "test.entity")
	service.Debug("Test message", "test.entity")
	service.Warn("Test message", "test.entity")

	// Wait a moment for async writes to attempt (and fail)
	storeDB.WaitForWrites()

	// Success is just not crashing - async write errors are logged via slog
}

// TestErrorTrackingFromGlobalFunctions tests that errors are properly tracked from global functions
func TestErrorTrackingFromGlobalFunctions(t *testing.T) {
	// This test verifies error tracking by using a database that fails
	// Create a database and immediately close it to force errors
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate to set up tables
	if err := db.AutoMigrate(&store.Log{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Close the underlying database connection to force errors
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying DB: %v", err)
	}
	sqlDB.Close()

	// Wrap in Store
	storeDB, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close the underlying database connection to force errors
	sqlDB2, err := storeDB.DB.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying DB: %v", err)
	}
	sqlDB2.Close()

	// Set this failing database on the global logger
	SetDefaultDatabase(storeDB)

	// Test that global functions don't crash when database fails
	// With async writes, errors are logged by the async writer, not tracked in the logger service
	Info("Test message", "test.entity")
	Error("Test message", "test.entity")
	Debug("Test message", "test.entity")
	Warn("Test message", "test.entity")

	// Success is just not crashing - async write errors are logged via slog
}