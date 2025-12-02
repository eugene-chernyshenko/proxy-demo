# MyProxy

Модульное прокси-приложение на Go, вдохновленное архитектурой xray.

## Архитектура

Приложение построено на модульной архитектуре с разделением на:

- **Inbound** - обработчики входящих соединений
- **Outbound** - обработчики исходящих соединений
- **Proxy Core** - основная логика пересылки трафика

## Этап 1 (текущий)

- SOCKS5 inbound - протокол SOCKS5 без авторизации
- Direct outbound - прямое подключение в интернет
- SOCKS5 outbound - подключение через SOCKS5 прокси без авторизации
- Поддержка IPv4, IPv6 и доменных имен
- Система плагинов для учета трафика
- Динамический роутер для выбора outbound
- **Outbound Pool** - динамическое управление пулом устройств через WSS (control-plane) и QUIC (data-plane)
- **Device Client** - клиент для подключения устройств к прокси через WSS и QUIC

## Использование

### Конфигурация

Создайте файл `config.json`:

**Direct outbound (прямое подключение):**

```json
{
  "inbound": {
    "type": "socks5",
    "port": 1080
  },
  "outbound": {
    "type": "direct"
  }
}
```

**SOCKS5 outbound (через другой SOCKS5 прокси):**

```json
{
  "inbound": {
    "type": "socks5",
    "port": 1080
  },
  "outbound": {
    "type": "socks5",
    "proxy_address": "127.0.0.1:1081"
  }
}
```

