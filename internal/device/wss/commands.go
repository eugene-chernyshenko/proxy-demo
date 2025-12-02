package wss

import (
	"context"

	"example.com/me/myproxy/internal/device"
	pb "example.com/me/myproxy/internal/protocol/pb"
)

// CommandSender отправляет команды устройствам
type CommandSender struct {
	registry *device.Registry
	handler  *Handler
}

// NewCommandSender создает новый CommandSender
func NewCommandSender(registry *device.Registry, handler *Handler) *CommandSender {
	return &CommandSender{
		registry: registry,
		handler:  handler,
	}
}

// SendOpenTCP отправляет команду открытия TCP stream
func (c *CommandSender) SendOpenTCP(ctx context.Context, deviceID, connID, targetAddress string) error {
	cmd := &pb.Command{
		ConnId: connID,
		Command: &pb.Command_OpenTcp{
			OpenTcp: &pb.OpenTCP{
				TargetAddress: targetAddress,
			},
		},
	}

	return c.handler.SendCommand(ctx, deviceID, cmd)
}

// SendOpenUDP отправляет команду открытия UDP datagram
func (c *CommandSender) SendOpenUDP(ctx context.Context, deviceID, connID, targetAddress string) error {
	cmd := &pb.Command{
		ConnId: connID,
		Command: &pb.Command_OpenUdp{
			OpenUdp: &pb.OpenUDP{
				TargetAddress: targetAddress,
			},
		},
	}

	return c.handler.SendCommand(ctx, deviceID, cmd)
}

// SendClose отправляет команду закрытия соединения
func (c *CommandSender) SendClose(ctx context.Context, deviceID, connID string) error {
	cmd := &pb.Command{
		ConnId: connID,
		Command: &pb.Command_Close{
			Close: &pb.Close{},
		},
	}

	return c.handler.SendCommand(ctx, deviceID, cmd)
}

