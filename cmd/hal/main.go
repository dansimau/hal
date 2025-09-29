package main

import (
	"os"

	"github.com/dansimau/hal/cmd/hal/commands"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hal",
	Short: "HAL - Home Automation Library CLI",
	Long: `HAL is a comprehensive home automation library that connects to Home Assistant
and provides powerful automation capabilities with built-in metrics and monitoring.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(commands.NewStatsCmd())
	rootCmd.AddCommand(commands.NewLogsCmd())
	rootCmd.AddCommand(commands.NewEntitiesCmd())
	rootCmd.AddCommand(commands.NewPruneCmd())
}