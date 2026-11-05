package hal

import (
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dansimau/hal/store"
)

// Timer calls OnExpire after a delay, and can be cancelled or restarted.
//
// Timers are durable: a timer declared on an automation (see
// AutomationConfig.WithTimer and TimerProvider) saves its state to the database
// whenever it starts, fires or is cancelled, and is restored after a restart.
// A restored timer fires at its original expiry, or immediately if that passed
// while hal was down.
//
// The zero value is ready to use. Name, OnExpire and Clock must be set before
// the timer is started or registered, normally with WithTimer.
type Timer struct {
	// Name identifies the timer in the database. It must be unique across all
	// timers; a timer with no name, or a name already in use, is not saved.
	Name string
	// OnExpire is called when the timer expires. It may be nil.
	OnExpire func(ctx context.Context)
	// Clock is the clock the timer runs on. It is nil for the real clock, and
	// can be set to a mock clock in tests.
	Clock clock.Clock

	// state holds the timer's mutable state behind a pointer, so that a Timer
	// can be passed by value as configuration (see WithTimer) without copying
	// a lock. It is allocated on first use.
	state *timerState
}

// timerState is the mutable state of a Timer.
type timerState struct {
	// mutex guards the fields below, which the timer callback goroutine mutates
	// concurrently with callers of Start/Cancel/IsRunning.
	mutex sync.Mutex
	timer *clock.Timer
	// status is the timer's current state; it is running only while a
	// countdown is pending. It is the zero value until the first start.
	status store.TimerStatus
	// ctx is the context for the pending callback, from the latest Start.
	ctx context.Context //nolint:containedctx // context is stored for the pending callback
	// generation is incremented on every start and cancel. A pending expiry
	// carries the generation it was scheduled under and is ignored if the
	// timer has since been restarted or cancelled, so OnExpire never runs for
	// a timer that was cancelled or reset just as it expired.
	generation uint64
	// startedAt and expiresAt describe the current (or most recent) countdown.
	startedAt time.Time
	expiresAt time.Time
	// persist is set when the timer is registered with a connection. It is
	// called under the lock whenever the timer's state changes and must not
	// block.
	persist func(*store.Timer)
}

// timerInitMutex guards the allocation of a zero Timer's state, so that a
// timer first used from several goroutines at once is initialised exactly once.
var timerInitMutex sync.Mutex //nolint:gochecknoglobals // process-wide lock for lazy initialisation

// realClock is the clock timers run on unless Clock is set.
var realClock = clock.New() //nolint:gochecknoglobals // the real clock is stateless

// Start starts the timer, or restarts it if it is already running, to call
// OnExpire with ctx after duration.
func (t *Timer) Start(ctx context.Context, duration time.Duration) {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.ctx = ctx

	now := t.now()
	t.schedule(state, now, now.Add(duration))
	t.save(state)
}

// Cancel stops the timer. OnExpire will not run, even if the timer expired at
// the same moment.
func (t *Timer) Cancel() {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	if state.timer == nil {
		return
	}

	state.timer.Stop()
	state.generation++

	if state.status == store.TimerStatusRunning {
		state.status = store.TimerStatusCancelled
		t.save(state)
	}
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	return state.status == store.TimerStatusRunning
}

// ExpiresAt returns when the running timer will fire. If the timer is not
// running it returns when it last fired or was cancelled, or the zero time if
// it has never been started.
func (t *Timer) ExpiresAt() time.Time {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	return state.expiresAt
}

// restore starts the timer from its saved state. It fires at saved.ExpiresAt,
// or immediately if that has already passed (e.g. while hal was down), with
// ctx. A timer that is already running is left alone, since its in-memory
// state is newer than the saved one. Unlike Start, restore does not save the
// state it was just given.
func (t *Timer) restore(ctx context.Context, saved store.Timer) {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	if state.status == store.TimerStatusRunning {
		return
	}

	state.ctx = ctx
	t.schedule(state, saved.StartedAt, saved.ExpiresAt)
}

// sync saves the timer's current state, so the database matches the timer
// exactly (e.g. at shutdown). A timer that has never been started has no
// state to save.
func (t *Timer) sync() {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	if state.timer == nil {
		return
	}

	t.save(state)
}

// bind makes the timer save its state through persist, which is called under
// the lock and must not block.
func (t *Timer) bind(persist func(*store.Timer)) {
	state := t.getState()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.persist = persist
}

// schedule arms the timer to fire at expiresAt (immediately if that has
// passed), replacing any pending expiry. Called with the lock held.
func (t *Timer) schedule(state *timerState, startedAt, expiresAt time.Time) {
	if state.timer != nil {
		state.timer.Stop()
	}

	state.generation++
	generation := state.generation
	state.startedAt = startedAt
	state.expiresAt = expiresAt
	state.status = store.TimerStatusRunning

	state.timer = t.afterFunc(max(expiresAt.Sub(t.now()), 0), func() {
		t.fire(generation)
	})
}

// fire runs when the timer expires. It does nothing if the timer was
// restarted or cancelled since this expiry was scheduled.
func (t *Timer) fire(generation uint64) {
	state := t.getState()

	state.mutex.Lock()

	if generation != state.generation {
		state.mutex.Unlock()

		return
	}

	state.status = store.TimerStatusExpired
	ctx := state.ctx
	t.save(state)
	state.mutex.Unlock()

	if t.OnExpire != nil {
		t.OnExpire(ctx)
	}
}

// save persists the timer's state if it is registered. Called with the lock
// held.
func (t *Timer) save(state *timerState) {
	if state.persist == nil {
		return
	}

	state.persist(&store.Timer{
		Name:      t.Name,
		Status:    state.status,
		StartedAt: state.startedAt,
		ExpiresAt: state.expiresAt,
	})
}

// getState returns the timer's state, allocating it on first use.
func (t *Timer) getState() *timerState {
	timerInitMutex.Lock()
	defer timerInitMutex.Unlock()

	if t.state == nil {
		t.state = &timerState{}
	}

	return t.state
}

// afterFunc schedules fn to run after d on the timer's clock.
func (t *Timer) afterFunc(d time.Duration, fn func()) *clock.Timer {
	if t.Clock != nil {
		return t.Clock.AfterFunc(d, fn)
	}

	return realClock.AfterFunc(d, fn)
}

// now returns the current time from the timer's clock.
func (t *Timer) now() time.Time {
	if t.Clock != nil {
		return t.Clock.Now()
	}

	return realClock.Now()
}
