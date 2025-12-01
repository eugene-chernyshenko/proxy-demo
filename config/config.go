package config

// InboundConfig представляет конфигурацию inbound
type InboundConfig struct {
	Type string `json:"type"` // "socks5" для этапа 1
	Port int    `json:"port"`
	ID   string `json:"id,omitempty"` // Идентификатор inbound (опционально, для плагинов)
}

// OutboundConfig представляет конфигурацию outbound
type OutboundConfig struct {
	Type         string `json:"type"`          // "direct" или "socks5"
	ProxyAddress string `json:"proxy_address"` // Адрес SOCKS5 прокси (для типа "socks5")
	ID           string `json:"id,omitempty"`  // Идентификатор outbound (опционально, для плагинов)
}

// PluginConfig представляет конфигурацию плагина
type PluginConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// PluginsConfig представляет конфигурацию всех плагинов
type PluginsConfig struct {
	TrafficInbound  *PluginConfig `json:"traffic_inbound,omitempty"`
	TrafficOutbound *PluginConfig `json:"traffic_outbound,omitempty"`
}

// Config представляет полную конфигурацию приложения
type Config struct {
	Inbound  InboundConfig  `json:"inbound"`
	Outbound OutboundConfig `json:"outbound"`
	Plugins  PluginsConfig  `json:"plugins,omitempty"`
}

