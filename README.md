# MyProxy

Модульное прокси-приложение на Go, вдохновленное архитектурой xray.

## Возможности

- **SOCKS5 inbound** - протокол SOCKS5 без авторизации
- **Direct outbound** - прямое подключение в интернет
- **SOCKS5 outbound** - подключение через SOCKS5 прокси
- **Outbound Pool** - динамическое управление пулом устройств через WSS (control-plane) и QUIC (data-plane)
- **Device Client** - клиент для подключения устройств к прокси
- **Система плагинов** - учет трафика по inbound/outbound ID
- **Динамический роутер** - выбор outbound из пула устройств
- **Load Testing Utility** - утилита для нагрузочного тестирования с детальными метриками

## Быстрый старт

### Proxy Server

**Конфигурация (`config.json`):**

```json
{
  "inbound": {
    "type": "socks5",
    "port": 1080,
    "id": "inbound-1"
  },
  "outbound": {
    "type": "direct",
    "id": "outbound-1"
  },
  "outbound_pool": {
    "enabled": true,
    "wss_port": 8443,
    "quic_port": 8444,
    "tls": {
      "enabled": false
    },
    "heartbeat_interval": 30,
    "heartbeat_timeout": 90
  }
}
```

**Запуск:**

```bash
go build -o proxy ./cmd/proxy
./proxy -config config.json -debug
```

### Device Client

**Конфигурация (`device_config.json`):**

```json
{
  "proxy_host": "127.0.0.1",
  "wss_port": 8443,
  "quic_port": 8444,
  "device_id": "device-1",
  "location": "us-east",
  "tags": ["mobile", "wifi"],
  "heartbeat_interval": 30,
  "tls_enabled": false,
  "tls_skip_verify": true
}
```

**Запуск:**

```bash
go build -o device ./cmd/device
./device -config device_config.json -debug
```

## Использование

**Базовый прокси:**

```bash
curl --socks5-hostname 127.0.0.1:1080 https://example.com
```

**С Device Client:**

```bash
# Терминал 1: Proxy
./proxy -config config.json -debug

# Терминал 2: Device
./device -config device_config.json -debug

# Терминал 3: Тест
curl --socks5-hostname 127.0.0.1:1080 http://httpbin.org/get
```

**Load Testing:**

```bash
# Сборка утилиты
go build -o loadtest ./cmd/loadtest

# Запуск нагрузочного теста
./loadtest -proxy 127.0.0.1:1080 -url http://httpbin.org/get -c 10 -n 100

# Параметры:
#   -proxy: адрес SOCKS5 прокси (по умолчанию: 127.0.0.1:1080)
#   -url: целевой URL для тестирования (по умолчанию: http://httpbin.org/get)
#   -c: количество конкурентных запросов (по умолчанию: 10)
#   -n: общее количество запросов (по умолчанию: 100)
```

Утилита выводит отчет с метриками:

- Success/Error rate
- Разрез по типам ошибок
- HTTP status codes
- Latency метрики (TTFB, DNS resolve, Connect, TLSHandshake) с перцентилями (50, 75, 85, 90, 95, 99, 100)
- RPS (Requests Per Second)
- Throughput (KB/s)

## Протокол POP-Device

- **WSS Control-Plane**: Регистрация устройств, heartbeat, команды (порт `wss_port`, по умолчанию 443)
- **QUIC Data-Plane**: Передача данных через streams (порт `quic_port`, по умолчанию 443)
- **Protocol Buffers**: Формат сообщений для WSS
- **NAT-friendly**: QUIC работает через UDP, поддерживает устройства за NAT

## Архитектурные решения

Подробные архитектурные решения задокументированы в ADR:

- [ADR 0001: Plugin System Architecture](docs/adr/0001-plugin-system.md)
- [ADR 0003: Device Client Architecture](docs/adr/0003-device-client.md)
- [ADR 0004: WSS + QUIC Protocol](docs/adr/0004-wss-quic-protocol.md)

## Планы развития

- Авторизация для SOCKS5
- Дополнительные стратегии роутинга (least connections, latency-based)
- Персистентность устройств (Redis)
- IP-migration для QUIC
- UDP через QUIC datagrams
- Автоматическое переподключение device
