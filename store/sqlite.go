package store

import (
	"fmt"
	"strings"

	"github.com/dansimau/hal/homeassistant"

	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"
)

// Store wraps the database connection and async writer.
type Store struct {
	*gorm.DB
	asyncWriter *AsyncWriter
}

func Open(path string) (*Store, error) {
	// busy_timeout lets concurrent writers (the async writer, the rollup and
	// prune loops, and separate CLI processes) wait for the lock instead of
	// failing with SQLITE_BUSY. Pragmas are set in the DSN so they apply to every
	// connection in the pool, not just the one that happens to run an Exec.
	//
	// auto_vacuum is INCREMENTAL rather than FULL: FULL relocates pages on every
	// commit that frees them, which made large prunes hold the write lock for
	// minutes on slow disks. Free pages are reclaimed in bounded steps by
	// IncrementalVacuum instead. (Switching between FULL and INCREMENTAL is
	// allowed on an existing database; switching to or from NONE is not.)
	//
	// _txlock=immediate makes transactions take the write lock at BEGIN, so a
	// transaction waits for the lock once up front instead of failing midway
	// (and, in WAL mode, avoids SQLITE_BUSY_SNAPSHOT on read-to-write upgrades).
	dsn := path + "?_pragma=auto_vacuum(INCREMENTAL)&_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// In-memory SQLite databases are scoped to a single connection: every new
	// connection in the pool gets its own empty database, so a migration run on
	// one connection is invisible to a query that lands on another ("no such
	// table"). Pin the pool to a single connection so the whole Store shares one
	// in-memory database. File-backed databases keep the default pool.
	if strings.Contains(path, ":memory:") {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}

		sqlDB.SetMaxOpenConns(1)
	}

	if err := db.AutoMigrate(&Entity{}, &Metric{}, &MetricRollup{}, &Log{}); err != nil {
		return nil, err
	}

	// Create and start async writer
	asyncWriter := NewAsyncWriter(db)
	asyncWriter.Start()

	return &Store{
		DB:          db,
		asyncWriter: asyncWriter,
	}, nil
}

// IncrementalVacuum returns up to pages free pages to the filesystem. Bounding
// the page count keeps the write transaction short.
//
// The pragma frees one page per step of the statement, so it must be read to
// completion: Exec only takes the first step and would free a single page.
func IncrementalVacuum(db *gorm.DB, pages int) error {
	rows, err := db.Raw(fmt.Sprintf("PRAGMA incremental_vacuum(%d)", pages)).Rows()
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
	}

	return rows.Err()
}

// EnqueueWrite adds a write operation to the async queue. It never blocks; the
// write is dropped if the queue is full.
func (s *Store) EnqueueWrite(op WriteOperation) {
	s.asyncWriter.Enqueue(op)
}

// EnqueueEntityState asynchronously persists the latest state of an entity.
func (s *Store) EnqueueEntityState(id string, state *homeassistant.State) {
	s.asyncWriter.EnqueueEntityState(id, state)
}

// DroppedWrites returns the number of writes dropped because the queue was full.
func (s *Store) DroppedWrites() int64 {
	return s.asyncWriter.Dropped()
}

// WaitForWrites blocks until all queued writes complete (for testing).
func (s *Store) WaitForWrites() {
	s.asyncWriter.WaitForWrites()
}

// Close gracefully shuts down the store, flushing pending writes.
func (s *Store) Close() error {
	s.asyncWriter.Shutdown()
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
