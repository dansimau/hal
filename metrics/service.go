package metrics

import (
	"log/slog"
	"time"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
)

// Service handles metrics collection and pruning
type Service struct {
	db             *gorm.DB
	batchSize      int
	metricsBuffer  []store.Metric
	bufferChan     chan store.Metric
	pruneInterval  time.Duration
	retentionTime  time.Duration
	stopChan       chan struct{}
}

// NewService creates a new metrics service
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:            db,
		batchSize:     100,
		metricsBuffer: make([]store.Metric, 0, 100),
		bufferChan:    make(chan store.Metric, 1000), // Buffer up to 1000 metrics
		pruneInterval: 24 * time.Hour,                // Prune daily
		retentionTime: 30 * 24 * time.Hour,          // Keep 30 days of metrics
		stopChan:      make(chan struct{}),
	}
}

// Start begins the metrics collection and pruning goroutines
func (s *Service) Start() {
	go s.collectMetrics()
	go s.pruneMetrics()
	slog.Info("Metrics service started")
}

// Stop stops the metrics service
func (s *Service) Stop() {
	close(s.stopChan)
	slog.Info("Metrics service stopped")
}

// RecordCounter records a counter metric (value = 1)
func (s *Service) RecordCounter(metricType, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          1,
		EntityID:       entityID,
		AutomationName: automationName,
	}
	
	select {
	case s.bufferChan <- metric:
	default:
		slog.Warn("Metrics buffer full, dropping metric", "type", metricType)
	}
}

// RecordTimer records a timer metric (value = duration in nanoseconds)
func (s *Service) RecordTimer(metricType string, duration time.Duration, entityID, automationName string) {
	metric := store.Metric{
		Timestamp:      time.Now(),
		MetricType:     metricType,
		Value:          duration.Nanoseconds(),
		EntityID:       entityID,
		AutomationName: automationName,
	}
	
	select {
	case s.bufferChan <- metric:
	default:
		slog.Warn("Metrics buffer full, dropping metric", "type", metricType)
	}
}

// collectMetrics runs in a goroutine to batch write metrics to database
func (s *Service) collectMetrics() {
	ticker := time.NewTicker(1 * time.Second) // Flush every second
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopChan:
			s.flushBuffer()
			return
		case metric := <-s.bufferChan:
			s.metricsBuffer = append(s.metricsBuffer, metric)
			if len(s.metricsBuffer) >= s.batchSize {
				s.flushBuffer()
			}
		case <-ticker.C:
			if len(s.metricsBuffer) > 0 {
				s.flushBuffer()
			}
		}
	}
}

// flushBuffer writes all buffered metrics to the database
func (s *Service) flushBuffer() {
	if len(s.metricsBuffer) == 0 {
		return
	}
	
	if err := s.db.Create(&s.metricsBuffer).Error; err != nil {
		slog.Error("Failed to write metrics to database", "error", err, "count", len(s.metricsBuffer))
	} else {
		slog.Debug("Flushed metrics to database", "count", len(s.metricsBuffer))
	}
	
	s.metricsBuffer = s.metricsBuffer[:0] // Clear buffer, keep capacity
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
				slog.Error("Failed to prune old metrics", "error", result.Error)
			} else if result.RowsAffected > 0 {
				slog.Info("Pruned old metrics", "count", result.RowsAffected, "cutoff", cutoffTime)
			}
		}
	}
}