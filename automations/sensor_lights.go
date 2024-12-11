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
	log  *slog.Logger

	sensors       []hal.EntityInterface
	lights        []hal.LightInterface
	turnsOffAfter *time.Duration

	turnOffTimer *time.Timer
}

func NewSensorsTriggersLights() *SensorsTriggerLights {
	return &SensorsTriggerLights{
		log: slog.Default(),
	}
}

func (a *SensorsTriggerLights) WithName(name string) *SensorsTriggerLights {
	a.name = name
	a.log = slog.With("automation", a.name)

	return a
}

func (a *SensorsTriggerLights) WithSensors(sensors ...hal.EntityInterface) *SensorsTriggerLights {
	a.sensors = sensors

	return a
}

func (a *SensorsTriggerLights) WithLights(lights ...hal.LightInterface) *SensorsTriggerLights {
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
	a.log.Info("Turning off lights")

	for _, light := range a.lights {
		if err := light.TurnOff(); err != nil {
			slog.Error("Error turning off light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) Action() {
	if a.triggered() {
		a.log.Info("Sensor triggered, turning on lights")
		a.stopTurnOffTimer()
		a.turnOnLights()
	} else {
		a.log.Info("Sensor cleared, starting turn off countdown")
		a.startTurnOffTimer()
	}
}

func (a *SensorsTriggerLights) Entities() hal.Entities {
	return hal.Entities(a.sensors)
}

func (a *SensorsTriggerLights) Name() string {
	return a.name
}
