package store_test

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

// testTimerName is the timer used throughout the timer store tests.
const testTimerName = "kitchen/turn_off"

// TestTimerStatesAreCoalesced verifies that only the latest state per timer is
// written, that states for different timers are all written, and that Timers
// returns them ordered by name.
func TestTimerStatesAreCoalesced(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)
	release := stallWriter(t, db)

	started := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	statuses := []store.TimerStatus{store.TimerStatusRunning, store.TimerStatusCancelled, store.TimerStatusRunning}
	for _, status := range statuses {
		db.EnqueueTimerState(&store.Timer{
			Name: testTimerName, Status: status, StartedAt: started, ExpiresAt: started.Add(time.Minute),
		})
	}

	db.EnqueueTimerState(&store.Timer{
		Name: "hall/turn_off", Status: store.TimerStatusExpired, StartedAt: started, ExpiresAt: started.Add(time.Hour),
	})

	release()
	db.WaitForWrites()

	timers, err := db.Timers()
	assert.NilError(t, err)
	assert.Equal(t, len(timers), 2)
	assert.Equal(t, timers[0].Name, "hall/turn_off")
	assert.Equal(t, timers[0].Status, store.TimerStatusExpired)
	assert.Equal(t, timers[1].Name, testTimerName)
	assert.Equal(t, timers[1].Status, store.TimerStatusRunning)
	assert.Assert(t, timers[1].ExpiresAt.Equal(started.Add(time.Minute)))
}

// TestTimerStateIsUpdatedInPlace verifies that a later write for the same timer
// replaces the saved row rather than failing on the primary key.
func TestTimerStateIsUpdatedInPlace(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)

	started := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	db.EnqueueTimerState(&store.Timer{
		Name: testTimerName, Status: store.TimerStatusRunning, StartedAt: started, ExpiresAt: started.Add(time.Minute),
	})
	db.WaitForWrites()

	db.EnqueueTimerState(&store.Timer{
		Name: testTimerName, Status: store.TimerStatusExpired, StartedAt: started, ExpiresAt: started.Add(time.Minute),
	})
	db.WaitForWrites()

	timers, err := db.Timers()
	assert.NilError(t, err)
	assert.Equal(t, len(timers), 1)
	assert.Equal(t, timers[0].Status, store.TimerStatusExpired)
}
