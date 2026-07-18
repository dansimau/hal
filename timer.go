package hal

import (
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
)

// Timer wraps time.Timer to add functionality for checking if the timer is running.
type Timer struct {
	// mutex guards the fields below, which the timer callback goroutine mutates
	// concurrently with callers of StartContext/Cancel/IsRunning.
	mutex   sync.Mutex
	clock   clock.Clock
	timer   *clock.Timer
	running bool
	ctx     context.Context
}

func NewTimer(clock clock.Clock) *Timer {
	return &Timer{
		clock: clock,
	}
}

func (t *Timer) Cancel() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.timer == nil {
		return
	}

	t.timer.Stop()
}

// StartContext starts a timer with context that will be passed to the callback
func (t *Timer) StartContext(ctx context.Context, fn func(context.Context), duration time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.clock == nil {
		t.clock = clock.New()
	}

	t.ctx = ctx

	if t.timer == nil {
		t.timer = t.clock.AfterFunc(duration, func() {
			t.mutex.Lock()
			t.running = false
			t.mutex.Unlock()

			if fn != nil {
				fn(ctx)
			}
		})
	} else {
		t.timer.Reset(duration)
	}

	t.running = true
}

// Start starts the timer or resets it to a new duration.
// Deprecated: Use StartContext to propagate context for tracing.
func (t *Timer) Start(fn func(), duration time.Duration) {
	t.StartContext(context.Background(), func(context.Context) {
		if fn != nil {
			fn()
		}
	}, duration)
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.running
}
