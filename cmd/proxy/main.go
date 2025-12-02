package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/server"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")

	// Load configuration (this will call flag.Parse())
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set debug level after flags are parsed
	if debug {
		logger.SetLevel(logger.LevelDebug)
		logger.Debug("main", "Debug logging enabled")
	}

	// Create and initialize server
	srv := server.NewServer(cfg)
	if err := srv.Initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start server
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("main", "Shutting down proxy...")
	if err := srv.Stop(); err != nil {
		logger.Error("main", "Error stopping server: %v", err)
	}
}

