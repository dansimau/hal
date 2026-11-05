package store

import (
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
)

// memoryDatabase is the SQLite data source name for an in-memory database.
const memoryDatabase = ":memory:"

// Inode flags from linux/fs.h, as read by FS_IOC_GETFLAGS (lsattr).
const (
	fsComprFlag = 0x00000004 // FS_COMPR_FL: compressed ("c" in lsattr)
	fsNoCOWFlag = 0x00800000 // FS_NOCOW_FL: no copy-on-write ("C" in lsattr)
)

// cowFixHint explains how to move a database onto storage without
// copy-on-write. The flag only takes effect on empty files, so the database
// must be copied (not moved or reflinked) into a directory that has it set.
const cowFixHint = "stop HAL, create a new directory, run 'chattr +C' on it while it is " +
	"empty, copy the database (and any -wal file) into it with " +
	"'cp --reflink=never', and point HAL at the copy"

// cowState describes how a database file and its directory are stored.
type cowState struct {
	btrfs     bool   // the database is on a btrfs filesystem
	fileFlags uint32 // inode flags of the database file
	dirFlags  uint32 // inode flags of the directory containing it
}

// cowProblems returns a description of each way the storage described by state
// is unsuitable for SQLite. Copy-on-write filesystems relocate every rewritten
// block, so SQLite's small in-place page writes fragment the file badly, which
// makes reads very slow on spinning disks. Compression makes it worse: each 4KB
// page write rewrites a whole compressed extent (up to 128KB).
//
// Only btrfs is checked: it is copy-on-write by default and common on NAS
// devices. The directory matters as well as the file because SQLite creates
// its -wal and -journal files afresh, and new files inherit the directory's
// flags.
func cowProblems(state cowState) []string {
	if !state.btrfs {
		return nil
	}

	var problems []string
	if state.fileFlags&fsNoCOWFlag == 0 {
		problems = append(problems, "database file uses copy-on-write")
	}

	if state.fileFlags&fsComprFlag != 0 {
		problems = append(problems, "database file is compressed")
	}

	if state.dirFlags&fsNoCOWFlag == 0 {
		problems = append(problems, "database directory uses copy-on-write, so new -wal/-journal files will too")
	}

	return problems
}

// prepareDatabaseFile creates the database file at path with copy-on-write
// disabled if it does not exist yet (see createNoCOWFile), then warns if the
// database is on copy-on-write storage. Failures are logged, never returned:
// they only affect performance. Callers must serialise this with opening the
// database (see openMu) so the database file is never opened and closed here
// while SQLite has it open.
func prepareDatabaseFile(path string) {
	if !createNoCOWFile(path) {
		return
	}

	warnCopyOnWrite(path)
}

// warnCopyOnWrite logs a warning if the database at path is on copy-on-write
// or compressed storage. It never fails: the database still works, just slowly.
func warnCopyOnWrite(path string) {
	state, ok := readCOWState(path, filepath.Dir(path))
	if !ok {
		return
	}

	for _, problem := range cowProblems(state) {
		slog.Warn("SQLite database is on copy-on-write storage, which makes it slow and fragmented; "+cowFixHint,
			"path", path, "problem", problem)
	}
}

// sqliteFilePath returns the filesystem path SQLite opens for the data source
// name, or false for in-memory and temporary databases. It mirrors the driver:
// a query string ("?...") is not part of the filename, and "file:" URIs are
// decoded, including file:///absolute/path and percent-escapes.
func sqliteFilePath(name string) (string, bool) {
	if !strings.HasPrefix(name, "file:") {
		path, _, _ := strings.Cut(name, "?")
		if path == "" || path == memoryDatabase {
			return "", false
		}

		return path, true
	}

	u, err := url.Parse(name)
	if err != nil || u.Query().Get("mode") == "memory" {
		return "", false
	}

	if u.Host != "" && u.Host != "localhost" {
		return "", false // SQLite rejects URIs with any other authority
	}

	path := u.Path

	if u.Opaque != "" {
		path, err = url.PathUnescape(u.Opaque)
		if err != nil {
			return "", false
		}
	}

	if path == "" || path == memoryDatabase {
		return "", false
	}

	return path, true
}
