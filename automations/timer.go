package halautomations

import (
	"context"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dansimau/hal"
	"github.com/dansimau/hal/logger"
)

// Timer is an automation that runs an action once its conditions have held for
// a set duration, restarting the countdown whenever a watched entity changes.
// The countdown survives a restart of hal (see hal.TimerProvider).
type Timer struct {
	action     func(ctx context.Context)
	conditions []func() bool
	delay      time.Duration
	entities   hal.Entities
	name       string
	timer      hal.Timer
}

func NewTimer(name string) *Timer {
	a := &Timer{
		name: name,
	}

	a.timer.OnExpire = a.elapsed

	return a
}

// WithClock can be used to pass in a mock clock for testing.
func (a *Timer) WithClock(c clock.Clock) *Timer {
	a.timer.Clock = c

	return a
}

// Condition sets a condition that must be true for the timer to start.
func (a *Timer) Condition(condition func() bool) *Timer {
	a.conditions = append(a.conditions, condition)

	return a
}

// Duration sets the duration of the delay.
func (a *Timer) Duration(duration time.Duration) *Timer {
	a.delay = duration

	return a
}

// WithEntities sets the entities that trigger or reset the timer.
func (a *Timer) WithEntities(entities ...hal.EntityInterface) *Timer {
	a.entities = entities

	return a
}

// Run sets the action to be run after the delay.
func (a *Timer) Run(action func(ctx context.Context)) *Timer {
	a.action = action

	return a
}

func (a *Timer) Name() string {
	return a.name
}

func (a *Timer) Entities() hal.Entities {
	return a.entities
}

// Timers returns the countdown timer so that the connection saves it and
// restores it after a restart (see hal.TimerProvider). It is named after the
// automation.
func (a *Timer) Timers() []*hal.Timer {
	a.timer.Name = a.name + "/delay"

	return []*hal.Timer{&a.timer}
}

func (a *Timer) Action(ctx context.Context, _ hal.EntityInterface) {
	if a.checkConditions(ctx) {
		a.startTimer(ctx)
	} else {
		a.stopTimer(ctx)
	}
}

func (a *Timer) checkConditions(ctx context.Context) bool {
	for i, condition := range a.conditions {
		if !condition() {
			logger.InfoContext(ctx, "Timer condition not met", "condition", i)

			return false
		}
	}

	return true
}

// startTimer starts the timer, or restarts it with the latest context if it
// is already running.
func (a *Timer) startTimer(ctx context.Context) {
	logger.InfoContext(ctx, "Starting timer")

	a.timer.Start(ctx, a.delay)
}

// stopTimer stops the timer.
func (a *Timer) stopTimer(ctx context.Context) {
	logger.InfoContext(ctx, "Stopping timer")

	a.timer.Cancel()
}

func (a *Timer) elapsed(ctx context.Context) {
	logger.InfoContext(ctx, "Timer elapsed")
	a.runAction(ctx)
}

func (a *Timer) runAction(ctx context.Context) {
	// Check conditions once more before executing action
	if !a.checkConditions(ctx) {
		return
	}

	logger.InfoContext(ctx, "Executing action")
	a.action(ctx)
}
