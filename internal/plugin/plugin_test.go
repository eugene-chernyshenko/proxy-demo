package plugin

import (
	"testing"
	"time"
)

func TestNewConnectionContext(t *testing.T) {
	ctx := NewConnectionContext("127.0.0.1:1234", "example.com:80")
	
	if ctx.RemoteAddr != "127.0.0.1:1234" {
		t.Errorf("Expected RemoteAddr 127.0.0.1:1234, got %s", ctx.RemoteAddr)
	}
	
	if ctx.TargetAddress != "example.com:80" {
		t.Errorf("Expected TargetAddress example.com:80, got %s", ctx.TargetAddress)
	}
	
	if ctx.Metadata == nil {
		t.Error("Expected Metadata to be initialized")
	}
	
	if ctx.StartTime.IsZero() {
		t.Error("Expected StartTime to be set")
	}
	
	// Проверяем что StartTime примерно сейчас
	now := time.Now()
	if ctx.StartTime.After(now) || ctx.StartTime.Before(now.Add(-time.Second)) {
		t.Errorf("StartTime should be approximately now, got %v", ctx.StartTime)
	}
}

func TestManager_RegisterAndCall(t *testing.T) {
	manager := NewManager()
	
	mockPlugin := &mockInboundPlugin{}
	manager.RegisterInboundPlugin(mockPlugin)
	
	ctx := NewConnectionContext("127.0.0.1:1234", "example.com:80")
	err := manager.OnInboundConnection(ctx)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if !mockPlugin.called {
		t.Error("Expected OnInboundConnection to be called")
	}
}

type mockInboundPlugin struct {
	called bool
}

func (m *mockInboundPlugin) Name() string {
	return "mock"
}

func (m *mockInboundPlugin) Init(config map[string]interface{}) error {
	return nil
}

func (m *mockInboundPlugin) Close() error {
	return nil
}

func (m *mockInboundPlugin) OnInboundConnection(ctx *ConnectionContext) error {
	m.called = true
	return nil
}

