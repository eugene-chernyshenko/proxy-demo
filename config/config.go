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

// TLSConfig представляет конфигурацию TLS
type TLSConfig struct {
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// OutboundPoolConfig представляет конфигурацию пула outbound устройств
type OutboundPoolConfig struct {
	Enabled           bool       `json:"enabled"`
	WSSPort           int        `json:"wss_port"`            // Порт для WSS control-plane (default: 443)
	QUICPort          int        `json:"quic_port"`           // Порт для QUIC data-plane (default: 443)
	TLS               *TLSConfig `json:"tls,omitempty"`       // TLS конфигурация (опционально)
	HeartbeatInterval int        `json:"heartbeat_interval"` // Интервал heartbeat (секунды, default: 30)
	HeartbeatTimeout  int        `json:"heartbeat_timeout"`   // Таймаут offline (секунды, default: 90)
}

// Config представляет полную конфигурацию приложения
type Config struct {
	Inbound      InboundConfig      `json:"inbound"`
	Outbound     OutboundConfig     `json:"outbound"`
	Plugins      PluginsConfig      `json:"plugins,omitempty"`
	OutboundPool *OutboundPoolConfig `json:"outbound_pool,omitempty"`
}

