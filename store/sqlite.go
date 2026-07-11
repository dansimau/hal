package store

import (
	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"
)

// Store wraps the database connection and async writer.
type Store struct {
	*gorm.DB
	asyncWriter *AsyncWriter
}

func Open(path string) (*Store, error) {
	// auto_vacuum must be set before the database is created. busy_timeout lets
	// concurrent writers (the async writer, the rollup loop, and separate CLI
	// processes) wait for the lock instead of failing with SQLITE_BUSY.
	dsn := path + "?_pragma=auto_vacuum(FULL)&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Configure SQLite settings
	if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return nil, err
	}
	if err := db.Exec("PRAGMA synchronous = NORMAL").Error; err != nil {
		return nil, err
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

// EnqueueWrite adds a write operation to the async queue.
func (s *Store) EnqueueWrite(op WriteOperation) {
	s.asyncWriter.Enqueue(op)
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
