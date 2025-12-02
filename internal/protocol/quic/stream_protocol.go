package quic

import (
	"fmt"
	"io"
	"strings"

	"example.com/me/myproxy/internal/constants"
	"github.com/quic-go/quic-go"
)

// WriteTargetAddress записывает target address в QUIC stream
// Формат: "address\n" (строка с новой строкой в конце)
func WriteTargetAddress(stream *quic.Stream, address string) error {
	addrBytes := []byte(address + "\n")
	if _, err := (*stream).Write(addrBytes); err != nil {
		return fmt.Errorf("failed to write target address: %w", err)
	}
	return nil
}

// ReadTargetAddress читает target address из QUIC stream
// Читает байты до новой строки включительно
// Возвращает адрес без завершающей новой строки
func ReadTargetAddress(stream *quic.Stream) (string, error) {
	var targetAddressBytes []byte
	buf := make([]byte, 1)
	
	for {
		n, err := (*stream).Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			targetAddressBytes = append(targetAddressBytes, buf[0])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read target address: %w", err)
		}
		if len(targetAddressBytes) > constants.MaxTargetAddressLen {
			return "", fmt.Errorf("target address too long (max %d bytes)", constants.MaxTargetAddressLen)
		}
	}

	targetAddress := strings.TrimSpace(string(targetAddressBytes))
	return targetAddress, nil
}

