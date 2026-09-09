package providers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

func TestNewSub2APIHTTPClientAddsAPIKeyHeader(t *testing.T) {
	headerValue := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		headerValue <- req.Header.Get(sub2APIKeyHeader)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := NewSub2APIHTTPClient("tenant-key", time.Second)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := <-headerValue; got != "tenant-key" {
		t.Fatalf("%s = %q", sub2APIKeyHeader, got)
	}
}

func TestSub2APILoginReturnsStructuredInvalidCredentialsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":401,"message":"invalid email or password","reason":"INVALID_CREDENTIALS"}`)
	}))
	defer server.Close()

	provider := NewSub2APIProvider(&config.Sub2APIConfig{
		BaseURL:    server.URL,
		Timeout:    time.Second,
		MaxRetries: 1,
	})
	_, err := provider.Login(context.Background(), Sub2APILoginRequest{
		Email:    "tenant@example.com",
		Password: "wrong-password",
	})
	var clientErr *Sub2APIClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("error = %T %v, want Sub2APIClientError", err, err)
	}
	if clientErr.StatusCode != http.StatusUnauthorized || clientErr.Code != "401" || clientErr.Reason != "INVALID_CREDENTIALS" {
		t.Fatalf("client error = %+v", clientErr)
	}
}
