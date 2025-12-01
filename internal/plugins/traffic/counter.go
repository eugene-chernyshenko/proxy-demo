package traffic

import (
	"sync"
	"time"
)

// Stats представляет статистику трафика
type Stats struct {
	Connections   int64     // Количество соединений
	BytesSent     int64     // Всего отправлено байт
	BytesReceived int64     // Всего получено байт
	LastActivity  time.Time // Время последней активности
}

// BaseCounter базовый счетчик трафика с thread-safe хранилищем
type BaseCounter struct {
	mu     sync.RWMutex
	stats  map[string]*Stats // ID -> Stats
}

// NewBaseCounter создает новый базовый счетчик
func NewBaseCounter() *BaseCounter {
	return &BaseCounter{
		stats: make(map[string]*Stats),
	}
}

// GetStats возвращает статистику для указанного ID
func (b *BaseCounter) GetStats(id string) *Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	stats, exists := b.stats[id]
	if !exists {
		return &Stats{}
	}
	
	// Возвращаем копию для thread-safety
	return &Stats{
		Connections:   stats.Connections,
		BytesSent:     stats.BytesSent,
		BytesReceived: stats.BytesReceived,
		LastActivity:  stats.LastActivity,
	}
}

// AddConnection увеличивает счетчик соединений
func (b *BaseCounter) AddConnection(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.stats[id] == nil {
		b.stats[id] = &Stats{}
	}
	b.stats[id].Connections++
	b.stats[id].LastActivity = time.Now()
}

// AddBytes добавляет байты к статистике
func (b *BaseCounter) AddBytes(id string, sent, received int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.stats[id] == nil {
		b.stats[id] = &Stats{}
	}
	b.stats[id].BytesSent += sent
	b.stats[id].BytesReceived += received
	b.stats[id].LastActivity = time.Now()
}

// GetAllStats возвращает все статистики (для отладки/мониторинга)
func (b *BaseCounter) GetAllStats() map[string]*Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	result := make(map[string]*Stats)
	for id, stats := range b.stats {
		result[id] = &Stats{
			Connections:   stats.Connections,
			BytesSent:     stats.BytesSent,
			BytesReceived: stats.BytesReceived,
			LastActivity:  stats.LastActivity,
		}
	}
	return result
}

