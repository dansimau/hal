package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dansimau/hal/cmd/hal/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "stat", "stats":
		if err := commands.RunStatsCommand(os.Args[2:]...); err != nil {
			log.Fatalf("Error running stats command: %v", err)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("HAL - Home Automation Library CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  hal <command> [options]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  stat, stats    Display metrics statistics")
	fmt.Println("  help           Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  hal stats      Show metrics for all time periods")
}