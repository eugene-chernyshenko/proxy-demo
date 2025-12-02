package wss

import (
	"context"
	"fmt"

	"example.com/me/myproxy/internal/logger"
	pb "example.com/me/myproxy/internal/protocol/pb"
)

// Handler обрабатывает команды от POP
type Handler struct {
	deviceID string
	// Callback для обработки команд (будет установлен извне)
	onOpenTCP func(connID, targetAddress string) error
	onOpenUDP func(connID, targetAddress string) error
	onClose   func(connID string) error
}

// NewHandler создает новый handler
func NewHandler(deviceID string) *Handler {
	return &Handler{
		deviceID: deviceID,
	}
}

// SetCallbacks устанавливает callbacks для обработки команд
func (h *Handler) SetCallbacks(
	onOpenTCP func(connID, targetAddress string) error,
	onOpenUDP func(connID, targetAddress string) error,
	onClose func(connID string) error,
) {
	h.onOpenTCP = onOpenTCP
	h.onOpenUDP = onOpenUDP
	h.onClose = onClose
}

// HandleCommand обрабатывает команду от POP
func (h *Handler) HandleCommand(ctx context.Context, cmd *pb.Command) error {
	logger.Debug("device", "Received command: conn_id=%s", cmd.ConnId)

	switch c := cmd.Command.(type) {
	case *pb.Command_OpenTcp:
		if h.onOpenTCP == nil {
			return fmt.Errorf("onOpenTCP callback not set")
		}
		return h.onOpenTCP(cmd.ConnId, c.OpenTcp.TargetAddress)

	case *pb.Command_OpenUdp:
		if h.onOpenUDP == nil {
			return fmt.Errorf("onOpenUDP callback not set")
		}
		return h.onOpenUDP(cmd.ConnId, c.OpenUdp.TargetAddress)

	case *pb.Command_Close:
		if h.onClose == nil {
			return fmt.Errorf("onClose callback not set")
		}
		return h.onClose(cmd.ConnId)

	default:
		return fmt.Errorf("unknown command type")
	}
}

