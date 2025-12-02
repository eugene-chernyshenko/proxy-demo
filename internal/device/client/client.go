package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"example.com/me/myproxy/internal/device/client/quic"
	"example.com/me/myproxy/internal/device/client/wss"
	"example.com/me/myproxy/internal/logger"
)

// Client основной клиент для device
type Client struct {
	wssClient      *wss.Client
	quicClient     *quic.Client
	deviceID       string
	stopChan       chan struct{}
	heartbeatStop  chan struct{}
	heartbeatTicker *time.Ticker
}

// NewClient создает новый device client
func NewClient(proxyHost string, wssPort, quicPort int, deviceID string, tlsEnabled bool, tlsSkipVerify bool) *Client {
	var tlsConfig *tls.Config
	if tlsEnabled {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		}
	}

	return &Client{
		wssClient:  wss.NewClient(proxyHost, wssPort, deviceID, tlsConfig),
		quicClient: quic.NewClient(proxyHost, quicPort, deviceID, tlsConfig),
		deviceID:   deviceID,
		stopChan:   make(chan struct{}),
	}
}

// Start запускает device client
func (c *Client) Start(location string, tags []string, heartbeatInterval int) error {
	ctx := context.Background()

	// Шаг 1: Подключение к WSS
	if err := c.wssClient.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to WSS: %w", err)
	}

	// Шаг 2: Регистрация через WSS
	registerResp, err := c.wssClient.Register(ctx, location, tags)
	if err != nil {
		c.wssClient.Close()
		return fmt.Errorf("failed to register device: %w", err)
	}

	logger.Info("device", "Device %s registered successfully, quic_address: %s", c.deviceID, registerResp.QuicAddress)

	// Шаг 3: Подключение к QUIC
	if err := c.quicClient.Connect(ctx); err != nil {
		c.wssClient.Close()
		return fmt.Errorf("failed to connect to QUIC: %w", err)
	}

	// Шаг 3.5: Запуск обработки входящих QUIC streams (от POP)
	go func() {
		if err := c.quicClient.HandleStreams(ctx); err != nil {
			logger.Error("device", "QUIC stream handling error: %v", err)
		}
	}()

	// Шаг 4: Настройка обработчиков команд
	c.setupCommandHandlers()

	// Шаг 5: Запуск обработки сообщений WSS (команды от POP)
	go func() {
		if err := c.wssClient.HandleMessages(ctx); err != nil {
			logger.Error("device", "WSS message handling error: %v", err)
		}
	}()

	// Шаг 6: Запуск heartbeat
	if heartbeatInterval > 0 {
		c.startHeartbeat(ctx, heartbeatInterval)
	}

	logger.Info("device", "Device client started for device %s", c.deviceID)
	return nil
}

// setupCommandHandlers настраивает обработчики команд от POP
func (c *Client) setupCommandHandlers() {
	wssHandler := c.wssClient.GetHandler()
	wssHandler.SetCallbacks(
		// onOpenTCP
		func(connID, targetAddress string) error {
			logger.Debug("device", "Opening TCP stream: conn_id=%s, target=%s", connID, targetAddress)
			stream, err := c.quicClient.OpenStream(context.Background(), connID)
			if err != nil {
				return fmt.Errorf("failed to open QUIC stream: %w", err)
			}

			// Проксируем TCP трафик через QUIC stream
			go func() {
				if err := quic.ProxyTCP(stream, targetAddress); err != nil {
					logger.Error("device", "Error proxying TCP: %v", err)
				}
			}()

			return nil
		},
		// onOpenUDP
		func(connID, targetAddress string) error {
			logger.Debug("device", "Opening UDP datagram: conn_id=%s, target=%s", connID, targetAddress)
			// TODO: Реализовать UDP через QUIC datagrams
			return fmt.Errorf("UDP not implemented yet")
		},
		// onClose
		func(connID string) error {
			logger.Debug("device", "Closing connection: conn_id=%s", connID)
			// TODO: Закрыть соответствующий stream
			return nil
		},
	)
}

// startHeartbeat запускает периодическую отправку heartbeat
func (c *Client) startHeartbeat(ctx context.Context, interval int) {
	c.heartbeatTicker = time.NewTicker(time.Duration(interval) * time.Second)
	c.heartbeatStop = make(chan struct{})

	go func() {
		for {
			select {
			case <-c.heartbeatTicker.C:
				if err := c.wssClient.SendHeartbeat(ctx); err != nil {
					logger.Error("device", "Heartbeat failed: %v", err)
				} else {
					logger.Debug("device", "Heartbeat sent for device %s", c.deviceID)
				}
			case <-c.heartbeatStop:
				return
			case <-c.stopChan:
				return
			}
		}
	}()
}

// Stop останавливает device client
func (c *Client) Stop() error {
	logger.Info("device", "Stopping device client for device %s", c.deviceID)

	close(c.stopChan)

	if c.heartbeatStop != nil {
		close(c.heartbeatStop)
	}
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}

	if c.wssClient != nil {
		c.wssClient.Close()
	}
	if c.quicClient != nil {
		c.quicClient.Close()
	}

	return nil
}
