package traffic

import (
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
)

// InboundCounter плагин для подсчета трафика по InboundID
type InboundCounter struct {
	counter *BaseCounter
}

// NewInboundCounter создает новый Inbound Counter плагин
func NewInboundCounter() *InboundCounter {
	return &InboundCounter{
		counter: NewBaseCounter(),
	}
}

// Name возвращает имя плагина
func (i *InboundCounter) Name() string {
	return "traffic_inbound"
}

// Init инициализирует плагин
func (i *InboundCounter) Init(config map[string]interface{}) error {
	logger.Debug("plugin", "InboundCounter initialized")
	return nil
}

// Close закрывает плагин
func (i *InboundCounter) Close() error {
	logger.Debug("plugin", "InboundCounter closed")
	return nil
}

// OnInboundConnection вызывается при новом inbound соединении
func (i *InboundCounter) OnInboundConnection(ctx *plugin.ConnectionContext) error {
	if ctx.InboundID == "" {
		// InboundID не установлен, пропускаем
		return nil
	}
	
	i.counter.AddConnection(ctx.InboundID)
	logger.Debug("plugin", "InboundCounter: connection from inbound %s", ctx.InboundID)
	return nil
}

// OnDataTransfer вызывается при передаче данных
func (i *InboundCounter) OnDataTransfer(ctx *plugin.ConnectionContext, direction string, bytes int64) {
	if ctx.InboundID == "" {
		return
	}
	
	if direction == "sent" {
		ctx.BytesSent += bytes
		i.counter.AddBytes(ctx.InboundID, bytes, 0)
	} else if direction == "received" {
		ctx.BytesReceived += bytes
		i.counter.AddBytes(ctx.InboundID, 0, bytes)
	}
}

// OnConnectionClosed вызывается при закрытии соединения
func (i *InboundCounter) OnConnectionClosed(ctx *plugin.ConnectionContext) {
	if ctx.InboundID == "" {
		return
	}
	
	// Обновляем финальную статистику
	i.counter.AddBytes(ctx.InboundID, ctx.BytesSent, ctx.BytesReceived)
	
	stats := i.counter.GetStats(ctx.InboundID)
	logger.Debug("plugin", "InboundCounter: connection closed for inbound %s, total: sent=%d, received=%d", 
		ctx.InboundID, stats.BytesSent, stats.BytesReceived)
}

// GetStats возвращает статистику для указанного InboundID
func (i *InboundCounter) GetStats(inboundID string) *Stats {
	return i.counter.GetStats(inboundID)
}

