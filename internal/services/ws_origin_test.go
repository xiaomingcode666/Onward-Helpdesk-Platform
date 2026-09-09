package services

import (
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/config"
)

func TestWebsocketOriginAllowedUsesSameOriginAndConfiguredAllowlist(t *testing.T) {
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{Server: config.ServerConfig{CORS: config.CORSConfig{
		AllowedOrigins: []string{"https://support.example.com", "capacitor://localhost"},
	}}})
	t.Cleanup(func() { config.SetCurrent(&previous) })

	tests := []struct {
		name    string
		origin  string
		host    string
		proto   string
		headers map[string]string
		allowed bool
	}{
		{name: "native client", host: "api.example.com", allowed: true},
		{name: "same origin", origin: "https://api.example.com", host: "api.example.com", proto: "https", allowed: true},
		{
			name:   "forwarded host with external port",
			origin: "https://api.example.com:8443",
			host:   "remotehelpdesk-api:8083",
			proto:  "https",
			headers: map[string]string{
				"X-Forwarded-Host": "api.example.com",
				"X-Forwarded-Port": "8443",
			},
			allowed: true,
		},
		{name: "configured frontend", origin: "https://support.example.com", host: "api.example.com", proto: "https", allowed: true},
		{name: "configured native webview", origin: "capacitor://localhost", host: "api.example.com", allowed: true},
		{name: "unconfigured native webview", origin: "capacitor://app", host: "api.example.com", allowed: false},
		{name: "untrusted browser", origin: "https://evil.example", host: "api.example.com", proto: "https", allowed: false},
		{name: "null origin", origin: "null", host: "api.example.com", proto: "https", allowed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://"+tt.host+"/api/ws/open", nil)
			request.Host = tt.host
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			if tt.proto != "" {
				request.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			for name, value := range tt.headers {
				request.Header.Set(name, value)
			}
			if got := websocketOriginAllowed(request); got != tt.allowed {
				t.Fatalf("websocketOriginAllowed() = %v, want %v", got, tt.allowed)
			}
		})
	}
}
