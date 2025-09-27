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

// MetricType represents the type of metric being recorded
type MetricType string

// MetricType constants
const (
	MetricTypeAutomationTriggered MetricType = "automation_triggered"
	MetricTypeTickProcessingTime  MetricType = "tick_processing_time"
)

// Metric represents a single metric data point
type Metric struct {
	ID             uint       `gorm:"primaryKey;autoIncrement"`
	Timestamp      time.Time  `gorm:"index;not null"`
	MetricType     MetricType `gorm:"not null;size:50"`
	Value          int64      `gorm:"not null"` // For counters: 1, for timers: nanoseconds
	EntityID       string     `gorm:"size:100"` // Optional: which entity triggered this
	AutomationName string     `gorm:"size:100"` // Optional: which automation was involved
}
