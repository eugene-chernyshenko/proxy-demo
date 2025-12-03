package wss

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"example.com/me/myproxy/internal/constants"
	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SendMessage отправляет Protocol Buffers сообщение через WebSocket
func SendMessage(ctx context.Context, conn *websocket.Conn, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Отправляем длину сообщения (4 байта, big-endian) + само сообщение
	lengthBytes := make([]byte, constants.MessageLengthSize)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))

	writer, err := conn.Writer(ctx, websocket.MessageBinary)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	if _, err := writer.Write(lengthBytes); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write length: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Важно: writer.Close() должен быть вызван для отправки frame
	// Вызываем явно, чтобы убедиться, что данные отправлены
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}

// ReadMessage читает Protocol Buffers сообщение из WebSocket
func ReadMessage(ctx context.Context, conn *websocket.Conn) (proto.Message, error) {
	// Получаем reader для следующего сообщения
	// ВАЖНО: Reader() блокируется до получения следующего сообщения от peer
	msgType, reader, err := conn.Reader(ctx)
	if err != nil {
		// Проверяем, не EOF ли это (нормальное закрытие соединения)
		errStr := err.Error()
		if err == io.EOF || 
		   errStr == "EOF" || 
		   errStr == "failed to read frame header: EOF" {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to get reader: %w", err)
	}

	if msgType != websocket.MessageBinary {
		// Читаем до конца frame, чтобы освободить reader
		DiscardReader(reader)
		return nil, fmt.Errorf("unexpected message type: %v", msgType)
	}

	// Читаем длину сообщения (4 байта, big-endian)
	lengthBytes := make([]byte, constants.MessageLengthSize)
	if _, err := io.ReadFull(reader, lengthBytes); err != nil {
		DiscardReader(reader) // Освобождаем reader
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	length := binary.BigEndian.Uint32(lengthBytes)

	// Читаем само сообщение
	messageBytes := make([]byte, length)
	if _, err := io.ReadFull(reader, messageBytes); err != nil {
		// Пытаемся прочитать остаток reader перед возвратом ошибки
		DiscardReader(reader)
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// ВАЖНО: Читаем остаток reader до EOF перед возвратом
	// Это гарантирует, что следующий Reader() не получит старые данные
	// В библиотеке nhooyr.io/websocket reader должен быть полностью прочитан
	DiscardReader(reader)

	// Определяем тип сообщения
	// Логируем первые несколько байт для отладки
	if len(messageBytes) > 0 && len(messageBytes) < 100 {
		logger.Debug("wss", "Unmarshaling message: length=%d, first_bytes=%x", length, messageBytes[:min(len(messageBytes), 20)])
	}
	msg, err := UnmarshalMessage(messageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message (length=%d): %w", length, err)
	}
	return msg, nil
}

// DiscardReader читает и отбрасывает все данные из reader до EOF
func DiscardReader(reader io.Reader) {
	// Используем io.Copy для полного чтения WebSocket reader до EOF
	// io.Copy гарантированно читает все данные до EOF или ошибки
	// Это критично для nhooyr.io/websocket - reader должен быть полностью прочитан
	// перед вызовом следующего conn.Reader()
	_, _ = io.Copy(io.Discard, reader)
}

// UnmarshalMessage определяет тип Protocol Buffers сообщения и unmarshals его
func UnmarshalMessage(data []byte) (proto.Message, error) {
	// Порядок важен! Проверяем запросы перед ответами, так как они имеют более уникальные поля
	
	// Пробуем RegisterRequest (проверяем по DeviceId и наличию Location/Tags)
	var registerReq pb.RegisterRequest
	if err := proto.Unmarshal(data, &registerReq); err == nil && registerReq.DeviceId != "" {
		// Сначала проверяем, что это не RegisterResponse
		var registerRespTest pb.RegisterResponse
		proto.Unmarshal(data, &registerRespTest)
		// Если Status = "ok" или "error", это точно RegisterResponse, не RegisterRequest
		if registerRespTest.Status == "ok" || registerRespTest.Status == "error" {
			// Это RegisterResponse, пропускаем RegisterRequest
		} else if registerRespTest.QuicAddress != "" {
			// Если есть QuicAddress, это RegisterResponse, не RegisterRequest
		} else {
			// Если есть Location или Tags, это точно RegisterRequest (у RegisterResponse их нет)
			if registerReq.Location != "" || len(registerReq.Tags) > 0 {
				return &registerReq, nil
			}
			// Если нет Location/Tags и нет QuicAddress/Status, это может быть RegisterRequest
			if registerRespTest.QuicAddress == "" && registerRespTest.Status == "" {
				return &registerReq, nil
			}
		}
	}

	// Пробуем HeartbeatRequest (проверяем по DeviceId и Timestamp)
	var heartbeatReq pb.HeartbeatRequest
	if err := proto.Unmarshal(data, &heartbeatReq); err == nil && heartbeatReq.DeviceId != "" && heartbeatReq.Timestamp > 0 {
		return &heartbeatReq, nil
	}

	// Пробуем RegisterResponse (проверяем по Status и QuicAddress - уникальные поля для ответа)
	var registerResp pb.RegisterResponse
	if err := proto.Unmarshal(data, &registerResp); err == nil {
		// RegisterResponse имеет Status и может иметь QuicAddress
		// Если Status = "ok" или "error", это точно RegisterResponse (эти значения не могут быть в RegisterRequest)
		if registerResp.Status == "ok" || registerResp.Status == "error" {
			return &registerResp, nil
		}
		// Если есть QuicAddress, это точно RegisterResponse (RegisterRequest не имеет этого поля)
		if registerResp.QuicAddress != "" {
			// Дополнительная проверка: убеждаемся, что это не RegisterRequest
			var registerReqTest pb.RegisterRequest
			proto.Unmarshal(data, &registerReqTest)
			// Если нет Location и Tags, это RegisterResponse
			if registerReqTest.Location == "" && len(registerReqTest.Tags) == 0 {
				return &registerResp, nil
			}
		}
	}

	// Пробуем HeartbeatResponse (проверяем по Status, но НЕ по DeviceId/Timestamp)
	var heartbeatResp pb.HeartbeatResponse
	if err := proto.Unmarshal(data, &heartbeatResp); err == nil {
		// HeartbeatResponse имеет только Status, без DeviceId или Timestamp
		// Проверяем, что это не HeartbeatRequest
		var heartbeatReqTest pb.HeartbeatRequest
		proto.Unmarshal(data, &heartbeatReqTest)
		if heartbeatResp.Status != "" && heartbeatReqTest.Timestamp == 0 {
			return &heartbeatResp, nil
		}
	}


	// Пробуем LoadReport (проверяем по специфичным полям)
	var loadReport pb.LoadReport
	if err := proto.Unmarshal(data, &loadReport); err == nil && loadReport.DeviceId != "" && loadReport.Timestamp > 0 {
		return &loadReport, nil
	}

	// Пробуем Command (проверяем по ConnId и наличию command)
	var cmd pb.Command
	if err := proto.Unmarshal(data, &cmd); err == nil && cmd.ConnId != "" && cmd.Command != nil {
		return &cmd, nil
	}

	// Пробуем CommandResponse (проверяем по ConnId)
	var cmdResp pb.CommandResponse
	if err := proto.Unmarshal(data, &cmdResp); err == nil && cmdResp.ConnId != "" {
		return &cmdResp, nil
	}

	return nil, fmt.Errorf("unknown message type")
}

