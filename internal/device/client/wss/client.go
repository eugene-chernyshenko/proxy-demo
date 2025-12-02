package wss

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
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

	if err := c.sendMessage(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to send register request: %w", err)
	}

	// Читаем ответ
	resp, err := c.readMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read register response: %w", err)
	}

	registerResp, ok := resp.(*pb.RegisterResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected message type")
	}

	if registerResp.Status != "ok" {
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

	if err := c.sendMessage(ctx, req); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	// Читаем ответ
	resp, err := c.readMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read heartbeat response: %w", err)
	}

	heartbeatResp, ok := resp.(*pb.HeartbeatResponse)
	if !ok {
		return fmt.Errorf("unexpected message type")
	}

	if heartbeatResp.Status != "ok" {
		return fmt.Errorf("heartbeat failed: %s", heartbeatResp.Status)
	}

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
	for {
		msg, err := c.readMessage(ctx)
		if err != nil {
			if err == io.EOF || websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return nil // Нормальное закрытие
			}
			return fmt.Errorf("failed to read message: %w", err)
		}

		// Обрабатываем команду
		if cmd, ok := msg.(*pb.Command); ok {
			if err := c.handler.HandleCommand(ctx, cmd); err != nil {
				logger.Error("device", "Error handling command: %v", err)
				// Отправляем ответ об ошибке
				resp := &pb.CommandResponse{
					ConnId:  cmd.ConnId,
					Success: false,
					Error:   err.Error(),
				}
				c.sendMessage(ctx, resp)
			}
		}
	}
}

// sendMessage отправляет сообщение через WebSocket
func (c *Client) sendMessage(ctx context.Context, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Отправляем длину сообщения (4 байта) + само сообщение
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))

	writer, err := c.conn.Writer(ctx, websocket.MessageBinary)
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

// readMessage читает сообщение из WebSocket
func (c *Client) readMessage(ctx context.Context) (proto.Message, error) {
	msgType, reader, err := c.conn.Reader(ctx)
	if err != nil {
		return nil, err
	}

	if msgType != websocket.MessageBinary {
		// Читаем до конца frame, чтобы освободить reader
		io.Copy(io.Discard, reader)
		return nil, fmt.Errorf("unexpected message type: %v", msgType)
	}

	// Читаем длину сообщения (4 байта)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBytes); err != nil {
		io.Copy(io.Discard, reader) // Освобождаем reader
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	length := binary.BigEndian.Uint32(lengthBytes)

	// Читаем само сообщение
	messageBytes := make([]byte, length)
	if _, err := io.ReadFull(reader, messageBytes); err != nil {
		// Пытаемся прочитать остаток reader перед возвратом ошибки
		io.Copy(io.Discard, reader)
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// В библиотеке nhooyr.io/websocket reader автоматически закрывается после чтения всего frame
	// Но мы должны убедиться, что прочитали все данные до конца frame
	// Читаем остаток до EOF (если есть)
	// Используем io.Copy для гарантированного чтения всего reader
	io.Copy(io.Discard, reader)

	// Пытаемся определить тип сообщения
	// Пробуем Command
	var cmd pb.Command
	if err := proto.Unmarshal(messageBytes, &cmd); err == nil && cmd.ConnId != "" {
		return &cmd, nil
	}

	// Пробуем RegisterResponse
	var registerResp pb.RegisterResponse
	if err := proto.Unmarshal(messageBytes, &registerResp); err == nil && registerResp.DeviceId != "" {
		return &registerResp, nil
	}

	// Пробуем HeartbeatResponse
	var heartbeatResp pb.HeartbeatResponse
	if err := proto.Unmarshal(messageBytes, &heartbeatResp); err == nil {
		return &heartbeatResp, nil
	}

	return nil, fmt.Errorf("unknown message type")
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

