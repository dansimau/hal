package hal

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

const (
	// testAutomationName is the name of the automation used in timer tests.
	testAutomationName = "test automation"
	// testTimerName is the name of the test automation's timer.
	testTimerName = "TestCountdown"
	// recordedTimerName is the name of timers whose saves are recorded directly.
	recordedTimerName = "auto/timer"
	// testTimerDuration is how long the test automation's timer runs for.
	testTimerDuration = time.Minute
)

// timerAutomation is an automation with one timer that counts how many times
// the timer fires.
type timerAutomation struct {
	name   string
	entity *Entity
	timer  Timer
	fired  atomic.Int32
}

func newTimerAutomation(c clock.Clock) *timerAutomation {
	a := &timerAutomation{
		name:   testAutomationName,
		entity: NewEntity(testEntityID),
	}

	a.timer = Timer{Name: testTimerName, Clock: c, OnExpire: a.onExpire}

	return a
}

func (a *timerAutomation) Name() string { return a.name }

func (a *timerAutomation) Entities() Entities { return Entities{a.entity} }

func (a *timerAutomation) Action(ctx context.Context, _ EntityInterface) {
	a.timer.Start(ctx, testTimerDuration)
}

func (a *timerAutomation) Timers() []*Timer { return []*Timer{&a.timer} }

func (a *timerAutomation) onExpire(context.Context) { a.fired.Add(1) }

// recorder collects the states a timer saves.
type recorder struct {
	mu     sync.Mutex
	states []store.Timer
}

func (r *recorder) persist(state *store.Timer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.states = append(r.states, *state)
}

func (r *recorder) statuses() []store.TimerStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	statuses := make([]store.TimerStatus, 0, len(r.states))
	for _, state := range r.states {
		statuses = append(statuses, state.Status)
	}

	return statuses
}

func (r *recorder) last() store.Timer {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.states[len(r.states)-1]
}

func TestTimerSavesStateChanges(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()
	mock.Set(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))

	timer := Timer{Name: recordedTimerName, Clock: mock}

	var saved recorder

	timer.bind(saved.persist)

	// Cancelling a timer that was never started saves nothing.
	timer.Cancel()
	assert.Equal(t, len(saved.statuses()), 0)

	timer.Start(context.Background(), time.Minute)
	assert.DeepEqual(t, saved.statuses(), []store.TimerStatus{store.TimerStatusRunning})
	assert.Equal(t, saved.last().Name, recordedTimerName)
	assert.Assert(t, saved.last().StartedAt.Equal(mock.Now()))
	assert.Assert(t, saved.last().ExpiresAt.Equal(mock.Now().Add(time.Minute)))

	// Restarting saves the new expiry.
	mock.Add(30 * time.Second)
	timer.Start(context.Background(), time.Minute)
	assert.Assert(t, saved.last().ExpiresAt.Equal(mock.Now().Add(time.Minute)))

	timer.Cancel()
	assert.Equal(t, saved.last().Status, store.TimerStatusCancelled)

	// Cancelling again saves nothing more.
	timer.Cancel()
	assert.Equal(t, len(saved.statuses()), 3)

	timer.Start(context.Background(), time.Minute)
	mock.Add(time.Minute)
	assert.Equal(t, saved.last().Status, store.TimerStatusExpired)
	assert.Equal(t, len(saved.statuses()), 5)
}

func TestTimerSyncSavesCurrentState(t *testing.T) {
	t.Parallel()

	timer := Timer{Name: recordedTimerName, Clock: clock.NewMock()}

	var saved recorder

	timer.bind(saved.persist)

	// A timer that has never started has no state to save.
	timer.sync()
	assert.Equal(t, len(saved.statuses()), 0)

	timer.Start(context.Background(), time.Minute)
	timer.sync()
	assert.DeepEqual(t, saved.statuses(), []store.TimerStatus{store.TimerStatusRunning, store.TimerStatusRunning})

	timer.Cancel()
	timer.sync()
	assert.Equal(t, saved.last().Status, store.TimerStatusCancelled)
}

