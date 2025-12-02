package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// DeviceConfig представляет конфигурацию device client
type DeviceConfig struct {
	ProxyHost        string   `json:"proxy_host"`
	WSSPort          int      `json:"wss_port"`            // Порт для WSS control-plane (default: 443)
	QUICPort         int      `json:"quic_port"`           // Порт для QUIC data-plane (default: 443)
	DeviceID         string   `json:"device_id"`
	Location         string   `json:"location,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	TLSEnabled       bool     `json:"tls_enabled"`         // Использовать TLS (default: false)
	TLSSkipVerify    bool     `json:"tls_skip_verify"`     // Пропустить проверку TLS сертификатов (для тестирования)
}

// LoadDeviceConfig загружает конфигурацию device из файла и переопределяет через CLI аргументы
func LoadDeviceConfig() (*DeviceConfig, error) {
	var configFile string
	var proxyHost string
	var wssPort int
	var quicPort int
	var deviceID string
	var location string
	var heartbeatInterval int
	var tlsSkipVerify bool

	flag.StringVar(&configFile, "config", "device_config.json", "Path to device configuration file")
	flag.StringVar(&proxyHost, "proxy", "", "Proxy host")
	flag.IntVar(&wssPort, "wss-port", 0, "WSS control-plane port (default: 443)")
	flag.IntVar(&quicPort, "quic-port", 0, "QUIC data-plane port (default: 443)")
	flag.StringVar(&deviceID, "device-id", "", "Device ID")
	flag.StringVar(&location, "location", "", "Device location")
	flag.IntVar(&heartbeatInterval, "heartbeat-interval", 0, "Heartbeat interval in seconds")
	flag.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification")
	flag.Parse()

	cfg := &DeviceConfig{
		ProxyHost:        "127.0.0.1",
		WSSPort:          443,
		QUICPort:         443,
		HeartbeatInterval: 30,
		TLSEnabled:       false,
		TLSSkipVerify:    false,
	}

	// Load from file if exists
	if _, err := os.Stat(configFile); err == nil {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Override via CLI arguments
	if proxyHost != "" {
		cfg.ProxyHost = proxyHost
	}
	if wssPort > 0 {
		cfg.WSSPort = wssPort
	}
	if quicPort > 0 {
		cfg.QUICPort = quicPort
	}
	if deviceID != "" {
		cfg.DeviceID = deviceID
	}
	if location != "" {
		cfg.Location = location
	}
	if heartbeatInterval > 0 {
		cfg.HeartbeatInterval = heartbeatInterval
	}
	if tlsSkipVerify {
		cfg.TLSSkipVerify = true
	}

	// Validate required fields
	if cfg.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	return cfg, nil
}

