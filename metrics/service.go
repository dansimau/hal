package metrics

import (
	"time"

	"github.com/dansimau/hal/logger"
	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
)

// Service handles metrics collection and pruning
// Writes directly to SQLite, leveraging SQLite's WAL mode for performance
type Service struct {
	db            *gorm.DB
	pruneInterval time.Duration // How often to prune old metrics (default: daily)
	retentionTime time.Duration // How long to keep metrics (default: 3 months)
	stopChan      chan struct{}
}

// NewService creates a new metrics service
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:            db,
		pruneInterval: 24 * time.Hour,     // Prune daily
		retentionTime: 90 * 24 * time.Hour, // Keep 3 months of metrics
		stopChan:      make(chan struct{}),
	}
}

// Start begins the metrics pruning goroutine
func (s *Service) Start() {
	go s.pruneMetrics()
	logger.Info("Metrics service started", "")
}

// Stop stops the metrics service
func (s *Service) Stop() {
	close(s.stopChan)
	logger.Info("Metrics service stopped", "")
}

// RecordCounter records a counter metric (value = 1)
// Writes directly to SQLite, leveraging WAL mode for performance
func (s *Service) RecordCounter(metricType store.MetricType, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          1,
		EntityID:       entityID,
		AutomationName: automationName,
	}
	
	if err := s.db.Create(&metric).Error; err != nil {
		logger.Error("Failed to record counter metric", "", "error", err, "type", metricType)
	}
}

// RecordTimer records a timer metric (value = duration in nanoseconds)
// Writes directly to SQLite, leveraging WAL mode for performance
func (s *Service) RecordTimer(metricType store.MetricType, duration time.Duration, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          duration.Nanoseconds(),
		EntityID:       entityID,
		AutomationName: automationName,
	}
	
	if err := s.db.Create(&metric).Error; err != nil {
		logger.Error("Failed to record timer metric", "", "error", err, "type", metricType)
	}
}

// pruneMetrics runs in a goroutine to periodically remove old metrics
func (s *Service) pruneMetrics() {
	ticker := time.NewTicker(s.pruneInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			cutoffTime := time.Now().Add(-s.retentionTime)
			result := s.db.Where("timestamp < ?", cutoffTime).Delete(&store.Metric{})
			if result.Error != nil {
				logger.Error("Failed to prune old metrics", "", "error", result.Error)
			} else if result.RowsAffected > 0 {
				logger.Info("Pruned old metrics", "", "count", result.RowsAffected, "cutoff", cutoffTime)
			}
		}
	}
}