package config

// InboundConfig представляет конфигурацию inbound
type InboundConfig struct {
	Type string `json:"type"` // "socks5" для этапа 1
	Port int    `json:"port"`
}

// OutboundConfig представляет конфигурацию outbound
type OutboundConfig struct {
	Type         string `json:"type"`          // "direct" или "socks5"
	ProxyAddress string `json:"proxy_address"` // Адрес SOCKS5 прокси (для типа "socks5")
}

// Config представляет полную конфигурацию приложения
type Config struct {
	Inbound  InboundConfig  `json:"inbound"`
	Outbound OutboundConfig `json:"outbound"`
}

