package halautomations

import (
	"log/slog"
	"time"

	"github.com/dansimau/hal"
)

// SensorsTriggerLights is an automation that combines one or more sensors
// (motion or presence sensors) and a set of lights. Lights are turned on when
// any of the sensors are triggered and turned off after a given duration.
type SensorsTriggerLights struct {
	name string

	sensors       []hal.EntityLike
	lights        []*hal.Light
	turnsOffAfter *time.Duration

	turnOffTimer *time.Timer
}

func NewSensorsTriggersLights() *SensorsTriggerLights {
	return &SensorsTriggerLights{}
}

func (a *SensorsTriggerLights) WithName(name string) *SensorsTriggerLights {
	a.name = name

	return a
}

func (a *SensorsTriggerLights) WithSensors(sensors ...hal.EntityLike) *SensorsTriggerLights {
	a.sensors = sensors

	return a
}

func (a *SensorsTriggerLights) WithLights(lights ...*hal.Light) *SensorsTriggerLights {
	a.lights = lights

	return a
}

func (a *SensorsTriggerLights) TurnsOffAfter(turnsOffAfter time.Duration) *SensorsTriggerLights {
	a.turnsOffAfter = &turnsOffAfter

	return a
}

// triggered returns true if any of the sensors have been triggered.
func (a *SensorsTriggerLights) triggered() bool {
	for _, sensor := range a.sensors {
		if sensor.GetState().State == "on" {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) startTurnOffTimer() {
	if a.turnsOffAfter == nil {
		return
	}

	if a.turnOffTimer == nil {
		a.turnOffTimer = time.AfterFunc(*a.turnsOffAfter, a.turnOffLights)
	} else {
		a.turnOffTimer.Reset(*a.turnsOffAfter)
	}
}

func (a *SensorsTriggerLights) stopTurnOffTimer() {
	if a.turnOffTimer != nil {
		a.turnOffTimer.Stop()
	}
}

func (a *SensorsTriggerLights) turnOnLights() {
	for _, light := range a.lights {
		if err := light.TurnOn(); err != nil {
			slog.Error("Error turning on light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) turnOffLights() {
	for _, light := range a.lights {
		if err := light.TurnOff(); err != nil {
			slog.Error("Error turning off light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) Action() {
	if a.triggered() {
		a.stopTurnOffTimer()
		a.turnOnLights()
	} else {
		a.startTurnOffTimer()
	}
}

func (a *SensorsTriggerLights) Entities() hal.Entities {
	return hal.Entities(a.sensors)
}

func (a *SensorsTriggerLights) Name() string {
	return a.name
}
