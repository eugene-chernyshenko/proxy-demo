package socks5

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// ParseAddress парсит адрес из SOCKS5 connection request
// Читает из reader: [ATYP, address, port]
// Возвращает строку адреса в формате "host:port" или "[ipv6]:port"
func ParseAddress(reader io.Reader) (string, error) {
	// Читаем ATYP (1 byte)
	atypBuf := make([]byte, 1)
	if _, err := io.ReadFull(reader, atypBuf); err != nil {
		return "", fmt.Errorf("failed to read address type: %w", err)
	}
	atyp := atypBuf[0]

	switch atyp {
	case 0x01: // IPv4
		addrBuf := make([]byte, 6) // 4 bytes IP + 2 bytes port
		if _, err := io.ReadFull(reader, addrBuf); err != nil {
			return "", fmt.Errorf("failed to read IPv4 address: %w", err)
		}
		ip := net.IP(addrBuf[0:4])
		port := binary.BigEndian.Uint16(addrBuf[4:6])
		return fmt.Sprintf("%s:%d", ip.String(), port), nil

	case 0x03: // Domain name
		// Читаем длину домена (1 byte)
		domainLenBuf := make([]byte, 1)
		if _, err := io.ReadFull(reader, domainLenBuf); err != nil {
			return "", fmt.Errorf("failed to read domain length: %w", err)
		}
		domainLen := int(domainLenBuf[0])
		if domainLen == 0 || domainLen > 255 {
			return "", fmt.Errorf("invalid domain length: %d", domainLen)
		}

		// Читаем домен + порт
		domainBuf := make([]byte, domainLen+2)
		if _, err := io.ReadFull(reader, domainBuf); err != nil {
			return "", fmt.Errorf("failed to read domain: %w", err)
		}
		domain := string(domainBuf[0:domainLen])
		port := binary.BigEndian.Uint16(domainBuf[domainLen : domainLen+2])
		return fmt.Sprintf("%s:%d", domain, port), nil

	case 0x04: // IPv6
		addrBuf := make([]byte, 18) // 16 bytes IP + 2 bytes port
		if _, err := io.ReadFull(reader, addrBuf); err != nil {
			return "", fmt.Errorf("failed to read IPv6 address: %w", err)
		}
		ip := net.IP(addrBuf[0:16])
		port := binary.BigEndian.Uint16(addrBuf[16:18])
		return fmt.Sprintf("[%s]:%d", ip.String(), port), nil

	default:
		return "", fmt.Errorf("unsupported address type: %d", atyp)
	}
}

