package hal

import "time"

// Timer wraps time.Timer to add functionality for checking if the timer is running.
type Timer struct {
	timer   *time.Timer
	running bool
}

func (t *Timer) Cancel() {
	if t.timer == nil {
		return
	}

	t.timer.Stop()
}

// Start starts the timer or resets it to a new duration.
func (t *Timer) Start(fn func(), d time.Duration) {
	if t.timer == nil {
		t.timer = time.AfterFunc(d, func() {
			t.running = false
			if fn != nil {
				fn()
			}
		})
	} else {
		t.timer.Reset(d)
	}

	t.running = true
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	return t.running
}
