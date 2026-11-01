package store

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCOWProblems(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		state cowState
		want  int
	}{
		{"not btrfs", cowState{btrfs: false}, 0},
		{"btrfs nocow file and dir", cowState{true, fsNoCOWFlag, fsNoCOWFlag}, 0},
		{"btrfs default", cowState{true, 0, 0}, 2},
		{"btrfs compressed file", cowState{true, fsComprFlag, fsNoCOWFlag}, 2},
		{"btrfs nocow file in cow dir", cowState{true, fsNoCOWFlag, 0}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, len(cowProblems(tc.state)), tc.want)
		})
	}
}

func TestSQLiteFilePath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		want   string
		isFile bool
	}{
		{"state.db", "state.db", true},
		{"/data/sqlite.db", "/data/sqlite.db", true},
		{"state.db?_pragma=foo", "state.db", true},
		{"file:state.db", "state.db", true},
		{"file:data/state.db?mode=rwc", "data/state.db", true},
		{"file:///abs/state.db", "/abs/state.db", true},
		{"file://localhost/abs/state.db", "/abs/state.db", true},
		{"file:my%20state.db", "my state.db", true},
		{"", "", false},
		{":memory:", "", false},
		{"file::memory:?cache=shared", "", false},
		{"file:state.db?mode=memory", "", false},
		{"file://otherhost/state.db", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, isFile := sqliteFilePath(tc.name)
			assert.Equal(t, isFile, tc.isFile)
			assert.Equal(t, got, tc.want)
		})
	}
}
