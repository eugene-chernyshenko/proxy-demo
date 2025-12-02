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
		// Убеждаемся, что NextProtos установлен
		if tlsConf.NextProtos == nil {
			tlsConf.NextProtos = []string{"quic-proxy"}
		}
	} else {
		// Для тестирования с самоподписанным сертификатом
		tlsConf = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         c.proxyHost, // Устанавливаем ServerName для SNI
			NextProtos:         []string{"quic-proxy"}, // ALPN протокол для QUIC
		}
	}

	config := &quic.Config{
		// Настройки QUIC
	}

	logger.Debug("device", "Dialing QUIC to %s...", addr)
	conn, err := quic.Dial(ctx, udpConn, udpAddr, tlsConf, config)
	if err != nil {
		udpConn.Close()
		logger.Error("device", "Failed to dial QUIC: %v", err)
		return fmt.Errorf("failed to dial QUIC: %w", err)
	}

	c.conn = conn
	logger.Debug("device", "QUIC connection established to %s", addr)

	// Отправляем device_id в первом stream для идентификации
	logger.Debug("device", "Opening registration stream...")
	regStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to open registration stream")
		logger.Error("device", "Failed to open registration stream: %v", err)
		return fmt.Errorf("failed to open registration stream: %w", err)
	}
	logger.Debug("device", "Registration stream opened")

	// Отправляем device_id как простую строку с новой строкой
	deviceIDBytes := []byte(c.deviceID + "\n")
	logger.Debug("device", "Sending device_id: %s", c.deviceID)
	if _, err := regStream.Write(deviceIDBytes); err != nil {
		regStream.Close()
		conn.CloseWithError(0, "failed to send device_id")
		logger.Error("device", "Failed to send device_id: %v", err)
		return fmt.Errorf("failed to send device_id: %w", err)
	}
	logger.Debug("device", "Device_id sent, waiting before closing stream...")
	
	// Даем серверу время прочитать данные
	time.Sleep(100 * time.Millisecond)
	regStream.Close()
	logger.Debug("device", "Registration stream closed")

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
	logger.Debug("device", "QUIC stream handler started, waiting for streams...")
	for {
		stream, err := c.conn.AcceptStream(ctx)
		if err != nil {
			logger.Debug("device", "QUIC stream accept error: %v", err)
			return fmt.Errorf("failed to accept stream: %w", err)
		}

		logger.Debug("device", "Accepted new QUIC stream: stream_id=%d", stream.StreamID())
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

