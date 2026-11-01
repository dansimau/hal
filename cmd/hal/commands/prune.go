package commands

import (
	"fmt"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
)

// NewPruneCmd creates the prune command
func NewPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old data from the database",
		Long:  `Delete old data from the HAL database to manage storage and performance.`,
	}

	// Add logs subcommand
	cmd.AddCommand(NewPruneLogsCmd())

	return cmd
}

// NewPruneLogsCmd creates the prune logs command
func NewPruneLogsCmd() *cobra.Command {
	var lastDuration string
	var beforeTime string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Delete old log entries",
		Long: `Delete old log entries from the HAL automation system.
Requires either --last or --before flag to specify which logs to delete.`,
		Example: `  hal prune logs --last 1d                    # Delete logs older than 1 day
  hal prune logs --last 7d                    # Delete logs older than 7 days
  hal prune logs --before "2024-01-01"        # Delete logs before specific date`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Require at least one of --last or --before
			if lastDuration == "" && beforeTime == "" {
				return fmt.Errorf("either --last or --before flag is required")
			}
			// Don't allow both flags
			if lastDuration != "" && beforeTime != "" {
				return fmt.Errorf("cannot use both --last and --before flags")
			}
			return runPruneLogsCommand(lastDuration, beforeTime)
		},
	}

	cmd.Flags().StringVar(&lastDuration, "last", "", "Delete logs older than duration (e.g., 1d, 7d, 30d)")
	cmd.Flags().StringVar(&beforeTime, "before", "", "Delete logs before this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")

	return cmd
}

func runPruneLogsCommand(lastDuration, beforeTime string) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Apply time filters
	var cutoffTime time.Time
	if lastDuration != "" {
		duration, err := parseDuration(lastDuration)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		cutoffTime = time.Now().Add(-duration)
	} else if beforeTime != "" {
		before, err := parseTime(beforeTime)
		if err != nil {
			return fmt.Errorf("invalid before time format: %w", err)
		}
		cutoffTime = before
	}

	// Delete in chunks so a running HAL instance is not locked out of the
	// database for the duration of a large delete
	deleted, err := store.DeleteInChunks(db.DB, &store.Log{}, "timestamp < ?", cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	// Print summary
	fmt.Printf("Deleted %d log entries before %s\n",
		deleted,
		cutoffTime.Format("2006-01-02 15:04:05"),
	)

	return nil
}