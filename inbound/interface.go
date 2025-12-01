package inbound

import (
	"net"

	"example.com/me/myproxy/internal/plugin"
)

// Handler функция для обработки нового соединения
// conn - соединение от клиента
// targetAddress - целевой адрес для подключения (определяется протоколом)
// ctx - контекст соединения с метаданными
type Handler func(conn net.Conn, targetAddress string, ctx *plugin.ConnectionContext) error

// Inbound интерфейс для inbound обработчиков
type Inbound interface {
	// Start запускает слушатель и вызывает handler для каждого нового соединения
	Start(handler Handler) error
	// Stop останавливает слушатель
	Stop() error
}
