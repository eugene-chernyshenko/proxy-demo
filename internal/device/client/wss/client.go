package wss

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"example.com/me/myproxy/internal/constants"
	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
	"example.com/me/myproxy/internal/protocol/wss"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

// Client представляет WSS client для device
type Client struct {
	proxyHost string
	wssPort   int
	deviceID  string
	tlsConfig *tls.Config
	conn      *websocket.Conn
	handler   *Handler
	readMu    sync.Mutex // Мьютекс для синхронизации чтения из WebSocket
	writeMu   sync.Mutex // Мьютекс для синхронизации записи в WebSocket
}

// NewClient создает новый WSS client
func NewClient(proxyHost string, wssPort int, deviceID string, tlsConfig *tls.Config) *Client {
	return &Client{
		proxyHost: proxyHost,
		wssPort:   wssPort,
		deviceID:  deviceID,
		tlsConfig: tlsConfig,
		handler:   NewHandler(deviceID),
	}
}

// Connect подключается к POP через WSS
func (c *Client) Connect(ctx context.Context) error {
	scheme := "ws"
	if c.tlsConfig != nil {
		scheme = "wss"
	}

	url := fmt.Sprintf("%s://%s:%d/", scheme, c.proxyHost, c.wssPort)
	
	dialOptions := &websocket.DialOptions{}
	// TLS конфигурация применяется автоматически для wss://

	conn, _, err := websocket.Dial(ctx, url, dialOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to WSS: %w", err)
	}

	c.conn = conn
	logger.Debug("device", "WSS connection established to %s", url)
	return nil
}

// Register регистрирует устройство через WSS
func (c *Client) Register(ctx context.Context, location string, tags []string) (*pb.RegisterResponse, error) {
	req := &pb.RegisterRequest{
		DeviceId: c.deviceID,
		Location: location,
		Tags:     tags,
	}

	logger.Debug("device", "Sending RegisterRequest: device_id=%s, location=%s, tags=%v", c.deviceID, location, tags)
	if err := c.sendMessage(ctx, req); err != nil {
		logger.Error("device", "Failed to send register request: %v", err)
		return nil, fmt.Errorf("failed to send register request: %w", err)
	}
	logger.Debug("device", "RegisterRequest sent, waiting for response...")

	// Читаем ответ
	// Важно: читаем ответ до запуска HandleMessages(), чтобы избежать конфликта чтения
	logger.Debug("device", "Calling readMessage() to read RegisterResponse...")
	resp, err := c.readMessage(ctx)
	if err != nil {
		logger.Error("device", "Failed to read register response: %v", err)
		return nil, fmt.Errorf("failed to read register response: %w", err)
	}

	logger.Debug("device", "readMessage() returned, received message type: %T", resp)
	logger.Debug("device", "Register response received, checking type...")
	registerResp, ok := resp.(*pb.RegisterResponse)
	if !ok {
		logger.Error("device", "Unexpected message type: %T, expected *pb.RegisterResponse", resp)
		return nil, fmt.Errorf("unexpected message type: %T", resp)
	}

	logger.Debug("device", "RegisterResponse received: status=%s, device_id=%s, quic_address=%s", 
		registerResp.Status, registerResp.DeviceId, registerResp.QuicAddress)

	if registerResp.Status != constants.StatusOK {
		logger.Error("device", "Registration failed with status: %s", registerResp.Status)
		return nil, fmt.Errorf("registration failed: %s", registerResp.Status)
	}

	logger.Debug("device", "Device %s registered successfully, quic_address: %s", c.deviceID, registerResp.QuicAddress)
	return registerResp, nil
}

