package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"crypto/tls"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/inbound"
	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/device/quic"
	"example.com/me/myproxy/internal/device/wss"
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
	"example.com/me/myproxy/internal/plugins/traffic"
	"example.com/me/myproxy/internal/router"
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

	// Initialize Router and OutboundPool
	var rtr router.Router
	var outboundPool *outbound.Pool
	var deviceRegistry *device.Registry
	var wssServer *wss.Server
	var quicServer *quic.Server

	if cfg.OutboundPool != nil && cfg.OutboundPool.Enabled {
		// Initialize Device Registry
		heartbeatInterval := cfg.OutboundPool.HeartbeatInterval
		if heartbeatInterval == 0 {
			heartbeatInterval = 30 // default
		}
		heartbeatTimeout := cfg.OutboundPool.HeartbeatTimeout
		if heartbeatTimeout == 0 {
			heartbeatTimeout = 90 // default
		}

		deviceRegistry = device.NewRegistry(heartbeatInterval, heartbeatTimeout)

		// Initialize OutboundPool
		outboundPool = outbound.NewPool(deviceRegistry)

		// Initialize Dynamic Router with RoundRobin strategy
		strategy := router.NewRoundRobinStrategy()
		rtr = router.NewDynamicRouter(deviceRegistry, strategy)

		// Prepare TLS config if enabled
		var tlsConfig *tls.Config
		if cfg.OutboundPool.TLS != nil && cfg.OutboundPool.TLS.Enabled {
			if cfg.OutboundPool.TLS.CertFile != "" && cfg.OutboundPool.TLS.KeyFile != "" {
				cert, err := tls.LoadX509KeyPair(cfg.OutboundPool.TLS.CertFile, cfg.OutboundPool.TLS.KeyFile)
				if err != nil {
					log.Fatalf("Failed to load TLS certificate: %v", err)
				}
				tlsConfig = &tls.Config{
					Certificates: []tls.Certificate{cert},
				}
			}
		}

		// Initialize WSS server for control-plane
		wssPort := cfg.OutboundPool.WSSPort
		if wssPort == 0 {
			wssPort = 443 // default
		}
		wssServer = wss.NewServer(deviceRegistry, wssPort, tlsConfig)

		// Initialize QUIC server for data-plane
		quicPort := cfg.OutboundPool.QUICPort
		if quicPort == 0 {
			quicPort = 443 // default
		}
		quicServer = quic.NewServer(deviceRegistry, quicPort, tlsConfig)

		logger.Info("main", "Outbound pool enabled: WSS control-plane on port %d, QUIC data-plane on port %d", wssPort, quicPort)
	} else {
		// Use static router
		rtr = router.NewStaticRouter()
	}

	// Initialize Plugin Manager
	pluginManager := plugin.NewManager()

	// Load and initialize plugins
	if cfg.Plugins.TrafficInbound != nil && cfg.Plugins.TrafficInbound.Enabled {
		trafficInbound := traffic.NewInboundCounter()
		if err := trafficInbound.Init(cfg.Plugins.TrafficInbound.Config); err != nil {
			log.Fatalf("Failed to initialize traffic_inbound plugin: %v", err)
		}
		pluginManager.RegisterInboundPlugin(trafficInbound)
		pluginManager.RegisterTrafficPlugin(trafficInbound)
		logger.Info("main", "Traffic inbound plugin enabled")
	}

	if cfg.Plugins.TrafficOutbound != nil && cfg.Plugins.TrafficOutbound.Enabled {
		trafficOutbound := traffic.NewOutboundCounter()
		if err := trafficOutbound.Init(cfg.Plugins.TrafficOutbound.Config); err != nil {
			log.Fatalf("Failed to initialize traffic_outbound plugin: %v", err)
		}
		pluginManager.RegisterOutboundPlugin(trafficOutbound)
		pluginManager.RegisterTrafficPlugin(trafficOutbound)
		logger.Info("main", "Traffic outbound plugin enabled")
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
	handler := func(conn net.Conn, targetAddress string, ctx *plugin.ConnectionContext) error {
		if targetAddress == "" {
			return fmt.Errorf("target address not specified")
		}
		// Устанавливаем InboundID из конфигурации
		ctx.InboundID = cfg.Inbound.ID
		return proxy.HandleConnection(conn, ob, cfg.Outbound.ID, &cfg.Outbound, targetAddress, cfg.Inbound.ID, rtr, pluginManager, outboundPool)
	}

	// Start inbound
	if err := ib.Start(handler); err != nil {
		log.Fatalf("Failed to start inbound: %v", err)
	}

	// Start WSS server if enabled
	if wssServer != nil {
		go func() {
			if err := wssServer.Start(); err != nil {
				log.Fatalf("Failed to start WSS server: %v", err)
			}
		}()
	}

	// Start QUIC server if enabled
	if quicServer != nil {
		go func() {
			if err := quicServer.Start(); err != nil {
				log.Fatalf("Failed to start QUIC server: %v", err)
			}
		}()
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
	if wssServer != nil {
		if err := wssServer.Stop(); err != nil {
			logger.Error("main", "Error stopping WSS server: %v", err)
		}
	}
	if quicServer != nil {
		if err := quicServer.Stop(); err != nil {
			logger.Error("main", "Error stopping QUIC server: %v", err)
		}
	}
	if deviceRegistry != nil {
		if err := deviceRegistry.Close(); err != nil {
			logger.Error("main", "Error closing device registry: %v", err)
		}
	}
	if err := pluginManager.Close(); err != nil {
		logger.Error("main", "Error closing plugins: %v", err)
	}
}

