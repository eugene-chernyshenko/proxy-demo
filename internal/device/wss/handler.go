package wss

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

// Handler обрабатывает WSS соединения от devices
type Handler struct {
	registry *device.Registry
}

// NewHandler создает новый WSS handler
func NewHandler(registry *device.Registry) *Handler {
	return &Handler{
		registry: registry,
	}
}

// HandleConnection обрабатывает одно WSS соединение
func (h *Handler) HandleConnection(ctx context.Context, conn *websocket.Conn, remoteAddr string) error {
	// Читаем сообщения в цикле
	for {
		// Читаем сообщение
		_, reader, err := conn.Reader(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				return fmt.Errorf("failed to read message: %w", err)
			}
			return nil // Нормальное закрытие
		}

		// Читаем длину сообщения (4 байта, big-endian)
		lengthBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, lengthBytes); err != nil {
			return fmt.Errorf("failed to read message length: %w", err)
		}
		length := binary.BigEndian.Uint32(lengthBytes)

		// Читаем само сообщение
		messageBytes := make([]byte, length)
		if _, err := io.ReadFull(reader, messageBytes); err != nil {
			// Пытаемся прочитать остаток reader перед возвратом ошибки
			io.Copy(io.Discard, reader)
			return fmt.Errorf("failed to read message: %w", err)
		}

		// Убеждаемся, что reader прочитан до конца (читаем остаток до EOF)
		// Используем io.Copy для гарантированного чтения всего reader
		io.Copy(io.Discard, reader)

		// Обрабатываем сообщение ПОСЛЕ того, как reader полностью прочитан
		if err := h.handleMessage(ctx, conn, messageBytes, remoteAddr); err != nil {
			logger.Error("device", "Error handling message: %v", err)
			// Продолжаем обработку других сообщений
		}
	}
}

// handleMessage обрабатывает одно сообщение
func (h *Handler) handleMessage(ctx context.Context, conn *websocket.Conn, data []byte, remoteAddr string) error {
	// Пытаемся определить тип сообщения по первым байтам
	// Для простоты используем простую эвристику или добавляем type field в proto

	// Пробуем RegisterRequest
	var registerReq pb.RegisterRequest
	if err := proto.Unmarshal(data, &registerReq); err == nil && registerReq.DeviceId != "" {
		return h.handleRegister(ctx, conn, &registerReq, remoteAddr)
	}

	// Пробуем HeartbeatRequest
	var heartbeatReq pb.HeartbeatRequest
	if err := proto.Unmarshal(data, &heartbeatReq); err == nil && heartbeatReq.DeviceId != "" {
		return h.handleHeartbeat(ctx, conn, &heartbeatReq, remoteAddr)
	}

	// Пробуем LoadReport
	var loadReport pb.LoadReport
	if err := proto.Unmarshal(data, &loadReport); err == nil && loadReport.DeviceId != "" {
		return h.handleLoadReport(ctx, conn, &loadReport, remoteAddr)
	}

	// Пробуем CommandResponse
	var cmdResp pb.CommandResponse
	if err := proto.Unmarshal(data, &cmdResp); err == nil && cmdResp.ConnId != "" {
		return h.handleCommandResponse(ctx, conn, &cmdResp, remoteAddr)
	}

	return fmt.Errorf("unknown message type")
}

// handleRegister обрабатывает регистрацию устройства
func (h *Handler) handleRegister(ctx context.Context, conn *websocket.Conn, req *pb.RegisterRequest, remoteAddr string) error {
	logger.Debug("device", "Register request from device %s at %s", req.DeviceId, remoteAddr)

	// Извлекаем метаданные
	metadata := make(map[string]interface{})
	if req.Location != "" {
		metadata["location"] = req.Location
	}
	if req.Capacity > 0 {
		metadata["capacity"] = int(req.Capacity)
	}
	if len(req.Tags) > 0 {
		metadata["tags"] = req.Tags
	}

	// Регистрируем устройство
	_, err := h.registry.RegisterWithWSS(req.DeviceId, remoteAddr, metadata, conn)
	if err != nil {
		logger.Error("device", "Failed to register device %s: %v", req.DeviceId, err)
		resp := &pb.RegisterResponse{
			Status:   "error",
			DeviceId: req.DeviceId,
		}
		return h.sendMessage(ctx, conn, resp)
	}

	// Формируем ответ
	resp := &pb.RegisterResponse{
		Status:      "ok",
		DeviceId:    req.DeviceId,
		QuicAddress: fmt.Sprintf("%s:443", remoteAddr), // TODO: получить из конфига
	}

	logger.Debug("device", "Device %s registered successfully", req.DeviceId)
	return h.sendMessage(ctx, conn, resp)
}

// handleHeartbeat обрабатывает heartbeat
func (h *Handler) handleHeartbeat(ctx context.Context, conn *websocket.Conn, req *pb.HeartbeatRequest, remoteAddr string) error {
	logger.Debug("device", "Heartbeat from device %s", req.DeviceId)

	// Обновляем heartbeat
	if err := h.registry.UpdateHeartbeat(req.DeviceId); err != nil {
		logger.Error("device", "Failed to update heartbeat for device %s: %v", req.DeviceId, err)
		resp := &pb.HeartbeatResponse{
			Status: "error",
		}
		return h.sendMessage(ctx, conn, resp)
	}

	resp := &pb.HeartbeatResponse{
		Status: "ok",
	}
	return h.sendMessage(ctx, conn, resp)
}

// handleLoadReport обрабатывает отчет о нагрузке
func (h *Handler) handleLoadReport(ctx context.Context, conn *websocket.Conn, report *pb.LoadReport, remoteAddr string) error {
	logger.Debug("device", "Load report from device %s: conns=%d, sent=%d, received=%d",
		report.DeviceId, report.ActiveConns, report.BytesSent, report.BytesReceived)

	// Обновляем метрики устройства
	device, err := h.registry.GetDevice(report.DeviceId)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	device.AddBytes(report.BytesSent, report.BytesReceived)
	// ActiveConns обновляется при открытии/закрытии соединений

	return nil
}

// handleCommandResponse обрабатывает ответ на команду
func (h *Handler) handleCommandResponse(ctx context.Context, conn *websocket.Conn, resp *pb.CommandResponse, remoteAddr string) error {
	logger.Debug("device", "Command response: conn_id=%s, success=%v, error=%s",
		resp.ConnId, resp.Success, resp.Error)

	// Обработка ответа на команду (можно использовать для логирования или уведомлений)
	return nil
}

// sendMessage отправляет сообщение через WebSocket
func (h *Handler) sendMessage(ctx context.Context, conn *websocket.Conn, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Отправляем длину сообщения (4 байта) + само сообщение
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))

	writer, err := conn.Writer(ctx, websocket.MessageBinary)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}
	defer writer.Close()

	if _, err := writer.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// SendCommand отправляет команду устройству
func (h *Handler) SendCommand(ctx context.Context, deviceID string, cmd *pb.Command) error {
	dev, err := h.registry.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	wssConn := dev.GetWSSConn()
	if wssConn == nil {
		return fmt.Errorf("WSS connection not established for device %s", deviceID)
	}

	return h.sendMessage(ctx, wssConn, cmd)
}

