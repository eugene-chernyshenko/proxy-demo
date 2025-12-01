package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/inbound"
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/outbound"
	"example.com/me/myproxy/proxy"
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

	// Initialize outbound
	var ob outbound.Outbound
	switch cfg.Outbound.Type {
	case "direct":
		ob = outbound.NewDirectOutbound()
	case "socks5":
		if cfg.Outbound.ProxyAddress == "" {
			log.Fatalf("proxy_address is required for SOCKS5 outbound")
		}
		ob = outbound.NewSOCKS5Outbound(cfg.Outbound.ProxyAddress)
	default:
		log.Fatalf("Unsupported outbound type: %s", cfg.Outbound.Type)
	}

	// Initialize inbound
	var ib inbound.Inbound
	switch cfg.Inbound.Type {
	case "socks5":
		ib = inbound.NewSOCKS5Inbound(cfg.Inbound.Port)
	default:
		log.Fatalf("Unsupported inbound type: %s", cfg.Inbound.Type)
	}

	// Connection handler
	handler := func(conn net.Conn, targetAddress string) error {
		if targetAddress == "" {
			return fmt.Errorf("target address not specified")
		}
		return proxy.HandleConnection(conn, ob.Dial, targetAddress)
	}

	// Start inbound
	if err := ib.Start(handler); err != nil {
		log.Fatalf("Failed to start inbound: %v", err)
	}

	outboundType := cfg.Outbound.Type
	if outboundType == "socks5" {
		logger.Info("main", "SOCKS5 proxy started on port %d (SOCKS5 outbound via %s)", cfg.Inbound.Port, cfg.Outbound.ProxyAddress)
	} else {
		logger.Info("main", "SOCKS5 proxy started on port %d (%s outbound)", cfg.Inbound.Port, outboundType)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("main", "Shutting down proxy...")
	if err := ib.Stop(); err != nil {
		logger.Error("main", "Error stopping inbound: %v", err)
	}
}

