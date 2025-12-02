package constants

import "time"

// Network ports
const (
	// DefaultWSSPort стандартный порт для WSS control-plane
	DefaultWSSPort = 443
	// DefaultQUICPort стандартный порт для QUIC data-plane
	DefaultQUICPort = 443
)

// Protocol sizes
const (
	// MessageLengthSize размер префикса длины сообщения в байтах (для WSS)
	MessageLengthSize = 4
	// MaxTargetAddressLen максимальная длина target address в байтах
	MaxTargetAddressLen = 256
)

// Timeouts and intervals
const (
	// DefaultHeartbeatInterval интервал heartbeat в секундах
	DefaultHeartbeatInterval = 30
	// DefaultHeartbeatTimeout таймаут для определения offline устройства в секундах
	DefaultHeartbeatTimeout = 90
	// RegistrationStreamTimeout таймаут для чтения device_id из QUIC registration stream
	RegistrationStreamTimeout = 5 * time.Second
)

// Status strings
const (
	// StatusOK статус успешного выполнения
	StatusOK = "ok"
	// StatusError статус ошибки
	StatusError = "error"
)

// Component names for logging
const (
	// ComponentDevice имя компонента для логирования device
	ComponentDevice = "device"
	// ComponentOutbound имя компонента для логирования outbound
	ComponentOutbound = "outbound"
	// ComponentInbound имя компонента для логирования inbound
	ComponentInbound = "inbound"
	// ComponentProxy имя компонента для логирования proxy
	ComponentProxy = "proxy"
	// ComponentPlugin имя компонента для логирования plugin
	ComponentPlugin = "plugin"
)

