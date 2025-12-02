package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/inbound"
	"example.com/me/myproxy/internal/constants"
	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/device/quic"
	"example.com/me/myproxy/internal/device/wss"
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
	"example.com/me/myproxy/internal/plugins/traffic"
	"example.com/me/myproxy/internal/router"
	tlsconfig "example.com/me/myproxy/internal/tls"
	"example.com/me/myproxy/outbound"
	"example.com/me/myproxy/proxy"
)

// Server представляет proxy server
type Server struct {
	cfg            *config.Config
	inbound        inbound.Inbound
	outbound       outbound.Outbound
	pluginManager  *plugin.Manager
	router         router.Router
	outboundPool   *outbound.Pool
	deviceRegistry *device.Registry
	wssServer      *wss.Server
	quicServer     *quic.Server
}

// NewServer создает новый server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

// Initialize инициализирует все компоненты server
func (s *Server) Initialize() error {
	// Initialize Router and OutboundPool
	if s.cfg.OutboundPool != nil && s.cfg.OutboundPool.Enabled {
		if err := s.initializeOutboundPool(); err != nil {
			return fmt.Errorf("failed to initialize outbound pool: %w", err)
		}
	} else {
		// Use static router
		s.router = router.NewStaticRouter()
	}

	// Initialize Plugin Manager
	s.pluginManager = plugin.NewManager()

	// Load and initialize plugins
	if err := s.initializePlugins(); err != nil {
		return fmt.Errorf("failed to initialize plugins: %w", err)
	}

	// Initialize outbound
	if err := s.initializeOutbound(); err != nil {
		return fmt.Errorf("failed to initialize outbound: %w", err)
	}

	// Initialize inbound
	if err := s.initializeInbound(); err != nil {
		return fmt.Errorf("failed to initialize inbound: %w", err)
	}

	return nil
}

// initializeOutboundPool инициализирует outbound pool и связанные компоненты
func (s *Server) initializeOutboundPool() error {
	// Initialize Device Registry
	heartbeatInterval := s.cfg.OutboundPool.HeartbeatInterval
	if heartbeatInterval == 0 {
		heartbeatInterval = constants.DefaultHeartbeatInterval
	}
	heartbeatTimeout := s.cfg.OutboundPool.HeartbeatTimeout
	if heartbeatTimeout == 0 {
		heartbeatTimeout = constants.DefaultHeartbeatTimeout
	}

	s.deviceRegistry = device.NewRegistry(heartbeatInterval, heartbeatTimeout)

	// Initialize OutboundPool
	s.outboundPool = outbound.NewPool(s.deviceRegistry)

	// Initialize Dynamic Router with RoundRobin strategy
	strategy := router.NewRoundRobinStrategy()
	s.router = router.NewDynamicRouter(s.deviceRegistry, strategy)

	// Prepare TLS config if enabled
	tlsConfig, err := s.prepareTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to prepare TLS config: %w", err)
	}

	// Initialize WSS server for control-plane
	wssPort := s.cfg.OutboundPool.WSSPort
	if wssPort == 0 {
		wssPort = constants.DefaultWSSPort
	}
	s.wssServer = wss.NewServer(s.deviceRegistry, wssPort, tlsConfig)

	// Initialize QUIC server for data-plane
	quicPort := s.cfg.OutboundPool.QUICPort
	if quicPort == 0 {
		quicPort = constants.DefaultQUICPort
	}
	s.quicServer = quic.NewServer(s.deviceRegistry, quicPort, tlsConfig)

	logger.Info("server", "Outbound pool enabled: WSS control-plane on port %d, QUIC data-plane on port %d", wssPort, quicPort)
	return nil
}

// prepareTLSConfig подготавливает TLS конфигурацию
func (s *Server) prepareTLSConfig() (*tls.Config, error) {
	return tlsconfig.NewTLSConfig(s.cfg.OutboundPool.TLS)
}

