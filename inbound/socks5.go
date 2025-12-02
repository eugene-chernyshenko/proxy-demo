package inbound

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
	"example.com/me/myproxy/internal/protocol/socks5"
)

// SOCKS5Inbound реализует SOCKS5 inbound
type SOCKS5Inbound struct {
	port     int
	listener net.Listener
}

// NewSOCKS5Inbound создает новый SOCKS5 inbound
func NewSOCKS5Inbound(port int) *SOCKS5Inbound {
	return &SOCKS5Inbound{
		port: port,
	}
}

// Start запускает SOCKS5 слушатель
func (s *SOCKS5Inbound) Start(handler Handler) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to start SOCKS5 listener: %w", err)
	}

	s.listener = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Listener closed
				return
			}

			remoteAddr := conn.RemoteAddr().String()
			logger.Debug("inbound", "New inbound connection from %s", remoteAddr)

			go func(c net.Conn) {
				if err := s.handleSOCKS5(c, handler); err != nil {
					logger.Error("inbound", "Error handling SOCKS5 connection from %s: %v", remoteAddr, err)
				}
			}(conn)
		}
	}()

	return nil
}

// Stop останавливает SOCKS5 слушатель
func (s *SOCKS5Inbound) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// handleSOCKS5 обрабатывает SOCKS5 соединение
func (s *SOCKS5Inbound) handleSOCKS5(conn net.Conn, handler Handler) error {
	remoteAddr := conn.RemoteAddr().String()
	defer func() {
		logger.Debug("inbound", "SOCKS5 connection closed from %s", remoteAddr)
		conn.Close()
	}()

	logger.Debug("inbound", "Processing SOCKS5 greeting from %s", remoteAddr)

	// Step 1: Greeting
	// Client sends: [VER, NMETHODS, METHODS...]
	buf := make([]byte, 257) // Maximum size: 1 + 1 + 255 methods
	n, err := conn.Read(buf[:2])
	if err != nil || n != 2 {
		return fmt.Errorf("failed to read greeting: %w", err)
	}

	if buf[0] != 0x05 { // VER = SOCKS5
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	nmethods := int(buf[1])
	if nmethods == 0 {
		return fmt.Errorf("no authentication methods")
	}

	n, err = conn.Read(buf[2 : 2+nmethods])
	if err != nil || n != nmethods {
		return fmt.Errorf("failed to read methods: %w", err)
	}

	// Look for method 0x00 (no authentication)
	hasNoAuth := false
	for i := 0; i < nmethods; i++ {
		if buf[2+i] == 0x00 {
			hasNoAuth = true
			break
		}
	}

	if !hasNoAuth {
		// Send 0xFF (no acceptable methods)
		conn.Write([]byte{0x05, 0xFF})
		return fmt.Errorf("authentication required (not supported)")
	}

	// Send response: [VER, METHOD]
	_, err = conn.Write([]byte{0x05, 0x00})
	if err != nil {
		return fmt.Errorf("failed to send greeting response: %w", err)
	}

	logger.Debug("inbound", "SOCKS5 greeting completed for %s", remoteAddr)

	// Step 2: Connection request
	// Client sends: [VER, CMD, RSV, ATYP, address, port]
	logger.Debug("inbound", "Reading connection request from %s", remoteAddr)
	n, err = conn.Read(buf[:4])
	if err != nil || n != 4 {
		return fmt.Errorf("failed to read request: %w", err)
	}

	if buf[0] != 0x05 {
		return fmt.Errorf("invalid version in request: %d", buf[0])
	}

	cmd := buf[1]
	if cmd != 0x01 { // CONNECT
		// Send error: command not supported
		conn.Write(socks5.BuildErrorResponse(socks5.ReplyCommandNotSupported))
		return fmt.Errorf("unsupported command: %d", cmd)
	}

	// ATYP уже прочитан в buf[3]
	atyp := buf[3]

	// Парсим адрес вручную, так как ATYP уже прочитан
	var targetAddress string
	switch atyp {
	case 0x01: // IPv4
		addrBuf := make([]byte, 6) // 4 bytes IP + 2 bytes port
		if _, err := io.ReadFull(conn, addrBuf); err != nil {
			conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
			return fmt.Errorf("failed to read IPv4 address: %w", err)
		}
		ip := net.IP(addrBuf[0:4])
		port := binary.BigEndian.Uint16(addrBuf[4:6])
		targetAddress = fmt.Sprintf("%s:%d", ip.String(), port)

	case 0x03: // Domain name
		domainLenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLenBuf); err != nil {
			conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
			return fmt.Errorf("failed to read domain length: %w", err)
		}
		domainLen := int(domainLenBuf[0])
		if domainLen == 0 || domainLen > 255 {
			conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
			return fmt.Errorf("invalid domain length: %d", domainLen)
		}
		domainBuf := make([]byte, domainLen+2)
		if _, err := io.ReadFull(conn, domainBuf); err != nil {
			conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
			return fmt.Errorf("failed to read domain: %w", err)
		}
		domain := string(domainBuf[0:domainLen])
		port := binary.BigEndian.Uint16(domainBuf[domainLen : domainLen+2])
		targetAddress = fmt.Sprintf("%s:%d", domain, port)

	case 0x04: // IPv6
		addrBuf := make([]byte, 18) // 16 bytes IP + 2 bytes port
		if _, err := io.ReadFull(conn, addrBuf); err != nil {
			conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
			return fmt.Errorf("failed to read IPv6 address: %w", err)
		}
		ip := net.IP(addrBuf[0:16])
		port := binary.BigEndian.Uint16(addrBuf[16:18])
		targetAddress = fmt.Sprintf("[%s]:%d", ip.String(), port)

	default:
		conn.Write(socks5.BuildErrorResponse(socks5.ReplyAddressTypeNotSupported))
		return fmt.Errorf("unsupported address type: %d", atyp)
	}

	logger.Debug("inbound", "SOCKS5 connection request from %s to %s", remoteAddr, targetAddress)

	// Send success response используя общий парсер
	response := socks5.BuildResponse(socks5.ReplySuccess)
	_, err = conn.Write(response)
	if err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	logger.Debug("inbound", "SOCKS5 connection established from %s to %s", remoteAddr, targetAddress)

	// Создаем контекст соединения
	ctx := plugin.NewConnectionContext(remoteAddr, targetAddress)

	// Now forward the connection through handler
	err = handler(conn, targetAddress, ctx)
	if err != nil {
		logger.Debug("inbound", "SOCKS5 connection from %s to %s closed with error: %v", remoteAddr, targetAddress, err)
	} else {
		logger.Debug("inbound", "SOCKS5 connection from %s to %s closed normally", remoteAddr, targetAddress)
	}
	return err
}
