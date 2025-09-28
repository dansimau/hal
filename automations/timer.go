package halautomations

import (
	"time"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/logger"
)

type Timer struct {
	action     func()
	conditions []func() bool
	delay      time.Duration
	entities   hal.Entities
	name       string
	timer      *time.Timer
}

func NewTimer(name string) *Timer {
	return &Timer{
		name: name,
	}
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
func (a *Timer) Run(action func()) *Timer {
	a.action = action

	return a
}

// startTimer starts the timer.
func (a *Timer) startTimer() {
	logger.Info("Starting timer", "", "automation", a.name)

	if a.timer == nil {
		a.timer = time.AfterFunc(a.delay, a.runAction)
	} else {
		a.timer.Reset(a.delay)
	}
}

// stopTimer stops the timer.
func (a *Timer) stopTimer() {
	if a.timer != nil {
		a.timer.Stop()
	}
}

func (a *Timer) runAction() {
	logger.Info("Timer elapsed, executing action", "", "automation", a.name)

	a.action()
}

func (a *Timer) Name() string {
	return a.name
}

func (a *Timer) Entities() hal.Entities {
	return a.entities
}

func (a *Timer) Action(_ hal.EntityInterface) {
	for i, condition := range a.conditions {
		if !condition() {
			logger.Info("Timer condition not met, stopping existing timer", "", "automation", a.name, "condition", i)
			a.stopTimer()
			return
		}
	}

	a.startTimer()
}
