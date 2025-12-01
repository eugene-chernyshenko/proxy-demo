package plugin

// Plugin базовый интерфейс для всех плагинов
type Plugin interface {
	// Name возвращает имя плагина
	Name() string
	// Init инициализирует плагин с конфигурацией
	Init(config map[string]interface{}) error
	// Close закрывает плагин и освобождает ресурсы
	Close() error
}

// InboundPlugin плагин для обработки inbound событий
type InboundPlugin interface {
	Plugin
	// OnInboundConnection вызывается при новом inbound соединении
	OnInboundConnection(ctx *ConnectionContext) error
}

// OutboundPlugin плагин для обработки outbound событий
type OutboundPlugin interface {
	Plugin
	// OnOutboundConnection вызывается при установке outbound соединения
	OnOutboundConnection(ctx *ConnectionContext) error
}

// TrafficPlugin плагин для подсчета трафика
type TrafficPlugin interface {
	Plugin
	// OnDataTransfer вызывается при передаче данных (опционально, для точного подсчета)
	OnDataTransfer(ctx *ConnectionContext, direction string, bytes int64)
	// OnConnectionClosed вызывается при закрытии соединения
	OnConnectionClosed(ctx *ConnectionContext)
}

