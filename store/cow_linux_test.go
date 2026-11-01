package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestPrepareDatabaseFile verifies that on btrfs a new database file is
// pre-created empty with copy-on-write disabled, and SQLite opens it as a new
// database; elsewhere nothing is created. An existing database is never
// modified.
func TestPrepareDatabaseFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/new.db"
	prepareDatabaseFile(path)

	info, err := os.Stat(path)
	if isBtrfs(dir) {
		assert.NilError(t, err)
		assert.Equal(t, info.Size(), int64(0))

		flags, err := inodeFlags(path)
		assert.NilError(t, err)
		assert.Assert(t, flags&fsNoCOWFlag != 0, "expected no-copy-on-write flag, got %#x", flags)
	} else {
		assert.Assert(t, os.IsNotExist(err), "expected no file to be created off btrfs")
	}

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
}

// posixLocks returns the /proc/locks entries for the file at path.
func posixLocks(t *testing.T, path string) []string {
	t.Helper()

	id, ok := statFileID(path)
	assert.Assert(t, ok)

	data, err := os.ReadFile("/proc/locks")
	assert.NilError(t, err)

	var locks []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, fmt.Sprintf(":%d ", id.ino)) {
			// Drop the leading lock number, which can change between reads
			_, rest, _ := strings.Cut(line, ":")
			locks = append(locks, strings.TrimSpace(rest))
		}
	}

	return locks
}

// TestOpenKeepsLocksOfOpenDatabase verifies that opening a database that is
// already open in this process does not release SQLite's POSIX locks on it.
// Closing any descriptor for a file releases all of the process's locks on it,
// so the copy-on-write check must not open and close a database that is open.
func TestOpenKeepsLocksOfOpenDatabase(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/locked.db"

	first, err := Open(path)
	assert.NilError(t, err)
	defer first.Close()

	// Hold a read transaction so SQLite holds a lock on the database file
	sqlDB, err := first.DB.DB()
	assert.NilError(t, err)
	tx, err := sqlDB.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	assert.NilError(t, err)
	defer func() { _ = tx.Rollback() }()

	var n int
	assert.NilError(t, tx.QueryRow("SELECT COUNT(*) FROM logs").Scan(&n))

	before := posixLocks(t, path)
	assert.Assert(t, len(before) > 0, "expected SQLite to hold a lock on %s", path)

	second, err := Open(path)
	assert.NilError(t, err)
	defer second.Close()

	assert.DeepEqual(t, posixLocks(t, path), before)
}

func TestTrackOpen(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/tracked.db"
	assert.NilError(t, os.WriteFile(path, nil, 0o644))
	assert.Assert(t, !isOpenInProcess(path))

	untrackA := trackOpen(path)
	untrackB := trackOpen(path)
	assert.Assert(t, isOpenInProcess(path))

	untrackA()
	untrackA() // idempotent
	assert.Assert(t, isOpenInProcess(path))

	untrackB()
	assert.Assert(t, !isOpenInProcess(path))
}
