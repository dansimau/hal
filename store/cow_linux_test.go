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

// openDescriptors counts this process's file descriptors open on path,
// matching by inode so differently spelled paths (e.g. via symlinks) count.
func openDescriptors(t *testing.T, path string) int {
	t.Helper()

	want, err := os.Stat(path)
	assert.NilError(t, err)

	entries, err := os.ReadDir("/proc/self/fd")
	assert.NilError(t, err)

	n := 0
	for _, e := range entries {
		if got, err := os.Stat("/proc/self/fd/" + e.Name()); err == nil && os.SameFile(got, want) {
			n++
		}
	}

	return n
}

// TestFailedOpenClosesDatabase verifies that when Open fails after SQLite has
// opened the file, the file is closed before it is forgotten as open in this
// process, so a later Open never runs the copy-on-write check while SQLite
// still has the file open.
func TestFailedOpenClosesDatabase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Sanity check that openDescriptors sees SQLite's descriptors at all
	okPath := dir + "/ok.db"
	ok, err := Open(okPath)
	assert.NilError(t, err)
	assert.NilError(t, ok.Exec("SELECT 1").Error)
	assert.Assert(t, openDescriptors(t, okPath) > 0)
	assert.NilError(t, ok.Close())
	assert.Equal(t, openDescriptors(t, okPath), 0)

	// A view named "logs" makes AutoMigrate fail after SQLite has opened the
	// (valid) database
	path := dir + "/conflict.db"
	raw, err := sql.Open("sqlite", path)
	assert.NilError(t, err)
	_, err = raw.Exec("CREATE VIEW logs AS SELECT 1 AS id")
	assert.NilError(t, err)
	assert.NilError(t, raw.Close())

	_, err = Open(path)
	assert.ErrorContains(t, err, "logs")

	assert.Equal(t, openDescriptors(t, path), 0)
	assert.Assert(t, !isOpenInProcess(path))
}