// TestTimerCancelledAtExpiryDoesNotFire verifies that a callback whose expiry
// is already in flight is dropped by Cancel.
func TestTimerCancelledAtExpiryDoesNotFire(t *testing.T) {
	t.Parallel()

	var fired atomic.Int32

	timer := Timer{Clock: clock.NewMock(), OnExpire: func(context.Context) { fired.Add(1) }}

	timer.Start(context.Background(), time.Minute)
	timer.Cancel()

	// The expiry scheduled by the start is stale once cancelled.
	timer.fire(1)
	assert.Equal(t, fired.Load(), int32(0))
	assert.Assert(t, !timer.IsRunning())
}

// TestTimerRestartedAtExpiryFiresOnce verifies that an expiry in flight when
// the timer is restarted is dropped, so the callback runs once, at the new
// expiry, with the new context.
func TestTimerRestartedAtExpiryFiresOnce(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()

	var (
		fired atomic.Int32
		names sync.Map
	)

	timer := Timer{Clock: mock, OnExpire: func(ctx context.Context) {
		fired.Add(1)
		names.Store(GetAutomationNameFromContext(ctx), true)
	}}

	timer.Start(NewAutomationContext("", "first"), time.Minute)
	timer.Start(NewAutomationContext("", "second"), time.Minute)

	// The first start's expiry is stale.
	timer.fire(1)
	assert.Equal(t, fired.Load(), int32(0))
	assert.Assert(t, timer.IsRunning())

	mock.Add(time.Minute)
	assert.Equal(t, fired.Load(), int32(1))

	_, first := names.Load("first")
	assert.Assert(t, !first)

	_, second := names.Load("second")
	assert.Assert(t, second)
}

func TestTimerRestore(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("fires at the saved expiry", func(t *testing.T) {
		t.Parallel()

		mock := clock.NewMock()
		mock.Set(now)

		var fired atomic.Int32

		timer := Timer{Name: recordedTimerName, Clock: mock, OnExpire: func(context.Context) { fired.Add(1) }}

		var saved recorder

		timer.bind(saved.persist)

		timer.restore(context.Background(), store.Timer{
			StartedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
		})

		// Restoring does not rewrite the state it was given.
		assert.Equal(t, len(saved.statuses()), 0)
		assert.Assert(t, timer.IsRunning())
		assert.Assert(t, timer.ExpiresAt().Equal(now.Add(time.Minute)))

		mock.Add(59 * time.Second)
		assert.Equal(t, fired.Load(), int32(0))

		mock.Add(time.Second)
		assert.Equal(t, fired.Load(), int32(1))
		assert.Equal(t, saved.last().Status, store.TimerStatusExpired)
	})

	t.Run("fires immediately when the expiry has passed", func(t *testing.T) {
		t.Parallel()

		mock := clock.NewMock()
		mock.Set(now)

		var fired atomic.Int32

		timer := Timer{Clock: mock, OnExpire: func(context.Context) { fired.Add(1) }}

		timer.restore(context.Background(), store.Timer{
			StartedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(-time.Minute),
		})

		mock.Add(0)
		assert.Equal(t, fired.Load(), int32(1))
	})

	t.Run("leaves a running timer alone", func(t *testing.T) {
		t.Parallel()

		mock := clock.NewMock()
		mock.Set(now)

		timer := Timer{Clock: mock}
		timer.Start(context.Background(), time.Hour)

		timer.restore(context.Background(), store.Timer{
			StartedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
		})

		assert.Assert(t, timer.ExpiresAt().Equal(now.Add(time.Hour)))
	})
}

