// Package logger provides a service for logging to both console and database.
package logger

import (
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/dansimau/hal/store"
	"gorm.io/gorm"
)

// BufferedLog represents a log entry waiting to be written to database
type BufferedLog struct {
	Timestamp time.Time
	EntityID  string
	LogText   string
}

// Service handles logging to both console and database
type Service struct {
	db            *gorm.DB
	pruneInterval time.Duration // How often to prune old logs (default: daily)
	retentionTime time.Duration // How long to keep logs (default: 1 month)
	stopChan      chan struct{}

	// Buffering for when database is not available
	mu          sync.RWMutex
	buffer      []BufferedLog
	bufferSize  int
	bufferHead  int // circular buffer head position
	bufferCount int // number of items in buffer

	// Error tracking
	lastError  error
	errorCount int
}

// NewService creates a new logging service
func NewService() *Service {
	return &Service{
		pruneInterval: 24 * time.Hour,      // Prune daily
		retentionTime: 30 * 24 * time.Hour, // Keep 1 month of logs
		stopChan:      make(chan struct{}),
		bufferSize:    1000,
		buffer:        make([]BufferedLog, 1000),
	}
}

// NewServiceWithDB creates a new logging service with database
func NewServiceWithDB(db *gorm.DB) *Service {
	s := NewService()
	s.SetDatabase(db)
	return s
}

// SetDatabase sets the database for the logging service and flushes buffered logs
func (s *Service) SetDatabase(db *gorm.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db = db

	// Flush buffered logs to database
	if s.bufferCount > 0 {
		flushCount := s.bufferCount
		for i := 0; i < s.bufferCount; i++ {
			idx := (s.bufferHead - s.bufferCount + i + s.bufferSize) % s.bufferSize
			bufferedLog := s.buffer[idx]
			log := store.Log{
				Timestamp: bufferedLog.Timestamp,
				EntityID:  bufferedLog.EntityID,
				LogText:   bufferedLog.LogText,
			}
			if err := s.db.Create(&log).Error; err != nil {
				slog.Error("Failed to write buffered log to database", "error", err, "message", bufferedLog.LogText)
			}
		}
		s.bufferCount = 0
		slog.Info("Flushed buffered logs to database", "count", flushCount)
	}
}

// Start begins the log pruning goroutine
func (s *Service) Start() {
	s.mu.Lock()
	// Create a new stopChan if the previous one was closed
	select {
	case <-s.stopChan:
		s.stopChan = make(chan struct{})
	default:
		// Channel is still open
	}
	hasDB := s.db != nil
	s.mu.Unlock()

	if hasDB {
		go s.pruneLogs()
	}
	slog.Info("Logging service started")
}

// Stop stops the logging service
func (s *Service) Stop() {
	select {
	case <-s.stopChan:
		// Already stopped
		return
	default:
		close(s.stopChan)
		slog.Info("Logging service stopped")
	}
}

// Info logs an info message to both console and database
func (s *Service) Info(msg string, entityID string, args ...any) {
	// Log to console using slog
	if entityID != "" {
		args = append([]any{"entity_id", entityID}, args...)
	}
	slog.Info(msg, args...)

	// Log to database
	s.logToDatabase(msg, entityID)
}

// Error logs an error message to both console and database
func (s *Service) Error(msg string, entityID string, args ...any) {
	// Log to console using slog
	if entityID != "" {
		args = append([]any{"entity_id", entityID}, args...)
	}
	slog.Error(msg, args...)

	// Log to database
	s.logToDatabase(msg, entityID)
}

// Debug logs a debug message to both console and database
func (s *Service) Debug(msg string, entityID string, args ...any) {
	// Log to console using slog
	if entityID != "" {
		args = append([]any{"entity_id", entityID}, args...)
	}
	slog.Debug(msg, args...)

	// Log to database
	s.logToDatabase(msg, entityID)
}

// Warn logs a warning message to both console and database
func (s *Service) Warn(msg string, entityID string, args ...any) {
	// Log to console using slog
	if entityID != "" {
		args = append([]any{"entity_id", entityID}, args...)
	}
	slog.Warn(msg, args...)

	// Log to database
	s.logToDatabase(msg, entityID)
}

// logToDatabase writes the log entry to the database or buffers it
func (s *Service) logToDatabase(msg string, entityID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		// Database available, write directly
		log := store.Log{
			Timestamp: time.Now(),
			EntityID:  entityID,
			LogText:   msg,
		}

		if err := s.db.Create(&log).Error; err != nil {

			slog.Error("Failed to write log to database", "error", err, "message", msg)

			// Print stack trace when a database error occurs
			stackBuf := make([]byte, 2048)
			n := runtime.Stack(stackBuf, false)
			println(string(stackBuf[:n]))
			println(s.db)

			s.lastError = err
			s.errorCount++
		}
	} else {
		// No database, add to circular buffer
		bufferedLog := BufferedLog{
			Timestamp: time.Now(),
			EntityID:  entityID,
			LogText:   msg,
		}

		s.buffer[s.bufferHead] = bufferedLog
		s.bufferHead = (s.bufferHead + 1) % s.bufferSize

		if s.bufferCount < s.bufferSize {
			s.bufferCount++
		}
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
			s.mu.RLock()
			db := s.db
			s.mu.RUnlock()

			if db != nil {
				cutoffTime := time.Now().Add(-s.retentionTime)
				result := db.Where("timestamp < ?", cutoffTime).Delete(&store.Log{})
				if result.Error != nil {
					slog.Error("Failed to prune old logs", "error", result.Error)
				} else if result.RowsAffected > 0 {
					slog.Info("Pruned old logs", "count", result.RowsAffected, "cutoff", cutoffTime)
				}
			}
		}
	}
}

// Global default logger instance
var defaultLogger = NewService()

func GetDefaultLogger() *Service {
	return defaultLogger
}

// Global logging functions that use the default logger

// Info logs an info message using the global default logger
func Info(msg string, entityID string, args ...any) {
	defaultLogger.Info(msg, entityID, args...)
}

// Error logs an error message using the global default logger
func Error(msg string, entityID string, args ...any) {
	defaultLogger.Error(msg, entityID, args...)
}

// Debug logs a debug message using the global default logger
func Debug(msg string, entityID string, args ...any) {
	defaultLogger.Debug(msg, entityID, args...)
}

// Warn logs a warning message using the global default logger
func Warn(msg string, entityID string, args ...any) {
	defaultLogger.Warn(msg, entityID, args...)
}

// SetDefaultDatabase sets the database for the global default logger
func SetDefaultDatabase(db *gorm.DB) {
	defaultLogger.SetDatabase(db)
}

// StartDefault starts the global default logger
func StartDefault() {
	defaultLogger.Start()
}

// StopDefault stops the global default logger
func StopDefault() {
	defaultLogger.Stop()
}

// LastError returns the last database error that occurred
func (s *Service) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// ErrorCount returns the total number of database errors that have occurred
func (s *Service) ErrorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorCount
}

// Global error tracking functions

// LastError returns the last database error from the global default logger
func LastError() error {
	return defaultLogger.LastError()
}

// ErrorCount returns the total number of database errors from the global default logger
func ErrorCount() int {
	return defaultLogger.ErrorCount()
}
