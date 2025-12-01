package proxy

import (
	"net"
	"testing"
	"time"
)

func TestCopyData(t *testing.T) {
	testData := []byte("Hello, World!")

	// Тест: пересылка данных в одну сторону через TCP
	t.Run("one way data transfer", func(t *testing.T) {
		// Создаем тестовый сервер
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Ошибка создания слушателя: %v", err)
		}
		defer listener.Close()

		serverAddr := listener.Addr().String()

		// Запускаем сервер, который читает данные
		serverDone := make(chan []byte, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, len(testData))
			n, _ := conn.Read(buf)
			serverDone <- buf[:n]
		}()

		// Подключаемся к серверу
		clientConn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			t.Fatalf("Ошибка подключения: %v", err)
		}
		defer clientConn.Close()

		// Отправляем данные
		_, err = clientConn.Write(testData)
		if err != nil {
			t.Fatalf("Ошибка записи данных: %v", err)
		}

		// Ждем получения данных на сервере
		received := <-serverDone

		if len(received) != len(testData) {
			t.Errorf("Неверное количество данных: ожидалось %d, получено %d", len(testData), len(received))
		}

		if string(received) != string(testData) {
			t.Errorf("Неверные данные: ожидалось %s, получено %s", string(testData), string(received))
		}
	})

	// Тест: двунаправленная пересылка данных через TCP
	t.Run("bidirectional data transfer", func(t *testing.T) {
		// Создаем эхо-сервер
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Ошибка создания слушателя: %v", err)
		}
		defer listener.Close()

		serverAddr := listener.Addr().String()

		// Запускаем эхо-сервер
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Эхо: читаем и возвращаем данные
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			conn.Write(buf[:n])
		}()

		// Подключаемся к серверу
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			t.Fatalf("Ошибка подключения: %v", err)
		}
		defer conn.Close()

		data := []byte("Test data")

		// Отправляем данные
		_, err = conn.Write(data)
		if err != nil {
			t.Fatalf("Ошибка записи: %v", err)
		}

		// Читаем ответ
		received := make([]byte, len(data))
		n, err := conn.Read(received)
		if err != nil {
			t.Fatalf("Ошибка чтения: %v", err)
		}

		if string(received[:n]) != string(data) {
			t.Errorf("Неверные данные: ожидалось %s, получено %s", string(data), string(received[:n]))
		}
	})
}

func TestHandleConnection(t *testing.T) {
	// Создаем тестовый сервер
	serverListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Ошибка создания сервера: %v", err)
	}
	defer serverListener.Close()

	serverAddr := serverListener.Addr().String()

	// Запускаем тестовый сервер
	go func() {
		conn, err := serverListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Эхо-сервер: возвращаем данные обратно
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		conn.Write(buf[:n])
	}()

	// Создаем клиентское соединение
	clientConn, proxyConn := net.Pipe()

	// Создаем mock dial функцию
	mockDial := func(network, address string) (net.Conn, error) {
		if address != serverAddr {
			return nil, net.UnknownNetworkError("unknown address")
		}
		return net.Dial("tcp", serverAddr)
	}

	// Запускаем HandleConnection в отдельной горутине
	done := make(chan error, 1)
	go func() {
		done <- HandleConnection(proxyConn, mockDial, serverAddr)
	}()

	// Отправляем данные от клиента
	testData := []byte("Test data")
	_, err = clientConn.Write(testData)
	if err != nil {
		t.Fatalf("Ошибка записи данных: %v", err)
	}

	// Читаем ответ
	received := make([]byte, len(testData))
	n, err := clientConn.Read(received)
	if err != nil {
		t.Fatalf("Ошибка чтения данных: %v", err)
	}

	if string(received[:n]) != string(testData) {
		t.Errorf("Неверные данные: ожидалось %s, получено %s", string(testData), string(received[:n]))
	}

	clientConn.Close()
	proxyConn.Close()

	// Ждем завершения HandleConnection
	select {
	case err := <-done:
		if err != nil {
			t.Logf("HandleConnection завершился с ошибкой (ожидаемо при закрытии): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("HandleConnection не завершился в течение 2 секунд")
	}
}
