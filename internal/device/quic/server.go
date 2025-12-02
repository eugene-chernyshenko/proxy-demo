package quic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"time"

	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
	"github.com/quic-go/quic-go"
)

// Server представляет QUIC server для data-plane
type Server struct {
	registry  *device.Registry
	port      int
	tlsConfig *tls.Config
	listener  *quic.Listener
}

// NewServer создает новый QUIC server
func NewServer(registry *device.Registry, port int, tlsConfig *tls.Config) *Server {
	return &Server{
		registry:  registry,
		port:      port,
		tlsConfig: tlsConfig,
	}
}

// Start запускает QUIC server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen UDP: %w", err)
	}

	config := &quic.Config{
		// Настройки QUIC
	}

	var tlsConf *tls.Config
	if s.tlsConfig != nil {
		tlsConf = s.tlsConfig
	} else {
		// Для тестирования создаем самоподписанный сертификат
		cert, err := generateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		tlsConf = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		logger.Debug("device", "Using self-signed certificate for QUIC (testing only)")
	}

	listener, err := quic.Listen(udpConn, tlsConf, config)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener: %w", err)
	}

	s.listener = listener
	logger.Info("device", "QUIC data-plane server starting on port %d", s.port)

	// Принимаем соединения в цикле
	go s.acceptLoop()

	return nil
}

// acceptLoop принимает QUIC соединения
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept(context.Background())
		if err != nil {
			logger.Error("device", "Failed to accept QUIC connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection обрабатывает QUIC соединение
func (s *Server) handleConnection(conn *quic.Conn) {
	logger.Debug("device", "New QUIC connection from %s", conn.RemoteAddr())

	// Читаем device_id из первого stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	regStream, err := conn.AcceptStream(ctx)
	if err != nil {
		logger.Error("device", "Failed to accept registration stream: %v", err)
		conn.CloseWithError(0, "registration failed")
		return
	}

	// Читаем device_id (строка с новой строкой)
	// Читаем данные до новой строки или EOF
	buf := make([]byte, 256)
	var deviceIDBytes []byte
	
	regStream.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	for {
		n, err := regStream.Read(buf)
		if n > 0 {
			deviceIDBytes = append(deviceIDBytes, buf[:n]...)
			// Проверяем, есть ли новая строка
			for i, b := range deviceIDBytes {
				if b == '\n' {
					deviceIDBytes = deviceIDBytes[:i]
					break
				}
			}
			if len(deviceIDBytes) > 0 && deviceIDBytes[len(deviceIDBytes)-1] != '\n' {
				// Проверяем, есть ли \n в данных
				if bytes.Contains(deviceIDBytes, []byte("\n")) {
					// Нашли новую строку, извлекаем device_id
					parts := bytes.SplitN(deviceIDBytes, []byte("\n"), 2)
					deviceIDBytes = parts[0]
					break
				}
			} else {
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Error("device", "Failed to read device_id: %v", err)
			regStream.Close()
			conn.CloseWithError(0, "failed to read device_id")
			return
		}
	}
	
	regStream.Close()
	
	deviceID := strings.TrimSpace(string(deviceIDBytes))

	logger.Debug("device", "Received device_id: %s from QUIC connection", deviceID)

	if err := s.registry.RegisterQUICConnection(deviceID, conn); err != nil {
		logger.Error("device", "Failed to register QUIC connection: %v", err)
		conn.CloseWithError(0, "registration failed")
		return
	}

	// Обрабатываем streams
	go s.handleStreams(conn, deviceID)
}

// handleStreams обрабатывает QUIC streams
func (s *Server) handleStreams(conn *quic.Conn, deviceID string) {
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			logger.Debug("device", "QUIC connection closed: %v", err)
			return
		}

		// TODO: Определить conn_id для stream
		// Пока что используем stream ID как conn_id
		connID := fmt.Sprintf("%d", stream.StreamID())

		logger.Debug("device", "New QUIC stream: conn_id=%s, device=%s", connID, deviceID)

		// Получаем device и добавляем stream
		device, err := s.registry.GetDevice(deviceID)
		if err != nil {
			logger.Error("device", "Device not found: %v", err)
			stream.Close()
			continue
		}

		device.AddStream(connID, stream)

		// Обрабатываем stream (будет использоваться в outbound)
		// Пока что просто логируем
		go func() {
			defer stream.Close()
			defer device.RemoveStream(connID)
			// Stream будет использоваться в QUIC outbound для проксирования данных
		}()
	}
}

// Stop останавливает QUIC server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// generateSelfSignedCert создает самоподписанный сертификат для тестирования
func generateSelfSignedCert() (tls.Certificate, error) {
	// Генерируем приватный ключ
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Создаем сертификат
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"MyProxy Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Подписываем сертификат
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Конвертируем в формат для TLS
	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	return cert, nil
}

