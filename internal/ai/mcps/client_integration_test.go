package mcps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMCPClientServerRoundTrip(t *testing.T) {
	server := httptest.NewServer(NewHTTPHandler())
	t.Cleanup(server.Close)

	client := NewClient()
	cfg := ServerConfig{Code: "system", Endpoint: server.URL, TimeoutMS: 2_000}
	connection, err := client.TestConnection(context.Background(), cfg)
	if err != nil {
		t.Fatalf("test connection: %v", err)
	}
	if connection.ServerName != "remote-helpdesk-mcp-server" || connection.Protocol == "" {
		t.Fatalf("unexpected connection result: %#v", connection)
	}

	tools, err := client.ListTools(context.Background(), cfg)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if !containsMCPTool(tools, "server_time") || !containsMCPTool(tools, "service_info") {
		t.Fatalf("expected built-in MCP tools, got %#v", tools)
	}

	result, err := client.CallTool(context.Background(), cfg, "server_time", map[string]any{"timezone": "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("call server_time: %v", err)
	}
	if result.IsError || result.StructuredContent == nil || len(result.Content) == 0 {
		t.Fatalf("unexpected tool result: %#v", result)
	}
}

func TestMCPClientHonorsConnectionTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	started := time.Now()
	_, err := NewClient().TestConnection(context.Background(), ServerConfig{
		Code:      "slow",
		Endpoint:  server.URL,
		TimeoutMS: 25,
	})
	if err == nil {
		t.Fatal("expected MCP connection timeout")
	}
	if time.Since(started) > 150*time.Millisecond {
		t.Fatalf("MCP timeout was not enforced promptly: %s", time.Since(started))
	}
	var timeoutError interface{ Timeout() bool }
	if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &timeoutError) || !timeoutError.Timeout()) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func containsMCPTool(items []ToolInfo, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}
