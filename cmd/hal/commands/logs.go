package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
)

// NewLogsCmd creates the logs command
func NewLogsCmd() *cobra.Command {
	var fromTime string
	var toTime string
	var lastDuration string
	var entityID string

	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"log"},
		Short:   "Display HAL log entries",
		Long: `Display log entries from the HAL automation system.
Shows logs in chronological order with optional filtering by time range or entity.`,
		Example: `  hal logs                           # Show all logs in chronological order
  hal logs --from "2024-01-01"         # Show logs from a specific date
  hal logs --to "2024-01-31"           # Show logs up to a specific date
  hal logs --from "2024-01-01" --to "2024-01-31"  # Show logs in date range
  hal logs --last 5m                   # Show logs from last 5 minutes
  hal logs --last 1h                   # Show logs from last 1 hour
  hal logs --last 1d                   # Show logs from last 1 day
  hal logs --entity-id "light.kitchen" # Show logs for specific entity`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogsCommand(fromTime, toTime, lastDuration, entityID)
		},
	}

	cmd.Flags().StringVar(&fromTime, "from", "", "Start time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&toTime, "to", "", "End time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&lastDuration, "last", "", "Show logs from last duration (e.g., 5m, 1h, 2d)")
	cmd.Flags().StringVar(&entityID, "entity-id", "", "Filter logs by entity ID")

	// Add prune subcommand
	cmd.AddCommand(NewLogsPruneCmd())

	return cmd
}

func runLogsCommand(fromTime, toTime, lastDuration, entityID string) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Build query with filters
	query := db.Model(&store.Log{})

	// Apply time filters
	if lastDuration != "" {
		duration, err := parseDuration(lastDuration)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		since := time.Now().Add(-duration)
		query = query.Where("timestamp > ?", since)
	} else {
		if fromTime != "" {
			from, err := parseTime(fromTime)
			if err != nil {
				return fmt.Errorf("invalid from time format: %w", err)
			}
			query = query.Where("timestamp >= ?", from)
		}
		if toTime != "" {
			to, err := parseTime(toTime)
			if err != nil {
				return fmt.Errorf("invalid to time format: %w", err)
			}
			query = query.Where("timestamp <= ?", to)
		}
	}

	// Apply entity filter
	if entityID != "" {
		query = query.Where("entity_id = ?", entityID)
	}

	// Execute query and get results
	var logs []store.Log
	if err := query.Order("timestamp ASC").Find(&logs).Error; err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	// Print results
	return printLogs(logs)
}

func parseDuration(durationStr string) (time.Duration, error) {
	// Handle common duration formats like 5m, 1h, 2d
	if strings.HasSuffix(durationStr, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(durationStr, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	
	// For other formats (m, h, s), use standard time.ParseDuration
	return time.ParseDuration(durationStr)
}

func parseTime(timeStr string) (time.Time, error) {
	// Try different time formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse time: %s (expected formats: YYYY-MM-DD, YYYY-MM-DD HH:MM, or YYYY-MM-DD HH:MM:SS)", timeStr)
}

func printLogs(logs []store.Log) error {
	if len(logs) == 0 {
		fmt.Println("No logs found")
		return nil
	}

	// Print logs without header to look like a log file
	for _, log := range logs {
		entityIDStr := ""
		if log.EntityID != "" {
			entityIDStr = fmt.Sprintf(" [%s]", log.EntityID)
		}
		
		fmt.Printf("%s%s %s\n",
			log.Timestamp.Format("2006-01-02 15:04:05"),
			entityIDStr,
			log.LogText,
		)
	}

	return nil
}

// NewLogsPruneCmd creates the logs prune command
func NewLogsPruneCmd() *cobra.Command {
	var lastDuration string
	var beforeTime string

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old log entries",
		Long: `Delete old log entries from the HAL automation system.
Requires either --last or --before flag to specify which logs to delete.`,
		Example: `  hal logs prune --last 1d                    # Delete logs older than 1 day
  hal logs prune --last 7d                    # Delete logs older than 7 days
  hal logs prune --before "2024-01-01"        # Delete logs before specific date`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Require at least one of --last or --before
			if lastDuration == "" && beforeTime == "" {
				return fmt.Errorf("either --last or --before flag is required")
			}
			// Don't allow both flags
			if lastDuration != "" && beforeTime != "" {
				return fmt.Errorf("cannot use both --last and --before flags")
			}
			return runLogsPruneCommand(lastDuration, beforeTime)
		},
	}

	cmd.Flags().StringVar(&lastDuration, "last", "", "Delete logs older than duration (e.g., 1d, 7d, 30d)")
	cmd.Flags().StringVar(&beforeTime, "before", "", "Delete logs before this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")

	return cmd
}

func runLogsPruneCommand(lastDuration, beforeTime string) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Build delete query with filters
	query := db.Model(&store.Log{})

	// Apply time filters
	var cutoffTime time.Time
	if lastDuration != "" {
		duration, err := parseDuration(lastDuration)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		cutoffTime = time.Now().Add(-duration)
		query = query.Where("timestamp < ?", cutoffTime)
	} else if beforeTime != "" {
		before, err := parseTime(beforeTime)
		if err != nil {
			return fmt.Errorf("invalid before time format: %w", err)
		}
		cutoffTime = before
		query = query.Where("timestamp < ?", before)
	}

	// Execute delete
	result := query.Delete(&store.Log{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete logs: %w", result.Error)
	}

	// Print summary
	fmt.Printf("Deleted %d log entries before %s\n",
		result.RowsAffected,
		cutoffTime.Format("2006-01-02 15:04:05"),
	)

	return nil
}
