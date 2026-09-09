package third

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/config"
)

func TestValidJigasiCredentialAcceptsOnlyPurposeScopedToken(t *testing.T) {
	secret := strings.Repeat("s", 48)
	cfg := config.SpeechConfig{Jigasi: config.JigasiSpeechConfig{Enabled: true, SharedSecret: secret}}
	websocketToken := jigasiCredentialToken(secret, jigasiWebSocketCredentialPurpose)
	eventToken := jigasiCredentialToken(secret, jigasiEventCredentialPurpose)
	if len(websocketToken) != 64 || !validJigasiCredential(cfg, websocketToken, jigasiWebSocketCredentialPurpose) {
		t.Fatal("derived Jigasi websocket credential was rejected")
	}
	if len(eventToken) != 64 || !validJigasiCredential(cfg, eventToken, jigasiEventCredentialPurpose) {
		t.Fatal("derived Jigasi event credential was rejected")
	}
	if websocketToken == eventToken {
		t.Fatal("Jigasi websocket and event credentials must differ")
	}
	if validJigasiCredential(cfg, secret, jigasiWebSocketCredentialPurpose) {
		t.Fatal("raw Jigasi shared secret was accepted")
	}
	if validJigasiCredential(cfg, eventToken, jigasiWebSocketCredentialPurpose) {
		t.Fatal("Jigasi event credential was accepted by the websocket endpoint")
	}
	if validJigasiCredential(cfg, strings.Repeat("x", 64), jigasiWebSocketCredentialPurpose) {
		t.Fatal("invalid Jigasi credential was accepted")
	}
	cfg.Jigasi.Enabled = false
	if validJigasiCredential(cfg, websocketToken, jigasiWebSocketCredentialPurpose) {
		t.Fatal("credential was accepted while Jigasi is disabled")
	}
}
