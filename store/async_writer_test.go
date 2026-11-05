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

// testLogLevel is the level used for log rows written by tests.
const testLogLevel = "INFO"

// errTest is returned by write operations that are meant to fail.
var errTest = errors.New("test failure")

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
			Level:     testLogLevel,
			LogText:   "hello",
		}).Error
	})

	db.WaitForWrites()

	var count int64

	err = db.Model(&store.Log{}).Count(&count).Error
	if err != nil {
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
			Level:     testLogLevel,
			LogText:   "flushed",
		}).Error
	})

	// Close should drain the queue before closing the DB.
	err = db.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer reopened.Close()

	var count int64

	err = reopened.Model(&store.Log{}).Count(&count).Error
	if err != nil {
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

	// Commit as soon as the writer is free, so tests that stall the writer
	// (stallWriter) do not wait out the flush interval.
	db.SetFlushInterval(0)

	return db
}

// countLogs returns the number of committed log rows.
func countLogs(t *testing.T, db *store.Store) int64 {
	t.Helper()

	var count int64

	assert.NilError(t, db.Model(&store.Log{}).Count(&count).Error)

	return count
}

// waitForLogs polls until db holds at least want committed log rows.
func waitForLogs(t *testing.T, db *store.Store, want int64) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for countLogs(t, db) < want {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d logs, got %d", want, countLogs(t, db))
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func enqueueLog(db *store.Store, text string) {
	db.EnqueueWrite(func(gdb *gorm.DB) error {
		return gdb.Create(&store.Log{Timestamp: time.Now(), Level: testLogLevel, LogText: text}).Error
	})
}

// TestWritesAreHeldUntilFlushInterval verifies that writes arriving within the
// flush interval are held back and then committed together, and that entity
// states keep coalescing while the batch waits.
func TestWritesAreHeldUntilFlushInterval(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	db.SetFlushInterval(500 * time.Millisecond)

	enqueueLog(db, "first")
	time.Sleep(100 * time.Millisecond)
	enqueueLog(db, "second")

	for _, state := range []string{"on", "unavailable"} {
		db.EnqueueEntityState("light.kitchen", &homeassistant.State{EntityID: "light.kitchen", State: state})
	}

	assert.Equal(t, countLogs(t, db), int64(0), "writes committed before the flush interval")

	waitForLogs(t, db, 2)

	var entity store.Entity

	assert.NilError(t, db.First(&entity, "id = ?", "light.kitchen").Error)
	assert.Equal(t, entity.State.State, "unavailable")
}

// TestWaitForWritesDoesNotWaitForFlushInterval verifies that a caller waiting on
// its writes gets them committed immediately.
func TestWaitForWritesDoesNotWaitForFlushInterval(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	db.SetFlushInterval(time.Hour)

	enqueueLog(db, "waited")

	start := time.Now()

	db.WaitForWrites()
	assert.Assert(t, time.Since(start) < 5*time.Second, "WaitForWrites waited for the flush interval")
	assert.Equal(t, countLogs(t, db), int64(1))
}

// TestFullBatchIsCommittedBeforeFlushInterval verifies that a batch is committed
// as soon as it is full, and the remainder waits for the next interval.
func TestFullBatchIsCommittedBeforeFlushInterval(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	db.SetFlushInterval(time.Hour)

	const maxBatch = 500
	for i := range maxBatch + 100 {
		enqueueLog(db, fmt.Sprintf("log %d", i))
	}

	waitForLogs(t, db, maxBatch)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, countLogs(t, db), int64(maxBatch), "remainder committed before the flush interval")

	db.WaitForWrites()
	assert.Equal(t, countLogs(t, db), int64(maxBatch+100))
}

// TestCloseDoesNotWaitForFlushInterval verifies that shutting down commits held
// writes immediately rather than waiting out the flush interval.
func TestCloseDoesNotWaitForFlushInterval(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/close.db"

	db, err := store.Open(path)
	assert.NilError(t, err)
	db.SetFlushInterval(time.Hour)

	enqueueLog(db, "held")
	db.EnqueueEntityState("light.hall", &homeassistant.State{EntityID: "light.hall", State: "on"})

	start := time.Now()

	assert.NilError(t, db.Close())
	assert.Assert(t, time.Since(start) < 5*time.Second, "Close waited for the flush interval")

	reopened, err := store.Open(path)
	assert.NilError(t, err)

	defer reopened.Close()

	assert.Equal(t, countLogs(t, reopened), int64(1))

	var entity store.Entity

	assert.NilError(t, reopened.First(&entity, "id = ?", "light.hall").Error)
	assert.Equal(t, entity.State.State, "on")
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
		for range 20000 {
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
	for i := range n {
		db.EnqueueWrite(func(gdb *gorm.DB) error {
			return gdb.Create(&store.Log{Timestamp: time.Now(), Level: testLogLevel, LogText: fmt.Sprintf("log %d", i)}).Error
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
		return gdb.Create(&store.Log{Timestamp: time.Now(), Level: testLogLevel, LogText: "first"}).Error
	})
	db.EnqueueWrite(func(*gorm.DB) error { return errTest })
	db.EnqueueWrite(func(gdb *gorm.DB) error {
		return gdb.Create(&store.Log{Timestamp: time.Now(), Level: testLogLevel, LogText: "second"}).Error
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

	for range 15000 {
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
