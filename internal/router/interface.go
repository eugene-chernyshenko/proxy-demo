package router

import "example.com/me/myproxy/config"
import "example.com/me/myproxy/internal/plugin"

// Router интерфейс для выбора outbound
type Router interface {
	// SelectOutbound выбирает outbound для целевого адреса
	// Возвращает:
	//   - outboundID != "" - использовать существующий outbound из пула
	//   - outboundConfig != nil - создать новый outbound с указанной конфигурацией
	//   - оба nil - использовать текущий outbound (currentOutboundConfig)
	SelectOutbound(
		ctx *plugin.ConnectionContext,
		targetAddress string,
		currentOutboundID string,
		currentOutboundConfig *config.OutboundConfig,
	) (outboundID string, outboundConfig *config.OutboundConfig, err error)
}

