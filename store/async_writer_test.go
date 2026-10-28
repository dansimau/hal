package store_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
	"gotest.tools/v3/assert"
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

func openTestStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := store.Open(":memory:")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// stallWriter blocks the writer goroutine until the returned function is called.
func stallWriter(t *testing.T, db *store.Store) func() {
	t.Helper()

	started := make(chan struct{})
	release := make(chan struct{})
	db.EnqueueWrite(func(*gorm.DB) error {
		close(started)
		<-release
		return nil
	})
	<-started

	return func() { close(release) }
}

func TestEnqueueDoesNotBlockWhenQueueFull(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	release := stallWriter(t, db)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 20000; i++ {
			db.EnqueueWrite(func(*gorm.DB) error { return nil })
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("EnqueueWrite blocked while the writer was stalled")
	}

	assert.Assert(t, db.DroppedWrites() > 0)

	release()
	db.WaitForWrites()
}

func TestWritesAreBatchedAndCommitted(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)

	const n = 1200
	for i := 0; i < n; i++ {
		db.EnqueueWrite(func(gdb *gorm.DB) error {
			return gdb.Create(&store.Log{Timestamp: time.Now(), Level: "INFO", LogText: fmt.Sprintf("log %d", i)}).Error
		})
	}
	db.WaitForWrites()

	var count int64
	assert.NilError(t, db.Model(&store.Log{}).Count(&count).Error)
	assert.Equal(t, count, int64(n))
	assert.Equal(t, db.DroppedWrites(), int64(0))
}

func TestFailedWriteDoesNotLoseRestOfBatch(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	release := stallWriter(t, db)

	db.EnqueueWrite(func(gdb *gorm.DB) error {
		return gdb.Create(&store.Log{Timestamp: time.Now(), Level: "INFO", LogText: "first"}).Error
	})
	db.EnqueueWrite(func(*gorm.DB) error { return errors.New("boom") })
	db.EnqueueWrite(func(gdb *gorm.DB) error {
		return gdb.Create(&store.Log{Timestamp: time.Now(), Level: "INFO", LogText: "second"}).Error
	})

	release()
	db.WaitForWrites()

	var count int64
	assert.NilError(t, db.Model(&store.Log{}).Count(&count).Error)
	assert.Equal(t, count, int64(2))
}

func TestEntityStatesAreCoalesced(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	release := stallWriter(t, db)

	for _, state := range []string{"on", "off", "on", "off"} {
		db.EnqueueEntityState("light.kitchen", &homeassistant.State{EntityID: "light.kitchen", State: state})
	}
	db.EnqueueEntityState("light.hall", &homeassistant.State{EntityID: "light.hall", State: "on"})

	release()
	db.WaitForWrites()

	var entities []store.Entity
	assert.NilError(t, db.Order("id").Find(&entities).Error)
	assert.Equal(t, len(entities), 2)
	assert.Equal(t, entities[0].ID, "light.hall")
	assert.Equal(t, entities[0].State.State, "on")
	assert.Equal(t, entities[1].ID, "light.kitchen")
	assert.Equal(t, entities[1].State.State, "off")
}

func TestWaitForWritesWithFullQueue(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	release := stallWriter(t, db)

	for i := 0; i < 15000; i++ {
		db.EnqueueWrite(func(*gorm.DB) error { return nil })
	}

	done := make(chan struct{})
	go func() {
		db.WaitForWrites()
		close(done)
	}()

	release()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("WaitForWrites did not complete")
	}
}
