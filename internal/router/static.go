package router

import "example.com/me/myproxy/config"
import "example.com/me/myproxy/internal/plugin"

// StaticRouter всегда возвращает nil, nil (использовать текущий outbound)
type StaticRouter struct{}

// NewStaticRouter создает новый Static Router
func NewStaticRouter() *StaticRouter {
	return &StaticRouter{}
}

// SelectOutbound всегда возвращает nil, nil для использования текущего outbound
func (s *StaticRouter) SelectOutbound(
	ctx *plugin.ConnectionContext,
	targetAddress string,
	currentOutboundID string,
	currentOutboundConfig *config.OutboundConfig,
) (string, *config.OutboundConfig, error) {
	// Всегда используем текущий outbound из конфигурации
	return "", nil, nil
}

