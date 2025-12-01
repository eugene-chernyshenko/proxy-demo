package traffic

import (
	"example.com/me/myproxy/internal/plugin"
	"testing"
)

func TestBaseCounter_AddConnection(t *testing.T) {
	counter := NewBaseCounter()
	
	counter.AddConnection("test-id")
	stats := counter.GetStats("test-id")
	
	if stats.Connections != 1 {
		t.Errorf("Expected 1 connection, got %d", stats.Connections)
	}
}

func TestBaseCounter_AddBytes(t *testing.T) {
	counter := NewBaseCounter()
	
	counter.AddBytes("test-id", 100, 200)
	stats := counter.GetStats("test-id")
	
	if stats.BytesSent != 100 {
		t.Errorf("Expected 100 bytes sent, got %d", stats.BytesSent)
	}
	
	if stats.BytesReceived != 200 {
		t.Errorf("Expected 200 bytes received, got %d", stats.BytesReceived)
	}
}

func TestInboundCounter_OnInboundConnection(t *testing.T) {
	counter := NewInboundCounter()
	
	ctx := plugin.NewConnectionContext("127.0.0.1:1234", "example.com:80")
	ctx.InboundID = "inbound-1"
	
	err := counter.OnInboundConnection(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	stats := counter.GetStats("inbound-1")
	if stats.Connections != 1 {
		t.Errorf("Expected 1 connection, got %d", stats.Connections)
	}
}

func TestOutboundCounter_OnOutboundConnection(t *testing.T) {
	counter := NewOutboundCounter()
	
	ctx := plugin.NewConnectionContext("127.0.0.1:1234", "example.com:80")
	ctx.OutboundID = "outbound-1"
	
	err := counter.OnOutboundConnection(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	stats := counter.GetStats("outbound-1")
	if stats.Connections != 1 {
		t.Errorf("Expected 1 connection, got %d", stats.Connections)
	}
}

