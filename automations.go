package hal

type Automation interface {
	// Name is a friendly name for the automation, used in logs and stats.
	Name() string

	// Entities should return a list of entities that this automation should
	// listen on. For every state change to one of these entities, the
	// automation will be trigged.
	Entities() Entities

	// Action is called when the automation is triggered.
	Action()
}
