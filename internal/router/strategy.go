package router

import (
	"fmt"
	"sync"

	"example.com/me/myproxy/internal/device"
)

// Strategy интерфейс для стратегий роутинга
type Strategy interface {
	// Select выбирает устройство из registry по критериям
	// Работает с registry напрямую, не получает список устройств (масштабируемо)
	Select(registry *device.Registry, criteria *device.DeviceCriteria, targetAddress string) (*device.Device, error)
}

// RoundRobinStrategy реализует round-robin стратегию роутинга
type RoundRobinStrategy struct {
	mu    sync.Mutex
	index int64
}

// NewRoundRobinStrategy создает новую round-robin стратегию
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// Select выбирает устройство по round-robin алгоритму
func (r *RoundRobinStrategy) Select(registry *device.Registry, criteria *device.DeviceCriteria, targetAddress string) (*device.Device, error) {
	// Получаем список доступных устройств для round-robin
	devices := registry.GetAvailableDevices(criteria)
	if len(devices) == 0 {
		return nil, fmt.Errorf("no available devices matching criteria")
	}

	r.mu.Lock()
	index := int(r.index % int64(len(devices)))
	r.index++
	r.mu.Unlock()

	return devices[index], nil
}

