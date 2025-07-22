package store_test

import (
	"os"
	"testing"

	"github.com/dansimau/hal/store"
)

func TestSQLitePragmaConfiguration(t *testing.T) {
	// Create a temporary database file
	tmpFile := "test_sqlite.db"
	defer os.Remove(tmpFile)

	// Test opening database with PRAGMA settings
	db, err := store.Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Verify WAL mode is set
	var journalMode string
	err = db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("Expected journal_mode to be 'wal', got: %s", journalMode)
	}

	// Verify synchronous mode is set
	var syncMode string
	err = db.Raw("PRAGMA synchronous").Scan(&syncMode).Error
	if err != nil {
		t.Fatalf("Failed to query synchronous: %v", err)
	}

	if syncMode != "1" { // NORMAL = 1
		t.Errorf("Expected synchronous to be '1' (NORMAL), got: %s", syncMode)
	}
}