// initializePlugins инициализирует плагины
func (s *Server) initializePlugins() error {
	if s.cfg.Plugins.TrafficInbound != nil && s.cfg.Plugins.TrafficInbound.Enabled {
		trafficInbound := traffic.NewInboundCounter()
		if err := trafficInbound.Init(s.cfg.Plugins.TrafficInbound.Config); err != nil {
			return fmt.Errorf("failed to initialize traffic_inbound plugin: %w", err)
		}
		s.pluginManager.RegisterInboundPlugin(trafficInbound)
		s.pluginManager.RegisterTrafficPlugin(trafficInbound)
		logger.Info("server", "Traffic inbound plugin enabled")
	}

	if s.cfg.Plugins.TrafficOutbound != nil && s.cfg.Plugins.TrafficOutbound.Enabled {
		trafficOutbound := traffic.NewOutboundCounter()
		if err := trafficOutbound.Init(s.cfg.Plugins.TrafficOutbound.Config); err != nil {
			return fmt.Errorf("failed to initialize traffic_outbound plugin: %w", err)
		}
		s.pluginManager.RegisterOutboundPlugin(trafficOutbound)
		s.pluginManager.RegisterTrafficPlugin(trafficOutbound)
		logger.Info("server", "Traffic outbound plugin enabled")
	}

	return nil
}

// initializeOutbound инициализирует outbound
func (s *Server) initializeOutbound() error {
	switch s.cfg.Outbound.Type {
	case "direct":
		s.outbound = outbound.NewDirectOutbound()
	case "socks5":
		if s.cfg.Outbound.ProxyAddress == "" {
			return fmt.Errorf("proxy_address is required for SOCKS5 outbound")
		}
		s.outbound = outbound.NewSOCKS5Outbound(s.cfg.Outbound.ProxyAddress)
	default:
		return fmt.Errorf("unsupported outbound type: %s", s.cfg.Outbound.Type)
	}
	return nil
}

// initializeInbound инициализирует inbound
func (s *Server) initializeInbound() error {
	switch s.cfg.Inbound.Type {
	case "socks5":
		s.inbound = inbound.NewSOCKS5Inbound(s.cfg.Inbound.Port)
	default:
		return fmt.Errorf("unsupported inbound type: %s", s.cfg.Inbound.Type)
	}
	return nil
}

// Start запускает server
func (s *Server) Start() error {
	// Connection handler
	handler := func(conn net.Conn, targetAddress string, ctx *plugin.ConnectionContext) error {
		if targetAddress == "" {
			return fmt.Errorf("target address not specified")
		}
		// Устанавливаем InboundID из конфигурации
		ctx.InboundID = s.cfg.Inbound.ID
		return proxy.HandleConnection(conn, s.outbound, s.cfg.Outbound.ID, &s.cfg.Outbound, targetAddress, s.cfg.Inbound.ID, s.router, s.pluginManager, s.outboundPool)
	}

	// Start inbound
	if err := s.inbound.Start(handler); err != nil {
		return fmt.Errorf("failed to start inbound: %w", err)
	}

	// Start WSS server if enabled
	if s.wssServer != nil {
		go func() {
			if err := s.wssServer.Start(); err != nil {
				log.Fatalf("Failed to start WSS server: %v", err)
			}
		}()
	}

	// Start QUIC server if enabled
	if s.quicServer != nil {
		go func() {
			if err := s.quicServer.Start(); err != nil {
				log.Fatalf("Failed to start QUIC server: %v", err)
			}
		}()
	}

	outboundType := s.cfg.Outbound.Type
	if outboundType == "socks5" {
		logger.Info("server", "SOCKS5 proxy started on port %d (SOCKS5 outbound via %s)", s.cfg.Inbound.Port, s.cfg.Outbound.ProxyAddress)
	} else {
		logger.Info("server", "SOCKS5 proxy started on port %d (%s outbound)", s.cfg.Inbound.Port, outboundType)
	}

	return nil
}

// Stop останавливает server
func (s *Server) Stop() error {
	var errs []error

	if s.inbound != nil {
		if err := s.inbound.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("error stopping inbound: %w", err))
		}
	}

	if s.wssServer != nil {
		if err := s.wssServer.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("error stopping WSS server: %w", err))
		}
	}

	if s.quicServer != nil {
		if err := s.quicServer.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("error stopping QUIC server: %w", err))
		}
	}

	if s.deviceRegistry != nil {
		if err := s.deviceRegistry.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing device registry: %w", err))
		}
	}

	if s.pluginManager != nil {
		if err := s.pluginManager.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing plugins: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping server: %v", errs)
	}

	return nil
}

