package main

import (
	"os"

	"github.com/dansimau/hal/cmd/hal/commands"
	"github.com/spf13/cobra"
)

func main() {
	err := newRootCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the root command with all subcommands attached.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "hal",
		Short: "HAL - Home Automation Library CLI",
		Long: `HAL is a comprehensive home automation library that connects to Home Assistant
and provides powerful automation capabilities with built-in metrics and monitoring.`,
	}

	rootCmd.AddCommand(commands.NewStatsCmd())
	rootCmd.AddCommand(commands.NewLogsCmd())
	rootCmd.AddCommand(commands.NewEntitiesCmd())
	rootCmd.AddCommand(commands.NewPruneCmd())
	rootCmd.AddCommand(commands.NewEventsCmd())

	return rootCmd
}
