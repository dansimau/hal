package halautomations_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dansimau/hal"
	halautomations "github.com/dansimau/hal/automations"
	"gotest.tools/v3/assert"
)

func TestTimerBuilder(t *testing.T) {
	t.Parallel()

	entity := hal.NewEntity("test.entity")

	timer := halautomations.NewTimer("test timer").
		Duration(time.Millisecond).
		WithEntities(entity).
		Run(func(_ context.Context) {})

	assert.Equal(t, timer.Name(), "test timer")
	assert.Equal(t, len(timer.Entities()), 1)
	assert.Equal(t, timer.Entities()[0].GetID(), "test.entity")
}

func TestTimerRunsActionAfterDelay(t *testing.T) {
	t.Parallel()

	var ran atomic.Bool

	timer := halautomations.NewTimer("test timer").
		Duration(10 * time.Millisecond).
		Run(func(_ context.Context) {
			ran.Store(true)
		})

	// Action with a passing (nil) condition set starts the timer.
	timer.Action(context.Background(), hal.NewEntity("test.entity"))

	assert.NilError(t, waitForBool(&ran, time.Second))
}

func TestTimerConditionBlocksStart(t *testing.T) {
	t.Parallel()

	var ran atomic.Bool

	timer := halautomations.NewTimer("test timer").
		Duration(10 * time.Millisecond).
		Condition(func() bool { return false }).
		Run(func(_ context.Context) {
			ran.Store(true)
		})

	// Condition is false, so Action should stop (never start) the timer.
	timer.Action(context.Background(), hal.NewEntity("test.entity"))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, ran.Load(), false)
}

func TestTimerConditionRecheckedBeforeAction(t *testing.T) {
	t.Parallel()

	var (
		ran     atomic.Bool
		allowed atomic.Bool
	)

	allowed.Store(true)

	timer := halautomations.NewTimer("test timer").
		Duration(10 * time.Millisecond).
		Condition(func() bool { return allowed.Load() }).
		Run(func(_ context.Context) {
			ran.Store(true)
		})

	// Condition passes, so the timer starts...
	timer.Action(context.Background(), hal.NewEntity("test.entity"))

	// ...but flips to false before it fires, so runAction must bail out.
	allowed.Store(false)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, ran.Load(), false)
}

// waitForBool polls the flag until it is true or the timeout elapses.
func waitForBool(flag *atomic.Bool, timeout time.Duration) error {
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			return context.DeadlineExceeded
		default:
			if flag.Load() {
				return nil
			}

			time.Sleep(time.Millisecond)
		}
	}
}