// startConnection creates a connection to a new mock Home Assistant server
// using the database at dbPath, registers the automations, starts it and
// waits for the first connection (and so the timer restore) to complete. Each
// connection gets its own server because the mock server shuts down when its
// client closes the connection.
func startConnection(t *testing.T, dbPath string, automations ...Automation) *Connection {
	t.Helper()

	server, err := hassws.NewServer(map[string]string{testToken: testUserID})
	assert.NilError(t, err)
	t.Cleanup(func() { _ = server.Close() })

	conn := NewConnection(Config{
		HomeAssistant: HomeAssistantConfig{
			Host:   server.ListenAddress(),
			Token:  testToken,
			UserID: testUserID,
		},
		DatabasePath: dbPath,
	})
	conn.RegisterAutomations(automations...)

	go func() {
		err := conn.Start()
		if err != nil {
			t.Errorf("Start() failed: %v", err)
		}
	}()

	waitFor(t, "connection established and timers restored", func() bool {
		conn.mutex.RLock()
		defer conn.mutex.RUnlock()

		return conn.timersRestored
	}, func() {})

	return conn
}

func savedTimers(t *testing.T, dbPath string) []store.Timer {
	t.Helper()

	db, err := store.Open(dbPath)
	assert.NilError(t, err)

	defer func() { assert.NilError(t, db.Close()) }()

	timers, err := db.Timers()
	assert.NilError(t, err)

	return timers
}

// TestTimersSurviveRestart verifies that a timer running when the connection is
// closed is saved, and that a new connection on the same database restores it
// to fire at the original expiry.
func TestTimersSurviveRestart(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/hal.db"

	stoppedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	// First run: start the timer, then shut down with it running. Each run has
	// its own clock, since the first run's timer lives on in this process but
	// would be gone with its process in reality.
	firstClock := clock.NewMock()
	firstClock.Set(stoppedAt)

	first := newTimerAutomation(firstClock)
	conn := startConnection(t, dbPath, first)

	first.Action(context.Background(), first.entity)
	expiresAt := first.timer.ExpiresAt()

	conn.Close()

	saved := savedTimers(t, dbPath)
	assert.Equal(t, len(saved), 1)
	assert.Equal(t, saved[0].Name, testTimerName)
	assert.Equal(t, saved[0].Status, store.TimerStatusRunning)
	assert.Assert(t, saved[0].ExpiresAt.Equal(expiresAt))

	// Second run, after half the countdown passed while hal was down: the
	// timer resumes and fires at the original expiry.
	secondClock := clock.NewMock()
	secondClock.Set(stoppedAt.Add(testTimerDuration / 2))

	second := newTimerAutomation(secondClock)
	conn = startConnection(t, dbPath, second)

	defer conn.Close()

	assert.Assert(t, second.timer.IsRunning())
	assert.Assert(t, second.timer.ExpiresAt().Equal(expiresAt))

	secondClock.Add(testTimerDuration/2 - time.Second)
	assert.Equal(t, second.fired.Load(), int32(0))

	secondClock.Add(time.Second)
	assert.Equal(t, second.fired.Load(), int32(1))

	conn.Close()

	saved = savedTimers(t, dbPath)
	assert.Equal(t, len(saved), 1)
	assert.Equal(t, saved[0].Status, store.TimerStatusExpired)
}

// TestTimerDueWhileDownFiresOnRestore verifies that a saved timer whose expiry
// passed while hal was down fires as soon as it is restored.
func TestTimerDueWhileDownFiresOnRestore(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/hal.db"

	db, err := store.Open(dbPath)
	assert.NilError(t, err)
	assert.NilError(t, db.Create(&store.Timer{
		Name:      testTimerName,
		Status:    store.TimerStatusRunning,
		StartedAt: time.Now().Add(-2 * time.Minute),
		ExpiresAt: time.Now().Add(-time.Minute),
	}).Error)
	// A timer that is not running, and one for an automation that no longer
	// exists, are left alone.
	assert.NilError(t, db.Create(&store.Timer{
		Name: "other/countdown", Status: store.TimerStatusRunning, StartedAt: time.Now(), ExpiresAt: time.Now(),
	}).Error)
	assert.NilError(t, db.Create(&store.Timer{
		Name: testAutomationName + "/other", Status: store.TimerStatusCancelled, StartedAt: time.Now(), ExpiresAt: time.Now(),
	}).Error)
	assert.NilError(t, db.Close())

	automation := newTimerAutomation(clock.New())
	conn := startConnection(t, dbPath, automation)

	defer conn.Close()

	waitFor(t, "timer fired", func() bool { return automation.fired.Load() == 1 }, func() {})
}

