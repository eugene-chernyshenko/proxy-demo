package quic

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"example.com/me/myproxy/internal/logger"
	"github.com/quic-go/quic-go"
)

// StreamHandler обрабатывает QUIC streams
type StreamHandler struct {
	// Callback для обработки stream (будет установлен извне)
	onStream func(connID string, stream *quic.Stream) error
}

// NewStreamHandler создает новый stream handler
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{}
}

// SetCallback устанавливает callback для обработки stream
func (h *StreamHandler) SetCallback(onStream func(connID string, stream *quic.Stream) error) {
	h.onStream = onStream
}

// HandleStream обрабатывает QUIC stream
func (h *StreamHandler) HandleStream(ctx context.Context, stream *quic.Stream) {
	defer stream.Close()

	// Используем stream ID как conn_id
	connID := fmt.Sprintf("%d", stream.StreamID())

	logger.Debug("device", "Handling QUIC stream from POP: conn_id=%s", connID)

	// Читаем target address из stream (адрес + \n)
	// Читаем байты до новой строки вручную, чтобы не использовать буфер Scanner
	// который может скрыть данные от ProxyTCP
	var targetAddressBytes []byte
	buf := make([]byte, 1)
	for {
		n, err := stream.Read(buf)
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
			logger.Error("device", "Failed to read target address from stream %s: %v", connID, err)
			return
		}
		if len(targetAddressBytes) > 256 {
			logger.Error("device", "Target address too long for stream %s", connID)
			return
		}
	}

	targetAddress := strings.TrimSpace(string(targetAddressBytes))
	logger.Debug("device", "Received target address %s for stream %s", targetAddress, connID)

	// Проксируем TCP трафик
	// Теперь stream готов для чтения данных от POP (HTTP запрос уже в stream)
	if err := ProxyTCP(stream, targetAddress); err != nil {
		logger.Error("device", "Error proxying TCP for stream %s: %v", connID, err)
	}
}

// ProxyTCP проксирует TCP трафик через QUIC stream
func ProxyTCP(stream *quic.Stream, targetAddress string) error {
	// Подключаемся к целевому адресу
	targetConn, err := net.Dial("tcp", targetAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", targetAddress, err)
	}
	defer targetConn.Close()

	logger.Debug("device", "Proxying TCP traffic: stream -> %s", targetAddress)

	// Пересылаем данные между stream и target connection
	done := make(chan error, 2)

	go func() {
		_, err := io.Copy(targetConn, stream)
		done <- err
	}()

	go func() {
		_, err := io.Copy(stream, targetConn)
		done <- err
	}()

	// Ждем завершения одной из сторон
	err = <-done
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

