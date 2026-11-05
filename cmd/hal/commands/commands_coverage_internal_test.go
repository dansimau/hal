package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

// testEntityID is the entity used throughout the command tests.
const testEntityID = "light.kitchen"

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
	t.Chdir(dir)
}

func TestNewEntitiesCmd(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		assert.NilError(t, printEntitiesTable(&out, nil))
		assert.Assert(t, strings.Contains(out.String(), "No entities found"))
	})

	t.Run("with rows", func(t *testing.T) {
		t.Parallel()

		entities := []EntitySummary{
			{ID: testEntityID, LastUpdate: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), LogCount: 7},
		}

		var out bytes.Buffer

		assert.NilError(t, printEntitiesTable(&out, entities))
		assert.Assert(t, strings.Contains(out.String(), testEntityID))
		assert.Assert(t, strings.Contains(out.String(), "2024-01-02 03:04:05"))
		assert.Assert(t, strings.Contains(out.String(), "7"))
	})
}

func TestPrintEntityStateJSON(t *testing.T) {
	t.Parallel()

	entity := store.Entity{
		ID: testEntityID,
		State: &homeassistant.State{
			EntityID: testEntityID,
			State:    "on",
		},
	}

	var out bytes.Buffer

	assert.NilError(t, printEntityStateJSON(&out, entity))
	assert.Assert(t, strings.Contains(out.String(), testEntityID))
	assert.Assert(t, strings.Contains(out.String(), "on"))
}

//nolint:paralleltest // uses os.Chdir
func TestRunEntitiesCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Entity{
			ID:    testEntityID,
			State: &homeassistant.State{EntityID: testEntityID, State: "on"},
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: levelInfo, EntityID: testEntityID, LogText: "hi",
		}).Error)
	})
	chdir(t, dir)

	var out bytes.Buffer

	assert.NilError(t, runEntitiesCommand(&out))
	assert.Assert(t, strings.Contains(out.String(), testEntityID))
}

//nolint:paralleltest // uses os.Chdir
func TestRunShowEntityCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Entity{
			ID:    testEntityID,
			State: &homeassistant.State{EntityID: testEntityID, State: "on"},
		}).Error)
	})
	chdir(t, dir)

	var out bytes.Buffer

	assert.NilError(t, runShowEntityCommand(&out, testEntityID))
	assert.Assert(t, strings.Contains(out.String(), "on"))

	// Unknown entity is an error.
	err := runShowEntityCommand(&out, "light.missing")
	assert.ErrorIs(t, err, errEntityNotFound)
}

func TestNewLogsCmd(t *testing.T) {
	t.Parallel()

	cmd := NewLogsCmd()
	assert.Equal(t, cmd.Use, logsCmdUse)
	// All documented flags are registered.
	for _, name := range []string{"from", "to", "last", "entity-id", "no-color"} {
		assert.Assert(t, cmd.Flags().Lookup(name) != nil, "missing flag %q", name)
	}
}

//nolint:paralleltest // printLogs sets the global color.NoColor
func TestPrintLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, printLogs(&out, nil, true))
		assert.Assert(t, strings.Contains(out.String(), "No logs found"))
	})

	t.Run("renders levels, entity and multiline diff", func(t *testing.T) {
		ts := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
		logs := []store.Log{
			{Timestamp: ts, Level: levelInfo, EntityID: testEntityID, LogText: "turned on foo=bar"},
			{Timestamp: ts, Level: levelError, LogText: "boom"},
			{Timestamp: ts, Level: levelWarn, LogText: "state changed\n-off\n+on"},
			{Timestamp: ts, Level: levelDebug, LogText: "payload\n{\"k\": 1}"},
		}

		// noColor=true so output is plain and easy to assert on.
		var out bytes.Buffer

		assert.NilError(t, printLogs(&out, logs, true))
		assert.Assert(t, strings.Contains(out.String(), "2024-05-06 07:08:09"))
		assert.Assert(t, strings.Contains(out.String(), levelInfo))
		assert.Assert(t, strings.Contains(out.String(), "["+testEntityID+"]"))
		assert.Assert(t, strings.Contains(out.String(), "turned on foo=bar"))
		assert.Assert(t, strings.Contains(out.String(), "-off"))
		assert.Assert(t, strings.Contains(out.String(), "+on"))
		assert.Assert(t, strings.Contains(out.String(), "\"k\": 1"))
	})
}

