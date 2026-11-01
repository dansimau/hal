package store

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// openFiles counts the database files this process has open through Open,
// keyed by inode. On Linux, closing any file descriptor for a file releases
// every POSIX lock the process holds on it, including SQLite's. The
// copy-on-write check must open the database file to read its flags, so it is
// skipped for files that are already open, which were checked when first
// opened. (This cannot see files opened by other SQLite code in the process.)
var openFiles = struct {
	sync.Mutex
	count map[fileID]int
}{count: make(map[fileID]int)}

type fileID struct{ dev, ino uint64 }

func statFileID(path string) (fileID, bool) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return fileID{}, false
	}

	return fileID{dev: uint64(st.Dev), ino: st.Ino}, true //nolint:unconvert // Dev is uint32 on some architectures
}

// trackOpen records that path is open in this process and returns a function
// that removes the record.
func trackOpen(path string) func() {
	id, ok := statFileID(path)
	if !ok {
		return func() {}
	}

	openFiles.Lock()
	openFiles.count[id]++
	openFiles.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			openFiles.Lock()
			defer openFiles.Unlock()

			if openFiles.count[id]--; openFiles.count[id] <= 0 {
				delete(openFiles.count, id)
			}
		})
	}
}

func isOpenInProcess(path string) bool {
	id, ok := statFileID(path)
	if !ok {
		return false
	}

	openFiles.Lock()
	defer openFiles.Unlock()

	return openFiles.count[id] > 0
}

// prepareDatabaseFile disables copy-on-write for a new database on btrfs, then
// warns if the database is on copy-on-write storage. The no-copy-on-write flag
// can only be set on an empty file, so the file is created (empty) before
// SQLite opens it. Failures are logged, never returned: they only affect
// performance. Callers must serialise this with opening the database (see
// openMu) so the database file is never opened and closed here while SQLite
// has it open.
func prepareDatabaseFile(path string) {
	if !isBtrfs(path) || isOpenInProcess(path) {
		return
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	switch {
	case err == nil:
		if err := setNoCOW(f); err != nil {
			slog.Warn("Could not disable copy-on-write for new SQLite database", "path", path, "error", err)
		}
		_ = f.Close()
	case !errors.Is(err, os.ErrExist):
		// Let SQLite report the problem when it opens the file
		return
	}

	warnCopyOnWrite(path)
}

// readCOWState reports the filesystem and inode flags of file and dir. ok is
// false if they could not be read.
func readCOWState(file, dir string) (cowState, bool) {
	fileFlags, err := inodeFlags(file)
	if err != nil {
		return cowState{}, false
	}

	dirFlags, err := inodeFlags(dir)
	if err != nil {
		return cowState{}, false
	}

	return cowState{btrfs: isBtrfs(file), fileFlags: fileFlags, dirFlags: dirFlags}, true
}

// isBtrfs reports whether path, or its directory if path does not exist, is
// on btrfs. It uses statfs, which does not open the file.
func isBtrfs(path string) bool {
	var fs unix.Statfs_t
	if err := unix.Statfs(path, &fs); err != nil {
		if err := unix.Statfs(filepath.Dir(path), &fs); err != nil {
			return false
		}
	}

	return uint32(fs.Type) == unix.BTRFS_SUPER_MAGIC
}

func inodeFlags(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
}

// setNoCOW sets the no-copy-on-write flag on f, which must be empty, and
// clears the compression flag (btrfs cannot compress no-copy-on-write files).
func setNoCOW(f *os.File) error {
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return err
	}

	flags = (flags | fsNoCOWFlag) &^ fsComprFlag

	return unix.IoctlSetPointerInt(int(f.Fd()), unix.FS_IOC_SETFLAGS, int(flags))
}
