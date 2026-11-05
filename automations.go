package hal

import "context"

type Automation interface {
	// Name is a friendly name for the automation, used in logs and stats.
	Name() string

	// Entities should return a list of entities that this automation should
	// listen on. For every state change to one of these entities, the
	// automation will be trigged.
	Entities() Entities

	// Action is called when the automation is triggered with context for tracing.
	Action(ctx context.Context, trigger EntityInterface)
}

type AutomationConfig struct {
	action   func(ctx context.Context, trigger EntityInterface)
	entities Entities
	name     string
	timers   []*Timer
}

func NewAutomation() *AutomationConfig {
	return &AutomationConfig{}
}

func (c *AutomationConfig) Entities() Entities {
	return c.entities
}

func (c *AutomationConfig) Action(ctx context.Context, trigger EntityInterface) {
	c.action(ctx, trigger)
}

func (c *AutomationConfig) Name() string {
	return c.name
}

func (c *AutomationConfig) WithAction(action func(ctx context.Context, trigger EntityInterface)) *AutomationConfig {
	c.action = action

	if c.name == "" {
		c.name = getShortFunctionName(action)
	}

	return c
}

func (c *AutomationConfig) WithEntities(entities ...EntityInterface) *AutomationConfig {
	c.entities = entities

	return c
}

func (c *AutomationConfig) WithName(name string) *AutomationConfig {
	c.name = name

	return c
}

// WithTimer initialises timer with the Name, OnExpire and Clock of config and
// declares it as one of the automation's timers, so that it is saved to the
// database and restored after a restart. The timer is typically a field of the
// struct the automation belongs to:
//
//	type Porch struct {
//		Light *hal.Light
//		// ...
//		lightsOffTimer hal.Timer
//	}
//
//	hal.NewAutomation().
//		WithTimer(&porch.lightsOffTimer, hal.Timer{
//			Name: "PorchLightsOff",
//			OnExpire: func(ctx context.Context) {
//				porch.Light.TurnOffContext(ctx)
//			},
//		}).
//		WithAction(func(ctx context.Context, _ hal.EntityInterface) {
//			porch.lightsOffTimer.Start(ctx, 10*time.Minute)
//		})
func (c *AutomationConfig) WithTimer(timer *Timer, config Timer) *AutomationConfig {
	timer.Name = config.Name
	timer.OnExpire = config.OnExpire
	timer.Clock = config.Clock
	c.timers = append(c.timers, timer)

	return c
}

// Timers returns the automation's timers; see TimerProvider.
func (c *AutomationConfig) Timers() []*Timer {
	return c.timers
}
