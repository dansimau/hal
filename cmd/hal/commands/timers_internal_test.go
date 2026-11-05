package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

func TestNewTimersCmd(t *testing.T) {
	t.Parallel()

	cmd := NewTimersCmd()
	assert.Equal(t, cmd.Use, "timers")
	assert.Assert(t, len(cmd.Aliases) > 0)
	assert.Assert(t, cmd.Flags().Lookup("db") != nil)
}

func TestPrintTimersTable(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		assert.NilError(t, printTimersTable(&out, nil, time.Now()))
		assert.Assert(t, strings.Contains(out.String(), "No timers found"))
	})

	t.Run("with rows", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		timers := []store.Timer{
			{
				Name: "kitchen/turn_off", Status: store.TimerStatusRunning,
				StartedAt: now, ExpiresAt: now.Add(90 * time.Second),
			},
			{
				Name: "hall/turn_off", Status: store.TimerStatusRunning,
				StartedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute),
			},
			{
				Name: "hall/dim_lights", Status: store.TimerStatusCancelled,
				StartedAt: now, ExpiresAt: now.Add(time.Minute),
			},
		}

		var out bytes.Buffer

		assert.NilError(t, printTimersTable(&out, timers, now))

		output := out.String()
		assert.Assert(t, strings.Contains(output, "kitchen/turn_off"))
		assert.Assert(t, strings.Contains(output, "2024-01-02 03:05:35"))
		assert.Assert(t, strings.Contains(output, "1m30s"))
		assert.Assert(t, strings.Contains(output, "due"))
		assert.Assert(t, strings.Contains(output, string(store.TimerStatusCancelled)))
	})
}

func TestFormatRemaining(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name   string
		status store.TimerStatus
		expiry time.Time
		want   string
	}{
		{
			name: "rounded", status: store.TimerStatusRunning,
			expiry: now.Add(time.Minute + 400*time.Millisecond), want: "1m0s",
		},
		{name: "due", status: store.TimerStatusRunning, expiry: now, want: "due"},
		{name: "expired", status: store.TimerStatusExpired, expiry: now.Add(time.Minute), want: "-"},
		{name: "cancelled", status: store.TimerStatusCancelled, expiry: now.Add(time.Minute), want: "-"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, formatRemaining(store.Timer{Status: tc.status, ExpiresAt: tc.expiry}, now), tc.want)
		})
	}
}

func TestRunTimersCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Timer{
			Name:      "kitchen/turn_off",
			Status:    store.TimerStatusRunning,
			StartedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		}).Error)
	})

	var out bytes.Buffer

	assert.NilError(t, runTimersCommand(&out, dir+"/sqlite.db", time.Now()))
	assert.Assert(t, strings.Contains(out.String(), "kitchen/turn_off"))
	assert.Assert(t, strings.Contains(out.String(), string(store.TimerStatusRunning)))

	// A database that cannot be opened is an error.
	err := runTimersCommand(&out, t.TempDir()+"/missing/sqlite.db", time.Now())
	assert.ErrorContains(t, err, "failed to open database")
}
