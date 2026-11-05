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

	ID    string               `gorm:"primaryKey"`
	State *homeassistant.State `gorm:"serializer:json"`
}

// TimerStatus is the last known state of a durable timer.
type TimerStatus string

// TimerStatus constants.
const (
	// TimerStatusRunning means the timer is counting down and will fire at
	// ExpiresAt. A running timer is restored when hal restarts.
	TimerStatusRunning TimerStatus = "running"
	// TimerStatusExpired means the timer fired at ExpiresAt.
	TimerStatusExpired TimerStatus = "expired"
	// TimerStatusCancelled means the timer was cancelled before it fired.
	TimerStatusCancelled TimerStatus = "cancelled"
)

// Timer is the saved state of a durable automation timer (see
// hal.DurableTimer). It is written whenever the timer starts, fires or is
// cancelled, so that timers still running at shutdown can be restored on the
// next start.
type Timer struct {
	Model

	// Name is the automation name and timer name joined with "/".
	Name      string      `gorm:"primaryKey"`
	Status    TimerStatus `gorm:"not null;size:20"`
	StartedAt time.Time   `gorm:"not null"` // when the timer was last started
	ExpiresAt time.Time   `gorm:"not null"` // when the timer fires, or fired or was cancelled
}

// MetricType represents the type of metric being recorded.
type MetricType string

// MetricType constants.
const (
	MetricTypeAutomationTriggered MetricType = "automation_triggered"
	MetricTypeTickProcessingTime  MetricType = "tick_processing_time"
)

// Metric represents a single metric data point.
type Metric struct {
	ID             uint       `gorm:"primaryKey;autoIncrement"`
	MetricType     MetricType `gorm:"index:idx_metric_type_timestamp,priority:1;not null;size:50"`
	Timestamp      time.Time  `gorm:"index:idx_metric_type_timestamp,priority:2;index:idx_metric_timestamp;not null"`
	Value          int64      `gorm:"not null"` // For counters: 1, for timers: nanoseconds
	EntityID       string     `gorm:"size:100"` // Optional: which entity triggered this
	AutomationName string     `gorm:"size:100"` // Optional: which automation was involved
}

// MetricRollup represents a pre-aggregated, per-minute summary of raw metrics.
// Rollups are kept far longer than raw metrics (which are pruned aggressively),
// keeping long-range queries (last day/month) fast and storage small.
type MetricRollup struct {
	ID         uint       `gorm:"primaryKey;autoIncrement"`
	MetricType MetricType `gorm:"uniqueIndex:idx_rollup_type_bucket,priority:1;not null;size:50"`
	// BucketStart is the Unix timestamp (seconds) of the start of the one-minute
	// bucket. Using an integer minute boundary makes bucketing identical whether
	// computed in Go or in SQL, so upserts conflict-match reliably.
	BucketStart int64 `gorm:"uniqueIndex:idx_rollup_type_bucket,priority:2;index:idx_rollup_bucket_start;not null"`
	Count       int64 `gorm:"not null"` // number of raw samples in the bucket
	Sum         int64 `gorm:"not null"` // sum of raw values in the bucket
	// Histogram is a log-scale histogram of the raw values in the bucket, keyed by
	// HistogramBucket index (see store/histogram.go). Because bucket boundaries are
	// fixed across all time, per-minute histograms sum exactly, so a quantile
	// computed over any range of rollups is a true quantile of the underlying
	// samples, accurate to a bounded relative error.
	Histogram map[int32]int64 `gorm:"serializer:json"`
}

// Log represents a single log entry.
type Log struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Timestamp time.Time `gorm:"index;not null"`
	Level     string    `gorm:"index;not null;size:10"` // Log level: DEBUG, INFO, WARN, ERROR
	EntityID  string    `gorm:"index;size:255"`         // Optional: which entity this log relates to
	LogText   string    `gorm:"not null;type:text"`
}
