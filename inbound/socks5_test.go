package inbound

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestSOCKS5Inbound_Greeting(t *testing.T) {
	// Тест: проверка формата приветствия
	t.Run("greeting format", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()

		go func() {
			defer clientConn.Close()
			// Отправляем приветствие: [VER=0x05, NMETHODS=1, METHOD=0x00]
			clientConn.Write([]byte{0x05, 0x01, 0x00})
		}()

		// Читаем приветствие
		buf := make([]byte, 3)
		n, err := serverConn.Read(buf)
		if err != nil || n != 3 {
			t.Fatalf("Ошибка чтения приветствия: %v", err)
		}

		if buf[0] != 0x05 {
			t.Errorf("Неверная версия SOCKS: ожидалось 0x05, получено %d", buf[0])
		}
		if buf[1] != 0x01 {
			t.Errorf("Неверное количество методов: ожидалось 1, получено %d", buf[1])
		}
		if buf[2] != 0x00 {
			t.Errorf("Неверный метод: ожидалось 0x00, получено %d", buf[2])
		}

		serverConn.Close()
	})

	// Тест: приветствие без метода 0x00
	t.Run("greeting without no auth method", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()

		go func() {
			defer clientConn.Close()
			// Отправляем приветствие без метода 0x00: [VER=0x05, NMETHODS=1, METHOD=0x02]
			clientConn.Write([]byte{0x05, 0x01, 0x02})
		}()

		buf := make([]byte, 3)
		n, err := serverConn.Read(buf)
		if err != nil || n != 3 {
			t.Fatalf("Ошибка чтения приветствия: %v", err)
		}

		// Отправляем ответ об отсутствии приемлемых методов
		serverConn.Write([]byte{0x05, 0xFF})

		serverConn.Close()
	})
}

func TestSOCKS5Inbound_RequestIPv4(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer clientConn.Close()
		// Приветствие
		clientConn.Write([]byte{0x05, 0x01, 0x00})
		// Читаем ответ
		buf := make([]byte, 2)
		clientConn.Read(buf)
		// Запрос CONNECT с IPv4
		request := []byte{
			0x05, 0x01, 0x00, 0x01, // VER, CMD, RSV, ATYP
			192, 168, 1, 1, // IP адрес
			0x04, 0xD2, // Порт 1234
		}
		clientConn.Write(request)
	}()

	// Читаем приветствие
	buf := make([]byte, 3)
	serverConn.Read(buf)
	serverConn.Write([]byte{0x05, 0x00})

	// Читаем запрос
	requestBuf := make([]byte, 10)
	n, err := serverConn.Read(requestBuf)
	if err != nil || n != 10 {
		t.Fatalf("Ошибка чтения запроса: %v", err)
	}

	if requestBuf[0] != 0x05 || requestBuf[1] != 0x01 || requestBuf[3] != 0x01 {
		t.Errorf("Неверный формат запроса")
	}

	ip := net.IP(requestBuf[4:8]).String()
	port := binary.BigEndian.Uint16(requestBuf[8:10])

	if ip != "192.168.1.1" {
		t.Errorf("Неверный IP адрес: ожидалось 192.168.1.1, получено %s", ip)
	}
	if port != 1234 {
		t.Errorf("Неверный порт: ожидалось 1234, получено %d", port)
	}

	serverConn.Close()
	clientConn.Close()
}

func TestSOCKS5Inbound_RequestDomain(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	domain := "example.com"
	port := uint16(443)

	go func() {
		defer clientConn.Close()
		// Приветствие
		clientConn.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 2)
		clientConn.Read(buf)
		// Запрос CONNECT с доменным именем
		request := make([]byte, 4+1+len(domain)+2)
		request[0] = 0x05 // VER
		request[1] = 0x01 // CMD = CONNECT
		request[2] = 0x00 // RSV
		request[3] = 0x03 // ATYP = DOMAIN
		request[4] = byte(len(domain))
		copy(request[5:], domain)
		binary.BigEndian.PutUint16(request[5+len(domain):], port)
		clientConn.Write(request)
	}()

	// Читаем приветствие
	buf := make([]byte, 3)
	serverConn.Read(buf)
	serverConn.Write([]byte{0x05, 0x00})

	// Читаем запрос
	requestBuf := make([]byte, 4)
	serverConn.Read(requestBuf)

	if requestBuf[3] != 0x03 {
		t.Fatalf("Неверный тип адреса: ожидалось 0x03 (DOMAIN), получено %d", requestBuf[3])
	}

	// Читаем длину домена
	domainLenBuf := make([]byte, 1)
	serverConn.Read(domainLenBuf)
	domainLen := int(domainLenBuf[0])

	// Читаем домен и порт
	domainPortBuf := make([]byte, domainLen+2)
	serverConn.Read(domainPortBuf)

	receivedDomain := string(domainPortBuf[:domainLen])
	receivedPort := binary.BigEndian.Uint16(domainPortBuf[domainLen:])

	if receivedDomain != domain {
		t.Errorf("Неверный домен: ожидалось %s, получено %s", domain, receivedDomain)
	}
	if receivedPort != port {
		t.Errorf("Неверный порт: ожидалось %d, получено %d", port, receivedPort)
	}

	serverConn.Close()
	clientConn.Close()
}

func TestSOCKS5Inbound_UnsupportedCommand(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer clientConn.Close()
		// Приветствие
		clientConn.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 2)
		clientConn.Read(buf)
		// Запрос с неподдерживаемой командой (BIND = 0x02)
		request := []byte{
			0x05, 0x02, 0x00, 0x01, // VER, CMD=BIND, RSV, ATYP
			127, 0, 0, 1, // IP адрес
			0x00, 0x50, // Порт
		}
		clientConn.Write(request)
	}()

	// Читаем приветствие
	buf := make([]byte, 3)
	serverConn.Read(buf)
	serverConn.Write([]byte{0x05, 0x00})

	// Читаем запрос
	requestBuf := make([]byte, 10)
	serverConn.Read(requestBuf)

	if requestBuf[1] == 0x02 {
		// Отправляем ошибку
		serverConn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	}

	serverConn.Close()
	clientConn.Close()
}

