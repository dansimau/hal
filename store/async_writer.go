package store

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// queueSize bounds the number of pending write operations. When the queue is
	// full, new writes are dropped rather than blocking the caller.
	queueSize = 10000
	// maxBatch is the maximum number of operations committed in one transaction.
	maxBatch = 500
	// dropReportInterval rate-limits warnings about dropped writes.
	dropReportInterval = 30 * time.Second
)

// WriteOperation represents a database write operation to be executed asynchronously.
type WriteOperation func(*gorm.DB) error

// queuedOp is a queued write. If done is set, it is closed once the batch
// containing the operation has been committed.
type queuedOp struct {
	op   WriteOperation
	done chan struct{}
}

// AsyncWriter manages asynchronous database write operations. Callers never
// block on disk I/O: operations are queued and committed in batches by a single
// goroutine, and are dropped if the queue is full (e.g. when the disk is slow).
type AsyncWriter struct {
	db      *gorm.DB
	queue   chan queuedOp
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	mu      sync.Mutex

	// Pending entity states, coalesced so only the latest state per entity is
	// written. These are never dropped; memory is bounded by the entity count.
	entityMu      sync.Mutex
	entityPending map[string]*Entity
	entitySignal  chan struct{}

	dropped        atomic.Int64 // total writes dropped
	droppedPending atomic.Int64 // writes dropped since the last warning
	lastDropReport atomic.Int64 // unix nanos of the last warning
}

