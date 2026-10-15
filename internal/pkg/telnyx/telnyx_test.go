package telnyx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return public, private
}

func signedHeaders(private ed25519.PrivateKey, timestamp int64, body []byte) http.Header {
	ts := strconv.FormatInt(timestamp, 10)
	headers := http.Header{}
	headers.Set("Telnyx-Timestamp", ts)
	headers.Set("Telnyx-Signature-Ed25519", base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(ts+"|"+string(body)))))
	return headers
}

func TestWebhookAuthenticatorVerifiesSignature(t *testing.T) {
	public, private := newKeyPair(t)
	now := time.Unix(1_000, 0)
	auth := NewWebhookAuthenticator(base64.StdEncoding.EncodeToString(public), "")
	auth.now = func() time.Time { return now }
	body := []byte(`{"data":{"id":"evt"}}`)

	if err := auth.Authenticate(body, signedHeaders(private, now.Unix(), body), ""); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := auth.Authenticate([]byte(`{"data":{"id":"other"}}`), signedHeaders(private, now.Unix(), body), ""); err == nil {
		t.Fatal("tampered body must be rejected")
	}
	if err := auth.Authenticate(body, signedHeaders(private, now.Unix()-301, body), ""); err == nil {
		t.Fatal("stale timestamp must be rejected")
	}
	if err := auth.Authenticate(body, http.Header{}, ""); err == nil {
		t.Fatal("missing signature must be rejected")
	}
}

func TestWebhookAuthenticatorAcceptsSPKIPublicKey(t *testing.T) {
	public, private := newKeyPair(t)
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	auth := NewWebhookAuthenticator(base64.StdEncoding.EncodeToString(der), "")
	body := []byte(`{}`)
	if err := auth.Authenticate(body, signedHeaders(private, time.Now().Unix(), body), ""); err != nil {
		t.Fatalf("SPKI key rejected: %v", err)
	}
}

func TestWebhookAuthenticatorFallsBackToURLTokenOnlyWithoutPublicKey(t *testing.T) {
	token := strings.Repeat("t", 40)
	auth := NewWebhookAuthenticator("", token)
	if err := auth.Authenticate([]byte(`{}`), http.Header{}, token); err != nil {
		t.Fatalf("matching token rejected: %v", err)
	}
	if err := auth.Authenticate([]byte(`{}`), http.Header{}, strings.Repeat("x", 40)); err == nil {
		t.Fatal("wrong token must be rejected")
	}
	if err := auth.Authenticate([]byte(`{}`), http.Header{}, ""); err == nil {
		t.Fatal("missing token must be rejected")
	}

	public, _ := newKeyPair(t)
	signed := NewWebhookAuthenticator(base64.StdEncoding.EncodeToString(public), token)
	if err := signed.Authenticate([]byte(`{}`), http.Header{}, token); err == nil {
		t.Fatal("a configured public key must not be bypassed by the URL token")
	}
}

func TestWebhookAuthenticatorFailsClosed(t *testing.T) {
	for _, auth := range []*WebhookAuthenticator{
		NewWebhookAuthenticator("", ""),
		NewWebhookAuthenticator("", "short"),
		NewWebhookAuthenticator("not base64!", ""),
	} {
		if err := auth.Authenticate([]byte(`{}`), http.Header{}, "short"); err == nil {
			t.Fatal("unconfigured authenticator must reject every webhook")
		}
	}
}

func TestClientSendsCommandAndDetectsEndedCall(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		if strings.Contains(r.URL.Path, "ended") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"code":"90018","title":"Call has already ended"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"result":"ok"}}`))
	}))
	defer server.Close()

	client := &Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()}
	if err := client.Command(context.Background(), "v3:abc", "speak", map[string]any{"payload": "hi"}); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if gotPath != "/calls/v3:abc/actions/speak" || gotAuth != "Bearer key" || gotBody["payload"] != "hi" {
		t.Fatalf("request = %s %s %v", gotPath, gotAuth, gotBody)
	}

	err := client.Command(context.Background(), "ended", "speak", map[string]any{})
	if !errors.Is(err, ErrCallEnded) {
		t.Fatalf("ended call error = %v, want ErrCallEnded", err)
	}
}
