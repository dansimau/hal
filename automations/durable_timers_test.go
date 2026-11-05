package halautomations_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dansimau/hal"
	halautomations "github.com/dansimau/hal/automations"
	"gotest.tools/v3/assert"
)

// TestSensorsTriggerLightsTimers verifies that every timer of the automation
// is declared, named after the automation, with a callback.
func TestSensorsTriggerLightsTimers(t *testing.T) {
	t.Parallel()

	var provider hal.TimerProvider = halautomations.NewSensorsTriggerLights().WithName("kitchen")

	timers := provider.Timers()
	assert.Equal(t, len(timers), 4)

	names := make(map[string]bool)

	for _, timer := range timers {
		assert.Assert(t, timer != nil)
		assert.Assert(t, strings.HasPrefix(timer.Name, "kitchen/"), "timer %q is not named after the automation", timer.Name)
		assert.Assert(t, !names[timer.Name], "duplicate timer name %q", timer.Name)
		names[timer.Name] = true
	}

	assert.Assert(t, names["kitchen/turn_off"])
}

// TestTimerAutomationTimers verifies that the Timer automation's countdown is
// declared and that its callback runs the action.
func TestTimerAutomationTimers(t *testing.T) {
	t.Parallel()

	var ran atomic.Bool

	var provider hal.TimerProvider = halautomations.NewTimer("test timer").
		Duration(time.Minute).
		Run(func(context.Context) { ran.Store(true) })

	timers := provider.Timers()
	assert.Equal(t, len(timers), 1)
	assert.Equal(t, timers[0].Name, "test timer/delay")

	timers[0].OnExpire(context.Background())
	assert.Assert(t, ran.Load())
}

// TestTimerAutomationWithClock verifies the automation's countdown follows the
// given clock, restarting whenever it is triggered.
func TestTimerAutomationWithClock(t *testing.T) {
	t.Parallel()

	mock := clock.NewMock()

	var ran atomic.Bool

	timer := halautomations.NewTimer("test timer").
		WithClock(mock).
		Duration(time.Minute).
		Run(func(context.Context) { ran.Store(true) })

	entity := hal.NewEntity("test.entity")

	timer.Action(context.Background(), entity)
	mock.Add(30 * time.Second)

	// Triggering again restarts the countdown.
	timer.Action(context.Background(), entity)
	mock.Add(59 * time.Second)
	assert.Assert(t, !ran.Load())

	mock.Add(time.Second)
	assert.Assert(t, ran.Load())
}
