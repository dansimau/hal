package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// timestampFormat is how timer timestamps are displayed.
const timestampFormat = "2006-01-02 15:04:05"

// NewTimersCmd creates the timers command.
func NewTimersCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "timers",
		Aliases: []string{"timer"},
		Short:   "Display the saved state of automation timers",
		Long: `Display the state of every durable automation timer as saved in the database:
whether it is running, when it was started and when it expires.

A running timer is restored when HAL restarts. Timer state is written through
the same batched writer as everything else, so the table can lag the running
process by up to the database flush interval.`,
		Example: `  hal timers                   # Show timers using default database
  hal timers --db custom.db    # Show timers from custom database`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTimersCommand(cmd.OutOrStdout(), dbPath, time.Now())
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "sqlite.db", "Database file path")

	return cmd
}

func runTimersCommand(out io.Writer, dbPath string, now time.Time) error {
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	defer func() { _ = db.Close() }()

	timers, err := db.Timers()
	if err != nil {
		return fmt.Errorf("failed to query timers: %w", err)
	}

	return printTimersTable(out, timers, now)
}

func printTimersTable(out io.Writer, timers []store.Timer, now time.Time) error {
	if len(timers) == 0 {
		fmt.Fprintln(out, "No timers found")

		return nil
	}

	table := tablewriter.NewWriter(out)
	table.Header("Timer", "Status", "Started", "Expires", "Remaining")

	for _, timer := range timers {
		err := table.Append(
			timer.Name,
			string(timer.Status),
			timer.StartedAt.Format(timestampFormat),
			timer.ExpiresAt.Format(timestampFormat),
			formatRemaining(timer, now),
		)
		if err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}

	return table.Render()
}

// formatRemaining returns how long until a running timer fires, "due" if that
// time has passed (the timer fires as soon as HAL is running), or "-" for a
// timer that is not running.
func formatRemaining(timer store.Timer, now time.Time) string {
	if timer.Status != store.TimerStatusRunning {
		return "-"
	}

	remaining := timer.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return "due"
	}

	return remaining.Round(time.Second).String()
}
