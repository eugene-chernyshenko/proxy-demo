package socks5

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// BuildRequest строит SOCKS5 connection request
// Формат: [VER, CMD, RSV, ATYP, address, port]
// VER = 0x05 (SOCKS5)
// CMD = 0x01 (CONNECT)
// RSV = 0x00 (reserved)
func BuildRequest(address string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address format: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port out of range: %d", port)
	}

	request := []byte{0x05, 0x01, 0x00} // VER, CMD=CONNECT, RSV

	ip := net.ParseIP(host)
	if ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			request = append(request, 0x01) // ATYP = IPv4
			request = append(request, ipv4...)
		} else {
			request = append(request, 0x04) // ATYP = IPv6
			request = append(request, ip...)
		}
	} else {
		request = append(request, 0x03) // ATYP = Domain name
		if len(host) > 255 {
			return nil, fmt.Errorf("domain name too long: %s", host)
		}
		request = append(request, byte(len(host)))
		request = append(request, []byte(host)...)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)

	return request, nil
}

