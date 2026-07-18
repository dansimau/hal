package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
)

func TestColorDiff(t *testing.T) {
	diff := "context line\n+added line\n-removed line"

	out := colorDiff(diff)

	// Regardless of whether color codes are emitted, the original text must be
	// preserved and all three line kinds handled without panicking.
	if !strings.Contains(out, "added line") {
		t.Errorf("expected output to contain added line, got %q", out)
	}
	if !strings.Contains(out, "removed line") {
		t.Errorf("expected output to contain removed line, got %q", out)
	}
	if !strings.Contains(out, "context line") {
		t.Errorf("expected output to contain context line, got %q", out)
	}
}

func TestPrettifyJSON(t *testing.T) {
	t.Run("valid JSON is indented", func(t *testing.T) {
		colored, plain := prettifyJSON(`{"a":1,"b":2}`)

		if !strings.Contains(plain, "\n") {
			t.Errorf("expected plain output to be indented, got %q", plain)
		}
		if colored == "" {
			t.Error("expected colored output to be non-empty")
		}
	})

	t.Run("invalid JSON returns original for both", func(t *testing.T) {
		colored, plain := prettifyJSON("not json")

		if colored != "not json" || plain != "not json" {
			t.Errorf("expected original string for invalid JSON, got colored=%q plain=%q", colored, plain)
		}
	})
}

func TestInfoDiffWritesToDatabase(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	service := NewServiceWithDB(db)

	service.InfoDiff("state changed", "light.kitchen", "-old\n+new", "reason", "manual")

	db.WaitForWrites()

	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if !strings.Contains(logs[0].LogText, "state changed") {
		t.Errorf("expected message in log text, got %q", logs[0].LogText)
	}
	if !strings.Contains(logs[0].LogText, "+new") {
		t.Errorf("expected diff in log text, got %q", logs[0].LogText)
	}
	if logs[0].EntityID != "light.kitchen" {
		t.Errorf("expected entity id, got %q", logs[0].EntityID)
	}
}

func TestDebugJSONWritesToDatabase(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	service := NewServiceWithDB(db)
	service.SetLevel(slog.LevelDebug)

	service.DebugJSON("payload", "sensor.temp", `{"value":21}`)

	db.WaitForWrites()

	var logs []store.Log
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if !strings.Contains(logs[0].LogText, "payload") {
		t.Errorf("expected message in log text, got %q", logs[0].LogText)
	}
	if !strings.Contains(logs[0].LogText, "\"value\": 21") {
		t.Errorf("expected indented JSON in log text, got %q", logs[0].LogText)
	}
}

func TestDebugJSONBelowLevelIsNotWritten(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Default level is Info, so a Debug-level entry must be dropped.
	service := NewServiceWithDB(db)

	service.DebugJSON("payload", "sensor.temp", `{"value":21}`)

	db.WaitForWrites()

	var count int64
	if err := db.Model(&store.Log{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no logs below level, got %d", count)
	}
}

func TestInfoDiffBuffersWithoutDatabase(t *testing.T) {
	// No database configured, so the entry lands in the in-memory buffer.
	service := NewService()

	service.InfoDiff("buffered diff", "light.x", "-a\n+b")

	service.mu.RLock()
	count := service.bufferCount
	service.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 buffered log, got %d", count)
	}
}

func TestContextLoggingWrites(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	service := NewServiceWithDB(db)
	service.SetLevel(slog.LevelDebug)

	ctx := context.Background()
	service.InfoContext(ctx, "info with ctx")
	service.ErrorContext(ctx, "error with ctx")
	service.DebugContext(ctx, "debug with ctx")
	service.WarnContext(ctx, "warn with ctx")

	db.WaitForWrites()

	var count int64
	if err := db.Model(&store.Log{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 context logs, got %d", count)
	}
}

func TestContextExtractionHelpers(t *testing.T) {
	// A background context carries neither value.
	if got := getEntityIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty entity id, got %q", got)
	}
	if got := getAutomationNameFromContext(context.Background()); got != "" {
		t.Errorf("expected empty automation name, got %q", got)
	}
}

func TestServiceStartStopLifecycle(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	service := NewServiceWithDB(db)

	service.Start()
	service.Stop()
	// Second stop is a no-op (already stopped).
	service.Stop()
	// Starting again must recreate the stop channel and not panic.
	service.Start()
	service.Stop()
}

func TestServiceStartWithoutDatabase(t *testing.T) {
	service := NewService()

	// Without a database the pruning goroutine is not started, but Start/Stop
	// must still be safe to call.
	service.Start()
	service.Stop()
}

func TestServicePruneLogsRemovesExpired(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	service := NewServiceWithDB(db)
	service.pruneInterval = 30 * time.Millisecond
	service.retentionTime = time.Hour

	// One expired log and one fresh log inserted directly.
	if err := db.Create(&store.Log{Timestamp: time.Now().Add(-2 * time.Hour), Level: "INFO", LogText: "old"}).Error; err != nil {
		t.Fatalf("Failed to insert old log: %v", err)
	}
	if err := db.Create(&store.Log{Timestamp: time.Now(), Level: "INFO", LogText: "new"}).Error; err != nil {
		t.Fatalf("Failed to insert new log: %v", err)
	}

	service.Start()
	defer service.Stop()

	// Wait for a prune tick to fire and its async delete to complete.
	deadline := time.After(3 * time.Second)
	for {
		db.WaitForWrites()

		var count int64
		if err := db.Model(&store.Log{}).Count(&count).Error; err != nil {
			t.Fatalf("Failed to count logs: %v", err)
		}
		if count == 1 {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for prune; expected 1 log, got %d", count)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestGlobalDiffAndContextWrappers(t *testing.T) {
	// Exercise the package-level convenience wrappers. The default logger has no
	// database in tests, so these just need to run without panicking.
	InfoDiff("global diff", "light.x", "-a\n+b")
	DebugJSON("global json", "light.x", `{"k":"v"}`)

	ctx := context.Background()
	InfoContext(ctx, "global info ctx")
	ErrorContext(ctx, "global error ctx")
	DebugContext(ctx, "global debug ctx")
	WarnContext(ctx, "global warn ctx")

	StartDefault()
	StopDefault()
}