//nolint:paralleltest // uses os.Chdir
func TestRunLogsCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: levelInfo, EntityID: testEntityID, LogText: "recent",
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now().Add(-48 * time.Hour), Level: levelInfo, LogText: "old",
		}).Error)
	})
	chdir(t, dir)

	t.Run("last filter", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, runLogsCommand(&out, "", "", "1h", "", true))
		assert.Assert(t, strings.Contains(out.String(), "recent"))
		assert.Assert(t, !strings.Contains(out.String(), "old"))
	})

	t.Run("entity filter", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, runLogsCommand(&out, "", "", "", testEntityID, true))
		assert.Assert(t, strings.Contains(out.String(), "recent"))
	})

	t.Run("from/to range", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, runLogsCommand(&out, "2000-01-01", "2100-01-01", "", "", true))
		assert.Assert(t, strings.Contains(out.String(), "recent"))
		assert.Assert(t, strings.Contains(out.String(), "old"))
	})

	t.Run("invalid duration", func(t *testing.T) {
		err := runLogsCommand(&bytes.Buffer{}, "", "", "notaduration", "", true)
		assert.Assert(t, err != nil)
	})

	t.Run("invalid from", func(t *testing.T) {
		err := runLogsCommand(&bytes.Buffer{}, "nope", "", "", "", true)
		assert.ErrorIs(t, err, errUnparsableTime)
	})

	t.Run("invalid to", func(t *testing.T) {
		err := runLogsCommand(&bytes.Buffer{}, "", "nope", "", "", true)
		assert.ErrorIs(t, err, errUnparsableTime)
	})
}

func TestNewPruneCmd(t *testing.T) {
	t.Parallel()

	cmd := NewPruneCmd()
	assert.Equal(t, cmd.Use, "prune")

	// "logs" subcommand exists.
	var logsCmd *cobra.Command

	for _, c := range cmd.Commands() {
		if c.Use == logsCmdUse {
			logsCmd = c
		}
	}

	assert.Assert(t, logsCmd != nil)
}

func TestPruneLogsCmdFlagValidation(t *testing.T) {
	t.Parallel()

	t.Run("requires a flag", func(t *testing.T) {
		t.Parallel()

		cmd := NewPruneLogsCmd()
		cmd.SetArgs(nil)
		err := cmd.Execute()
		assert.ErrorIs(t, err, errPruneFlagRequired)
		assert.Assert(t, strings.Contains(err.Error(), "required"))
	})

	t.Run("rejects both flags", func(t *testing.T) {
		t.Parallel()

		cmd := NewPruneLogsCmd()
		cmd.SetArgs([]string{"--last", "1d", "--before", "2024-01-01"})
		err := cmd.Execute()
		assert.ErrorIs(t, err, errPruneBothFlags)
		assert.Assert(t, strings.Contains(err.Error(), "cannot use both"))
	})
}

//nolint:paralleltest // uses os.Chdir
func TestRunPruneLogsCommand(t *testing.T) {
	dir := t.TempDir()
	seedDBInDir(t, dir, func(db *store.Store) {
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now().Add(-48 * time.Hour), Level: levelInfo, LogText: "old",
		}).Error)
		assert.NilError(t, db.Create(&store.Log{
			Timestamp: time.Now(), Level: levelInfo, LogText: "new",
		}).Error)
	})
	chdir(t, dir)

	t.Run("prune by last", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, runPruneLogsCommand(&out, "1d", ""))
		assert.Assert(t, strings.Contains(out.String(), "Deleted 1 log entries"))
	})

	t.Run("invalid duration", func(t *testing.T) {
		err := runPruneLogsCommand(&bytes.Buffer{}, "notaduration", "")
		assert.Assert(t, err != nil)
	})

	t.Run("invalid before", func(t *testing.T) {
		err := runPruneLogsCommand(&bytes.Buffer{}, "", "not-a-date")
		assert.ErrorIs(t, err, errUnparsableTime)
	})

	t.Run("prune by before", func(t *testing.T) {
		var out bytes.Buffer

		assert.NilError(t, runPruneLogsCommand(&out, "", "2100-01-01"))
		// The "new" row remains after the "last" prune; before=2100 removes it.
		assert.Assert(t, strings.Contains(out.String(), "Deleted 1 log entries"))
	})
}

func TestNewEventsCmd(t *testing.T) {
	t.Parallel()

	cmd := NewEventsCmd()
	assert.Equal(t, cmd.Use, "events")
	assert.Assert(t, cmd.Flags().Lookup("exclude") != nil)
	assert.Assert(t, cmd.Flags().Lookup("jq") != nil)
}

func TestMakeEmitterPrettyPrints(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	emit, cleanup, err := makeEmitter(&out, "")
	assert.NilError(t, err)

	defer cleanup()

	emit([]byte(`{"a":1}`))
	// Invalid JSON is written through unchanged rather than dropped.
	emit([]byte(`not json`))

	assert.Assert(t, strings.Contains(out.String(), "\"a\": 1"))
	assert.Assert(t, strings.Contains(out.String(), "not json"))
}
