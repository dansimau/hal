package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

// seedDBInDir creates a fresh sqlite.db inside dir, runs seed against it, and
// closes it so the command under test (which opens "sqlite.db" relative to cwd)
// can open it independently.
func seedDBInDir(t *testing.T, dir string, seed func(db *store.Store)) {
	t.Helper()

	db, err := store.Open(dir + "/sqlite.db")
	assert.NilError(t, err)

	if seed != nil {
		seed(db)
		db.WaitForWrites()
	}

	assert.NilError(t, db.Close())
}

// chdir switches into dir for the duration of the test and restores the
// previous working directory on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()

	orig, err := os.Getwd()
	assert.NilError(t, err)

	assert.NilError(t, os.Chdir(dir))
	t.Cleanup(func() {
		assert.NilError(t, os.Chdir(orig))
	})
}

func TestNewEntitiesCmd(t *testing.T) {
	cmd := NewEntitiesCmd()
	assert.Equal(t, cmd.Use, "entities")
	assert.Assert(t, len(cmd.Aliases) > 0)

	// The "show" subcommand is wired up.
	var show *cobra.Command
	for _, c := range cmd.Commands() {
		if strings.HasPrefix(c.Use, "show") {
			show = c
		}
	}
	assert.Assert(t, show != nil)
}

func TestPrintEntitiesTable(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, printEntitiesTable(nil))
		})
		assert.Assert(t, strings.Contains(out, "No entities found"))
	})

	t.Run("with rows", func(t *testing.T) {
		entities := []EntitySummary{
			{ID: "light.kitchen", LastUpdate: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), LogCount: 7},
		}
		out := captureOutput(func() {
			assert.NilError(t, printEntitiesTable(entities))
		})
		assert.Assert(t, strings.Contains(out, "light.kitchen"))
		assert.Assert(t, strings.Contains(out, "2024-01-02 03:04:05"))
		assert.Assert(t, strings.Contains(out, "7"))
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

func TestRunEntitiesCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Entity{
			ID:    "light.kitchen",
			State: &homeassistant.State{EntityID: "light.kitchen", State: "on"},
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: "INFO", EntityID: "light.kitchen", LogText: "hi",
		}).Error)
	})
	chdir(t, dir)

	out := captureOutput(func() {
		assert.NilError(t, runEntitiesCommand())
	})
	assert.Assert(t, strings.Contains(out, "light.kitchen"))
}

func TestRunShowEntityCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Entity{
			ID:    "light.kitchen",
			State: &homeassistant.State{EntityID: "light.kitchen", State: "on"},
		}).Error)
	})
	chdir(t, dir)

	out := captureOutput(func() {
		assert.NilError(t, runShowEntityCommand("light.kitchen"))
	})
	assert.Assert(t, strings.Contains(out, "on"))

	// Unknown entity is an error.
	err := runShowEntityCommand("light.missing")
	assert.Assert(t, err != nil)
}

func TestNewLogsCmd(t *testing.T) {
	cmd := NewLogsCmd()
	assert.Equal(t, cmd.Use, "logs")
	// All documented flags are registered.
	for _, name := range []string{"from", "to", "last", "entity-id", "no-color"} {
		assert.Assert(t, cmd.Flags().Lookup(name) != nil, "missing flag %q", name)
	}
}

func TestPrintLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, printLogs(nil, true))
		})
		assert.Assert(t, strings.Contains(out, "No logs found"))
	})

	t.Run("renders levels, entity and multiline diff", func(t *testing.T) {
		ts := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
		logs := []store.Log{
			{Timestamp: ts, Level: "INFO", EntityID: "light.kitchen", LogText: "turned on foo=bar"},
			{Timestamp: ts, Level: "ERROR", LogText: "boom"},
			{Timestamp: ts, Level: "WARN", LogText: "state changed\n-off\n+on"},
			{Timestamp: ts, Level: "DEBUG", LogText: "payload\n{\"k\": 1}"},
		}
		// noColor=true so output is plain and easy to assert on.
		out := captureOutput(func() {
			assert.NilError(t, printLogs(logs, true))
		})
		assert.Assert(t, strings.Contains(out, "2024-05-06 07:08:09"))
		assert.Assert(t, strings.Contains(out, "INFO"))
		assert.Assert(t, strings.Contains(out, "[light.kitchen]"))
		assert.Assert(t, strings.Contains(out, "turned on foo=bar"))
		assert.Assert(t, strings.Contains(out, "-off"))
		assert.Assert(t, strings.Contains(out, "+on"))
		assert.Assert(t, strings.Contains(out, "\"k\": 1"))
	})
}

func TestRunLogsCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: "INFO", EntityID: "light.kitchen", LogText: "recent",
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now().Add(-48 * time.Hour), Level: "INFO", LogText: "old",
		}).Error)
	})
	chdir(t, dir)

	t.Run("last filter", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, runLogsCommand("", "", "1h", "", true))
		})
		assert.Assert(t, strings.Contains(out, "recent"))
		assert.Assert(t, !strings.Contains(out, "old"))
	})

	t.Run("entity filter", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, runLogsCommand("", "", "", "light.kitchen", true))
		})
		assert.Assert(t, strings.Contains(out, "recent"))
	})

	t.Run("from/to range", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, runLogsCommand("2000-01-01", "2100-01-01", "", "", true))
		})
		assert.Assert(t, strings.Contains(out, "recent"))
		assert.Assert(t, strings.Contains(out, "old"))
	})

	t.Run("invalid duration", func(t *testing.T) {
		err := runLogsCommand("", "", "notaduration", "", true)
		assert.Assert(t, err != nil)
	})

	t.Run("invalid from", func(t *testing.T) {
		err := runLogsCommand("nope", "", "", "", true)
		assert.Assert(t, err != nil)
	})

	t.Run("invalid to", func(t *testing.T) {
		err := runLogsCommand("", "nope", "", "", true)
		assert.Assert(t, err != nil)
	})
}

func TestNewPruneCmd(t *testing.T) {
	cmd := NewPruneCmd()
	assert.Equal(t, cmd.Use, "prune")

	// "logs" subcommand exists.
	var logsCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Use == "logs" {
			logsCmd = c
		}
	}
	assert.Assert(t, logsCmd != nil)
}

func TestPruneLogsCmdFlagValidation(t *testing.T) {
	t.Run("requires a flag", func(t *testing.T) {
		cmd := NewPruneLogsCmd()
		cmd.SetArgs(nil)
		err := cmd.Execute()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(err.Error(), "required"))
	})

	t.Run("rejects both flags", func(t *testing.T) {
		cmd := NewPruneLogsCmd()
		cmd.SetArgs([]string{"--last", "1d", "--before", "2024-01-01"})
		err := cmd.Execute()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(err.Error(), "cannot use both"))
	})
}

func TestRunPruneLogsCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now().Add(-48 * time.Hour), Level: "INFO", LogText: "old",
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: "INFO", LogText: "new",
		}).Error)
	})
	chdir(t, dir)

	t.Run("prune by last", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, runPruneLogsCommand("1d", ""))
		})
		assert.Assert(t, strings.Contains(out, "Deleted 1 log entries"))
	})

	t.Run("invalid duration", func(t *testing.T) {
		err := runPruneLogsCommand("notaduration", "")
		assert.Assert(t, err != nil)
	})

	t.Run("invalid before", func(t *testing.T) {
		err := runPruneLogsCommand("", "not-a-date")
		assert.Assert(t, err != nil)
	})

	t.Run("prune by before", func(t *testing.T) {
		out := captureOutput(func() {
			assert.NilError(t, runPruneLogsCommand("", "2100-01-01"))
		})
		// The "new" row remains after the "last" prune; before=2100 removes it.
		assert.Assert(t, strings.Contains(out, "Deleted 1 log entries"))
	})
}

func TestNewEventsCmd(t *testing.T) {
	cmd := NewEventsCmd()
	assert.Equal(t, cmd.Use, "events")
	assert.Assert(t, cmd.Flags().Lookup("exclude") != nil)
	assert.Assert(t, cmd.Flags().Lookup("jq") != nil)
}

func TestMakeEmitterPrettyPrints(t *testing.T) {
	emit, cleanup, err := makeEmitter("")
	assert.NilError(t, err)
	defer cleanup()

	out := captureOutput(func() {
		emit([]byte(`{"a":1}`))
		// Invalid JSON is written through unchanged rather than dropped.
		emit([]byte(`not json`))
	})
	assert.Assert(t, strings.Contains(out, "\"a\": 1"))
	assert.Assert(t, strings.Contains(out, "not json"))
}
