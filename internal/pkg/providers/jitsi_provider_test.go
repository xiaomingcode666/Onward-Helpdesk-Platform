package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/golang-jwt/jwt/v5"
)

func TestJitsiTokenExpiryUsesDefaultAndConfiguredMeetingWindows(t *testing.T) {
	if got := (config.JitsiConfig{}).TokenExpiry(); got != 15*time.Minute {
		t.Fatalf("TokenExpiry() = %s, want %s", got, 15*time.Minute)
	}
	if got := (config.JitsiConfig{TokenTTLMinutes: 90}).TokenExpiry(); got != 90*time.Minute {
		t.Fatalf("configured TokenExpiry() = %s, want %s", got, 90*time.Minute)
	}
}

func TestJitsiGenerateTokenSupportsAnonymousDeployment(t *testing.T) {
	client := NewJitsiClient(&config.JitsiConfig{URL: "https://meet.jit.si"})
	token, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{UserID: "1", Name: "Engineer"})
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token != "" {
		t.Fatalf("anonymous deployment token = %q, want empty", token)
	}
}

func TestJitsiGenerateTokenValidatesJWTConfiguration(t *testing.T) {
	client := NewJitsiClient(&config.JitsiConfig{AppID: "remotehelpdesk"})
	if _, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{UserID: "1"}); err == nil {
		t.Fatal("GenerateToken() expected missing app secret error")
	}

	client = NewJitsiClient(&config.JitsiConfig{AppID: "remotehelpdesk", AppSecret: "test-secret"})
	token, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{UserID: "1", Name: "Engineer"})
	if err != nil || token == "" {
		t.Fatalf("GenerateToken() token=%q error=%v", token, err)
	}
}

func TestJitsiGenerateTokenRequiresCredentialsWhenAuthenticationIsEnabled(t *testing.T) {
	client := NewJitsiClient(&config.JitsiConfig{
		URL:         "https://meet.example.com",
		RequireAuth: true,
	})
	if _, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{UserID: "1"}); err == nil {
		t.Fatal("GenerateToken() expected missing credentials error")
	}
}

func TestJitsiGenerateTokenUsesFrontendDomainAndModeratorClaim(t *testing.T) {
	client := NewJitsiClient(&config.JitsiConfig{
		URL:         "http://localhost:8000/",
		AppID:       "remotehelpdesk",
		AppSecret:   "test-secret",
		JitsiDomain: "http://localhost:8000/ignored/path",
	})
	tokenValue, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{
		UserID:      "42",
		Name:        "Engineer",
		IsModerator: true,
	})
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims := &jitsiCustomClaims{}
	parsed, err := jwt.ParseWithClaims(tokenValue, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("ParseWithClaims() valid=%v error=%v", parsed != nil && parsed.Valid, err)
	}
	if claims.Subject != "localhost:8000" {
		t.Fatalf("subject = %q, want localhost:8000", claims.Subject)
	}
	if claims.Audience != "jitsi" {
		t.Fatalf("audience = %#v, want jitsi string", claims.Audience)
	}
	user, ok := claims.Context["user"].(map[string]any)
	if !ok || user["moderator"] != true {
		t.Fatalf("context.user = %#v, want moderator=true", claims.Context["user"])
	}
	if user["affiliation"] != "owner" {
		t.Fatalf("context.user affiliation = %#v, want owner", user["affiliation"])
	}
	features, ok := claims.Context["features"].(map[string]any)
	if !ok || features["transcription"] != true {
		t.Fatalf("context.features = %#v, want transcription=true", claims.Context["features"])
	}

	participantToken, err := client.GenerateToken("rhd-test-room", JitsiUserInfo{UserID: "43", Name: "Customer"})
	if err != nil {
		t.Fatalf("GenerateToken() participant error = %v", err)
	}
	participantClaims := &jitsiCustomClaims{}
	parsed, err = jwt.ParseWithClaims(participantToken, participantClaims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("ParseWithClaims() participant valid=%v error=%v", parsed != nil && parsed.Valid, err)
	}
	participant, ok := participantClaims.Context["user"].(map[string]any)
	if !ok || participant["moderator"] != false || participant["affiliation"] != "member" {
		t.Fatalf("participant context.user = %#v, want moderator=false affiliation=member", participantClaims.Context["user"])
	}
}

func TestJitsiDomainAndJoinURLAreNormalized(t *testing.T) {
	client := NewJitsiClient(&config.JitsiConfig{URL: "http://localhost:8000/"})
	if got := client.GetJitsiDomain(); got != "localhost:8000" {
		t.Fatalf("GetJitsiDomain() = %q, want localhost:8000", got)
	}
	if got := client.joinURL("/rhd-test-room"); got != "http://localhost:8000/rhd-test-room" {
		t.Fatalf("joinURL() = %q", got)
	}
}

func TestJitsiCheckHealthProbesIframeConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.js" {
			t.Fatalf("health probe path = %q, want /config.js", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte("var config = { hosts: {} };"))
	}))
	t.Cleanup(server.Close)

	client := NewJitsiClient(&config.JitsiConfig{URL: server.URL})
	health := client.CheckHealth(context.Background())
	if health.Status != "healthy" || health.HTTPStatus != http.StatusOK || health.ProbeURL != server.URL+"/config.js" {
		t.Fatalf("CheckHealth() = %#v", health)
	}
}

func TestJitsiCheckHealthReportsUnconfiguredAndUpstreamFailure(t *testing.T) {
	unconfigured := NewJitsiClient(&config.JitsiConfig{}).CheckHealth(context.Background())
	if unconfigured.Status != "unconfigured" || unconfigured.ProbeURL != "" {
		t.Fatalf("unconfigured CheckHealth() = %#v", unconfigured)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	health := NewJitsiClient(&config.JitsiConfig{URL: server.URL}).CheckHealth(context.Background())
	if health.Status != "unhealthy" || health.HTTPStatus != http.StatusServiceUnavailable || health.Error == "" {
		t.Fatalf("failed CheckHealth() = %#v", health)
	}
}
