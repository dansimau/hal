package store

import (
	"context"
	"log/slog"
	"sync"

	"gorm.io/gorm"
)

// WriteOperation represents a database write operation to be executed asynchronously.
type WriteOperation func(*gorm.DB) error

// AsyncWriter manages asynchronous database write operations.
type AsyncWriter struct {
	db      *gorm.DB
	queue   chan WriteOperation
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	mu      sync.Mutex
}

// NewAsyncWriter creates a new AsyncWriter instance.
func NewAsyncWriter(db *gorm.DB) *AsyncWriter {
	ctx, cancel := context.WithCancel(context.Background())
	return &AsyncWriter{
		db:     db,
		queue:  make(chan WriteOperation, 1000), // Buffered for burst writes
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins processing write operations in a dedicated goroutine.
func (aw *AsyncWriter) Start() {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if aw.started {
		return
	}

	aw.started = true
	aw.wg.Add(1)

	go func() {
		defer aw.wg.Done()
		aw.processWrites()
	}()
}

// processWrites consumes write operations from the queue.
func (aw *AsyncWriter) processWrites() {
	for {
		select {
		case <-aw.ctx.Done():
			// Drain remaining writes before shutdown
			aw.drainQueue()
			return
		case op := <-aw.queue:
			if err := op(aw.db); err != nil {
				slog.Error("Async database write failed", "error", err)
			}
		}
	}
}

// drainQueue processes all remaining writes in the queue during shutdown.
func (aw *AsyncWriter) drainQueue() {
	for {
		select {
		case op := <-aw.queue:
			if err := op(aw.db); err != nil {
				slog.Error("Async database write failed during shutdown", "error", err)
			}
		default:
			return
		}
	}
}

// Enqueue adds a write operation to the queue for asynchronous execution.
func (aw *AsyncWriter) Enqueue(op WriteOperation) {
	aw.mu.Lock()
	if !aw.started {
		aw.mu.Unlock()
		// Execute synchronously if not started (shouldn't happen in normal operation)
		if err := op(aw.db); err != nil {
			slog.Error("Synchronous database write failed", "error", err)
		}
		return
	}
	aw.mu.Unlock()

	select {
	case aw.queue <- op:
		// Write queued successfully
	case <-aw.ctx.Done():
		// Writer is shutting down, execute synchronously as last resort
		if err := op(aw.db); err != nil {
			slog.Error("Database write failed (writer shutdown)", "error", err)
		}
	}
}

// Shutdown gracefully stops the async writer and flushes pending writes.
func (aw *AsyncWriter) Shutdown() {
	aw.cancel()
	aw.wg.Wait()
}

// WaitForWrites blocks until all currently queued writes are processed.
// This is primarily useful for testing.
func (aw *AsyncWriter) WaitForWrites() {
	done := make(chan struct{})
	aw.Enqueue(func(db *gorm.DB) error {
		close(done)
		return nil
	})
	<-done
}
