package logger

import (
	"testing"

	"github.com/dansimau/hal/store"
)

func TestLoggingWithArgs(t *testing.T) {
	// Create temporary database
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create logging service
	service := NewServiceWithDB(db)

	// Test logging with various args
	service.Info("User logged in", "user.test", "user_id", 123, "action", "login", "ip", "192.168.1.1")
	service.Error("Database connection failed", "db.primary", "table", "users", "error", "connection timeout", "retry_count", 3)
	service.Debug("Processing item", "item.abc123", "item_id", "abc123", "status", "pending", "priority", "high")
	service.Warn("Rate limit exceeded", "", "endpoint", "/api/users", "rate", "100/min", "user_agent", "Mozilla/5.0 Chrome")

	// Verify logs were written to database with args
	var logs []store.Log
	if err := db.Order("id").Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	// Check first log with args
	expectedText1 := "User logged in entity_id=user.test user_id=123 action=login ip=192.168.1.1"
	if logs[0].LogText != expectedText1 {
		t.Errorf("Expected log text '%s', got '%s'", expectedText1, logs[0].LogText)
	}
	if logs[0].EntityID != "user.test" {
		t.Errorf("Expected entity ID 'user.test', got '%s'", logs[0].EntityID)
	}

	// Check second log with args including quoted values
	expectedText2 := "Database connection failed entity_id=db.primary table=users error=\"connection timeout\" retry_count=3"
	if logs[1].LogText != expectedText2 {
		t.Errorf("Expected log text '%s', got '%s'", expectedText2, logs[1].LogText)
	}

	// Check third log
	expectedText3 := "Processing item entity_id=item.abc123 item_id=abc123 status=pending priority=high"
	if logs[2].LogText != expectedText3 {
		t.Errorf("Expected log text '%s', got '%s'", expectedText3, logs[2].LogText)
	}

	// Check fourth log (no entity ID, but has quoted user agent)
	expectedText4 := "Rate limit exceeded endpoint=/api/users rate=100/min user_agent=\"Mozilla/5.0 Chrome\""
	if logs[3].LogText != expectedText4 {
		t.Errorf("Expected log text '%s', got '%s'", expectedText4, logs[3].LogText)
	}
	if logs[3].EntityID != "" {
		t.Errorf("Expected empty entity ID, got '%s'", logs[3].EntityID)
	}
}

func TestFormatArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected string
	}{
		{
			name:     "no args",
			args:     []any{},
			expected: "",
		},
		{
			name:     "simple key-value pairs",
			args:     []any{"user_id", 123, "action", "login"},
			expected: "user_id=123 action=login",
		},
		{
			name:     "values with spaces get quoted",
			args:     []any{"error", "connection timeout", "status", "failed"},
			expected: "error=\"connection timeout\" status=failed",
		},
		{
			name:     "odd number of args (last arg ignored)",
			args:     []any{"key1", "value1", "key2"},
			expected: "key1=value1",
		},
		{
			name:     "mixed types",
			args:     []any{"count", 42, "enabled", true, "name", "test user"},
			expected: "count=42 enabled=true name=\"test user\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArgs(tt.args...)
			if result != tt.expected {
				t.Errorf("formatArgs() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}