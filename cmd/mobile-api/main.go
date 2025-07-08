package main

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/dansimau/hal/mobile_api"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to run the mobile API server on")
	flag.Parse()

	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create and start the mobile API server
	server := mobile_api.NewMobileAPIServer(*port)

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		slog.Info("Received shutdown signal")
		if err := server.Stop(); err != nil {
			slog.Error("Error stopping server", "error", err)
		}
		os.Exit(0)
	}()

	// Start the server
	slog.Info("Starting HAL Mobile API Server", "port", *port)
	if err := server.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}