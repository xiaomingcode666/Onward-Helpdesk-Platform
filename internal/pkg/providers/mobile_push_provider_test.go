package providers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

func TestMobilePushProviderSendsAPNSAlertAndClassifiesInvalidToken(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate APNs key: %v", err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal APNs key: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})

	invalid := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/device/device-token" {
			t.Errorf("APNs path = %q", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "bearer ") || r.Header.Get("apns-topic") != "com.example.app" {
			t.Errorf("APNs headers = %+v", r.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode APNs payload: %v", err)
		}
		if !invalid && payload["conversationId"] != "321" {
			t.Errorf("APNs conversationId = %#v", payload["conversationId"])
		}
		if invalid {
			w.WriteHeader(http.StatusGone)
			_, _ = io.WriteString(w, `{"reason":"Unregistered"}`)
			return
		}
		w.Header().Set("apns-id", "apns-message-1")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{MobilePush: config.MobilePushConfig{
		Enabled: true,
		APNS:    config.APNSPushConfig{TeamID: "TEAM123", KeyID: "KEY123", PrivateKey: string(pemKey), BundleID: "com.example.app", Production: true},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })
	provider := NewMobilePushProvider()
	provider.httpClient = server.Client()
	provider.apnsBaseURL = server.URL
	provider.now = func() time.Time { return time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC) }

	messageID, err := provider.SendNotificationPush(context.Background(), "ios", "device-token", MobilePushMessage{
		Title: "New reply", Body: "Open the conversation", Data: map[string]string{"conversationId": "321"},
	})
	if err != nil || messageID != "apns-message-1" {
		t.Fatalf("SendNotificationPush() = %q, %v", messageID, err)
	}

	invalid = true
	_, err = provider.SendNotificationPush(context.Background(), "ios", "device-token", MobilePushMessage{Title: "New reply"})
	if err == nil || !IsInvalidMobilePushToken(err) || !IsPermanentMobilePushError(err) {
		t.Fatalf("invalid APNs token error = %v", err)
	}
}

func TestMobilePushProviderSendsFCMHTTPv1Message(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate FCM key: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"fcm-access-token","token_type":"Bearer","expires_in":3600}`)
		case "/send":
			if r.Header.Get("Authorization") != "Bearer fcm-access-token" {
				t.Errorf("FCM Authorization = %q", r.Header.Get("Authorization"))
			}
			var payload struct {
				Message struct {
					Token   string            `json:"token"`
					Data    map[string]string `json:"data"`
					Android struct {
						Priority     string `json:"priority"`
						Notification struct {
							ChannelID string `json:"channel_id"`
							Icon      string `json:"icon"`
						} `json:"notification"`
					} `json:"android"`
				} `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode FCM payload: %v", err)
			}
			if payload.Message.Token != "android-token" || payload.Message.Data["conversationId"] != "987" {
				t.Errorf("FCM payload = %+v", payload)
			}
			if payload.Message.Android.Priority != "HIGH" || payload.Message.Android.Notification.ChannelID != "conversation_updates" || payload.Message.Android.Notification.Icon != "ic_stat_remotehelpdesk" {
				t.Errorf("FCM Android notification = %+v", payload.Message.Android)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"name":"projects/test/messages/123"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	credentials, err := json.Marshal(map[string]string{
		"client_email":   "firebase@example.test",
		"private_key":    string(pemKey),
		"private_key_id": "key-1",
		"token_uri":      server.URL + "/token",
	})
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{MobilePush: config.MobilePushConfig{
		Enabled: true,
		FCM:     config.FCMPushConfig{ProjectID: "test", CredentialsJSON: string(credentials)},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })
	provider := NewMobilePushProvider()
	provider.httpClient = server.Client()
	provider.fcmBaseURL = server.URL + "/send"

	messageID, err := provider.SendNotificationPush(context.Background(), "android", "android-token", MobilePushMessage{
		Title: "New reply", Body: "Message", Data: map[string]string{"conversationId": "987"},
	})
	if err != nil || messageID != "projects/test/messages/123" {
		t.Fatalf("SendNotificationPush() = %q, %v", messageID, err)
	}
}
