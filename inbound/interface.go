package inbound

import "net"

// Handler функция для обработки нового соединения
// targetAddress - целевой адрес для подключения (определяется протоколом)
type Handler func(conn net.Conn, targetAddress string) error

// Inbound интерфейс для inbound обработчиков
type Inbound interface {
	// Start запускает слушатель и вызывает handler для каждого нового соединения
	Start(handler Handler) error
	// Stop останавливает слушатель
	Stop() error
}
