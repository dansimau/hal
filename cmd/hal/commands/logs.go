package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	// logsCmdUse is the name of the "logs" subcommand, shared by `hal logs` and
	// `hal prune logs`.
	logsCmdUse = "logs"

	// Log level names as stored in the database (slog.Level.String()).
	levelError = "ERROR"
	levelWarn  = "WARN"
	levelInfo  = "INFO"
	levelDebug = "DEBUG"
)

// errUnparsableTime is returned when a --from/--to/--before value matches none
// of the supported time formats.
var errUnparsableTime = errors.New("unable to parse time")

// NewLogsCmd creates the logs command.
func NewLogsCmd() *cobra.Command {
	var (
		fromTime     string
		toTime       string
		lastDuration string
		entityID     string
		noColor      bool
	)

	cmd := &cobra.Command{
		Use:     logsCmdUse,
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogsCommand(cmd.OutOrStdout(), fromTime, toTime, lastDuration, entityID, noColor)
		},
	}

	cmd.Flags().StringVar(&fromTime, "from", "", "Start time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&toTime, "to", "", "End time for filtering logs (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&lastDuration, "last", "", "Show logs from last duration (e.g., 5m, 1h, 2d)")
	cmd.Flags().StringVar(&entityID, "entity-id", "", "Filter logs by entity ID")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	return cmd
}

func runLogsCommand(out io.Writer, fromTime, toTime, lastDuration, entityID string, noColor bool) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Build query with filters
	query, err := applyTimeFilters(db.Model(&store.Log{}), fromTime, toTime, lastDuration)
	if err != nil {
		return err
	}

	// Apply entity filter
	if entityID != "" {
		query = query.Where("entity_id = ?", entityID)
	}

	// Execute query and get results
	var logs []store.Log

	err = query.Order("timestamp ASC").Find(&logs).Error
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	// Print results
	return printLogs(out, logs, noColor)
}

// applyTimeFilters narrows query to the requested time range. --last takes
// precedence over --from/--to.
func applyTimeFilters(query *gorm.DB, fromTime, toTime, lastDuration string) (*gorm.DB, error) {
	if lastDuration != "" {
		duration, err := parseDuration(lastDuration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration format: %w", err)
		}

		since := time.Now().Add(-duration)

		return query.Where("timestamp > ?", since), nil
	}

	if fromTime != "" {
		from, err := parseTime(fromTime)
		if err != nil {
			return nil, fmt.Errorf("invalid from time format: %w", err)
		}

		query = query.Where("timestamp >= ?", from)
	}

	if toTime != "" {
		to, err := parseTime(toTime)
		if err != nil {
			return nil, fmt.Errorf("invalid to time format: %w", err)
		}

		query = query.Where("timestamp <= ?", to)
	}

	return query, nil
}

func parseDuration(durationStr string) (time.Duration, error) {
	// Handle common duration formats like 5m, 1h, 2d
	if before, ok := strings.CutSuffix(durationStr, "d"); ok {
		days, err := strconv.Atoi(before)
		if err != nil {
			return 0, err
		}

		return time.Duration(days) * day, nil
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
		parsed, err := time.Parse(format, timeStr)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf(
		"%w: %s (expected formats: YYYY-MM-DD, YYYY-MM-DD HH:MM, or YYYY-MM-DD HH:MM:SS)",
		errUnparsableTime, timeStr,
	)
}

func printLogs(out io.Writer, logs []store.Log, noColor bool) error {
	if len(logs) == 0 {
		fmt.Fprintln(out, "No logs found")

		return nil
	}

	// Determine if colors should be enabled
	useColor := !noColor && isTerminal(out)
	if !useColor {
		color.NoColor = true
	}

	colorizer := newLogColorizer()

	// Print logs without header to look like a log file
	for _, log := range logs {
		// Format timestamp in dark grey
		timestamp := log.Timestamp.Format("2006-01-02 15:04:05")
		if useColor {
			timestamp = colorizer.darkGrey(timestamp)
		}

		// Format log level with appropriate color
		level := log.Level
		if useColor {
			level = colorizer.level(level)
		}

		// Format entity ID in dark grey
		entityIDStr := ""

		if log.EntityID != "" {
			if useColor {
				entityIDStr = fmt.Sprintf(" [%s]", colorizer.darkGrey(log.EntityID))
			} else {
				entityIDStr = fmt.Sprintf(" [%s]", log.EntityID)
			}
		}

		// Format log text with colorized tags; for multiline entries (e.g. diffs)
		// colorize the first line's tags and diff lines separately.
		logText := log.LogText
		if useColor {
			logText = colorizer.text(logText)
		}

		fmt.Fprintf(out, "%s %s%s %s\n",
			timestamp,
			level,
			entityIDStr,
			logText,
		)
	}

	return nil
}

// isTerminal reports whether out is an interactive terminal.
func isTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)

	return ok && isatty.IsTerminal(file.Fd())
}

// logColorizer colorizes the parts of a rendered log line.
type logColorizer struct {
	darkGrey func(a ...any) string
	red      func(a ...any) string
	yellow   func(a ...any) string
	green    func(a ...any) string

	// tagRegex matches tag keys like foo=
	tagRegex *regexp.Regexp
}

func newLogColorizer() *logColorizer {
	return &logColorizer{
		darkGrey: color.New(color.FgHiBlack).SprintFunc(),
		red:      color.New(color.FgRed).SprintFunc(),
		yellow:   color.New(color.FgYellow).SprintFunc(),
		green:    color.New(color.FgGreen).SprintFunc(),
		tagRegex: regexp.MustCompile(`(\w+)=`),
	}
}

// level colors a log level name according to its severity.
func (c *logColorizer) level(level string) string {
	switch level {
	case levelError:
		return c.red(level)
	case levelWarn:
		return c.yellow(level)
	case levelInfo:
		return c.green(level)
	case levelDebug:
		return c.darkGrey(level)
	default:
		return c.darkGrey(level)
	}
}

// text colorizes the tags on the first line of a log entry and, for multiline
// entries, the remaining lines as either JSON or a diff.
func (c *logColorizer) text(logText string) string {
	firstLine, rest, hasRest := strings.Cut(logText, "\n")

	firstLine = c.tagRegex.ReplaceAllStringFunc(firstLine, func(match string) string {
		return c.darkGrey(match)
	})

	if !hasRest {
		return firstLine
	}

	return firstLine + "\n" + c.rest(rest)
}

// rest colorizes the lines after the first: stored plain JSON is re-colorised,
// anything else is treated as a diff.
func (c *logColorizer) rest(rest string) string {
	trimmed := strings.TrimSpace(rest)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return c.json(rest)
	}

	return c.diff(rest)
}

// json re-colorises stored plain JSON, returning the input unchanged when it
// does not parse.
func (c *logColorizer) json(rest string) string {
	var value any

	err := json.Unmarshal([]byte(rest), &value)
	if err != nil {
		return rest
	}

	formatter := prettyjson.NewFormatter()
	formatter.Indent = 2

	formatted, err := formatter.Marshal(value)
	if err != nil {
		return rest
	}

	return string(formatted)
}

// diff colors added lines green and removed lines red.
func (c *logColorizer) diff(rest string) string {
	diffLines := strings.Split(rest, "\n")
	for i, line := range diffLines {
		switch {
		case strings.HasPrefix(line, "+"):
			diffLines[i] = c.green(line)
		case strings.HasPrefix(line, "-"):
			diffLines[i] = c.red(line)
		}
	}

	return strings.Join(diffLines, "\n")
}
