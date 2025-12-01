package outbound

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestSOCKS5Outbound_Dial(t *testing.T) {
	// Create a test SOCKS5 proxy server
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	proxyAddr := listener.Addr().String()

	// Start SOCKS5 proxy server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				// Read greeting
				buf := make([]byte, 3)
				c.Read(buf)

				// Send greeting response
				c.Write([]byte{0x05, 0x00})

				// Read connection request
				requestBuf := make([]byte, 4)
				c.Read(requestBuf)

				atyp := requestBuf[3]
				var remainingBytes int
				switch atyp {
				case 0x01: // IPv4
					remainingBytes = 6
				case 0x03: // Domain
					domainLenBuf := make([]byte, 1)
					c.Read(domainLenBuf)
					remainingBytes = int(domainLenBuf[0]) + 2
				case 0x04: // IPv6
					remainingBytes = 18
				}

				remaining := make([]byte, remainingBytes)
				c.Read(remaining)

				// Send success response
				response := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
				c.Write(response)
			}(conn)
		}
	}()

	outbound := NewSOCKS5Outbound(proxyAddr)

	t.Run("connect via SOCKS5 proxy", func(t *testing.T) {
		conn, err := outbound.Dial("tcp", "example.com:80")
		if err != nil {
			t.Fatalf("Failed to dial via SOCKS5: %v", err)
		}
		defer conn.Close()

		if conn == nil {
			t.Error("Connection is nil")
		}
	})

	t.Run("unsupported network", func(t *testing.T) {
		_, err := outbound.Dial("udp", "example.com:80")
		if err == nil {
			t.Error("Expected error for unsupported network")
		}
	})
}

func TestBuildConnectionRequest(t *testing.T) {
	outbound := NewSOCKS5Outbound("127.0.0.1:1080")

	t.Run("IPv4 address", func(t *testing.T) {
		request, err := outbound.buildConnectionRequest("192.168.1.1:80")
		if err != nil {
			t.Fatalf("Failed to build request: %v", err)
		}

		if len(request) != 10 {
			t.Errorf("Invalid request length: expected 10, got %d", len(request))
		}

		if request[0] != 0x05 || request[1] != 0x01 || request[3] != 0x01 {
			t.Error("Invalid request format")
		}

		port := binary.BigEndian.Uint16(request[8:10])
		if port != 80 {
			t.Errorf("Invalid port: expected 80, got %d", port)
		}
	})

	t.Run("domain name", func(t *testing.T) {
		request, err := outbound.buildConnectionRequest("example.com:443")
		if err != nil {
			t.Fatalf("Failed to build request: %v", err)
		}

		if request[0] != 0x05 || request[1] != 0x01 || request[3] != 0x03 {
			t.Error("Invalid request format")
		}

		domainLen := int(request[4])
		domain := string(request[5 : 5+domainLen])
		if domain != "example.com" {
			t.Errorf("Invalid domain: expected example.com, got %s", domain)
		}

		port := binary.BigEndian.Uint16(request[5+domainLen : 5+domainLen+2])
		if port != 443 {
			t.Errorf("Invalid port: expected 443, got %d", port)
		}
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := outbound.buildConnectionRequest("invalid")
		if err == nil {
			t.Error("Expected error for invalid address")
		}
	})
}

func TestNewSOCKS5Outbound(t *testing.T) {
	outbound := NewSOCKS5Outbound("127.0.0.1:1080")

	if outbound == nil {
		t.Error("NewSOCKS5Outbound returned nil")
	}

	if outbound.proxyAddress != "127.0.0.1:1080" {
		t.Errorf("Invalid proxy address: expected 127.0.0.1:1080, got %s", outbound.proxyAddress)
	}

	if outbound.dialer == nil {
		t.Error("Dialer is not initialized")
	}
}

