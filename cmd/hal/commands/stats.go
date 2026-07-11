package commands

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type MetricSummary struct {
	MetricType store.MetricType
	LastMinute interface{}
	LastHour   interface{}
	LastDay    interface{}
	LastMonth  interface{}
}

type TimePeriod struct {
	Name     string
	Duration time.Duration
}

// NewStatsCmd creates the stats command
func NewStatsCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "stats",
		Aliases: []string{"stat"},
		Short:   "Display HAL metrics statistics",
		Long: `Display comprehensive metrics statistics for HAL automation system.
Shows automation triggers, evaluations, and processing times across multiple time periods.`,
		Example: `  hal stats                    # Show stats using default database
  hal stats --db custom.db     # Show stats from custom database`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatsCommand(dbPath)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "sqlite.db", "Database file path")
	return cmd
}

func runStatsCommand(dbPath string) error {
	// Open database connection
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get stats for all metric types
	metricTypes := []store.MetricType{
		store.MetricTypeAutomationTriggered,
		store.MetricTypeTickProcessingTime,
	}

	var summaries []MetricSummary

	for _, metricType := range metricTypes {
		summary := MetricSummary{MetricType: metricType}

		if strings.Contains(string(metricType), "time") {
			// Timer metrics - calculate p99
			summary.LastMinute = calculateP99(db.DB, metricType, time.Minute)
			summary.LastHour = calculateP99(db.DB, metricType, time.Hour)
			summary.LastDay = calculateP99(db.DB, metricType, 24*time.Hour)
			summary.LastMonth = calculateP99(db.DB, metricType, 30*24*time.Hour)
		} else {
			// Counter metrics - sum
			summary.LastMinute = sumMetrics(db.DB, metricType, time.Minute)
			summary.LastHour = sumMetrics(db.DB, metricType, time.Hour)
			summary.LastDay = sumMetrics(db.DB, metricType, 24*time.Hour)
			summary.LastMonth = sumMetrics(db.DB, metricType, 30*24*time.Hour)
		}

		summaries = append(summaries, summary)
	}

	// Print results in table format
	return printTable(summaries)
}

// sumMetrics returns the total count of a counter metric over the given window.
// The "last minute" window is served entirely from raw points (exact, high
// resolution). Longer windows are served from pre-aggregated rollups plus the raw
// "tail" that has not been rolled up yet (the current minute and anything the
// rollup loop has not caught up on), so the result stays correct and current even
// when rollups are missing or lagging.
func sumMetrics(db *gorm.DB, metricType store.MetricType, duration time.Duration) int64 {
	since := time.Now().Add(-duration)

	if duration <= time.Minute {
		return rawSum(db, metricType, since)
	}

	var rollup struct {
		Total int64
	}
	db.Model(&store.MetricRollup{}).
		Select("COALESCE(SUM(count), 0) as total").
		Where("metric_type = ? AND bucket_start > ?", metricType, since.Unix()).
		Scan(&rollup)

	return rollup.Total + rawSum(db, metricType, rawTailStart(db, metricType, since))
}

// rawSum returns the sum of raw metric values at or after the given time.
func rawSum(db *gorm.DB, metricType store.MetricType, since time.Time) int64 {
	var result struct {
		Total int64
	}
	db.Model(&store.Metric{}).
		Select("COALESCE(SUM(value), 0) as total").
		Where("metric_type = ? AND timestamp >= ?", metricType, since).
		Scan(&result)

	return result.Total
}

// rawTailStart returns the time from which raw points should supplement rollups
// for a window beginning at `since`: the end of the latest rolled-up minute
// within the window, or `since` itself when no rollups cover the window (so the
// window falls back to raw entirely, e.g. before the first backfill has run).
// Because rolled minutes are contiguous up to the watermark, the raw tail never
// overlaps a rollup, so the two never double-count.
func rawTailStart(db *gorm.DB, metricType store.MetricType, since time.Time) time.Time {
	var result struct {
		Max *int64
	}
	db.Model(&store.MetricRollup{}).
		Select("MAX(bucket_start) as max").
		Where("metric_type = ? AND bucket_start > ?", metricType, since.Unix()).
		Scan(&result)

	if result.Max == nil {
		return since
	}

	return time.Unix(*result.Max, 0).Add(time.Minute)
}

// calculateP99 returns the p99 of a timer metric over the given window. The
// "last minute" window computes an exact p99 from raw points; longer windows
// merge the per-minute rollup histograms with the not-yet-rolled raw tail and
// compute the p99 of the merged distribution. Because the histograms share fixed
// bucket boundaries, the merge is exact and the result is a true p99 accurate to
// a bounded relative error.
func calculateP99(db *gorm.DB, metricType store.MetricType, duration time.Duration) string {
	if duration <= time.Minute {
		since := time.Now().Add(-duration)
		var values []int64

		db.Model(&store.Metric{}).
			Select("value").
			Where("metric_type = ? AND timestamp > ?", metricType, since).
			Scan(&values)

		if len(values) == 0 {
			return "0ms"
		}

		sort.Slice(values, func(i, j int) bool {
			return values[i] < values[j]
		})

		index := int(math.Ceil(float64(len(values))*0.99)) - 1
		if index < 0 {
			index = 0
		}

		return formatDuration(time.Duration(values[index]))
	}

	since := time.Now().Add(-duration)
	var rollups []store.MetricRollup

	db.Model(&store.MetricRollup{}).
		Select("histogram").
		Where("metric_type = ? AND bucket_start > ?", metricType, since.Unix()).
		Find(&rollups)

	merged := make(map[int32]int64)
	for _, r := range rollups {
		merged = store.MergeHistograms(merged, r.Histogram)
	}

	// Supplement with raw points not yet captured in a rollup.
	var rawValues []int64
	db.Model(&store.Metric{}).
		Select("value").
		Where("metric_type = ? AND timestamp >= ?", metricType, rawTailStart(db, metricType, since)).
		Scan(&rawValues)
	for _, v := range rawValues {
		merged[store.HistogramBucket(v)]++
	}

	p99 := store.HistogramQuantile(merged, 0.99)
	if p99 == 0 {
		return "0ms"
	}

	return formatDuration(time.Duration(p99))
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.0fns", float64(d))
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d)/float64(time.Microsecond))
	} else if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func formatMetricType(metricType store.MetricType) string {
	switch metricType {
	case store.MetricTypeAutomationTriggered:
		return "Automations Triggered"
	case store.MetricTypeTickProcessingTime:
		return "Tick Processing Time (p99)"
	default:
		return string(metricType)
	}
}

func printTable(summaries []MetricSummary) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintf(w, "Metric\tLast Minute\tLast Hour\tLast Day\tLast Month\n")
	fmt.Fprintf(w, "------\t-----------\t---------\t--------\t----------\n")

	// Print data rows
	for _, summary := range summaries {
		fmt.Fprintf(w, "%s\t%v\t%v\t%v\t%v\n",
			formatMetricType(summary.MetricType),
			summary.LastMinute,
			summary.LastHour,
			summary.LastDay,
			summary.LastMonth,
		)
	}

	return nil
}
