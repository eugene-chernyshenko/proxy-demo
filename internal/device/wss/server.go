package wss

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"example.com/me/myproxy/internal/device"
	"example.com/me/myproxy/internal/logger"
	"nhooyr.io/websocket"
)

// Server представляет WSS server для control-plane
type Server struct {
	registry   *device.Registry
	port       int
	tlsConfig  *tls.Config
	httpServer *http.Server
	handler    *Handler
}

// NewServer создает новый WSS server
func NewServer(registry *device.Registry, port int, tlsConfig *tls.Config) *Server {
	handler := NewHandler(registry)
	return &Server{
		registry:  registry,
		port:      port,
		tlsConfig: tlsConfig,
		handler:   handler,
	}
}

// Start запускает WSS server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebSocket)

	addr := fmt.Sprintf(":%d", s.port)
	s.httpServer = &http.Server{
		Addr:      addr,
		Handler:   mux,
		TLSConfig: s.tlsConfig,
	}

	logger.Info("device", "WSS control-plane server starting on port %d", s.port)

	if s.tlsConfig != nil {
		return s.httpServer.ListenAndServeTLS("", "")
	}

	// Для тестирования без TLS
	return s.httpServer.ListenAndServe()
}

// Stop останавливает WSS server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// handleWebSocket обрабатывает WebSocket соединения
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Принимаем WebSocket соединение
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // В продакшене нужно ограничить
	})
	if err != nil {
		logger.Error("device", "Failed to accept WebSocket connection: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "connection closed")

	logger.Debug("device", "New WSS connection from %s", r.RemoteAddr)

	// Обрабатываем соединение
	if err := s.handler.HandleConnection(r.Context(), conn, r.RemoteAddr); err != nil {
		logger.Error("device", "Error handling WSS connection: %v", err)
		conn.Close(websocket.StatusInternalError, err.Error())
		return
	}

	conn.Close(websocket.StatusNormalClosure, "connection closed")
}

