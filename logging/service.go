package logging

import (
	"log/slog"
	"time"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
)

// Service handles logging to both console and database
type Service struct {
	db            *gorm.DB
	pruneInterval time.Duration // How often to prune old logs (default: daily)
	retentionTime time.Duration // How long to keep logs (default: 1 month)
	stopChan      chan struct{}
}

// NewService creates a new logging service
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:            db,
		pruneInterval: 24 * time.Hour,     // Prune daily
		retentionTime: 30 * 24 * time.Hour, // Keep 1 month of logs
		stopChan:      make(chan struct{}),
	}
}

// Start begins the log pruning goroutine
func (s *Service) Start() {
	go s.pruneLogs()
	slog.Info("Logging service started")
}

// Stop stops the logging service
func (s *Service) Stop() {
	close(s.stopChan)
	slog.Info("Logging service stopped")
}

// Info logs an info message to both console and database
func (s *Service) Info(msg string, entityID *string, args ...any) {
	// Log to console using slog
	if entityID != nil {
		args = append([]any{"entity_id", *entityID}, args...)
	}
	slog.Info(msg, args...)
	
	// Log to database
	s.logToDatabase(msg, entityID)
}

// Error logs an error message to both console and database
func (s *Service) Error(msg string, entityID *string, args ...any) {
	// Log to console using slog
	if entityID != nil {
		args = append([]any{"entity_id", *entityID}, args...)
	}
	slog.Error(msg, args...)
	
	// Log to database
	s.logToDatabase(msg, entityID)
}

// Debug logs a debug message to both console and database
func (s *Service) Debug(msg string, entityID *string, args ...any) {
	// Log to console using slog
	if entityID != nil {
		args = append([]any{"entity_id", *entityID}, args...)
	}
	slog.Debug(msg, args...)
	
	// Log to database
	s.logToDatabase(msg, entityID)
}

// Warn logs a warning message to both console and database
func (s *Service) Warn(msg string, entityID *string, args ...any) {
	// Log to console using slog
	if entityID != nil {
		args = append([]any{"entity_id", *entityID}, args...)
	}
	slog.Warn(msg, args...)
	
	// Log to database
	s.logToDatabase(msg, entityID)
}

// logToDatabase writes the log entry to the database
func (s *Service) logToDatabase(msg string, entityID *string) {
	log := store.Log{
		Timestamp: time.Now(),
		EntityID:  entityID,
		LogText:   msg,
	}
	
	if err := s.db.Create(&log).Error; err != nil {
		slog.Error("Failed to write log to database", "error", err, "message", msg)
	}
}

// pruneLogs runs in a goroutine to periodically remove old logs
func (s *Service) pruneLogs() {
	ticker := time.NewTicker(s.pruneInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			cutoffTime := time.Now().Add(-s.retentionTime)
			result := s.db.Where("timestamp < ?", cutoffTime).Delete(&store.Log{})
			if result.Error != nil {
				slog.Error("Failed to prune old logs", "error", result.Error)
			} else if result.RowsAffected > 0 {
				slog.Info("Pruned old logs", "count", result.RowsAffected, "cutoff", cutoffTime)
			}
		}
	}
}