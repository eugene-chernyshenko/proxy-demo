package outbound

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
	"github.com/quic-go/quic-go"
)

// QUICOutbound реализует подключение через QUIC stream от device
type QUICOutbound struct {
	deviceID  string
	registry  *device.Registry
	mu        sync.Mutex
	streams   map[string]*quic.Stream // conn_id → stream
}

// NewQUICOutbound создает новый QUIC Outbound
func NewQUICOutbound(deviceID string, registry *device.Registry) *QUICOutbound {
	return &QUICOutbound{
		deviceID: deviceID,
		registry: registry,
		streams:  make(map[string]*quic.Stream),
	}
}

// Dial отправляет запрос через QUIC stream
func (q *QUICOutbound) Dial(network, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("unsupported network for QUIC outbound: %s", network)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Получаем device
	dev, err := q.registry.GetDevice(q.deviceID)
	if err != nil {
		return nil, fmt.Errorf("device %s not found: %w", q.deviceID, err)
	}

	quicConn := dev.GetQUICConn()
	if quicConn == nil {
		return nil, fmt.Errorf("QUIC connection not established for device %s", q.deviceID)
	}

	// Генерируем conn_id
	connID := fmt.Sprintf("%s-%d", q.deviceID, len(q.streams))

	logger.Debug("outbound", "Opening QUIC stream for %s, conn_id=%s", address, connID)

	// Открываем новый stream
	ctx := context.Background()
	stream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open QUIC stream: %w", err)
	}

	// Сохраняем stream
	q.streams[connID] = stream
	dev.AddStream(connID, stream)

	// Отправляем target address через stream (простой протокол: адрес + \n)
	addrBytes := []byte(address + "\n")
	if _, err := stream.Write(addrBytes); err != nil {
		stream.Close()
		delete(q.streams, connID)
		dev.RemoveStream(connID)
		return nil, fmt.Errorf("failed to send target address: %w", err)
	}

	logger.Debug("outbound", "QUIC stream opened for %s, conn_id=%s", address, connID)

	// Возвращаем wrapper для net.Conn
	return &quicStreamConn{
		stream:   stream,
		connID:   connID,
		outbound: q,
		device:   dev,
	}, nil
}

// quicStreamConn обертка для quic.Stream, реализующая net.Conn
type quicStreamConn struct {
	stream   *quic.Stream
	connID   string
	outbound *QUICOutbound
	device   *device.Device
	closed   bool
	mu       sync.Mutex
}

func (c *quicStreamConn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

func (c *quicStreamConn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

func (c *quicStreamConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	c.stream.Close()
	c.outbound.mu.Lock()
	delete(c.outbound.streams, c.connID)
	c.outbound.mu.Unlock()
	c.device.RemoveStream(c.connID)

	return nil
}

func (c *quicStreamConn) LocalAddr() net.Addr {
	return &net.TCPAddr{}
}

func (c *quicStreamConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (c *quicStreamConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *quicStreamConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *quicStreamConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

