package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"example.com/me/myproxy/internal/logger"
	"github.com/quic-go/quic-go"
)

// Client представляет QUIC client для device
type Client struct {
	proxyHost string
	quicPort  int
	deviceID  string
	tlsConfig *tls.Config
	conn      *quic.Conn
	handler   *StreamHandler
}

// NewClient создает новый QUIC client
func NewClient(proxyHost string, quicPort int, deviceID string, tlsConfig *tls.Config) *Client {
	return &Client{
		proxyHost: proxyHost,
		quicPort:  quicPort,
		deviceID:  deviceID,
		tlsConfig: tlsConfig,
		handler:   NewStreamHandler(),
	}
}

// Connect подключается к POP через QUIC
func (c *Client) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", c.proxyHost, c.quicPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	// QUIC требует UDP listener, а не dialer
	// Создаем UDP listener на случайном порту
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return fmt.Errorf("failed to listen UDP: %w", err)
	}

	var tlsConf *tls.Config
	if c.tlsConfig != nil {
		tlsConf = c.tlsConfig
	} else {
		// Для тестирования с самоподписанным сертификатом
		tlsConf = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         c.proxyHost, // Устанавливаем ServerName для SNI
		}
	}

	config := &quic.Config{
		// Настройки QUIC
	}

	conn, err := quic.Dial(ctx, udpConn, udpAddr, tlsConf, config)
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("failed to dial QUIC: %w", err)
	}

	c.conn = conn
	logger.Debug("device", "QUIC connection established to %s", addr)

	// Отправляем device_id в первом stream для идентификации
	regStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to open registration stream")
		return fmt.Errorf("failed to open registration stream: %w", err)
	}

	// Отправляем device_id как простую строку с новой строкой
	deviceIDBytes := []byte(c.deviceID + "\n")
	if _, err := regStream.Write(deviceIDBytes); err != nil {
		regStream.Close()
		conn.CloseWithError(0, "failed to send device_id")
		return fmt.Errorf("failed to send device_id: %w", err)
	}
	
	// Не закрываем stream сразу - даем серверу время прочитать данные
	// Stream будет закрыт сервером после чтения device_id
	// Или можно использовать небольшую задержку перед закрытием
	// Но лучше оставить stream открытым и закрыть его позже, если нужно
	// Для регистрации stream можно закрыть после небольшой задержки
	time.Sleep(100 * time.Millisecond) // Даем серверу время прочитать
	regStream.Close()

	logger.Debug("device", "Device ID %s sent to QUIC server", c.deviceID)
	return nil
}

// OpenStream открывает новый stream для проксирования
func (c *Client) OpenStream(ctx context.Context, connID string) (*quic.Stream, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("QUIC connection not established")
	}

	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	logger.Debug("device", "Opened QUIC stream: conn_id=%s", connID)
	return stream, nil
}

// HandleStreams обрабатывает входящие streams (если нужно)
func (c *Client) HandleStreams(ctx context.Context) error {
	for {
		stream, err := c.conn.AcceptStream(ctx)
		if err != nil {
			return fmt.Errorf("failed to accept stream: %w", err)
		}

		go c.handler.HandleStream(ctx, stream)
	}
}

// Close закрывает QUIC соединение
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.CloseWithError(0, "closing")
	}
	return nil
}

// GetConn возвращает QUIC соединение
func (c *Client) GetConn() *quic.Conn {
	return c.conn
}

