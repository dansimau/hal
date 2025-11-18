package commands

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// NewLogsCmd creates the logs command
func NewLogsCmd() *cobra.Command {
	var fromTime string
	var toTime string
	var lastDuration string
	var entityID string
	var noColor bool

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
			return runLogsCommand(fromTime, toTime, lastDuration, entityID, noColor)
		},
	}

	cmd.Flags().StringVar(&fromTime, "from", "", "Start time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&toTime, "to", "", "End time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&lastDuration, "last", "", "Show logs from last duration (e.g., 5m, 1h, 2d)")
	cmd.Flags().StringVar(&entityID, "entity-id", "", "Filter logs by entity ID")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	return cmd
}

func runLogsCommand(fromTime, toTime, lastDuration, entityID string, noColor bool) error {
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
	return printLogs(logs, noColor)
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

func printLogs(logs []store.Log, noColor bool) error {
	if len(logs) == 0 {
		fmt.Println("No logs found")
		return nil
	}

	// Determine if colors should be enabled
	useColor := !noColor && isatty.IsTerminal(os.Stdout.Fd())
	if !useColor {
		color.NoColor = true
	}

	// Define color functions
	darkGrey := color.New(color.FgHiBlack).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	// Regex to match tag keys like foo=
	tagRegex := regexp.MustCompile(`(\w+)=`)

	// Print logs without header to look like a log file
	for _, log := range logs {
		// Format timestamp in dark grey
		timestamp := log.Timestamp.Format("2006-01-02 15:04:05")
		if useColor {
			timestamp = darkGrey(timestamp)
		}

		// Format log level with appropriate color
		level := log.Level
		if useColor {
			switch level {
			case "ERROR":
				level = red(level)
			case "WARN":
				level = yellow(level)
			case "INFO":
				level = green(level)
			case "DEBUG":
				level = darkGrey(level)
			default:
				level = darkGrey(level)
			}
		}

		// Format entity ID in dark grey
		entityIDStr := ""
		if log.EntityID != "" {
			if useColor {
				entityIDStr = fmt.Sprintf(" [%s]", darkGrey(log.EntityID))
			} else {
				entityIDStr = fmt.Sprintf(" [%s]", log.EntityID)
			}
		}

		// Format log text with colorized tags
		logText := log.LogText
		if useColor {
			logText = tagRegex.ReplaceAllStringFunc(logText, func(match string) string {
				return darkGrey(match)
			})
		}

		fmt.Printf("%s %s%s %s\n",
			timestamp,
			level,
			entityIDStr,
			logText,
		)
	}

	return nil
}
