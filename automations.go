package hal

type Automation interface {
	Name() string
	Entities() Entities
	Action()
}
