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

**Примечание:** SOCKS5 прокси работает без авторизации. Целевой адрес определяется из SOCKS5 протокола. Direct outbound работает как прямой выход в интернет. SOCKS5 outbound позволяет цепочку прокси.

### Запуск

```bash
go run .
```

Или скомпилируйте и запустите:

```bash
go build -o myproxy .
./myproxy
```

### CLI аргументы

- `-config` - путь к файлу конфигурации (по умолчанию: `config.json`)
- `-port` - порт для inbound (переопределяет значение из конфига)

Пример:

```bash
./myproxy -port 8080 -config myconfig.json
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

После запуска прокси можно использовать его как SOCKS5 прокси:

```bash
# Использование с curl
curl --socks5-hostname 127.0.0.1:1080 https://example.com

# Использование с браузером
# Настройте прокси на 127.0.0.1:1080, тип SOCKS5
```

## Структура проекта

```
myproxy/
├── main.go                 # Точка входа
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
│   └── socks5_test.go     # Тесты SOCKS5 outbound
└── proxy/
    ├── core.go            # Основная логика прокси
    └── core_test.go       # Тесты прокси
```

## Планы развития

- **Позже**: Авторизация для SOCKS5
- **Позже**: Динамический выбор outbound
- **Позже**: Телеметрия и статистика трафика
