package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

// newExtraDebugService returns a service backed by an in-memory database with
// the level lowered to Debug so every log reaches the database.
func newExtraDebugService(t *testing.T) (*Service, *store.Store) {
	t.Helper()

	db, err := store.Open(":memory:")
	assert.NilError(t, err)

	s := NewServiceWithDB(db)
	s.SetLevel(slog.LevelDebug)

	return s, db
}

func extraFindLogs(t *testing.T, db *store.Store) []store.Log {
	t.Helper()

	db.WaitForWrites()

	var logs []store.Log
	assert.NilError(t, db.Order("id ASC").Find(&logs).Error)

	return logs
}

// TestContextLoggersExtractMetadata is the regression guard for the context-key
// fix: values stored under the exported keys (as hal.NewAutomationContext does)
// must be read back and written to the database. Before the fix the loggers read
// a function-local key type that never matched, silently dropping the metadata.
func TestContextLoggersExtractMetadata(t *testing.T) {
	s, db := newExtraDebugService(t)

	ctx := context.WithValue(context.Background(), EntityIDKey, "light.hall")
	ctx = context.WithValue(ctx, AutomationNameKey, "motion_lights")

	s.InfoContext(ctx, "running")
	s.WarnContext(ctx, "careful")
	s.ErrorContext(ctx, "boom")
	s.DebugContext(ctx, "trace")

	logs := extraFindLogs(t, db)
	assert.Equal(t, len(logs), 4)

	for _, log := range logs {
		assert.Equal(t, log.EntityID, "light.hall")
		assert.Assert(t, strings.Contains(log.LogText, "automation=motion_lights"),
			"expected automation in %q", log.LogText)
	}
}

// TestContextLoggersDoNotDuplicateAutomation verifies an explicit "automation"
// arg supplied by the caller is not overridden or duplicated by the context.
func TestContextLoggersDoNotDuplicateAutomation(t *testing.T) {
	s, db := newExtraDebugService(t)

	ctx := context.WithValue(context.Background(), AutomationNameKey, "motion_lights")

	s.InfoContext(ctx, "running", "automation", "explicit")

	logs := extraFindLogs(t, db)
	assert.Equal(t, len(logs), 1)
	assert.Assert(t, strings.Contains(logs[0].LogText, "automation=explicit"))
	assert.Assert(t, !strings.Contains(logs[0].LogText, "motion_lights"))
}

// TestErrorContextDoesNotOverrideExplicitAutomation is the ErrorContext analogue:
// a context automation name must not override an explicit "automation" arg.
func TestErrorContextDoesNotOverrideExplicitAutomation(t *testing.T) {
	s, db := newExtraDebugService(t)

	ctx := context.WithValue(context.Background(), AutomationNameKey, "context_automation")

	s.ErrorContext(ctx, "boom", "automation", "foo")

	logs := extraFindLogs(t, db)
	assert.Equal(t, len(logs), 1)
	assert.Equal(t, logs[0].Level, slog.LevelError.String())
	assert.Assert(t, strings.Contains(logs[0].LogText, "automation=foo"))
	assert.Assert(t, !strings.Contains(logs[0].LogText, "context_automation"))
}

func TestWriteLogTextRespectsLevel(t *testing.T) {
	db, err := store.Open(":memory:")
	assert.NilError(t, err)

	s := NewServiceWithDB(db)
	s.SetLevel(slog.LevelInfo)

	// Below the configured level: dropped.
	s.writeLogText(slog.LevelDebug, "e1", "debug text")
	// At/above level: written.
	s.writeLogText(slog.LevelInfo, "e2", "info text")

	logs := extraFindLogs(t, db)
	assert.Equal(t, len(logs), 1)
	assert.Equal(t, logs[0].EntityID, "e2")
	assert.Equal(t, logs[0].LogText, "info text")
}

// TestPruneLogsHandlesDeleteError covers the branch where the periodic delete
// itself fails. Dropping the logs table makes the DELETE error ("no such
// table"), which pruneLogs must log without panicking.
func TestPruneLogsHandlesDeleteError(t *testing.T) {
	db, err := store.Open(":memory:")
	assert.NilError(t, err)

	s := NewServiceWithDB(db)
	s.retentionTime = time.Hour
	s.pruneInterval = 5 * time.Millisecond

	// Remove the table the prune delete targets so the delete returns an error.
	assert.NilError(t, db.Migrator().DropTable(&store.Log{}))

	stopChan := make(chan struct{})
	done := make(chan struct{})
	go func() {
		s.pruneLogs(stopChan)
		close(done)
	}()

	// Give the ticker time to fire at least once and let the failing delete run
	// through the async writer.
	time.Sleep(30 * time.Millisecond)
	db.WaitForWrites()

	close(stopChan)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pruneLogs did not return after stop channel was closed")
	}
}
