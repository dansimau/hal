package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gorm.io/gorm"
)

func setupTestDBForStats(t *testing.T) *gorm.DB {
	t.Helper()
	
	db, err := store.Open(":memory:")
	assert.NilError(t, err)
	
	return db
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	f()
	
	w.Close()
	os.Stdout = old
	
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNewStatsCmd(t *testing.T) {
	cmd := NewStatsCmd()
	
	assert.Equal(t, cmd.Use, "stats")
	assert.Assert(t, len(cmd.Aliases) > 0)
	assert.Equal(t, cmd.Aliases[0], "stat")
	assert.Assert(t, strings.Contains(cmd.Short, "metrics"))
	assert.Assert(t, strings.Contains(cmd.Long, "automation"))
}

func TestStatsCommandWithEmptyDatabase(t *testing.T) {
	// Create temporary database file for testing
	tempFile := t.TempDir() + "/test.db"
	tempDB, err := store.Open(tempFile)
	assert.NilError(t, err)
	
	// Close the temp database so the command can open it
	sqlDB, err := tempDB.DB()
	assert.NilError(t, err)
	sqlDB.Close()
	
	// Capture output
	output := captureOutput(func() {
		err := runStatsCommand(tempFile)
		assert.NilError(t, err)
	})
	
	// Verify expected output format
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Assert(t, len(lines) >= 3) // Header, separator, at least one data row
	
	// Check header
	assert.Assert(t, strings.Contains(lines[0], "Metric"))
	assert.Assert(t, strings.Contains(lines[0], "Last Minute"))
	assert.Assert(t, strings.Contains(lines[0], "Last Hour"))
	
	// Check that empty database shows zeros
	assert.Assert(t, strings.Contains(output, "0"))
	assert.Assert(t, strings.Contains(output, "0ms"))
}

func TestStatsCommandWithSampleData(t *testing.T) {
	// Create temporary database file
	tempFile := t.TempDir() + "/test_with_data.db"
	db, err := store.Open(tempFile)
	assert.NilError(t, err)
	
	// Insert sample metrics
	now := time.Now()
	sampleMetrics := []store.Metric{
		{
			Timestamp:      now.Add(-30 * time.Second),
			MetricType:     store.MetricTypeAutomationTriggered,
			Value:          1,
			EntityID:       "test.light",
			AutomationName: "motion_light",
		},
		{
			Timestamp:      now.Add(-2 * time.Minute),
			MetricType:     store.MetricTypeAutomationEvaluated,
			Value:          1,
			EntityID:       "test.sensor",
			AutomationName: "",
		},
		{
			Timestamp:      now.Add(-1 * time.Minute),
			MetricType:     store.MetricTypeTickProcessingTime,
			Value:          (150 * time.Millisecond).Nanoseconds(),
			EntityID:       "test.sensor",
			AutomationName: "",
		},
		{
			Timestamp:      now.Add(-10 * time.Minute),
			MetricType:     store.MetricTypeTickProcessingTime,
			Value:          (75 * time.Millisecond).Nanoseconds(),
			EntityID:       "test.light",
			AutomationName: "",
		},
	}
	
	for _, metric := range sampleMetrics {
		assert.NilError(t, db.Create(&metric).Error)
	}
	
	// Close database so command can open it
	sqlDB, err := db.DB()
	assert.NilError(t, err)
	sqlDB.Close()
	
	// Capture output
	output := captureOutput(func() {
		err := runStatsCommand(tempFile)
		assert.NilError(t, err)
	})
	
	// Verify output contains expected data
	assert.Assert(t, strings.Contains(output, "Automations Triggered"))
	assert.Assert(t, strings.Contains(output, "Automations Evaluated"))
	assert.Assert(t, strings.Contains(output, "Tick Processing Time"))
	
	// Should show non-zero values for recent metrics
	lines := strings.Split(output, "\n")
	foundTriggerLine := false
	foundTimeLine := false
	
	for _, line := range lines {
		if strings.Contains(line, "Automations Triggered") {
			foundTriggerLine = true
			// Should show 1 trigger in last minute - check for any non-zero number
			assert.Assert(t, strings.Contains(line, "1") || strings.Contains(line, "2"), "Expected trigger count in line: %s", line)
		}
		if strings.Contains(line, "Tick Processing Time") {
			foundTimeLine = true
			// Should show processing time in milliseconds
			assert.Assert(t, strings.Contains(line, "ms"), "Expected ms in time line: %s", line)
		}
	}
	
	assert.Assert(t, foundTriggerLine, "Expected to find Automations Triggered line")
	assert.Assert(t, foundTimeLine, "Expected to find Tick Processing Time line")
}

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Nanosecond, "500ns"},
		{1500 * time.Nanosecond, "1.5μs"},
		{time.Duration(2500), "2.5μs"},
		{time.Duration(1.5 * float64(time.Millisecond)), "1.5ms"},
		{150 * time.Millisecond, "150.0ms"},
		{2500 * time.Millisecond, "2.50s"},
	}
	
	for _, tc := range testCases {
		result := formatDuration(tc.duration)
		assert.Equal(t, result, tc.expected, "Duration: %v", tc.duration)
	}
}

