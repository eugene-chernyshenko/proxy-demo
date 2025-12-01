package plugin

import "time"

// ConnectionContext содержит метаданные соединения для передачи между компонентами
type ConnectionContext struct {
	// Идентификаторы
	InboundID  string // Идентификатор inbound (из конфигурации или метаданных)
	OutboundID string // Идентификатор outbound (из конфигурации или метаданных)

	// Метаданные соединения
	RemoteAddr    string // Адрес клиента
	TargetAddress string // Целевой адрес для подключения

	// Временные метки
	StartTime time.Time // Время начала соединения

	// Статистика трафика
	BytesSent     int64 // Количество отправленных байт
	BytesReceived int64 // Количество полученных байт

	// Дополнительные метаданные для плагинов
	Metadata map[string]interface{}
}

// NewConnectionContext создает новый контекст соединения
func NewConnectionContext(remoteAddr, targetAddress string) *ConnectionContext {
	return &ConnectionContext{
		RemoteAddr:    remoteAddr,
		TargetAddress: targetAddress,
		StartTime:     time.Now(),
		Metadata:      make(map[string]interface{}),
	}
}

