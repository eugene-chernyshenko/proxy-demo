package outbound

import (
	"net"
)

// DirectOutbound реализует прямое подключение
type DirectOutbound struct {
	dialer *net.Dialer
}

// NewDirectOutbound создает новый direct outbound
func NewDirectOutbound() *DirectOutbound {
	return &DirectOutbound{
		dialer: &net.Dialer{},
	}
}

// Dial устанавливает прямое TCP соединение
func (d *DirectOutbound) Dial(network, address string) (net.Conn, error) {
	return d.dialer.Dial(network, address)
}

