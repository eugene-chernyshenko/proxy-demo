package router

import (
	"example.com/me/myproxy/config"
	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/plugin"
)

// DynamicRouter реализует Router с выбором из пула устройств
type DynamicRouter struct {
	registry *device.Registry
	strategy Strategy
}

// NewDynamicRouter создает новый Dynamic Router
func NewDynamicRouter(registry *device.Registry, strategy Strategy) *DynamicRouter {
	return &DynamicRouter{
		registry: registry,
		strategy: strategy,
	}
}

// SelectOutbound выбирает outbound из пула устройств
func (d *DynamicRouter) SelectOutbound(
	ctx *plugin.ConnectionContext,
	targetAddress string,
	currentOutboundID string,
	currentOutboundConfig *config.OutboundConfig,
) (string, *config.OutboundConfig, error) {
	// Создаем критерии поиска
	criteria := device.NewDeviceCriteria()

	// Выбираем устройство через стратегию
	selectedDevice, err := d.strategy.Select(d.registry, criteria, targetAddress)
	if err != nil {
		// Fallback на статический outbound если пул пуст
		return "", nil, nil
	}

	// Проверяем что reverse connection активна
	if !selectedDevice.IsOnline() {
		// Fallback на статический outbound
		return "", nil, nil
	}

	// Возвращаем outboundID для использования устройства из пула
	return selectedDevice.ID, nil, nil
}

