//go:build !linux

package store

// prepareDatabaseFile is a no-op outside Linux: inode flags and btrfs are
// Linux-specific.
func prepareDatabaseFile(string) {}

// readCOWState is unsupported outside Linux.
func readCOWState(string, string) (cowState, bool) {
	return cowState{}, false
}
