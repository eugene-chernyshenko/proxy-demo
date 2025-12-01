package outbound

import (
	"net"
	"testing"
)

func TestDirectOutbound_Dial(t *testing.T) {
	outbound := NewDirectOutbound()

	// Тест: подключение к несуществующему адресу должно вернуть ошибку
	t.Run("dial non-existent address", func(t *testing.T) {
		conn, err := outbound.Dial("tcp", "127.0.0.1:99999")
		if err == nil {
			conn.Close()
			t.Error("Ожидалась ошибка при подключении к несуществующему адресу")
		}
	})

	// Тест: подключение к локальному серверу
	t.Run("dial local server", func(t *testing.T) {
		// Создаем тестовый сервер
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Ошибка создания слушателя: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()

		// Подключаемся через direct outbound
		conn, err := outbound.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Ошибка подключения: %v", err)
		}
		defer conn.Close()

		// Проверяем, что соединение установлено
		if conn == nil {
			t.Error("Соединение не установлено")
		}
	})

	// Тест: проверка типа соединения
	t.Run("connection type", func(t *testing.T) {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Ошибка создания слушателя: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()
		conn, err := outbound.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Ошибка подключения: %v", err)
		}
		defer conn.Close()

		// Проверяем, что это TCP соединение
		if conn.RemoteAddr().Network() != "tcp" {
			t.Errorf("Неверный тип сети: ожидалось tcp, получено %s", conn.RemoteAddr().Network())
		}
	})
}

func TestNewDirectOutbound(t *testing.T) {
	outbound := NewDirectOutbound()

	if outbound == nil {
		t.Error("NewDirectOutbound вернул nil")
	}

	if outbound.dialer == nil {
		t.Error("Dialer не инициализирован")
	}
}