**С плагинами учета трафика:**

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
  "plugins": {
    "traffic_inbound": {
      "enabled": true
    },
    "traffic_outbound": {
      "enabled": true
    }
  }
}
```

**С Outbound Pool (динамический пул устройств):**

```json
{
  "inbound": {
    "type": "socks5",
    "port": 1080,
    "id": "edge-inbound"
  },
  "outbound": {
    "type": "direct",
    "id": "fallback-outbound"
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

**Примечание:**

- SOCKS5 прокси работает без авторизации. Целевой адрес определяется из SOCKS5 протокола.
- Direct outbound работает как прямой выход в интернет.
- SOCKS5 outbound позволяет цепочку прокси.
- Outbound Pool позволяет устройствам подключаться через WSS (control-plane на порту `wss_port`) и QUIC (data-plane на порту `quic_port`) для динамического роутинга трафика.
- Для тестирования можно использовать нестандартные порты (например, 8443 для WSS, 8444 для QUIC) и отключить TLS.

### Запуск Proxy Server

```bash
# Компиляция
go build -o proxy ./cmd/proxy

# Запуск
./proxy

# Или через go run
go run ./cmd/proxy
```

### Запуск Device Client

```bash
# Компиляция
go build -o device ./cmd/device

# Запуск
./device -config device_config.json

# Или через go run
go run ./cmd/device -config device_config.json
```

### CLI аргументы для Proxy

- `-config` - путь к файлу конфигурации (по умолчанию: `config.json`)
- `-port` - порт для inbound (переопределяет значение из конфига)
- `-debug` - включить debug логирование

Пример:

```bash
./proxy -port 8080 -config myconfig.json -debug
```

### CLI аргументы для Device Client

- `-config` - путь к JSON файлу конфигурации (по умолчанию: `device_config.json`)
- `-proxy` - proxy host (по умолчанию: `127.0.0.1`)
- `-wss-port` - WSS control-plane port (по умолчанию: `443`)
- `-quic-port` - QUIC data-plane port (по умолчанию: `443`)
- `-device-id` - device ID (required)
- `-location` - device location (optional)
- `-heartbeat-interval` - heartbeat interval в секундах (по умолчанию: `30`)
- `-tls-enabled` - включить TLS для WSS и QUIC соединений (по умолчанию: `false`)
- `-tls-skip-verify` - пропустить проверку TLS сертификатов (для тестирования)
- `-debug` - включить debug логирование

Пример:

```bash
./device -proxy 127.0.0.1 -wss-port 8443 -quic-port 8444 -device-id device-1 -debug
```

### Запуск тестов

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с подробным выводом
go test ./... -v

# Запуск тестов конкретного пакета
go test ./inbound -v
```

### Пример использования

**Базовый прокси:**

После запуска прокси можно использовать его как SOCKS5 прокси:

```bash
# Использование с curl
curl --socks5-hostname 127.0.0.1:1080 https://example.com

# Использование с браузером
# Настройте прокси на 127.0.0.1:1080, тип SOCKS5
```

**С Device Client (Outbound Pool):**

```bash
# Терминал 1: Запустите proxy с outbound pool
./proxy -config test_config.json -debug

# Терминал 2: Запустите device client
./device -config device_config.json -debug

# Терминал 3: Протестируйте через прокси (трафик пойдет через device)
curl --socks5-hostname 127.0.0.1:1080 https://example.com
```

### Тестирование Outbound Pool

**Полный тест с device client:**

```bash
# Терминал 1: Запустите proxy с outbound pool
./proxy -config test_config.json -debug

# Терминал 2: Запустите device client
./device -config device_config.json -debug

# Терминал 3: Протестируйте через прокси (трафик пойдет через device)
curl --socks5-hostname 127.0.0.1:1080 https://example.com
```

**Тест WSS + QUIC (полный тест):**

```bash
# Терминал 1: Запустите proxy с outbound pool
./proxy -config test_config.json -debug

# Терминал 2: Запустите device client
./device -config device_config.json -debug

# Терминал 3: Протестируйте через прокси (трафик пойдет через device)
curl --socks5-hostname 127.0.0.1:1080 http://httpbin.org/get
```

**Примечание:** Убедитесь, что в `test_config.json` и `device_config.json` указаны одинаковые порты для WSS и QUIC.

## Device Client

Device client позволяет устройствам подключаться к прокси и использоваться для роутинга трафика.

### Конфигурация

Создайте файл `device_config.json`:

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

### Запуск Device Client

```bash
# Компиляция
go build -o device ./cmd/device

# Запуск
./device -config device_config.json

# Или с CLI флагами
./device -proxy 127.0.0.1 -wss-port 443 -quic-port 443 -device-id device-1 -tls-skip-verify -debug
```

### CLI аргументы

- `-config` - путь к JSON файлу конфигурации (по умолчанию: `device_config.json`)
- `-proxy` - proxy host (по умолчанию: `127.0.0.1`)
- `-wss-port` - WSS control-plane port (по умолчанию: `443`)
- `-quic-port` - QUIC data-plane port (по умолчанию: `443`)
- `-device-id` - device ID (required)
- `-location` - device location (optional)
- `-heartbeat-interval` - heartbeat interval в секундах (по умолчанию: `30`)
- `-tls-enabled` - включить TLS для WSS и QUIC соединений (по умолчанию: `false`)
- `-tls-skip-verify` - пропустить проверку TLS сертификатов (для тестирования)
- `-debug` - включить debug логирование

### Как это работает

1. **WSS Control-Plane**: Device подключается к WSS на `wss_port` (ws:// или wss:// в зависимости от `tls_enabled`) и отправляет Protocol Buffers сообщение `RegisterRequest` с `device_id` и метаданными
2. **QUIC Data-Plane**: Device устанавливает QUIC соединение на `quic_port` (UDP) для передачи данных через streams. При первом подключении отправляет `device_id` через первый stream для идентификации
3. **Heartbeat**: Device отправляет heartbeat каждые `heartbeat_interval` секунд через WSS (Protocol Buffers `HeartbeatRequest`)
4. **Обработка трафика**: Когда POP получает входящее соединение через SOCKS5:
   - Router выбирает device из пула (round-robin)
   - POP открывает QUIC stream и отправляет target address (например, "httpbin.org:80\n")
   - Device читает target address из stream и подключается к целевому адресу через direct outbound
   - Device пересылает TCP трафик между QUIC stream и целевым соединением (bidirectional)
5. **Offline Detection**: Если device не отправляет heartbeat в течение `heartbeat_timeout` секунд, он помечается как offline и исключается из роутинга
6. **Автоматическое переподключение**: Если WSS или QUIC соединение разрывается, device должен переподключиться вручную (автоматическое переподключение планируется)

## Структура проекта

```
myproxy/
├── cmd/
│   ├── proxy/
│   │   └── main.go         # Точка входа proxy server
│   └── device/
│       └── main.go         # Точка входа device client
├── config/
│   ├── config.go          # Структуры конфигурации
│   ├── loader.go          # Загрузка из файла и CLI
│   └── loader_test.go     # Тесты конфигурации
├── inbound/
│   ├── interface.go       # Интерфейс Inbound
│   ├── socks5.go          # SOCKS5 inbound реализация
│   └── socks5_test.go     # Тесты SOCKS5
├── outbound/
│   ├── interface.go       # Интерфейс Outbound
│   ├── direct.go          # Direct outbound реализация
│   ├── direct_test.go     # Тесты direct outbound
│   ├── socks5.go          # SOCKS5 outbound реализация
│   ├── socks5_test.go     # Тесты SOCKS5 outbound
│   └── quic.go            # QUIC outbound для устройств
│   └── pool.go            # Outbound pool для кеширования
├── proxy/
│   ├── core.go            # Основная логика прокси
│   └── core_test.go       # Тесты прокси
└── internal/
    ├── logger/            # Система логирования
    ├── protocol/
    │   └── socks5/        # Общий SOCKS5 парсер
    │       ├── parser.go  # Парсинг адресов
    │       ├── request.go # Построение запросов
    │       └── response.go # Построение ответов
    ├── plugin/            # Система плагинов
    │   ├── interface.go   # Интерфейсы плагинов
    │   ├── manager.go     # Менеджер плагинов
    │   └── context.go     # Контекст соединения
    ├── plugins/
    │   └── traffic/       # Плагины учета трафика
    ├── router/            # Система роутинга
    │   ├── interface.go   # Интерфейс Router
    │   ├── static.go      # Статический роутер
    │   ├── dynamic.go     # Динамический роутер
    │   └── strategy.go    # Стратегии роутинга
    ├── protocol/
    │   └── pb/            # Protocol Buffers схемы
    │       ├── control.proto  # Сообщения WSS control-plane
    │       └── *.pb.go    # Сгенерированный Go код
    └── device/            # Управление устройствами
        ├── client/        # Клиентская логика device
        │   ├── client.go  # Основной клиент
        │   ├── wss/        # WSS control-plane client
        │   │   ├── client.go
        │   │   └── handler.go
        │   └── quic/       # QUIC data-plane client
        │       ├── client.go
        │       └── stream_handler.go
        ├── device.go      # Структура Device (server-side)
        ├── registry.go    # Реестр устройств (server-side)
        ├── wss/            # WSS control-plane server
        │   ├── server.go
        │   ├── handler.go
        │   └── commands.go
        └── quic/           # QUIC data-plane server
            └── server.go
```

## Outbound Pool

Outbound Pool позволяет устройствам динамически подключаться к прокси и использоваться для роутинга трафика. Это особенно полезно для устройств за NAT.

### Как это работает (Server-side)

1. **Регистрация устройства**: Устройство подключается к WSS и отправляет Protocol Buffers сообщение `RegisterRequest` с `device_id` и метаданными
2. **QUIC Connection**: Устройство устанавливает QUIC соединение на `quic_port` для передачи данных через streams
3. **Heartbeat**: Устройство отправляет heartbeat каждые 30 секунд (настраивается) через WSS
4. **Роутинг**: Router выбирает устройство из пула по round-robin стратегии для обработки трафика
5. **Offline Detection**: Устройство помечается как offline после 90 секунд без heartbeat (настраивается)
6. **Commands**: POP отправляет команды `OpenTCP`/`OpenUDP` через WSS, device открывает QUIC stream и проксирует трафик

### Конфигурация Proxy Server

```json
{
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

**Примечание:** Для тестирования можно использовать нестандартные порты (например, 8443 для WSS, 8444 для QUIC) и отключить TLS. В продакшене рекомендуется использовать стандартные порты (443 для обоих) и включить TLS.

### Протокол для устройств (используется device client автоматически)

**WSS Control-Plane (WebSocket Secure):**
- Device подключается к WSS на `wss_port` (ws:// если TLS отключен, wss:// если включен)
- Формат сообщений: Protocol Buffers с префиксом длины (4 байта, big-endian)
- Device отправляет:
  - `RegisterRequest` - регистрация устройства с `device_id` и метаданными
  - `HeartbeatRequest` - периодический heartbeat каждые `heartbeat_interval` секунд
  - `LoadReport` - отчет о нагрузке (опционально)
- POP отправляет:
  - `Command` с `conn_id` и типом команды (пока не используется, вместо этого используется QUIC stream)

**QUIC Data-Plane:**
- Device устанавливает QUIC соединение на `quic_port` (UDP)
- При первом подключении device отправляет `device_id` через первый stream (строка с новой строкой)
- POP открывает новый QUIC stream для каждого соединения и отправляет target address (например, "httpbin.org:80\n")
- Device читает target address из stream, подключается к целевому адресу и проксирует TCP трафик bidirectionally
- Каждый `conn_id` соответствует отдельному QUIC stream

## Архитектурные решения

Подробные архитектурные решения задокументированы в ADR (Architecture Decision Records):

- [ADR 0001: Plugin System Architecture](docs/adr/0001-plugin-system.md) - Система плагинов для учета трафика и расширяемости
- [ADR 0002: Outbound Pool Architecture](docs/adr/0002-outbound-pool.md) - Динамический пул устройств через reverse connections (устарело)
- [ADR 0004: WSS + QUIC Protocol](docs/adr/0004-wss-quic-protocol.md) - Новый протокол для POP-Device взаимодействия
- [ADR 0003: Device Client Architecture](docs/adr/0003-device-client.md) - Клиентское приложение для подключения устройств

### Ключевые архитектурные принципы

- **Модульность**: Четкое разделение на inbound, outbound, proxy core
- **Переиспользование кода**: Общий SOCKS5 парсер в `internal/protocol/socks5/` используется и server-side, и client-side
- **Инкапсуляция**: Клиентская логика device изолирована в `internal/device/client/`, отдельно от server-side логики
- **Расширяемость**: Система плагинов позволяет добавлять функциональность без изменения core кода
- **Multi-binary структура**: Разделение на `cmd/proxy/` и `cmd/device/` для разных приложений

## Планы развития

- **Позже**: Авторизация для SOCKS5
- **Позже**: Дополнительные стратегии роутинга (least connections, latency-based)
- **Позже**: Персистентность устройств (Redis)
- **Реализовано**: WSS (control-plane) и QUIC (data-plane) для POP-Device взаимодействия
- **Позже**: IP-migration для QUIC (архитектура заложена, реализация отложена)
- **Позже**: UDP через QUIC datagrams
- **Позже**: Автоматическое переподключение device при разрыве соединения
- **Позже**: TLS сертификаты для продакшена (сейчас используется самоподписанный сертификат для тестирования)
