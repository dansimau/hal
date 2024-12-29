package timerutil

import "time"

// Timer wraps time.Timer to add functionality for checking if the timer is running.
type Timer struct {
	timer   *time.Timer
	running bool
}

// NewTimer creates a new Timer that will execute fn after the specified delay.
func NewTimer(d time.Duration, fn func()) *Timer {
	t := &Timer{}
	t.timer = time.AfterFunc(d, func() {
		t.running = false

		fn()
	})

	t.running = true

	return t
}

// Stop stops the timer and returns whether it was running.
func (t *Timer) Stop() bool {
	wasRunning := t.timer.Stop()
	t.running = false

	return wasRunning
}

// Reset stops the timer and resets it to a new duration.
func (t *Timer) Reset(d time.Duration) bool {
	wasRunning := t.timer.Reset(d)
	t.running = true

	return wasRunning
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	return t.running
}
