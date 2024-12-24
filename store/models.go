package store

import (
	"time"

	"github.com/dansimau/hal/homeassistant"
)

type Model struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Entity struct {
	Model

	ID    string `gorm:"primaryKey"`
	Type  string
	State *homeassistant.State `gorm:"serializer:json"`
}
