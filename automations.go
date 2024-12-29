package hal

type Automation interface {
	// Name is a friendly name for the automation, used in logs and stats.
	Name() string

	// Entities should return a list of entities that this automation should
	// listen on. For every state change to one of these entities, the
	// automation will be trigged.
	Entities() Entities

	// Action is called when the automation is triggered.
	Action(trigger EntityInterface)
}

type AutomationConfig struct {
	action   func(trigger EntityInterface)
	entities Entities
	name     string
}

func NewAutomation() *AutomationConfig {
	return &AutomationConfig{}
}

func (c *AutomationConfig) Entities() Entities {
	return c.entities
}

func (c *AutomationConfig) Action(trigger EntityInterface) {
	c.action(trigger)
}

func (c *AutomationConfig) Name() string {
	return c.name
}

func (c *AutomationConfig) WithAction(action func(trigger EntityInterface)) *AutomationConfig {
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
