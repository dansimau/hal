package hal

import (
	"time"

	"github.com/benbjohnson/clock"
)

// Timer wraps time.Timer to add functionality for checking if the timer is running.
type Timer struct {
	clock   clock.Clock
	timer   *clock.Timer
	running bool
}

func NewTimer(clock clock.Clock) *Timer {
	return &Timer{
		clock: clock,
	}
}

func (t *Timer) Cancel() {
	if t.timer == nil {
		return
	}

	t.timer.Stop()
}

// Start starts the timer or resets it to a new duration.
func (t *Timer) Start(fn func(), duration time.Duration) {
	if t.clock == nil {
		t.clock = clock.New()
	}

	if t.timer == nil {
		t.timer = t.clock.AfterFunc(duration, func() {
			t.running = false

			if fn != nil {
				fn()
			}
		})
	} else {
		t.timer.Reset(duration)
	}

	t.running = true
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	return t.running
}