func TestFormatMetricType(t *testing.T) {
	testCases := []struct {
		metricType string
		expected   string
	}{
		{store.MetricTypeAutomationTriggered, "Automations Triggered"},
		{store.MetricTypeAutomationEvaluated, "Automations Evaluated"},
		{store.MetricTypeTickProcessingTime, "Tick Processing Time (p99)"},
		{"unknown_metric", "unknown_metric"},
	}
	
	for _, tc := range testCases {
		result := formatMetricType(tc.metricType)
		assert.Equal(t, result, tc.expected)
	}
}

func TestSumMetrics(t *testing.T) {
	db := setupTestDBForStats(t)
	
	now := time.Now()
	// Insert metrics within and outside the time window
	metrics := []store.Metric{
		{
			Timestamp:  now.Add(-30 * time.Second), // Within 1 minute
			MetricType: store.MetricTypeAutomationTriggered,
			Value:      1,
		},
		{
			Timestamp:  now.Add(-45 * time.Second), // Within 1 minute
			MetricType: store.MetricTypeAutomationTriggered,
			Value:      1,
		},
		{
			Timestamp:  now.Add(-2 * time.Minute), // Outside 1 minute window
			MetricType: store.MetricTypeAutomationTriggered,
			Value:      1,
		},
	}
	
	for _, metric := range metrics {
		assert.NilError(t, db.Create(&metric).Error)
	}
	
	// Test sum for last minute (should be 2)
	result := sumMetrics(db, store.MetricTypeAutomationTriggered, time.Minute)
	assert.Equal(t, result, int64(2))
	
	// Test sum for last 5 minutes (should be 3)
	result = sumMetrics(db, store.MetricTypeAutomationTriggered, 5*time.Minute)
	assert.Equal(t, result, int64(3))
}

func TestCalculateP99(t *testing.T) {
	db := setupTestDBForStats(t)
	
	now := time.Now()
	// Insert timer metrics with various values
	values := []int64{
		(10 * time.Millisecond).Nanoseconds(),
		(20 * time.Millisecond).Nanoseconds(),
		(30 * time.Millisecond).Nanoseconds(),
		(40 * time.Millisecond).Nanoseconds(),
		(100 * time.Millisecond).Nanoseconds(), // This should be p99
	}
	
	for _, value := range values {
		metric := store.Metric{
			Timestamp:  now.Add(-30 * time.Second),
			MetricType: store.MetricTypeTickProcessingTime,
			Value:      value,
		}
		assert.NilError(t, db.Create(&metric).Error)
	}
	
	// Calculate p99
	result := calculateP99(db, store.MetricTypeTickProcessingTime, time.Minute)
	
	// Should return the highest value (100ms) as p99
	assert.Equal(t, result, "100.0ms")
}

func TestCalculateP99EmptyDataset(t *testing.T) {
	db := setupTestDBForStats(t)
	
	// Test with empty dataset
	result := calculateP99(db, store.MetricTypeTickProcessingTime, time.Minute)
	assert.Equal(t, result, "0ms")
}

func TestStatsCommandCobraIntegration(t *testing.T) {
	cmd := NewStatsCmd()
	
	// Test that command can be executed (though it may fail due to missing database)
	rootCmd := &cobra.Command{Use: "test"}
	rootCmd.AddCommand(cmd)
	
	// Test help output
	rootCmd.SetArgs([]string{"stats", "--help"})
	err := rootCmd.Execute()
	assert.NilError(t, err)
}