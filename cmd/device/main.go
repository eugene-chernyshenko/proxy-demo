package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/internal/device/client"
	"example.com/me/myproxy/internal/logger"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")

	// Load configuration (this will call flag.Parse())
	cfg, err := config.LoadDeviceConfig()
	if err != nil {
		log.Fatalf("Failed to load device configuration: %v", err)
	}

	// Set debug level after flags are parsed
	if debug {
		logger.SetLevel(logger.LevelDebug)
		logger.Debug("main", "Debug logging enabled")
	}

	// Create device client
	deviceClient := client.NewClient(
		cfg.ProxyHost,
		cfg.WSSPort,
		cfg.QUICPort,
		cfg.DeviceID,
		cfg.TLSEnabled,
		cfg.TLSSkipVerify,
	)

	// Start device client
	if err := deviceClient.Start(cfg.Location, cfg.Tags, cfg.HeartbeatInterval); err != nil {
		log.Fatalf("Failed to start device client: %v", err)
	}

	logger.Info("main", "Device client started for device %s", cfg.DeviceID)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("main", "Shutting down device client...")
	if err := deviceClient.Stop(); err != nil {
		logger.Error("main", "Error stopping device client: %v", err)
	}
}

