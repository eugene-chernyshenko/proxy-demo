package wss

import (
	"context"
	"fmt"

	"example.com/me/myproxy/internal/constants"
	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
	wssproto "example.com/me/myproxy/internal/protocol/wss"
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
		logger.Debug("device", "Waiting for next message from %s...", remoteAddr)
		msg, err := wssproto.ReadMessage(ctx, conn)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				logger.Debug("device", "Error reading message from %s: %v", remoteAddr, err)
				return fmt.Errorf("failed to read message: %w", err)
			}
			logger.Debug("device", "Connection from %s closed normally", remoteAddr)
			return nil // Нормальное закрытие
		}

		logger.Debug("device", "Received message in HandleConnection loop from %s: %T", remoteAddr, msg)

		// Обрабатываем сообщение
		if err := h.handleMessage(ctx, conn, msg, remoteAddr); err != nil {
			logger.Error("device", "Error handling message from %s: %v", remoteAddr, err)
			// Продолжаем обработку других сообщений
		}
		logger.Debug("device", "Message from %s handled, continuing loop...", remoteAddr)
	}
}

// handleMessage обрабатывает одно сообщение
func (h *Handler) handleMessage(ctx context.Context, conn *websocket.Conn, msg proto.Message, remoteAddr string) error {
	logger.Debug("device", "Received message type: %T from %s", msg, remoteAddr)
	
	switch m := msg.(type) {
	case *pb.RegisterRequest:
		return h.handleRegister(ctx, conn, m, remoteAddr)
	case *pb.HeartbeatRequest:
		return h.handleHeartbeat(ctx, conn, m, remoteAddr)
	case *pb.LoadReport:
		return h.handleLoadReport(ctx, conn, m, remoteAddr)
	case *pb.CommandResponse:
		return h.handleCommandResponse(ctx, conn, m, remoteAddr)
	case *pb.RegisterResponse, *pb.HeartbeatResponse:
		// Игнорируем ответы - это сообщения, которые сервер отправляет device, а не получает от него
		logger.Debug("device", "Ignoring response message type: %T (server should not receive responses)", msg)
		return nil
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
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
			Status:   constants.StatusError,
			DeviceId: req.DeviceId,
		}
		return h.sendMessage(ctx, conn, resp)
	}

	// Формируем ответ
	// Извлекаем IP адрес из remoteAddr (формат "IP:port")
	quicHost := remoteAddr
	if idx := len(remoteAddr) - 1; idx >= 0 {
		// Убираем порт из remoteAddr, оставляем только IP
		for i := len(remoteAddr) - 1; i >= 0; i-- {
			if remoteAddr[i] == ':' {
				quicHost = remoteAddr[:i]
				break
			}
		}
	}
	resp := &pb.RegisterResponse{
		Status:      constants.StatusOK,
		DeviceId:    req.DeviceId,
		QuicAddress: fmt.Sprintf("%s:%d", quicHost, constants.DefaultQUICPort),
	}

	logger.Debug("device", "Device %s registered successfully", req.DeviceId)
	logger.Debug("device", "Sending RegisterResponse: status=%s, device_id=%s, quic_address=%s", resp.Status, resp.DeviceId, resp.QuicAddress)
	if err := h.sendMessage(ctx, conn, resp); err != nil {
		logger.Error("device", "Failed to send RegisterResponse: %v", err)
		return err
	}
	logger.Debug("device", "RegisterResponse sent successfully, returning from handleRegister")
	// Важно: после отправки ответа мы возвращаемся из handleRegister,
	// и HandleConnection продолжит читать следующее сообщение в цикле
	return nil
}

// handleHeartbeat обрабатывает heartbeat
func (h *Handler) handleHeartbeat(ctx context.Context, conn *websocket.Conn, req *pb.HeartbeatRequest, remoteAddr string) error {
	logger.Debug("device", "Heartbeat from device %s", req.DeviceId)

	// Обновляем heartbeat
	if err := h.registry.UpdateHeartbeat(req.DeviceId); err != nil {
		logger.Error("device", "Failed to update heartbeat for device %s: %v", req.DeviceId, err)
		resp := &pb.HeartbeatResponse{
			Status: constants.StatusError,
		}
		return h.sendMessage(ctx, conn, resp)
	}

	resp := &pb.HeartbeatResponse{
		Status: constants.StatusOK,
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
	return wssproto.SendMessage(ctx, conn, msg)
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

