package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
	"gotest.tools/v3/assert"
)

// gormDB returns a bare *gorm.DB (via a Store) usable for constructing a
// standalone AsyncWriter in tests.
func gormDB(t *testing.T) *gorm.DB {
	t.Helper()

	s, err := store.Open(":memory:")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	return s.DB
}

func TestAsyncWriterEnqueueBeforeStartRunsSynchronously(t *testing.T) {
	t.Parallel()

	aw := store.NewAsyncWriter(gormDB(t))

	// Not started: Enqueue executes the operation inline.
	var ran bool
	aw.Enqueue(func(*gorm.DB) error {
		ran = true
		return nil
	})
	assert.Assert(t, ran)

	// An error from a synchronous op is swallowed (logged), not propagated.
	aw.Enqueue(func(*gorm.DB) error {
		return errors.New("sync failure")
	})
}

func TestAsyncWriterDoubleStartIsNoOp(t *testing.T) {
	t.Parallel()

	aw := store.NewAsyncWriter(gormDB(t))
	aw.Start()
	// Second Start returns early without spawning another goroutine.
	aw.Start()
	aw.Shutdown()
}

func TestAsyncWriterProcessesAndLogsErrors(t *testing.T) {
	t.Parallel()

	aw := store.NewAsyncWriter(gormDB(t))
	aw.Start()

	var mu sync.Mutex
	ran := 0
	for range 3 {
		aw.Enqueue(func(*gorm.DB) error {
			mu.Lock()
			ran++
			mu.Unlock()
			// Returning an error exercises the async error-logging branch.
			return errors.New("async failure")
		})
	}

	aw.WaitForWrites()

	mu.Lock()
	assert.Equal(t, ran, 3)
	mu.Unlock()

	aw.Shutdown()
}

func TestAsyncWriterEnqueueAfterShutdownRunsSynchronously(t *testing.T) {
	t.Parallel()

	aw := store.NewAsyncWriter(gormDB(t))
	aw.Start()
	aw.Shutdown()

	// After shutdown the context is cancelled; Enqueue falls back to running the
	// operation synchronously as a last resort.
	var ran bool
	aw.Enqueue(func(*gorm.DB) error {
		ran = true
		return nil
	})
	assert.Assert(t, ran)
}
