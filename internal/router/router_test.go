package router

import (
	"example.com/me/myproxy/config"
	"example.com/me/myproxy/internal/plugin"
	"testing"
)

func TestStaticRouter_SelectOutbound(t *testing.T) {
	rtr := NewStaticRouter()
	
	ctx := plugin.NewConnectionContext("127.0.0.1:1234", "example.com:80")
	currentConfig := &config.OutboundConfig{
		Type: "direct",
	}
	
	outboundID, outboundConfig, err := rtr.SelectOutbound(ctx, "example.com:80", "outbound-1", currentConfig)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if outboundID != "" {
		t.Errorf("Expected empty outboundID, got %s", outboundID)
	}
	
	if outboundConfig != nil {
		t.Errorf("Expected nil outboundConfig, got %v", outboundConfig)
	}
}

