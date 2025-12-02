package device

import (
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"example.com/me/myproxy/internal/logger"
	"nhooyr.io/websocket"
)

// Registry управляет зарегистрированными устройствами
type Registry struct {
	mu                sync.RWMutex
	devices           map[string]*Device
	heartbeatTimeout  time.Duration
	heartbeatInterval time.Duration
	stopChan          chan struct{}
}

// NewRegistry создает новый registry
func NewRegistry(heartbeatInterval, heartbeatTimeout int) *Registry {
	r := &Registry{
		devices:           make(map[string]*Device),
		heartbeatInterval: time.Duration(heartbeatInterval) * time.Second,
		heartbeatTimeout:  time.Duration(heartbeatTimeout) * time.Second,
		stopChan:          make(chan struct{}),
	}

	// Запускаем фоновую проверку heartbeat timeout
	go r.checkHeartbeatLoop()

	return r
}

// Register регистрирует новое устройство
func (r *Registry) Register(deviceID, remoteAddr string, metadata map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[deviceID]; exists {
		return fmt.Errorf("device %s already registered", deviceID)
	}

	device := NewDevice(deviceID, remoteAddr, metadata)
	r.devices[deviceID] = device

	logger.Debug("device", "Device %s registered from %s", deviceID, remoteAddr)
	return nil
}

// RegisterWithWSS регистрирует новое устройство с WSS connection
func (r *Registry) RegisterWithWSS(deviceID, remoteAddr string, metadata map[string]interface{}, wssConn *websocket.Conn) (*Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем, существует ли устройство
	device, exists := r.devices[deviceID]
	if !exists {
		// Создаем новое устройство
		device = NewDevice(deviceID, remoteAddr, metadata)
		r.devices[deviceID] = device
	}

	// Устанавливаем WSS connection
	device.SetWSSConn(wssConn)
	device.UpdateHeartbeat()

	logger.Debug("device", "Device %s registered from %s with WSS", deviceID, remoteAddr)
	return device, nil
}

// RegisterQUICConnection регистрирует QUIC connection для устройства
func (r *Registry) RegisterQUICConnection(deviceID string, conn *quic.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		conn.CloseWithError(0, "device not found")
		return fmt.Errorf("device %s not found", deviceID)
	}

	// Закрываем старое соединение если есть
	if device.QUICConn != nil {
		device.QUICConn.CloseWithError(0, "new connection")
	}

	device.SetQUICConn(conn)
	device.UpdateHeartbeat()
	device.Status = StatusOnline

	logger.Debug("device", "QUIC connection registered for device %s", deviceID)
	return nil
}

// UpdateHeartbeat обновляет heartbeat для устройства
func (r *Registry) UpdateHeartbeat(deviceID string) error {
	r.mu.RLock()
	device, exists := r.devices[deviceID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.UpdateHeartbeat()
	return nil
}

// FindDevice находит устройство по критериям (масштабируемый поиск)
func (r *Registry) FindDevice(criteria *DeviceCriteria) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Получаем список доступных устройств
	availableDevices := make([]*Device, 0)
	for _, device := range r.devices {
		if device.Status == criteria.Status && device.IsOnline() {
			// Проверка тегов
			if len(criteria.Tags) > 0 {
				hasAllTags := true
				for _, requiredTag := range criteria.Tags {
					found := false
					for _, deviceTag := range device.Tags {
						if deviceTag == requiredTag {
							found = true
							break
						}
					}
					if !found {
						hasAllTags = false
						break
					}
				}
				if !hasAllTags {
					continue
				}
			}

			// Проверка локации
			if criteria.Location != "" && device.Location != criteria.Location {
				continue
			}

			availableDevices = append(availableDevices, device)
		}
	}

	if len(availableDevices) == 0 {
		return nil, fmt.Errorf("no available devices matching criteria")
	}

	// Возвращаем первое подходящее (для round-robin будет использоваться стратегия)
	return availableDevices[0], nil
}

// GetDevice возвращает устройство по ID
func (r *Registry) GetDevice(deviceID string) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

// MarkOffline помечает устройство как offline
func (r *Registry) MarkOffline(deviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if exists {
		device.MarkOffline()
		logger.Debug("device", "Device %s marked as offline", deviceID)
	}
}

// GetDeviceCount возвращает количество доступных устройств
func (r *Registry) GetDeviceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, device := range r.devices {
		if device.IsOnline() {
			count++
		}
	}
	return count
}

// GetAvailableDevices возвращает список доступных устройств по критериям
func (r *Registry) GetAvailableDevices(criteria *DeviceCriteria) []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	availableDevices := make([]*Device, 0)
	for _, device := range r.devices {
		if device.Status == StatusOnline && device.IsOnline() {
			// Проверка тегов
			if len(criteria.Tags) > 0 {
				hasAllTags := true
				for _, requiredTag := range criteria.Tags {
					found := false
					for _, deviceTag := range device.Tags {
						if deviceTag == requiredTag {
							found = true
							break
						}
					}
					if !found {
						hasAllTags = false
						break
					}
				}
				if !hasAllTags {
					continue
				}
			}

			// Проверка локации
			if criteria.Location != "" && device.Location != criteria.Location {
				continue
			}

			availableDevices = append(availableDevices, device)
		}
	}

	return availableDevices
}

// checkHeartbeatLoop проверяет heartbeat timeout в фоне
func (r *Registry) checkHeartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkHeartbeatTimeout()
		case <-r.stopChan:
			return
		}
	}
}

// checkHeartbeatTimeout проверяет timeout heartbeat для всех устройств
func (r *Registry) checkHeartbeatTimeout() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for id, device := range r.devices {
		if device.Status == StatusOnline {
			if now.Sub(device.LastHeartbeat) > r.heartbeatTimeout {
				logger.Debug("device", "Device %s heartbeat timeout, marking offline", id)
				device.MarkOffline()
			}
		}
	}
}

// Close закрывает registry и все соединения
func (r *Registry) Close() error {
	close(r.stopChan)

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, device := range r.devices {
		device.MarkOffline()
		logger.Debug("device", "Device %s closed", id)
	}

	return nil
}
