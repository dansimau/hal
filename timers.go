package hal

import (
	"github.com/dansimau/hal/logger"
	"github.com/dansimau/hal/store"
)

// TimerProvider is implemented by automations that have timers, so that the
// connection can save their state and restore them after a restart.
// RegisterAutomations registers the timers of every automation that implements
// it. Automations built with NewAutomation declare their timers with
// WithTimer; custom automation types implement this directly.
type TimerProvider interface {
	Timers() []*Timer
}

// registeredTimer is a timer registered with a connection.
type registeredTimer struct {
	timer      *Timer
	automation string
}

// registerTimers registers the timers of an automation, if it has any, so
// their state is saved whenever it changes and restored on the next start.
func (h *Connection) registerTimers(automation Automation) {
	provider, ok := automation.(TimerProvider)
	if !ok {
		return
	}

	for _, timer := range provider.Timers() {
		if timer == nil {
			logger.Error("Automation declared a nil timer", "", "automation", automation.Name())

			continue
		}

		if timer.Name == "" {
			logger.Error("Timer has no name, it will not be saved or restored", "", "automation", automation.Name())

			continue
		}

		if existing, ok := h.timers[timer.Name]; ok {
			if existing.timer != timer {
				logger.Error("Duplicate timer name, timer will not be saved or restored", "",
					"timer", timer.Name, "automation", automation.Name())
			}

			continue
		}

		timer.bind(h.db.EnqueueTimerState)
		h.timers[timer.Name] = registeredTimer{timer: timer, automation: automation.Name()}
	}
}

// restoreTimers restarts every registered timer that was running when its
// state was last saved. It runs once, on the first successful connection, so
// restored timers that fire immediately can act on Home Assistant. It must be
// called with the state lock held so that no event is dispatched to an
// automation before its timers are back in place.
func (h *Connection) restoreTimers() {
	if h.timersRestored {
		return
	}

	h.timersRestored = true

	saved, err := h.db.Timers()
	if err != nil {
		logger.Error("Failed to load saved timers, not restoring", "", "error", err)

		return
	}

	for _, state := range saved {
		if state.Status != store.TimerStatusRunning {
			continue
		}

		timer, ok := h.timers[state.Name]
		if !ok {
			logger.Warn("Saved timer is not registered, not restoring", "", "timer", state.Name)

			continue
		}

		ctx := NewAutomationContext("", timer.automation)
		logger.InfoContext(ctx, "Restoring timer", "timer", state.Name, "expiresAt", state.ExpiresAt)
		timer.timer.restore(ctx, state)
	}
}

// syncTimers queues the current state of every registered timer for writing,
// so that the database reflects the timers exactly as they are at shutdown.
func (h *Connection) syncTimers() {
	for _, timer := range h.timers {
		timer.timer.sync()
	}
}
