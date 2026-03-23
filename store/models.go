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
	MetricType     MetricType `gorm:"index:idx_metric_type_timestamp,priority:1;not null;size:50"`
	Timestamp      time.Time  `gorm:"index:idx_metric_type_timestamp,priority:2;not null"`
	Value          int64      `gorm:"not null"` // For counters: 1, for timers: nanoseconds
	EntityID       string     `gorm:"size:100"` // Optional: which entity triggered this
	AutomationName string     `gorm:"size:100"` // Optional: which automation was involved
}

// Log represents a single log entry
type Log struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Timestamp time.Time `gorm:"index;not null"`
	Level     string    `gorm:"index;not null;size:10"` // Log level: DEBUG, INFO, WARN, ERROR
	EntityID  string    `gorm:"index;size:255"`         // Optional: which entity this log relates to
	LogText   string    `gorm:"not null;type:text"`
}
