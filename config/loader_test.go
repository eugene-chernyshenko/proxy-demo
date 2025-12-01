package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultConfig(t *testing.T) {
	// Сохраняем оригинальные значения флагов
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Устанавливаем пустые аргументы
	os.Args = []string{"test"}

	// Удаляем временный конфиг файл если существует
	configFile := "config.json"
	os.Remove(configFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Ошибка загрузки конфигурации по умолчанию: %v", err)
	}

	if cfg.Inbound.Type != "socks5" {
		t.Errorf("Неверный тип inbound: ожидалось socks5, получено %s", cfg.Inbound.Type)
	}

	if cfg.Inbound.Port != 1080 {
		t.Errorf("Неверный порт по умолчанию: ожидалось 1080, получено %d", cfg.Inbound.Port)
	}

	if cfg.Outbound.Type != "direct" {
		t.Errorf("Неверный тип outbound: ожидалось direct, получено %s", cfg.Outbound.Type)
	}
}

func TestLoad_FromFile(t *testing.T) {
	// Создаем временный конфиг файл
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.json")

	testConfig := Config{
		Inbound: InboundConfig{
			Type: "socks5",
			Port: 8080,
		},
		Outbound: OutboundConfig{
			Type: "direct",
		},
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("Ошибка маршалинга конфига: %v", err)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		t.Fatalf("Ошибка записи конфига: %v", err)
	}

	// Сохраняем оригинальные значения
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Устанавливаем аргументы с путем к конфигу
	os.Args = []string{"test", "-config", configFile}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Ошибка загрузки конфигурации из файла: %v", err)
	}

	if cfg.Inbound.Port != 8080 {
		t.Errorf("Неверный порт из файла: ожидалось 8080, получено %d", cfg.Inbound.Port)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	// Создаем временный файл с невалидным JSON
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid_config.json")

	err := os.WriteFile(configFile, []byte("{invalid json}"), 0644)
	if err != nil {
		t.Fatalf("Ошибка записи файла: %v", err)
	}

	// Сохраняем оригинальные значения
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	os.Args = []string{"test", "-config", configFile}

	_, err = Load()
	if err == nil {
		t.Error("Ожидалась ошибка при загрузке невалидного JSON")
	}
}

func TestLoad_PortOverride(t *testing.T) {
	// Создаем временный конфиг файл
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.json")

	testConfig := Config{
		Inbound: InboundConfig{
			Type: "socks5",
			Port: 8080,
		},
		Outbound: OutboundConfig{
			Type: "direct",
		},
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("Ошибка маршалинга конфига: %v", err)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		t.Fatalf("Ошибка записи конфига: %v", err)
	}

	// Сохраняем оригинальные значения
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Устанавливаем аргументы с переопределением порта
	os.Args = []string{"test", "-config", configFile, "-port", "9090"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	if cfg.Inbound.Port != 9090 {
		t.Errorf("Порт не переопределен: ожидалось 9090, получено %d", cfg.Inbound.Port)
	}
}

