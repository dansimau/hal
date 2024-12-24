package halautomations

import (
	"log/slog"
	"time"

	"github.com/dansimau/hal"
)

type ConditionScene struct {
	Condition func() bool
	Scene     map[string]any
}

// SensorsTriggerLights is an automation that combines one or more sensors
// (motion or presence sensors) and a set of lights. Lights are turned on when
// any of the sensors are triggered and turned off after a given duration.
type SensorsTriggerLights struct {
	name string
	log  *slog.Logger

	condition      func() bool // optional: func that must return true for the automation to run
	conditionScene []ConditionScene
	sensors        []hal.EntityInterface
	turnsOnLights  []hal.LightInterface
	turnsOffLights []hal.LightInterface
	turnsOffAfter  *time.Duration // optional: duration after which lights will turn off after being turned on

	turnOffTimer *time.Timer
}

func NewSensorsTriggerLights() *SensorsTriggerLights {
	return &SensorsTriggerLights{
		log: slog.Default(),
	}
}

// WithCondition sets a condition that must be true for the automation to run.
func (a *SensorsTriggerLights) WithCondition(condition func() bool) *SensorsTriggerLights {
	a.condition = condition

	return a
}

// WithConditionScene allows you to specify a scene to trigger based on a condition.
func (a *SensorsTriggerLights) WithConditionScene(condition func() bool, scene map[string]any) *SensorsTriggerLights {
	a.conditionScene = append(a.conditionScene, ConditionScene{
		Condition: condition,
		Scene:     scene,
	})

	return a
}

// WithLights sets the lights that will be turned on and off. Overrides
// TurnsOnLights and TurnsOffLights.
func (a *SensorsTriggerLights) WithLights(lights ...hal.LightInterface) *SensorsTriggerLights {
	a.turnsOnLights = lights
	a.turnsOffLights = lights

	return a
}

// WithName sets the name of the automation (appears in logs).
func (a *SensorsTriggerLights) WithName(name string) *SensorsTriggerLights {
	a.name = name
	a.log = slog.With("automation", a.name)

	return a
}

// WithSensors sets the sensors that will trigger the lights.
func (a *SensorsTriggerLights) WithSensors(sensors ...hal.EntityInterface) *SensorsTriggerLights {
	a.sensors = sensors

	return a
}

// TurnsOnLights sets the lights that will be turned on by the sensor. This can
// be used in conjunction with TurnsOffLights to turn on and off different sets
// of lights.
func (a *SensorsTriggerLights) TurnsOnLights(lights ...hal.LightInterface) *SensorsTriggerLights {
	a.turnsOnLights = lights

	return a
}

// TurnsOffLights sets the lights that will be turned off by the sensor. This can
// be used in conjunction with TurnsOnLights to turn on and off different sets
// of lights.
func (a *SensorsTriggerLights) TurnsOffLights(lights ...hal.LightInterface) *SensorsTriggerLights {
	a.turnsOffLights = lights

	return a
}

// TurnsOffAfter sets the duration after which the lights will turn off after being
// turned on.
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
	var attributes map[string]any

	for _, conditionScene := range a.conditionScene {
		if conditionScene.Condition() {
			attributes = conditionScene.Scene
		}
	}

	for _, light := range a.turnsOnLights {
		if err := light.TurnOn(attributes); err != nil {
			slog.Error("Error turning on light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) turnOffLights() {
	a.log.Info("Turning off lights")

	for _, light := range a.turnsOffLights {
		if err := light.TurnOff(); err != nil {
			slog.Error("Error turning off light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) Action() {
	if a.condition != nil && !a.condition() {
		a.log.Info("Condition not met, skipping")

		return
	}

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
