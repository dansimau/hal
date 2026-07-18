package store_test

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
)

// TestStoreEnqueueWrite verifies that a write enqueued through the Store's async
// writer is eventually persisted, and that WaitForWrites blocks until it is.
func TestStoreEnqueueWrite(t *testing.T) {
	t.Parallel()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	db.EnqueueWrite(func(tx *gorm.DB) error {
		return tx.Create(&store.Log{
			Timestamp: time.Now(),
			Level:     "INFO",
			LogText:   "hello",
		}).Error
	})

	db.WaitForWrites()

	var count int64
	if err := db.Model(&store.Log{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 log, got %d", count)
	}
}

// TestStoreCloseFlushesWrites verifies that Close drains queued writes before
// shutting the store down.
func TestStoreCloseFlushesWrites(t *testing.T) {
	t.Parallel()

	// Use a file-backed database so it survives closing the connection, letting
	// us re-open and assert the write was flushed during Close.
	path := t.TempDir() + "/flush.db"

	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	db.EnqueueWrite(func(tx *gorm.DB) error {
		return tx.Create(&store.Log{
			Timestamp: time.Now(),
			Level:     "INFO",
			LogText:   "flushed",
		}).Error
	})

	// Close should drain the queue before closing the DB.
	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer reopened.Close()

	var count int64
	if err := reopened.Model(&store.Log{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 flushed log, got %d", count)
	}
}
