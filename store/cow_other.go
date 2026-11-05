//go:build !linux

package store

// createNoCOWFile is a no-op outside Linux: inode flags and btrfs are
// Linux-specific. It reports false so the copy-on-write check is skipped.
func createNoCOWFile(string) bool { return false }

// readCOWState is unsupported outside Linux.
func readCOWState(string, string) (cowState, bool) {
	return cowState{}, false
}

// trackOpen is a no-op outside Linux.
func trackOpen(string) func() { return func() {} }
