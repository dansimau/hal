package store

import (
	"time"

	"gorm.io/gorm"
)

const (
	// deleteChunkSize is the number of rows removed per statement by
	// DeleteInChunks. Small chunks keep each write transaction short.
	deleteChunkSize = 1000
	// deleteChunkPause is how long DeleteInChunks sleeps between chunks so other
	// writers (e.g. the async writer) can take the write lock in between.
	deleteChunkPause = 50 * time.Millisecond
)

// DeleteInChunks deletes the rows of model matching where/args in chunks of
// deleteChunkSize rows, each in its own short transaction, pausing between
// chunks. A single large DELETE holds SQLite's write lock for its whole
// duration, blocking every other writer; chunking bounds how long any one
// statement holds it. The table must have an integer "id" primary key. It
// returns the total number of rows deleted, including on error.
func DeleteInChunks(db *gorm.DB, model any, where string, args ...any) (int64, error) {
	var total int64

	for {
		sub := db.Model(model).Select("id").Where(where, args...).Limit(deleteChunkSize)

		result := db.Where("id IN (?)", sub).Delete(model)
		total += result.RowsAffected

		if result.Error != nil {
			return total, result.Error
		}

		if result.RowsAffected < deleteChunkSize {
			return total, nil
		}

		time.Sleep(deleteChunkPause)
	}
}
