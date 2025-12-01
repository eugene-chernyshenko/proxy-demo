package traffic

import (
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
)

// OutboundCounter плагин для подсчета трафика по OutboundID
type OutboundCounter struct {
	counter *BaseCounter
}

// NewOutboundCounter создает новый Outbound Counter плагин
func NewOutboundCounter() *OutboundCounter {
	return &OutboundCounter{
		counter: NewBaseCounter(),
	}
}

// Name возвращает имя плагина
func (o *OutboundCounter) Name() string {
	return "traffic_outbound"
}

// Init инициализирует плагин
func (o *OutboundCounter) Init(config map[string]interface{}) error {
	logger.Debug("plugin", "OutboundCounter initialized")
	return nil
}

// Close закрывает плагин
func (o *OutboundCounter) Close() error {
	logger.Debug("plugin", "OutboundCounter closed")
	return nil
}

// OnOutboundConnection вызывается при установке outbound соединения
func (o *OutboundCounter) OnOutboundConnection(ctx *plugin.ConnectionContext) error {
	if ctx.OutboundID == "" {
		// OutboundID не установлен, пропускаем
		return nil
	}
	
	o.counter.AddConnection(ctx.OutboundID)
	logger.Debug("plugin", "OutboundCounter: connection to outbound %s", ctx.OutboundID)
	return nil
}

// OnDataTransfer вызывается при передаче данных
func (o *OutboundCounter) OnDataTransfer(ctx *plugin.ConnectionContext, direction string, bytes int64) {
	if ctx.OutboundID == "" {
		return
	}
	
	if direction == "sent" {
		ctx.BytesSent += bytes
		o.counter.AddBytes(ctx.OutboundID, bytes, 0)
	} else if direction == "received" {
		ctx.BytesReceived += bytes
		o.counter.AddBytes(ctx.OutboundID, 0, bytes)
	}
}

// OnConnectionClosed вызывается при закрытии соединения
func (o *OutboundCounter) OnConnectionClosed(ctx *plugin.ConnectionContext) {
	if ctx.OutboundID == "" {
		return
	}
	
	// Обновляем финальную статистику
	o.counter.AddBytes(ctx.OutboundID, ctx.BytesSent, ctx.BytesReceived)
	
	stats := o.counter.GetStats(ctx.OutboundID)
	logger.Debug("plugin", "OutboundCounter: connection closed for outbound %s, total: sent=%d, received=%d", 
		ctx.OutboundID, stats.BytesSent, stats.BytesReceived)
}

// GetStats возвращает статистику для указанного OutboundID
func (o *OutboundCounter) GetStats(outboundID string) *Stats {
	return o.counter.GetStats(outboundID)
}

