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

// TestTimerZeroValue verifies that a zero Timer runs on the real clock.
func TestTimerZeroValue(t *testing.T) {
	t.Parallel()

	var fired atomic.Bool

	var timer hal.Timer

	timer.OnExpire = func(context.Context) { fired.Store(true) }
	timer.Start(context.Background(), 50*time.Millisecond)

	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, fired.Load())
}

func TestTimerStartAndIsRunning(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()

	var fired atomic.Bool

	timer := hal.Timer{
		Clock:    mock,
		OnExpire: func(context.Context) { fired.Store(true) },
	}

	// A fresh timer is not running.
	assert.Assert(t, !timer.IsRunning())
	assert.Assert(t, timer.ExpiresAt().IsZero())

	timer.Start(context.Background(), time.Minute)

	// Running until the mock clock advances past the duration.
	assert.Assert(t, timer.IsRunning())
	assert.Assert(t, timer.ExpiresAt().Equal(mock.Now().Add(time.Minute)))
	assert.Assert(t, !fired.Load())

	mock.Add(time.Minute)

	assert.Assert(t, fired.Load())
	assert.Assert(t, !timer.IsRunning())
}

func TestTimerCancel(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()

	var fired atomic.Bool

	timer := hal.Timer{
		Clock:    mock,
		OnExpire: func(context.Context) { fired.Store(true) },
	}

	// Cancel before start is a no-op.
	timer.Cancel()
	assert.Assert(t, !timer.IsRunning())

	timer.Start(context.Background(), time.Minute)
	assert.Assert(t, timer.IsRunning())

	timer.Cancel()
	assert.Assert(t, !timer.IsRunning())

	// Advancing past the original duration must not fire the cancelled timer.
	mock.Add(2 * time.Minute)
	assert.Assert(t, !fired.Load())
}

// TestTimerRestartUsesLatestContext verifies that restarting a running timer
// fires once, at the new expiry, with the context of the latest start.
func TestTimerRestartUsesLatestContext(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()

	var (
		fired atomic.Int32
		name  atomic.Value
	)

	timer := hal.Timer{
		Clock: mock,
		OnExpire: func(ctx context.Context) {
			fired.Add(1)
			name.Store(hal.GetAutomationNameFromContext(ctx))
		},
	}

	timer.Start(hal.NewAutomationContext("", "first"), time.Minute)
	mock.Add(30 * time.Second)
	timer.Start(hal.NewAutomationContext("", "second"), time.Minute)

	mock.Add(30 * time.Second)
	assert.Equal(t, fired.Load(), int32(0))

	mock.Add(30 * time.Second)
	assert.Equal(t, fired.Load(), int32(1))
	assert.Equal(t, name.Load(), "second")
}

// TestTimerNilOnExpire verifies that a timer without a callback still runs.
func TestTimerNilOnExpire(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()
	timer := hal.Timer{Clock: mock}

	timer.Start(context.Background(), time.Minute)
	assert.Assert(t, timer.IsRunning())

	mock.Add(time.Minute)
	assert.Assert(t, !timer.IsRunning())
}
