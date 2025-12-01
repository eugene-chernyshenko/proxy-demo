package outbound

import (
	"encoding/binary"
	"fmt"
	"net"

	"example.com/me/myproxy/internal/logger"
)

// SOCKS5Outbound реализует подключение через SOCKS5 прокси
type SOCKS5Outbound struct {
	proxyAddress string
	dialer       *net.Dialer
}

// NewSOCKS5Outbound создает новый SOCKS5 outbound
func NewSOCKS5Outbound(proxyAddress string) *SOCKS5Outbound {
	return &SOCKS5Outbound{
		proxyAddress: proxyAddress,
		dialer:       &net.Dialer{},
	}
}

// Dial устанавливает соединение с целевым адресом через SOCKS5 прокси
func (s *SOCKS5Outbound) Dial(network, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	logger.Debug("outbound", "Connecting to SOCKS5 proxy at %s", s.proxyAddress)

	// Connect to SOCKS5 proxy
	conn, err := s.dialer.Dial("tcp", s.proxyAddress)
	if err != nil {
		logger.Debug("outbound", "Failed to connect to SOCKS5 proxy %s: %v", s.proxyAddress, err)
		return nil, fmt.Errorf("failed to connect to SOCKS5 proxy: %w", err)
	}

	// Perform SOCKS5 handshake
	if err := s.performSOCKS5Handshake(conn, address); err != nil {
		conn.Close()
		logger.Debug("outbound", "SOCKS5 handshake failed for %s: %v", address, err)
		return nil, fmt.Errorf("SOCKS5 handshake failed: %w", err)
	}

	logger.Debug("outbound", "SOCKS5 connection established to %s via %s", address, s.proxyAddress)
	return conn, nil
}

// performSOCKS5Handshake выполняет SOCKS5 handshake
func (s *SOCKS5Outbound) performSOCKS5Handshake(conn net.Conn, targetAddress string) error {
	// Step 1: Send greeting
	// [VER=0x05, NMETHODS=1, METHOD=0x00 (no auth)]
	greeting := []byte{0x05, 0x01, 0x00}
	if _, err := conn.Write(greeting); err != nil {
		return fmt.Errorf("failed to send greeting: %w", err)
	}

	// Read greeting response
	response := make([]byte, 2)
	if _, err := conn.Read(response); err != nil {
		return fmt.Errorf("failed to read greeting response: %w", err)
	}

	if response[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version in response: %d", response[0])
	}

	if response[1] != 0x00 {
		return fmt.Errorf("authentication required (not supported): method %d", response[1])
	}

	// Step 2: Send connection request
	request, err := s.buildConnectionRequest(targetAddress)
	if err != nil {
		return fmt.Errorf("failed to build connection request: %w", err)
	}

	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("failed to send connection request: %w", err)
	}

	// Read connection response
	responseBuf := make([]byte, 4)
	if _, err := conn.Read(responseBuf); err != nil {
		return fmt.Errorf("failed to read connection response: %w", err)
	}

	if responseBuf[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS version in response: %d", responseBuf[0])
	}

	if responseBuf[1] != 0x00 {
		return fmt.Errorf("connection failed: error code %d", responseBuf[1])
	}

	// Read remaining response based on address type
	atyp := responseBuf[3]
	var remainingBytes int
	switch atyp {
	case 0x01: // IPv4
		remainingBytes = 6 // 4 bytes IP + 2 bytes port
	case 0x03: // Domain name
		domainLenBuf := make([]byte, 1)
		if _, err := conn.Read(domainLenBuf); err != nil {
			return fmt.Errorf("failed to read domain length: %w", err)
		}
		remainingBytes = int(domainLenBuf[0]) + 2 // domain + port
	case 0x04: // IPv6
		remainingBytes = 18 // 16 bytes IP + 2 bytes port
	default:
		return fmt.Errorf("unsupported address type in response: %d", atyp)
	}

	remaining := make([]byte, remainingBytes)
	if _, err := conn.Read(remaining); err != nil {
		return fmt.Errorf("failed to read remaining response: %w", err)
	}

	return nil
}

// buildConnectionRequest строит SOCKS5 connection request
func (s *SOCKS5Outbound) buildConnectionRequest(address string) ([]byte, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address format: %w", err)
	}

	portNum := 0
	if port != "" {
		_, err := fmt.Sscanf(port, "%d", &portNum)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(portNum))

	// Try to parse as IP address
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.To4() != nil {
			// IPv4
			request := make([]byte, 10)
			request[0] = 0x05 // VER
			request[1] = 0x01 // CMD = CONNECT
			request[2] = 0x00 // RSV
			request[3] = 0x01 // ATYP = IPv4
			copy(request[4:8], ip.To4())
			copy(request[8:10], portBytes)
			return request, nil
		} else {
			// IPv6
			request := make([]byte, 22)
			request[0] = 0x05 // VER
			request[1] = 0x01 // CMD = CONNECT
			request[2] = 0x00 // RSV
			request[3] = 0x04 // ATYP = IPv6
			copy(request[4:20], ip.To16())
			copy(request[20:22], portBytes)
			return request, nil
		}
	}

	// Domain name
	hostBytes := []byte(host)
	if len(hostBytes) > 255 {
		return nil, fmt.Errorf("domain name too long: %d bytes", len(hostBytes))
	}

	request := make([]byte, 4+1+len(hostBytes)+2)
	request[0] = 0x05 // VER
	request[1] = 0x01 // CMD = CONNECT
	request[2] = 0x00 // RSV
	request[3] = 0x03 // ATYP = DOMAIN
	request[4] = byte(len(hostBytes))
	copy(request[5:5+len(hostBytes)], hostBytes)
	copy(request[5+len(hostBytes):], portBytes)

	return request, nil
}
