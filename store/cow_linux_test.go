package store

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

// TestPrepareDatabaseFileCreatesUsableDatabase verifies that pre-creating the
// database file (to set no-copy-on-write) leaves an empty file that SQLite
// opens as a new database, and that an existing database is left untouched.
func TestPrepareDatabaseFileCreatesUsableDatabase(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/new.db"
	prepareDatabaseFile(path)

	info, err := os.Stat(path)
	assert.NilError(t, err)
	assert.Equal(t, info.Size(), int64(0))

	db, err := Open(path)
	assert.NilError(t, err)
	assert.NilError(t, db.Create(&Log{LogText: "x", Level: "INFO"}).Error)
	assert.NilError(t, db.Close())

	before, err := os.ReadFile(path)
	assert.NilError(t, err)
	prepareDatabaseFile(path)
	after, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.DeepEqual(t, before, after)

	_, ok := readCOWState(path, t.TempDir())
	assert.Assert(t, ok)
}