func TestRegisterTimers(t *testing.T) {
	t.Parallel()

	conn := NewConnection(Config{DatabasePath: memoryDatabasePath})
	defer conn.Close()

	automation := newTimerAutomation(clock.NewMock())
	conn.RegisterAutomations(automation)
	assert.Equal(t, len(conn.timers), 1)
	assert.Equal(t, conn.timers[testTimerName].automation, testAutomationName)

	// Registering the same automation again is a no-op.
	conn.RegisterAutomations(automation)
	assert.Equal(t, len(conn.timers), 1)

	// Another automation with a clashing timer name is not registered, and
	// its timer is not saved.
	duplicate := newTimerAutomation(clock.NewMock())
	conn.RegisterAutomations(duplicate)
	assert.Equal(t, len(conn.timers), 1)
	assert.Assert(t, conn.timers[testTimerName].timer == &automation.timer)
	assert.Assert(t, duplicate.timer.getState().persist == nil)

	// A nil timer, and a timer without a name, are skipped.
	conn.RegisterAutomations(&badTimerAutomation{})
	assert.Equal(t, len(conn.timers), 1)

	// Automations without timers are fine.
	conn.RegisterAutomations(NewAutomation().WithName("plain").WithAction(func(context.Context, EntityInterface) {}))
	assert.Equal(t, len(conn.timers), 1)

	// Built automations initialise and declare timers with WithTimer.
	var built Timer

	onExpire := func(context.Context) {}

	conn.RegisterAutomations(NewAutomation().
		WithName("built").
		WithTimer(&built, Timer{Name: "Built", OnExpire: onExpire}).
		WithAction(func(context.Context, EntityInterface) {}))
	assert.Equal(t, len(conn.timers), 2)
	assert.Assert(t, conn.timers["Built"].timer == &built)
	assert.Equal(t, built.Name, "Built")
	assert.Assert(t, built.OnExpire != nil)
	assert.Assert(t, built.getState().persist != nil)
}

type badTimerAutomation struct {
	unnamed Timer
}

func (*badTimerAutomation) Name() string                            { return "bad" }
func (*badTimerAutomation) Entities() Entities                      { return nil }
func (*badTimerAutomation) Action(context.Context, EntityInterface) {}
func (a *badTimerAutomation) Timers() []*Timer                      { return []*Timer{nil, &a.unnamed} }

// TestRestoreTimersRunsOnce verifies that timers are only restored from the
// database on the first connection, not again on reconnection.
func TestRestoreTimersRunsOnce(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/hal.db"

	db, err := store.Open(dbPath)
	assert.NilError(t, err)
	assert.NilError(t, db.Create(&store.Timer{
		Name:      testTimerName,
		Status:    store.TimerStatusRunning,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)
	assert.NilError(t, db.Close())

	conn := NewConnection(Config{DatabasePath: dbPath})
	defer conn.Close()

	automation := newTimerAutomation(clock.NewMock())
	conn.RegisterAutomations(automation)

	conn.mutex.Lock()
	conn.restoreTimers()
	conn.mutex.Unlock()

	assert.Assert(t, automation.timer.IsRunning())

	automation.timer.Cancel()

	conn.mutex.Lock()
	conn.restoreTimers()
	conn.mutex.Unlock()

	assert.Assert(t, !automation.timer.IsRunning())
}

// TestRestoreTimersLoadFailure verifies that a database the timers cannot be
// read from is logged and skipped rather than crashing startup.
func TestRestoreTimersLoadFailure(t *testing.T) {
	t.Parallel()

	conn := NewConnection(Config{DatabasePath: memoryDatabasePath})
	defer conn.Close()

	assert.NilError(t, conn.db.Exec("DROP TABLE timers").Error)

	conn.mutex.Lock()
	conn.restoreTimers()
	conn.mutex.Unlock()

	assert.Assert(t, conn.timersRestored)
}
