package config

// InboundConfig представляет конфигурацию inbound
type InboundConfig struct {
	Type string `json:"type"` // "socks5" для этапа 1
	Port int    `json:"port"`
}

// OutboundConfig представляет конфигурацию outbound
type OutboundConfig struct {
	Type string `json:"type"` // "direct" для этапа 0
}

// Config представляет полную конфигурацию приложения
type Config struct {
	Inbound  InboundConfig  `json:"inbound"`
	Outbound OutboundConfig `json:"outbound"`
}

