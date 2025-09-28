package hal

import (
	"time"
)

// buttonPressTimeout is the amount of time to listen for repeat presses.
const buttonPressTimeout = 2 * time.Second

// Button is an event entity that represents a button.
type Button struct {
	*Entity

	lastPressed  time.Time
	pressedTimes int32
}

func NewButton(id string) *Button {
	return &Button{Entity: NewEntity(id)}
}

func (b *Button) Name() string {
	return b.GetID()
}

func (b *Button) Entities() Entities {
	return Entities{b}
}

func (b *Button) Action(_ EntityInterface) {
	if b.Entity.GetState().Attributes["event_type"] != "initial_press" {
		return
	}

	if time.Since(b.lastPressed) < buttonPressTimeout {
		b.pressedTimes++
	} else {
		b.pressedTimes = 1
	}

	entityID := b.GetID()
	b.connection.loggingService.Info("Button pressed", &entityID, "entity", entityID, "times", b.pressedTimes)

	b.lastPressed = time.Now()
}

func (b *Button) PressedTimes() int32 {
	return b.pressedTimes
}
