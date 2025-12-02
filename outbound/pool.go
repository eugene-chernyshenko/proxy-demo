package outbound

import (
	"fmt"
	"sync"

	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
)

// Pool управляет пулом outbound объектов для устройств
type Pool struct {
	mu       sync.RWMutex
	outbounds map[string]Outbound // deviceID -> Outbound
	registry  *device.Registry
}

// NewPool создает новый Pool
func NewPool(registry *device.Registry) *Pool {
	return &Pool{
		outbounds: make(map[string]Outbound),
		registry:  registry,
	}
}

// GetOutbound возвращает outbound для устройства, создавая его при необходимости
func (p *Pool) GetOutbound(deviceID string) (Outbound, error) {
	p.mu.RLock()
	outbound, exists := p.outbounds[deviceID]
	p.mu.RUnlock()

	if exists {
		// Проверяем что устройство все еще онлайн
		device, err := p.registry.GetDevice(deviceID)
		if err != nil {
			// Устройство не найдено, удаляем из кеша
			p.mu.Lock()
			delete(p.outbounds, deviceID)
			p.mu.Unlock()
			return nil, fmt.Errorf("device %s not found", deviceID)
		}

		if !device.IsOnline() {
			// Устройство offline, удаляем из кеша
			p.mu.Lock()
			delete(p.outbounds, deviceID)
			p.mu.Unlock()
			return nil, fmt.Errorf("device %s is offline", deviceID)
		}

		return outbound, nil
	}

	// Создаем новый outbound
	device, err := p.registry.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device %s not found: %w", deviceID, err)
	}

	if !device.IsOnline() {
		return nil, fmt.Errorf("device %s is offline", deviceID)
	}

	quicConn := device.GetQUICConn()
	if quicConn == nil {
		return nil, fmt.Errorf("device %s has no QUIC connection", deviceID)
	}

	// Создаем QUICOutbound для устройства
	outbound = NewQUICOutbound(deviceID, p.registry)

	p.mu.Lock()
	p.outbounds[deviceID] = outbound
	p.mu.Unlock()

	logger.Debug("outbound", "Created new outbound for device %s", deviceID)
	return outbound, nil
}

// RemoveOutbound удаляет outbound из пула
func (p *Pool) RemoveOutbound(deviceID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.outbounds, deviceID)
	logger.Debug("outbound", "Removed outbound for device %s", deviceID)
}

// Clear очищает весь пул
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outbounds = make(map[string]Outbound)
	logger.Debug("outbound", "Cleared outbound pool")
}

