package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

// chdirTemp switches into a fresh temp dir for the duration of the test so
// commands that open the hardcoded "sqlite.db" path operate on a throwaway
// database. os.Chdir is process-wide, so these tests must not run in parallel.
func chdirTemp(t *testing.T) {
	t.Helper()

	oldWd, err := os.Getwd()
	assert.NilError(t, err)

	assert.NilError(t, os.Chdir(t.TempDir()))

	t.Cleanup(func() { _ = os.Chdir(oldWd) })
}

func TestNewLogsCmd(t *testing.T) {
	cmd := NewLogsCmd()

	assert.Equal(t, cmd.Use, "logs")
	assert.Assert(t, len(cmd.Aliases) > 0)
	assert.Assert(t, cmd.Flags().Lookup("from") != nil)
	assert.Assert(t, cmd.Flags().Lookup("last") != nil)
	assert.Assert(t, cmd.Flags().Lookup("entity-id") != nil)
}

func TestNewEntitiesCmd(t *testing.T) {
	cmd := NewEntitiesCmd()

	assert.Equal(t, cmd.Use, "entities")
	assert.Assert(t, len(cmd.Commands()) > 0) // has the "show" subcommand
}

func TestNewPruneCmd(t *testing.T) {
	cmd := NewPruneCmd()

	assert.Equal(t, cmd.Use, "prune")
	assert.Assert(t, len(cmd.Commands()) > 0) // has the "logs" subcommand
}

func TestNewEventsCmd(t *testing.T) {
	cmd := NewEventsCmd()

	assert.Equal(t, cmd.Use, "events")
	assert.Assert(t, cmd.Flags().Lookup("exclude") != nil)
	assert.Assert(t, cmd.Flags().Lookup("jq") != nil)
}

func TestPruneLogsCmdValidation(t *testing.T) {
	t.Run("requires a flag", func(t *testing.T) {
		cmd := NewPruneLogsCmd()
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		assert.ErrorContains(t, err, "either --last or --before")
	})

	t.Run("rejects both flags", func(t *testing.T) {
		cmd := NewPruneLogsCmd()
		cmd.SetArgs([]string{"--last", "1d", "--before", "2024-01-01"})

		err := cmd.Execute()
		assert.ErrorContains(t, err, "cannot use both")
	})
}

func TestPrintLogs(t *testing.T) {
	t.Run("empty prints message", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, printLogs(nil, true))
		})
		assert.Assert(t, strings.Contains(out, "No logs found"))
	})

	t.Run("renders entries", func(t *testing.T) {
		logs := []store.Log{
			{Timestamp: time.Now(), Level: "INFO", EntityID: "light.kitchen", LogText: "single line"},
			{Timestamp: time.Now(), Level: "ERROR", LogText: "first line\nsecond line"},
		}

		out := captureOutput(func() {
			assert.NilError(t, printLogs(logs, true))
		})

		assert.Assert(t, strings.Contains(out, "single line"))
		assert.Assert(t, strings.Contains(out, "light.kitchen"))
		assert.Assert(t, strings.Contains(out, "second line"))
	})
}

func TestPrintEntitiesTable(t *testing.T) {
	t.Run("empty prints message", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, printEntitiesTable(nil))
		})
		assert.Assert(t, strings.Contains(out, "No entities found"))
	})

	t.Run("renders rows", func(t *testing.T) {
		entities := []EntitySummary{
			{ID: "light.kitchen", LastUpdate: time.Now(), LogCount: 5},
		}

		out := captureOutput(func() {
			assert.NilError(t, printEntitiesTable(entities))
		})

		assert.Assert(t, strings.Contains(out, "light.kitchen"))
	})
}

func TestPrintEntityStateJSON(t *testing.T) {
	entity := store.Entity{
		ID: "light.kitchen",
		State: &homeassistant.State{
			EntityID: "light.kitchen",
			State:    "on",
		},
	}

	out := captureOutput(func() {
		assert.NilError(t, printEntityStateJSON(entity))
	})

	assert.Assert(t, strings.Contains(out, "light.kitchen"))
	assert.Assert(t, strings.Contains(out, "on"))
}

func TestMakeEmitterPlain(t *testing.T) {
	emit, cleanup, err := makeEmitter("")
	assert.NilError(t, err)
	defer cleanup()

	out := captureOutput(func() {
		emit([]byte(`{"a":1}`))  // valid JSON, gets indented
		emit([]byte("not json")) // invalid, written as-is
	})

	assert.Assert(t, strings.Contains(out, "\"a\": 1"))
	assert.Assert(t, strings.Contains(out, "not json"))
}

func TestRunEntitiesCommandEmpty(t *testing.T) {
	chdirTemp(t)

	out := captureOutput(func() {
		assert.NilError(t, runEntitiesCommand())
	})

	assert.Assert(t, strings.Contains(out, "No entities found"))
}

func TestRunLogsCommandEmpty(t *testing.T) {
	chdirTemp(t)

	out := captureOutput(func() {
		assert.NilError(t, runLogsCommand("", "", "", "", true))
	})

	assert.Assert(t, strings.Contains(out, "No logs found"))
}

func TestRunLogsCommandInvalidDuration(t *testing.T) {
	chdirTemp(t)

	err := runLogsCommand("", "", "notaduration", "", true)
	assert.ErrorContains(t, err, "invalid duration")
}

func TestRunShowEntityCommandNotFound(t *testing.T) {
	chdirTemp(t)

	err := runShowEntityCommand("light.missing")
	assert.ErrorContains(t, err, "entity not found")
}

func TestRunPruneLogsCommand(t *testing.T) {
	t.Run("by duration", func(t *testing.T) {
		chdirTemp(t)

		out := captureOutput(func() {
			assert.NilError(t, runPruneLogsCommand("1d", ""))
		})
		assert.Assert(t, strings.Contains(out, "Deleted"))
	})

	t.Run("by before date", func(t *testing.T) {
		chdirTemp(t)

		out := captureOutput(func() {
			assert.NilError(t, runPruneLogsCommand("", "2024-01-01"))
		})
		assert.Assert(t, strings.Contains(out, "Deleted"))
	})

	t.Run("invalid duration", func(t *testing.T) {
		chdirTemp(t)

		err := runPruneLogsCommand("bogus", "")
		assert.ErrorContains(t, err, "invalid duration")
	})
}