// NewAsyncWriter creates a new AsyncWriter instance.
func NewAsyncWriter(db *gorm.DB) *AsyncWriter {
	ctx, cancel := context.WithCancel(context.Background())
	return &AsyncWriter{
		db:            db,
		queue:         make(chan queuedOp, queueSize),
		ctx:           ctx,
		cancel:        cancel,
		entityPending: make(map[string]*Entity),
		entitySignal:  make(chan struct{}, 1),
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

// processWrites consumes write operations from the queue, committing everything
// that is already pending in a single transaction.
func (aw *AsyncWriter) processWrites() {
	for {
		select {
		case <-aw.ctx.Done():
			// Drain remaining writes before shutdown
			aw.drainQueue()
			return
		case op := <-aw.queue:
			aw.writeBatch(aw.collectBatch(&op))
		case <-aw.entitySignal:
			aw.writeBatch(aw.collectBatch(nil))
		}
	}
}

// collectBatch gathers first (if non-nil), any already-queued operations up to
// maxBatch, and all pending entity states into a single batch without waiting.
func (aw *AsyncWriter) collectBatch(first *queuedOp) []queuedOp {
	batch := make([]queuedOp, 0, maxBatch)
	if first != nil {
		batch = append(batch, *first)
	}

collect:
	for len(batch) < maxBatch {
		select {
		case op := <-aw.queue:
			batch = append(batch, op)
		default:
			break collect
		}
	}

	aw.entityMu.Lock()
	entities := aw.entityPending
	aw.entityPending = make(map[string]*Entity)
	aw.entityMu.Unlock()

	for _, entity := range entities {
		batch = append(batch, queuedOp{op: upsertEntity(entity)})
	}

	return batch
}

// writeBatch commits a batch in one transaction. If the transaction fails, each
// operation is retried individually so one bad write does not lose the others.
func (aw *AsyncWriter) writeBatch(batch []queuedOp) {
	if len(batch) == 0 {
		return
	}

	defer func() {
		for _, q := range batch {
			if q.done != nil {
				close(q.done)
			}
		}
	}()

	err := aw.db.Transaction(func(tx *gorm.DB) error {
		for _, q := range batch {
			if err := q.op(tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		return
	}

	for _, q := range batch {
		if err := q.op(aw.db); err != nil {
			slog.Error("Async database write failed", "error", err)
		}
	}
}

// drainQueue processes all remaining writes in the queue during shutdown.
func (aw *AsyncWriter) drainQueue() {
	for {
		batch := aw.collectBatch(nil)
		if len(batch) == 0 {
			return
		}
		aw.writeBatch(batch)
	}
}

// Enqueue adds a write operation to the queue for asynchronous execution. It
// never blocks: if the queue is full the operation is dropped.
func (aw *AsyncWriter) Enqueue(op WriteOperation) {
	if aw.runSyncIfNotRunning(op) {
		return
	}

	// If the writer is already shutting down, run synchronously as a last resort.
	// This must be checked before the queue send below: once the context is
	// cancelled the worker has stopped draining the queue, so an op that landed
	// in the (still-writable) buffered queue would never run. A plain two-case
	// select would pick between the ready queue-send and ctx.Done() at random,
	// which both drops writes and can wedge WaitForWrites forever.
	select {
	case <-aw.ctx.Done():
		if err := op(aw.db); err != nil {
			slog.Error("Database write failed (writer shutdown)", "error", err)
		}

		return
	default:
	}

	select {
	case aw.queue <- queuedOp{op: op}:
		// Write queued successfully
	default:
		aw.recordDrop()
	}
}

// runSyncIfNotRunning executes op synchronously if the writer has not been
// started or is shutting down, returning true if it did so.
func (aw *AsyncWriter) runSyncIfNotRunning(op WriteOperation) bool {
	aw.mu.Lock()
	started := aw.started
	aw.mu.Unlock()

	if started && aw.ctx.Err() == nil {
		return false
	}

	// Execute synchronously if not started (shouldn't happen in normal operation)
	// or shutting down
	if err := op(aw.db); err != nil {
		slog.Error("Synchronous database write failed", "error", err)
	}
	return true
}

// EnqueueEntityState records the latest state for an entity to be written
// asynchronously. Repeated updates for the same entity before the next write are
// coalesced, and entity states are never dropped.
func (aw *AsyncWriter) EnqueueEntityState(id string, state *homeassistant.State) {
	entity := &Entity{ID: id, State: state}

	if aw.runSyncIfNotRunning(upsertEntity(entity)) {
		return
	}

	aw.entityMu.Lock()
	aw.entityPending[id] = entity
	aw.entityMu.Unlock()

	select {
	case aw.entitySignal <- struct{}{}:
	default:
		// Writer already signalled
	}
}

// Dropped returns the total number of write operations dropped because the
// queue was full.
func (aw *AsyncWriter) Dropped() int64 {
	return aw.dropped.Load()
}

// recordDrop counts a dropped write and logs a rate-limited warning. It logs via
// slog rather than the hal logger, which would enqueue yet more writes.
func (aw *AsyncWriter) recordDrop() {
	aw.dropped.Add(1)
	aw.droppedPending.Add(1)

	now := time.Now().UnixNano()
	last := aw.lastDropReport.Load()
	if now-last < int64(dropReportInterval) || !aw.lastDropReport.CompareAndSwap(last, now) {
		return
	}

	slog.Warn("Async writer queue full, dropped writes (database too slow)",
		"dropped", aw.droppedPending.Swap(0), "total", aw.dropped.Load())
}

// Shutdown gracefully stops the async writer and flushes pending writes.
func (aw *AsyncWriter) Shutdown() {
	aw.cancel()
	aw.wg.Wait()
}

// WaitForWrites blocks until all currently queued writes have been committed.
// This is primarily useful for testing.
func (aw *AsyncWriter) WaitForWrites() {
	noop := func(*gorm.DB) error { return nil }
	if aw.runSyncIfNotRunning(noop) {
		return
	}

	done := make(chan struct{})
	select {
	case aw.queue <- queuedOp{op: noop, done: done}:
		<-done
	case <-aw.ctx.Done():
		// Writer is shutting down and flushes pending writes itself
	}
}

func upsertEntity(entity *Entity) WriteOperation {
	return func(db *gorm.DB) error {
		return db.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(entity).Error
	}
}