// SendHeartbeat отправляет heartbeat
func (c *Client) SendHeartbeat(ctx context.Context) error {
	req := &pb.HeartbeatRequest{
		DeviceId:  c.deviceID,
		Timestamp: time.Now().Unix(),
	}

	logger.Debug("device", "Sending HeartbeatRequest: device_id=%s, timestamp=%d", c.deviceID, req.Timestamp)
	if err := c.sendMessage(ctx, req); err != nil {
		logger.Error("device", "Failed to send heartbeat: %v", err)
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	// Читаем ответ
	resp, err := c.readMessage(ctx)
	if err != nil {
		logger.Error("device", "Failed to read heartbeat response: %v", err)
		return fmt.Errorf("failed to read heartbeat response: %w", err)
	}

	logger.Debug("device", "Received heartbeat response type: %T", resp)
	heartbeatResp, ok := resp.(*pb.HeartbeatResponse)
	if !ok {
		logger.Error("device", "Unexpected message type for heartbeat: %T", resp)
		return fmt.Errorf("unexpected message type: %T", resp)
	}

	logger.Debug("device", "HeartbeatResponse received: status=%s", heartbeatResp.Status)
	if heartbeatResp.Status != constants.StatusOK {
		logger.Error("device", "Heartbeat failed with status: %s", heartbeatResp.Status)
		return fmt.Errorf("heartbeat failed: %s", heartbeatResp.Status)
	}

	logger.Debug("device", "Heartbeat successful for device %s", c.deviceID)
	return nil
}

// SendLoadReport отправляет отчет о нагрузке
func (c *Client) SendLoadReport(ctx context.Context, activeConns int32, bytesSent, bytesReceived int64) error {
	report := &pb.LoadReport{
		DeviceId:      c.deviceID,
		Timestamp:     time.Now().Unix(),
		ActiveConns:   activeConns,
		BytesSent:     bytesSent,
		BytesReceived: bytesReceived,
	}

	return c.sendMessage(ctx, report)
}

// HandleMessages обрабатывает входящие сообщения (команды от POP)
func (c *Client) HandleMessages(ctx context.Context) error {
	logger.Debug("device", "WSS message handler started, waiting for messages...")
		for {
			// Используем контекст с таймаутом для чтения, чтобы не блокировать слишком долго
			// Это позволяет SendHeartbeat прервать чтение и прочитать свой ответ
			readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			msg, err := c.readMessage(readCtx)
			cancel()
			
			if err != nil {
				// Проверяем, не истек ли родительский контекст
				if ctx.Err() != nil {
					logger.Debug("device", "WSS message handler context cancelled")
					return ctx.Err()
				}
				// Если это таймаут, продолжаем ожидание (это нормально)
				// Увеличили таймаут до 30 секунд, чтобы уменьшить ложные срабатывания
				if err == context.DeadlineExceeded {
					logger.Debug("device", "WSS read timeout, continuing...")
					continue
				}
				// Проверяем различные типы ошибок закрытия соединения
				errStr := err.Error()
				if err == io.EOF || 
				   websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				   errStr == "use of closed network connection" ||
				   errStr == "failed to get reader: use of closed network connection" ||
				   errStr == "failed to get reader: EOF" ||
				   errStr == "failed to get reader: failed to read frame header: EOF" ||
				   errStr == "failed to read message: failed to get reader: EOF" ||
				   errStr == "failed to read message: failed to get reader: failed to read frame header: EOF" {
					logger.Debug("device", "WSS connection closed normally")
					return nil // Нормальное закрытие
				}
				logger.Error("device", "Failed to read WSS message: %v", err)
				return fmt.Errorf("failed to read message: %w", err)
			}

		logger.Debug("device", "Received WSS message type: %T", msg)

		// Игнорируем ответы (HeartbeatResponse, RegisterResponse) - они обрабатываются в других местах
		if _, ok := msg.(*pb.HeartbeatResponse); ok {
			logger.Debug("device", "Ignoring HeartbeatResponse in HandleMessages (handled in SendHeartbeat)")
			continue
		}
		if _, ok := msg.(*pb.RegisterResponse); ok {
			logger.Debug("device", "Ignoring RegisterResponse in HandleMessages (handled in Register)")
			continue
		}

		// Обрабатываем только команды
		if cmd, ok := msg.(*pb.Command); ok {
			logger.Debug("device", "Processing command: conn_id=%s", cmd.ConnId)
			if err := c.handler.HandleCommand(ctx, cmd); err != nil {
				logger.Error("device", "Error handling command: %v", err)
				// Отправляем ответ об ошибке
				resp := &pb.CommandResponse{
					ConnId:  cmd.ConnId,
					Success: false,
					Error:   err.Error(),
				}
				c.sendMessage(ctx, resp)
			} else {
				logger.Debug("device", "Command processed successfully: conn_id=%s", cmd.ConnId)
			}
		} else {
			logger.Debug("device", "Received non-command message: %T", msg)
		}
	}
}

// sendMessage отправляет сообщение через WebSocket
// Использует мьютекс для предотвращения одновременной записи
func (c *Client) sendMessage(ctx context.Context, msg proto.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return wss.SendMessage(ctx, c.conn, msg)
}

// readMessage читает сообщение из WebSocket
// ВАЖНО: Использует мьютекс для предотвращения одновременного чтения
// из одного WebSocket соединения (SendHeartbeat и HandleMessages)
func (c *Client) readMessage(ctx context.Context) (proto.Message, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	return wss.ReadMessage(ctx, c.conn)
}

// Close закрывает WSS соединение
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "closing")
	}
	return nil
}

// GetHandler возвращает handler для установки callbacks
func (c *Client) GetHandler() *Handler {
	return c.handler
}

// GetConn возвращает WebSocket соединение
func (c *Client) GetConn() *websocket.Conn {
	return c.conn
}

