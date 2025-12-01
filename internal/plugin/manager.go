package plugin

import (
	"fmt"
	"sync"
)

// Manager управляет плагинами и вызывает hooks
type Manager struct {
	inboundPlugins  []InboundPlugin
	outboundPlugins []OutboundPlugin
	trafficPlugins  []TrafficPlugin
	mu              sync.RWMutex
}

// NewManager создает новый менеджер плагинов
func NewManager() *Manager {
	return &Manager{
		inboundPlugins:  make([]InboundPlugin, 0),
		outboundPlugins: make([]OutboundPlugin, 0),
		trafficPlugins:  make([]TrafficPlugin, 0),
	}
}

// RegisterInboundPlugin регистрирует inbound плагин
func (m *Manager) RegisterInboundPlugin(plugin InboundPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inboundPlugins = append(m.inboundPlugins, plugin)
}

// RegisterOutboundPlugin регистрирует outbound плагин
func (m *Manager) RegisterOutboundPlugin(plugin OutboundPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outboundPlugins = append(m.outboundPlugins, plugin)
}

// RegisterTrafficPlugin регистрирует traffic плагин
func (m *Manager) RegisterTrafficPlugin(plugin TrafficPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trafficPlugins = append(m.trafficPlugins, plugin)
}

// OnInboundConnection вызывает OnInboundConnection hook для всех inbound плагинов
func (m *Manager) OnInboundConnection(ctx *ConnectionContext) error {
	m.mu.RLock()
	plugins := make([]InboundPlugin, len(m.inboundPlugins))
	copy(plugins, m.inboundPlugins)
	m.mu.RUnlock()

	for _, plugin := range plugins {
		if err := plugin.OnInboundConnection(ctx); err != nil {
			return fmt.Errorf("plugin %s OnInboundConnection error: %w", plugin.Name(), err)
		}
	}
	return nil
}

// OnOutboundConnection вызывает OnOutboundConnection hook для всех outbound плагинов
func (m *Manager) OnOutboundConnection(ctx *ConnectionContext) error {
	m.mu.RLock()
	plugins := make([]OutboundPlugin, len(m.outboundPlugins))
	copy(plugins, m.outboundPlugins)
	m.mu.RUnlock()

	for _, plugin := range plugins {
		if err := plugin.OnOutboundConnection(ctx); err != nil {
			return fmt.Errorf("plugin %s OnOutboundConnection error: %w", plugin.Name(), err)
		}
	}
	return nil
}

// OnDataTransfer вызывает OnDataTransfer hook для всех traffic плагинов
func (m *Manager) OnDataTransfer(ctx *ConnectionContext, direction string, bytes int64) {
	m.mu.RLock()
	plugins := make([]TrafficPlugin, len(m.trafficPlugins))
	copy(plugins, m.trafficPlugins)
	m.mu.RUnlock()

	for _, plugin := range plugins {
		plugin.OnDataTransfer(ctx, direction, bytes)
	}
}

// OnConnectionClosed вызывает OnConnectionClosed hook для всех traffic плагинов
func (m *Manager) OnConnectionClosed(ctx *ConnectionContext) {
	m.mu.RLock()
	plugins := make([]TrafficPlugin, len(m.trafficPlugins))
	copy(plugins, m.trafficPlugins)
	m.mu.RUnlock()

	for _, plugin := range plugins {
		plugin.OnConnectionClosed(ctx)
	}
}

// Close закрывает все плагины
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for _, plugin := range m.inboundPlugins {
		if err := plugin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s close error: %w", plugin.Name(), err))
		}
	}

	for _, plugin := range m.outboundPlugins {
		if err := plugin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s close error: %w", plugin.Name(), err))
		}
	}

	for _, plugin := range m.trafficPlugins {
		if err := plugin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s close error: %w", plugin.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing plugins: %v", errs)
	}

	return nil
}

