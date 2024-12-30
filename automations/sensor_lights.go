package halautomations

import (
	"log/slog"
	"time"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/timerutil"
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

	// Brightness is the brightness of the lights when they are turned on. We
	// have to set a brightness here so support dimming lights before turn off.
	// The reason is that Home Assistant doesn't support changing the brightness of
	// a light while it is off. Because the light is dimmed prior to turn off,
	// turning it back on would mean it comes back on in a dimmed state. Thus we
	// have to specify a default brightness when turning on to avoid this.
	brightness float64
	scene      map[string]any

	condition        func() bool // optional: func that must return true for the automation to run
	conditionScene   []ConditionScene
	humanOverrideFor *time.Duration // optional: duration after which lights will turn off after being turned on from outside this system
	sensors          []hal.EntityInterface
	turnsOnLights    []hal.LightInterface
	turnsOffLights   []hal.LightInterface
	turnsOffAfter    *time.Duration // optional: duration after which lights will turn off after being turned on

	dimLightsTimer     *time.Timer
	humanOverrideTimer *timerutil.Timer
	turnOffTimer       *time.Timer
}

func NewSensorsTriggerLights() *SensorsTriggerLights {
	return &SensorsTriggerLights{
		brightness: 255,
		log:        slog.Default(),
	}
}

// WithBrightness sets the brightness of the lights when they are turned on.
func (a *SensorsTriggerLights) WithBrightness(brightness float64) *SensorsTriggerLights {
	a.brightness = brightness

	return a
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

// WithHumanOverrideFor sets a secondary timer that will kick in if the light
// was turned on from outside this system.
func (a *SensorsTriggerLights) WithHumanOverrideFor(duration time.Duration) *SensorsTriggerLights {
	a.humanOverrideFor = &duration

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

func (a *SensorsTriggerLights) SetScene(scene map[string]any) *SensorsTriggerLights {
	a.scene = scene

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

func (a *SensorsTriggerLights) lightsOn() bool {
	for _, light := range a.turnsOnLights {
		if light.GetState().State == "on" {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) startDimLightsTimer() {
	if a.turnsOffAfter == nil {
		return
	}

	// TODO: Make this configurable
	dimLightsAfter := *a.turnsOffAfter - 10*time.Second
	if dimLightsAfter < 1*time.Second {
		return
	}

	if a.dimLightsTimer == nil {
		a.dimLightsTimer = time.AfterFunc(dimLightsAfter, a.dimLights)
	} else {
		a.dimLightsTimer.Reset(dimLightsAfter)
	}
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

	a.startDimLightsTimer()
}

func (a *SensorsTriggerLights) stopTurnOffTimer() {
	if a.turnOffTimer != nil {
		a.turnOffTimer.Stop()
	}
}

func (a *SensorsTriggerLights) stopDimLightsTimer() {
	if a.dimLightsTimer != nil {
		a.dimLightsTimer.Stop()
		// TODO: Detect if the timer was actually running and if so print a log
		// message saying we stopped the timer.
	}
}

func (a *SensorsTriggerLights) turnOnLights() {
	var attributes map[string]any

	// If a scene is set, use it.
	if a.scene != nil {
		attributes = a.scene
	}

	// If a condition scene matches use that
	for _, conditionScene := range a.conditionScene {
		if conditionScene.Condition() {
			attributes = conditionScene.Scene
		}
	}

	// Otherwise use the default brightness.
	if attributes == nil {
		attributes = map[string]any{"brightness": a.brightness}
	}

	for _, light := range a.turnsOnLights {
		if err := light.TurnOn(attributes); err != nil {
			slog.Error("Error turning on light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) dimLights() {
	a.log.Info("Dimming lights prior to turning off")

	for _, light := range a.turnsOffLights {
		brightness := light.GetBrightness()
		if brightness < 2 {
			a.log.Info("Light is already at minimum brightness, skipping dimming", "light", light.GetID())

			continue
		}

		dimmedBrightness := brightness / 2

		if err := light.TurnOn(map[string]any{"brightness": dimmedBrightness}); err != nil {
			slog.Error("Error dimming light", "error", err)
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

func (a *SensorsTriggerLights) lightTriggered(trigger hal.EntityInterface) bool {
	for _, light := range a.turnsOnLights {
		if light.GetID() == trigger.GetID() {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) sensorTriggered(trigger hal.EntityInterface) bool {
	for _, sensor := range a.sensors {
		if sensor.GetID() == trigger.GetID() {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) handleSensorTriggered() {
	if a.humanOverrideTimer != nil && a.humanOverrideTimer.IsRunning() {
		a.log.Info("Light overridden by human, skipping")

		return
	}

	if a.condition != nil && !a.condition() {
		a.log.Info("Condition not met, skipping")

		return
	}

	if a.triggered() {
		a.log.Info("Sensor triggered, turning on lights")
		a.stopTurnOffTimer()
		a.stopDimLightsTimer()
		a.turnOnLights()
	} else {
		a.log.Info("Sensor cleared, starting turn off countdown")
		a.startTurnOffTimer()
	}
}

func (a *SensorsTriggerLights) handleLightTriggered() {
	a.stopDimLightsTimer()

	if a.humanOverrideFor != nil {
		a.stopTurnOffTimer()

		if a.lightsOn() {
			a.humanOverrideTimer.Reset(*a.humanOverrideFor)
		} else {
			a.humanOverrideTimer.Stop()
		}
	}
}

func (a *SensorsTriggerLights) Action(trigger hal.EntityInterface) {
	if a.sensorTriggered(trigger) {
		a.handleSensorTriggered()
	} else if a.lightTriggered(trigger) {
		a.handleLightTriggered()
	}
}

func (a *SensorsTriggerLights) Entities() hal.Entities {
	return hal.Entities(a.sensors)
}

func (a *SensorsTriggerLights) Name() string {
	return a.name
}
