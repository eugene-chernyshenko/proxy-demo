package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// Load загружает конфигурацию из файла и переопределяет через CLI аргументы
func Load() (*Config, error) {
	var configFile string
	var port int

	flag.StringVar(&configFile, "config", "config.json", "Path to configuration file")
	flag.IntVar(&port, "port", 0, "Port for inbound (overrides config)")
	flag.Parse()

	cfg := &Config{
		Inbound: InboundConfig{
			Type: "socks5",
			Port: 1080, // default value
		},
		Outbound: OutboundConfig{
			Type: "direct",
		},
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
	if port > 0 {
		cfg.Inbound.Port = port
	}

	return cfg, nil
}

