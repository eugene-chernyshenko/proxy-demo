package device

import (
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"nhooyr.io/websocket"
)

// DeviceStatus представляет статус устройства
type DeviceStatus int

const (
	StatusOffline DeviceStatus = iota
	StatusOnline
)

// Device представляет зарегистрированное устройство
type Device struct {
	mu sync.RWMutex

	// Идентификатор
	ID string

	// Адрес из соединения (защита от подмены)
	RemoteAddr string

	// WSS control connection
	WSSConn *websocket.Conn

	// QUIC data connection
	QUICConn *quic.Conn

	// Активные QUIC streams (conn_id → stream)
	Streams map[string]*quic.Stream

	// Статус
	Status DeviceStatus

	// Временные метки
	LastHeartbeat time.Time
	RegisteredAt  time.Time

	// Метрики
	ActiveConns   int
	BytesSent     int64
	BytesReceived int64

	// Метаданные
	Location string
	Capacity int
	Tags     []string
}

// NewDevice создает новое устройство
func NewDevice(id, remoteAddr string, metadata map[string]interface{}) *Device {
	d := &Device{
		ID:         id,
		RemoteAddr: remoteAddr,
		Status:     StatusOnline,
		RegisteredAt: time.Now(),
		LastHeartbeat: time.Now(),
		Streams:     make(map[string]*quic.Stream),
	}

	// Извлечь метаданные
	if location, ok := metadata["location"].(string); ok {
		d.Location = location
	}
	if capacity, ok := metadata["capacity"].(float64); ok {
		d.Capacity = int(capacity)
	}
	if tags, ok := metadata["tags"].([]interface{}); ok {
		d.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				d.Tags = append(d.Tags, tagStr)
			}
		}
	}

	return d
}

// SetWSSConn устанавливает WSS connection
func (d *Device) SetWSSConn(conn *websocket.Conn) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.WSSConn = conn
}

// GetWSSConn возвращает WSS connection
func (d *Device) GetWSSConn() *websocket.Conn {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.WSSConn
}

// SetQUICConn устанавливает QUIC connection
func (d *Device) SetQUICConn(conn *quic.Conn) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.QUICConn = conn
}

// GetQUICConn возвращает QUIC connection
func (d *Device) GetQUICConn() *quic.Conn {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.QUICConn
}

// AddStream добавляет QUIC stream
func (d *Device) AddStream(connID string, stream *quic.Stream) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Streams[connID] = stream
}

// GetStream возвращает QUIC stream по conn_id
func (d *Device) GetStream(connID string) *quic.Stream {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Streams[connID]
}

// RemoveStream удаляет QUIC stream
func (d *Device) RemoveStream(connID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.Streams, connID)
}

// UpdateHeartbeat обновляет время последнего heartbeat
func (d *Device) UpdateHeartbeat() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.LastHeartbeat = time.Now()
	d.Status = StatusOnline
}

// MarkOffline помечает устройство как offline
func (d *Device) MarkOffline() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Status = StatusOffline
	if d.WSSConn != nil {
		d.WSSConn.Close(websocket.StatusNormalClosure, "device offline")
		d.WSSConn = nil
	}
	if d.QUICConn != nil {
		d.QUICConn.CloseWithError(0, "device offline")
		d.QUICConn = nil
	}
	// Закрываем все streams
	for connID, stream := range d.Streams {
		stream.Close()
		delete(d.Streams, connID)
	}
}

// IsOnline проверяет, онлайн ли устройство
func (d *Device) IsOnline() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Status == StatusOnline && d.WSSConn != nil && d.QUICConn != nil
}

// AddBytes добавляет байты к статистике
func (d *Device) AddBytes(sent, received int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.BytesSent += sent
	d.BytesReceived += received
}

// IncrementConn увеличивает счетчик активных соединений
func (d *Device) IncrementConn() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ActiveConns++
}

// DecrementConn уменьшает счетчик активных соединений
func (d *Device) DecrementConn() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ActiveConns > 0 {
		d.ActiveConns--
	}
}

// DeviceCriteria представляет критерии поиска устройства
type DeviceCriteria struct {
	Status   DeviceStatus
	Tags     []string
	Location string
	// Для будущего расширения:
	// MinCapacity int
	// MaxLatency  time.Duration
}

// NewDeviceCriteria создает критерии поиска
func NewDeviceCriteria() *DeviceCriteria {
	return &DeviceCriteria{
		Status: StatusOnline,
	}
}

// WithTags добавляет теги к критериям
func (c *DeviceCriteria) WithTags(tags ...string) *DeviceCriteria {
	c.Tags = tags
	return c
}

// WithLocation добавляет локацию к критериям
func (c *DeviceCriteria) WithLocation(location string) *DeviceCriteria {
	c.Location = location
	return c
}

