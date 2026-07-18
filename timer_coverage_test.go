package hal_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

func TestTimerNewAndIsRunning(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()
	timer := hal.NewTimer(mock)

	// A freshly constructed timer is not running.
	assert.Assert(t, !timer.IsRunning())

	var fired atomic.Bool
	timer.StartContext(context.Background(), func(context.Context) {
		fired.Store(true)
	}, time.Minute)

	// Running until the mock clock advances past the duration.
	assert.Assert(t, timer.IsRunning())
	assert.Assert(t, !fired.Load())

	mock.Add(time.Minute)

	assert.Assert(t, fired.Load())
	// After firing, the timer reports it is no longer running.
	assert.Assert(t, !timer.IsRunning())
}

func TestTimerCancel(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()
	timer := hal.NewTimer(mock)

	// Cancel before start is a no-op (timer is nil internally).
	timer.Cancel()
	assert.Assert(t, !timer.IsRunning())

	var fired atomic.Bool
	timer.StartContext(context.Background(), func(context.Context) {
		fired.Store(true)
	}, time.Minute)
	assert.Assert(t, timer.IsRunning())

	timer.Cancel()
	assert.Assert(t, !timer.IsRunning())

	// Advancing past the original duration must not fire the cancelled timer.
	mock.Add(2 * time.Minute)
	assert.Assert(t, !fired.Load())
}

func TestTimerResetPicksUpLatestCallback(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()
	timer := hal.NewTimer(mock)

	var first, second atomic.Bool
	timer.StartContext(context.Background(), func(context.Context) {
		first.Store(true)
	}, time.Minute)

	// Restart with a new callback before the first fires; the reset timer must
	// invoke the latest callback, not the original one.
	timer.StartContext(context.Background(), func(context.Context) {
		second.Store(true)
	}, time.Minute)

	mock.Add(time.Minute)

	assert.Assert(t, !first.Load())
	assert.Assert(t, second.Load())
}
