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
	"gorm.io/gorm"
)

type MetricSummary struct {
	MetricType string
	LastMinute interface{}
	LastHour   interface{}
	LastDay    interface{}
	LastMonth  interface{}
}

type TimePeriod struct {
	Name     string
	Duration time.Duration
}

var timePeriods = []TimePeriod{
	{"Last Minute", time.Minute},
	{"Last Hour", time.Hour},
	{"Last Day", 24 * time.Hour},
	{"Last Month", 30 * 24 * time.Hour},
}

func RunStatsCommand(args ...string) error {
	// Open database connection
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get stats for all metric types
	metricTypes := []string{
		store.MetricTypeAutomationTriggered,
		store.MetricTypeAutomationEvaluated,
		store.MetricTypeTickProcessingTime,
	}

	var summaries []MetricSummary

	for _, metricType := range metricTypes {
		summary := MetricSummary{MetricType: metricType}

		if strings.Contains(metricType, "time") {
			// Timer metrics - calculate p99
			summary.LastMinute = calculateP99(db, metricType, time.Minute)
			summary.LastHour = calculateP99(db, metricType, time.Hour)
			summary.LastDay = calculateP99(db, metricType, 24*time.Hour)
			summary.LastMonth = calculateP99(db, metricType, 30*24*time.Hour)
		} else {
			// Counter metrics - sum
			summary.LastMinute = sumMetrics(db, metricType, time.Minute)
			summary.LastHour = sumMetrics(db, metricType, time.Hour)
			summary.LastDay = sumMetrics(db, metricType, 24*time.Hour)
			summary.LastMonth = sumMetrics(db, metricType, 30*24*time.Hour)
		}

		summaries = append(summaries, summary)
	}

	// Print results in table format
	return printTable(summaries)
}

func sumMetrics(db *gorm.DB, metricType string, duration time.Duration) int64 {
	since := time.Now().Add(-duration)
	var result struct {
		Total int64
	}

	db.Model(&store.Metric{}).
		Select("COALESCE(SUM(value), 0) as total").
		Where("metric_type = ? AND timestamp > ?", metricType, since).
		Scan(&result)

	return result.Total
}

func calculateP99(db *gorm.DB, metricType string, duration time.Duration) string {
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

	// Calculate p99 (99th percentile)
	index := int(math.Ceil(float64(len(values))*0.99)) - 1
	if index < 0 {
		index = 0
	}

	p99Nanos := values[index]
	return formatDuration(time.Duration(p99Nanos))
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

func formatMetricType(metricType string) string {
	switch metricType {
	case store.MetricTypeAutomationTriggered:
		return "Automations Triggered"
	case store.MetricTypeAutomationEvaluated:
		return "Automations Evaluated"
	case store.MetricTypeTickProcessingTime:
		return "Tick Processing Time (p99)"
	default:
		return metricType
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