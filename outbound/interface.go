package outbound

import "net"

// Outbound интерфейс для outbound обработчиков
type Outbound interface {
	// Dial устанавливает соединение с целевым адресом
	Dial(network, address string) (net.Conn, error)
}

