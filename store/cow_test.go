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
