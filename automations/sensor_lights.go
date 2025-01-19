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

	// Brightness is the brightness of the lights when they are turned on. We
	// have to set a brightness here so support dimming lights before turn off.
	// The reason is that Home Assistant doesn't support changing the brightness of
	// a light while it is off. Because the light is dimmed prior to turn off,
	// turning it back on would mean it comes back on in a dimmed state. Thus we
	// have to specify a default brightness when turning on to avoid this.
	brightness float64
	scene      map[string]any

	condition              func() bool // optional: func that must return true for the automation to run
	conditionScene         []ConditionScene
	dimLightsBeforeTurnOff time.Duration
	humanOverrideFor       *time.Duration // optional: duration after which lights will turn off after being turned on from outside this system
	sensors                []hal.EntityInterface
	turnsOnLights          []hal.LightInterface
	turnsOffLights         []hal.LightInterface
	turnsOffAfter          *time.Duration // optional: duration after which lights will turn off after being turned on

	dimLightsTimer     hal.Timer
	humanOverrideTimer hal.Timer
	turnOffTimer       hal.Timer
}

func NewSensorsTriggerLights() *SensorsTriggerLights {
	return &SensorsTriggerLights{
		dimLightsBeforeTurnOff: time.Second * 10,
		brightness:             255,
		log:                    slog.Default(),
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

// DimLightsBeforeTurnOff sets the duration before lights will turn off after
// being turned on.
func (a *SensorsTriggerLights) DimLightsBeforeTurnOff(duration time.Duration) *SensorsTriggerLights {
	a.dimLightsBeforeTurnOff = duration

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

	if a.dimLightsBeforeTurnOff < 0 {
		return
	}

	dimLightsAfter := *a.turnsOffAfter - a.dimLightsBeforeTurnOff
	if dimLightsAfter < 1*time.Second {
		return
	}

	a.log.Info("Starting dim lights timer", "duration", dimLightsAfter.String())
	a.dimLightsTimer.Start(a.dimLights, dimLightsAfter)
}

func (a *SensorsTriggerLights) startTurnOffTimer() {
	if a.turnsOffAfter == nil {
		return
	}

	a.log.Info("Starting turn off timer", "duration", a.turnsOffAfter.String())
	a.turnOffTimer.Start(a.turnOffLights, *a.turnsOffAfter)

	a.startDimLightsTimer()
}

func (a *SensorsTriggerLights) stopTurnOffTimer() {
	wasRunning := a.turnOffTimer.IsRunning()

	a.log.Info("Cancelling turn off timer", "wasRunning", wasRunning)

	a.turnOffTimer.Cancel()
}

func (a *SensorsTriggerLights) stopDimLightsTimer() {
	wasRunning := a.dimLightsTimer.IsRunning()

	a.log.Info("Cancelling dim lights timer", "wasRunning", wasRunning)

	a.dimLightsTimer.Cancel()
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

	a.log.Info("Turning on lights", "attributes", attributes)

	for _, light := range a.turnsOnLights {
		if err := light.TurnOn(attributes); err != nil {
			a.log.Error("Error turning on light", "error", err)
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
			a.log.Error("Error dimming light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) turnOffLights() {
	a.log.Info("Turning off lights")

	for _, light := range a.turnsOffLights {
		if err := light.TurnOff(); err != nil {
			a.log.Error("Error turning off light", "error", err)
		}
	}
}

func (a *SensorsTriggerLights) isTurnOnLight(entity hal.EntityInterface) bool {
	for _, light := range a.turnsOnLights {
		if light.GetID() == entity.GetID() {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) isSensor(entity hal.EntityInterface) bool {
	for _, sensor := range a.sensors {
		if sensor.GetID() == entity.GetID() {
			return true
		}
	}

	return false
}

func (a *SensorsTriggerLights) handleSensorStateChange() {
	a.log.Info("Sensor state change")

	if a.humanOverrideTimer.IsRunning() {
		a.log.Info("Light overridden by human, skipping")

		return
	}

	if a.condition != nil && !a.condition() {
		a.log.Info("Condition not met, skipping")

		return
	}

	if a.triggered() {
		lightsWereDimmedFromTimer := a.isLightDimmedFromTimer()

		a.log.Info("Sensor triggered", "lightsWereDimmedFromTimer", lightsWereDimmedFromTimer)

		a.stopTurnOffTimer()
		a.stopDimLightsTimer()

		// This avoids a situation where the user has changed the lights state
		// but it gets overridden by a sensor being triggered again.
		if a.lightsOn() && !lightsWereDimmedFromTimer {
			a.log.Info("Sensor triggered, but lights are already on, ignoring")

			return
		}

		a.log.Info("Sensor triggered, turning on lights")
		a.turnOnLights()
	} else {
		a.log.Info("Sensor cleared, starting turn off countdown")
		a.startTurnOffTimer()
	}
}

func (a *SensorsTriggerLights) handleLightStateChanged() {
	a.log.Info("Light state change")

	// Light was either turned on or off, or brightness changed or whatever,
	// in which case we want to stop any further automations since the user has
	// overridden it and we want to respect that.
	a.stopDimLightsTimer()
	a.stopTurnOffTimer()

	if a.humanOverrideFor != nil {
		if a.lightsOn() {
			a.log.Info("Light turned on, setting human override", "duration", a.humanOverrideFor.String())
			a.humanOverrideTimer.Start(nil, *a.humanOverrideFor)
		} else {
			a.log.Info("Light turned off, cancelling human override")
			a.humanOverrideTimer.Cancel()
		}
	}
}

// isLightDimmedFromTimer returns true if the lights are dimmed from the timer.
func (a *SensorsTriggerLights) isLightDimmedFromTimer() bool {
	return a.dimLightsBeforeTurnOff > 0 && !a.dimLightsTimer.IsRunning() && a.turnOffTimer.IsRunning()
}

func (a *SensorsTriggerLights) Action(triggerEntity hal.EntityInterface) {
	a.log.Info("Automation triggered with event", "state", triggerEntity.GetState())

	if a.isSensor(triggerEntity) {
		a.handleSensorStateChange()
	} else if a.isTurnOnLight(triggerEntity) {
		a.handleLightStateChanged()
	}
}

func (a *SensorsTriggerLights) Entities() hal.Entities {
	entities := []hal.EntityInterface{}
	entities = append(entities, a.sensors...)

	for _, light := range a.turnsOnLights {
		entities = append(entities, light)
	}

	return hal.Entities(entities)
}

func (a *SensorsTriggerLights) Name() string {
	return a.name
}
