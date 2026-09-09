package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestOnePanelConfigUsesSupportedTicketDispatchKeys(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "1panel", "remotehelpdesk.yaml")
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read 1Panel config: %v", err)
	}

	if got := v.GetInt("ticketDispatch.acceptDeadlineMinutes"); got != 10 {
		t.Fatalf("acceptDeadlineMinutes = %d, want 10", got)
	}
	if got := v.GetInt("ticketDispatch.ownerReplyTakeoverMinutes"); got != 30 {
		t.Fatalf("ownerReplyTakeoverMinutes = %d, want 30", got)
	}
	if got := v.GetInt("ticketDispatch.unassignedEscalationMinutes"); got != 10 {
		t.Fatalf("unassignedEscalationMinutes = %d, want 10", got)
	}
	if got := v.GetInt("ticketDispatch.maxDispatchAttempts"); got != 3 {
		t.Fatalf("maxDispatchAttempts = %d, want 3", got)
	}
	if v.IsSet("ticketDispatch.scheduleFallbackToTeam") {
		t.Fatal("legacy scheduleFallbackToTeam must not return to the deployment config")
	}
	if got := v.GetInt("ticketDispatch.onlineFreshnessMinutes"); got != 2 {
		t.Fatalf("onlineFreshnessMinutes = %d, want 2", got)
	}
	if v.IsSet("ticketDispatch.escalationPollIntervalSeconds") {
		t.Fatal("unsupported escalationPollIntervalSeconds must not return to the deployment config")
	}
}

func TestLoadReadsCORSAllowedOrigins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`server:
  port: 8083
  cors:
    allowedOrigins:
      - https://console.example.com
      - http://localhost:3000
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := cfg.Server.CORS.AllowedOrigins
	want := []string{"https://console.example.com", "http://localhost:3000"}
	if len(got) != len(want) {
		t.Fatalf("len(AllowedOrigins)=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllowedOrigins[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestLoadOverridesValuesFromEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`server:
  port: 8083
db:
  type: sqlite
  dsn: file:./data/app.db?_busy_timeout=5000
storage:
  local:
    baseUrl: /storage
mcp:
  servers:
    system:
      endpoint: http://127.0.0.1:8083/api/mcp
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("RHD_SERVER_PORT", "8090")
	t.Setenv("RHD_DB_DSN", "postgres-dsn")
	t.Setenv("RHD_STORAGE_LOCAL_BASEURL", "/files")
	t.Setenv("RHD_MCP_SERVERS_SYSTEM_ENDPOINT", "http://127.0.0.1:8090/api/mcp")
	mcpToken := strings.Repeat("mcp-test-", 4)
	encryptionKey := strings.Repeat("enc-test-", 4)
	t.Setenv("MCP_SERVER_TOKEN", mcpToken)
	t.Setenv("ENCRYPTION_KEY", encryptionKey)
	t.Setenv("ENCRYPTION_KEY_FALLBACKS", "legacy-one, legacy-two ; legacy-three")
	t.Setenv("MOBILE_PUSH_ENABLED", "true")
	t.Setenv("FCM_PROJECT_ID", "remotehelpdesk-mobile")
	t.Setenv("FCM_CREDENTIALS_JSON", `{"type":"service_account"}`)
	t.Setenv("APNS_TEAM_ID", "TEAM123")
	t.Setenv("APNS_KEY_ID", "KEY123")
	t.Setenv("APNS_PRIVATE_KEY", "base64-private-key")
	t.Setenv("APNS_BUNDLE_ID", "com.example.remotehelpdesk")
	t.Setenv("APNS_PRODUCTION", "false")
	t.Setenv("SMTP_HOST", "smtp.qq.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "sender@example.com")
	t.Setenv("SMTP_PASSWORD", "smtp-app-password")
	t.Setenv("SMTP_FROM_ADDRESS", "sender@example.com")
	t.Setenv("SMTP_FROM_NAME", "RemoteHelpDesk Mail")
	t.Setenv("SMTP_USE_TLS", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8090 {
		t.Fatalf("Server.Port=%d want 8090", cfg.Server.Port)
	}
	if cfg.DB.Type != "sqlite" {
		t.Fatalf("DB.Type=%q want sqlite", cfg.DB.Type)
	}
	if cfg.DB.DSN != "postgres-dsn" {
		t.Fatalf("DB.DSN=%q want postgres-dsn", cfg.DB.DSN)
	}
	if cfg.Storage.Local.BaseURL != "/files" {
		t.Fatalf("Storage.Local.BaseURL=%q want /files", cfg.Storage.Local.BaseURL)
	}
	if cfg.MCP.Servers["system"].Endpoint != "http://127.0.0.1:8090/api/mcp" {
		t.Fatalf("MCP system endpoint=%q", cfg.MCP.Servers["system"].Endpoint)
	}
	if cfg.MCP.ServerToken != mcpToken {
		t.Fatalf("MCP server token was not loaded from MCP_SERVER_TOKEN")
	}
	if cfg.EncryptionKey != encryptionKey {
		t.Fatalf("EncryptionKey was not loaded from ENCRYPTION_KEY")
	}
	if !cfg.MobilePush.Enabled || cfg.MobilePush.FCM.ProjectID != "remotehelpdesk-mobile" || cfg.MobilePush.FCM.CredentialsJSON == "" {
		t.Fatalf("FCM mobile push config was not loaded: %+v", cfg.MobilePush.FCM)
	}
	if cfg.MobilePush.APNS.TeamID != "TEAM123" || cfg.MobilePush.APNS.KeyID != "KEY123" || cfg.MobilePush.APNS.BundleID != "com.example.remotehelpdesk" || cfg.MobilePush.APNS.Production {
		t.Fatalf("APNs mobile push config was not loaded: %+v", cfg.MobilePush.APNS)
	}
	if cfg.Email.SMTPHost != "smtp.qq.com" || cfg.Email.SMTPPort != 465 || cfg.Email.Username != "sender@example.com" || cfg.Email.Password != "smtp-app-password" || cfg.Email.FromAddress != "sender@example.com" || cfg.Email.FromName != "RemoteHelpDesk Mail" || !cfg.Email.UseTLS {
		t.Fatalf("SMTP email config was not loaded: %+v", cfg.Email)
	}
	wantFallbacks := []string{"legacy-one", "legacy-two", "legacy-three"}
	if len(cfg.EncryptionKeyFallbacks) != len(wantFallbacks) {
		t.Fatalf("len(EncryptionKeyFallbacks)=%d want %d", len(cfg.EncryptionKeyFallbacks), len(wantFallbacks))
	}
	for i := range wantFallbacks {
		if cfg.EncryptionKeyFallbacks[i] != wantFallbacks[i] {
			t.Fatalf("EncryptionKeyFallbacks[%d]=%q want %q", i, cfg.EncryptionKeyFallbacks[i], wantFallbacks[i])
		}
	}
}

func TestLoadSupportsLegacyAgentDeskCoreEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`server:
  port: 8083
db:
  type: sqlite
  dsn: file:./data/app.db?_busy_timeout=5000
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("RHD_SERVER_PORT", "")
	t.Setenv("RHD_DB_DSN", "")
	t.Setenv("AGENT_DESK_SERVER_PORT", "8091")
	t.Setenv("AGENT_DESK_DB_DSN", "legacy-postgres-dsn")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8091 {
		t.Fatalf("Server.Port=%d want 8091", cfg.Server.Port)
	}
	if cfg.DB.DSN != "legacy-postgres-dsn" {
		t.Fatalf("DB.DSN=%q want legacy-postgres-dsn", cfg.DB.DSN)
	}
}

func TestHasEnvironmentOverrideUsesSharedBindingCatalog(t *testing.T) {
	for _, name := range []string{
		"RHD_SUB2API_BASE_URL", "SUB2API_BASE_URL",
		"RHD_SUB2API_DEFAULT_KEY", "SUB2API_DEFAULT_KEY",
		"RHD_SUB2API_ADMIN_API_KEY", "SUB2API_ADMIN_API_KEY",
		"RHD_SUB2API_DEFAULT_MODEL", "SUB2API_DEFAULT_MODEL", "SUB2API_LLM_MODEL",
		"RHD_MEETING_AR_PROVIDER", "MEETING_AR_PROVIDER",
		"RHD_MEETING_AR_ENDPOINT", "MEETING_AR_ENDPOINT",
		"RHD_MEETING_AR_API_KEY", "MEETING_AR_API_KEY",
		"RHD_MEETING_AR_HEALTH_PATH", "MEETING_AR_HEALTH_PATH",
		"RHD_EMAIL_SMTP_HOST", "EMAIL_SMTP_HOST", "SMTP_HOST",
		"RHD_EMAIL_PASSWORD", "EMAIL_PASSWORD", "SMTP_PASSWORD",
	} {
		t.Setenv(name, "")
	}
	if HasEnvironmentOverride("sub2api") {
		t.Fatal("empty Sub2API environment unexpectedly counted as an override")
	}
	t.Setenv("SUB2API_ADMIN_API_KEY", "environment-admin-key")
	if !HasEnvironmentOverride("sub2api") {
		t.Fatal("bound Sub2API environment was not detected")
	}
	if HasEnvironmentOverride("meetingAR") {
		t.Fatal("Sub2API environment leaked into another config namespace")
	}
	t.Setenv("SMTP_HOST", "smtp.qq.com")
	if !HasEnvironmentOverride("email") {
		t.Fatal("bound SMTP environment was not detected")
	}
	if HasEnvironmentOverride("meetingAR") {
		t.Fatal("SMTP environment leaked into another config namespace")
	}
}

func TestLoadRejectsShortMCPServerToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("language: zh-CN\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("MCP_SERVER_TOKEN", "too-short")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "mcp.serverToken") {
		t.Fatalf("Load() error = %v, want mcp.serverToken validation", err)
	}
}

func TestLoadRejectsShortEncryptionKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("language: zh-CN\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("ENCRYPTION_KEY", "too-short")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "encryptionKey") {
		t.Fatalf("Load() error = %v, want encryptionKey validation", err)
	}
}

func TestLoadUsesDockerRedisPasswordForLocalDev(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`redis:
  addr: localhost:6379
  password: config-password
  db: 0
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("AGENT_DESK_REDIS_PASSWORD", "")
	t.Setenv("RHD_REDIS_PASSWORD", "")
	t.Setenv("REDIS_PASSWORD", "dotenv-password")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Password != "dotenv-password" {
		t.Fatalf("Redis.Password=%q want dotenv-password", cfg.Redis.Password)
	}
}

func TestLoadSupportsLegacyAgentDeskRedisPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`redis:
  addr: localhost:6379
  password: config-password
  db: 0
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("REDIS_PASSWORD", "dotenv-password")
	t.Setenv("AGENT_DESK_REDIS_PASSWORD", "agent-desk-password")
	t.Setenv("RHD_REDIS_PASSWORD", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Password != "agent-desk-password" {
		t.Fatalf("Redis.Password=%q want agent-desk-password", cfg.Redis.Password)
	}
}

func TestLoadPrefersLegacyAgentDeskAliasOverGenericEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("public:\n  baseUrl: https://config.example\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("RHD_PUBLIC_BASEURL", "")
	t.Setenv("AGENT_DESK_PUBLIC_BASEURL", "https://legacy.example")
	t.Setenv("PUBLIC_APP_URL", "https://generic.example")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Public.BaseURL != "https://legacy.example" {
		t.Fatalf("Public.BaseURL=%q want legacy alias", cfg.Public.BaseURL)
	}
}

func TestLoadPrefersRemoteHelpDeskRedisPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`redis:
  addr: localhost:6379
  password: config-password
  db: 0
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("REDIS_PASSWORD", "dotenv-password")
	t.Setenv("AGENT_DESK_REDIS_PASSWORD", "legacy-password")
	t.Setenv("RHD_REDIS_PASSWORD", "remote-helpdesk-password")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Password != "remote-helpdesk-password" {
		t.Fatalf("Redis.Password=%q want remote-helpdesk-password", cfg.Redis.Password)
	}
}

func TestLoadDefaultsDBAutoMigrateToEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`db:
  type: sqlite
  dsn: file:./data/app.db?_busy_timeout=5000
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.DB.AutoMigrateEnabled() {
		t.Fatal("DB.AutoMigrateEnabled() = false, want true by default")
	}
}

func TestLoadCanDisableDBAutoMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`db:
  type: postgres
  dsn: host=127.0.0.1 user=remote_helpdesk password=secret dbname=remote_helpdesk port=5432 sslmode=disable TimeZone=UTC
  autoMigrate: false
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.AutoMigrateEnabled() {
		t.Fatal("DB.AutoMigrateEnabled() = true, want false when db.autoMigrate=false")
	}
}

func TestLoadMapsUnprefixedJitsiEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`jitsi:
  url: https://config.example.com
  timeout: 7s
  maxRetries: 2
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	appSecret := strings.Repeat("a", 32)
	webhookSecret := strings.Repeat("b", 32)
	for _, name := range []string{
		"RHD_JITSI_URL",
		"RHD_JITSI_APP_ID",
		"RHD_JITSI_APP_SECRET",
		"RHD_JITSI_WEBHOOK_SECRET",
		"RHD_JITSI_DOMAIN",
		"RHD_JITSI_TOKEN_TTL_MINUTES",
		"RHD_JITSI_REQUIRE_AUTH",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("JITSI_URL", "https://meet.example.com")
	t.Setenv("JITSI_APP_ID", "remotehelpdesk")
	t.Setenv("JITSI_APP_SECRET", appSecret)
	t.Setenv("JITSI_WEBHOOK_SECRET", webhookSecret)
	t.Setenv("JITSI_DOMAIN", "meet.example.com")
	t.Setenv("JITSI_TOKEN_TTL_MINUTES", "45")
	t.Setenv("JITSI_REQUIRE_AUTH", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Jitsi.URL != "https://meet.example.com" || cfg.Jitsi.AppID != "remotehelpdesk" {
		t.Fatalf("Jitsi endpoint/app id = %q/%q", cfg.Jitsi.URL, cfg.Jitsi.AppID)
	}
	if cfg.Jitsi.AppSecret != appSecret || cfg.Jitsi.WebhookSecret != webhookSecret {
		t.Fatal("Jitsi secrets were not loaded from environment")
	}
	if cfg.Jitsi.JitsiDomain != "meet.example.com" || !cfg.Jitsi.RequireAuth {
		t.Fatalf("Jitsi domain/requireAuth = %q/%v", cfg.Jitsi.JitsiDomain, cfg.Jitsi.RequireAuth)
	}
	if cfg.Jitsi.TokenTTLMinutes != 45 || cfg.Jitsi.Timeout != "7s" || cfg.Jitsi.MaxRetries != 2 {
		t.Fatalf("Jitsi ttl/timeout/retries = %d/%q/%d", cfg.Jitsi.TokenTTLMinutes, cfg.Jitsi.Timeout, cfg.Jitsi.MaxRetries)
	}
}

func TestLoadMapsSub2APIAndMeetingAREnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("language: zh-CN\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for _, name := range []string{
		"RHD_SUB2API_BASE_URL", "RHD_SUB2API_ADMIN_API_KEY", "RHD_SUB2API_DEFAULT_KEY", "RHD_SUB2API_DEFAULT_MODEL",
		"RHD_MEETING_AR_PROVIDER", "RHD_MEETING_AR_ENDPOINT", "RHD_MEETING_AR_API_KEY", "RHD_MEETING_AR_HEALTH_PATH",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("SUB2API_BASE_URL", "https://gateway.example.com")
	t.Setenv("SUB2API_ADMIN_API_KEY", "admin-environment-key")
	t.Setenv("SUB2API_DEFAULT_MODEL", "gpt-environment")
	t.Setenv("MEETING_AR_PROVIDER", "http")
	t.Setenv("MEETING_AR_ENDPOINT", "https://vision.example.com/v1/detect")
	t.Setenv("MEETING_AR_API_KEY", "vision-environment-key")
	t.Setenv("MEETING_AR_HEALTH_PATH", "/ready")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Sub2API.BaseURL != "https://gateway.example.com" || cfg.Sub2API.AdminAPIKey != "admin-environment-key" || cfg.Sub2API.DefaultModel != "gpt-environment" {
		t.Fatalf("Sub2API environment config = %+v", cfg.Sub2API)
	}
	if cfg.MeetingAR.ProviderName() != "http" || cfg.MeetingAR.ProviderOptions["endpoint"] != "https://vision.example.com/v1/detect" || cfg.MeetingAR.ProviderOptions["apiKey"] != "vision-environment-key" || cfg.MeetingAR.ProviderOptions["healthPath"] != "/ready" {
		t.Fatalf("Meeting AR environment config = %+v", cfg.MeetingAR)
	}
}

func TestLoadMapsSub2APIDefaultKeyAsAdminFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("language: zh-CN\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for _, name := range []string{
		"RHD_SUB2API_ADMIN_API_KEY", "SUB2API_ADMIN_API_KEY",
		"RHD_SUB2API_DEFAULT_KEY",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("SUB2API_DEFAULT_KEY", "legacy-admin-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Sub2API.AdminAPIKey != "legacy-admin-key" {
		t.Fatalf("Sub2API.AdminAPIKey=%q want legacy fallback", cfg.Sub2API.AdminAPIKey)
	}
}

func TestLoadRejectsAuthenticatedJitsiWithoutProductionSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`jitsi:
  requireAuth: true
  url: https://meet.example.com
  appId: remotehelpdesk
  appSecret: change_me_in_production
  webhookSecret: short
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for _, name := range []string{
		"RHD_JITSI_URL", "JITSI_URL",
		"RHD_JITSI_APP_ID", "JITSI_APP_ID",
		"RHD_JITSI_APP_SECRET", "JITSI_APP_SECRET",
		"RHD_JITSI_WEBHOOK_SECRET", "JITSI_WEBHOOK_SECRET",
		"RHD_JITSI_DOMAIN", "JITSI_DOMAIN",
		"RHD_JITSI_TOKEN_TTL_MINUTES", "JITSI_TOKEN_TTL_MINUTES",
		"RHD_JITSI_REQUIRE_AUTH", "JITSI_REQUIRE_AUTH",
	} {
		t.Setenv(name, "")
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected authenticated Jitsi secret validation error")
	}
	if !strings.Contains(err.Error(), "jitsi.appSecret") {
		t.Fatalf("Load() error = %v, want app secret validation", err)
	}
}

func TestJitsiConfigAllowsAuthenticatedHTTPOnLoopbackOnly(t *testing.T) {
	valid := JitsiConfig{
		RequireAuth:   true,
		URL:           "http://localhost:8000",
		AppID:         "remotehelpdesk",
		AppSecret:     strings.Repeat("a", 32),
		WebhookSecret: strings.Repeat("b", 32),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() loopback error = %v", err)
	}

	valid.URL = "http://meet.example.com"
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Validate() error = %v, want HTTPS requirement", err)
	}
}

func TestJitsiConfigAllowsFiveDayTokenTTL(t *testing.T) {
	valid := JitsiConfig{
		RequireAuth:     true,
		URL:             "https://meet.example.com",
		AppID:           "remotehelpdesk",
		AppSecret:       strings.Repeat("a", 32),
		WebhookSecret:   strings.Repeat("b", 32),
		TokenTTLMinutes: 7200,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() five-day ttl error = %v", err)
	}
	if got := valid.TokenExpiry(); got != 120*time.Hour {
		t.Fatalf("TokenExpiry() = %v, want 120h", got)
	}

	invalid := valid
	invalid.TokenTTLMinutes = 7201
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "7200") {
		t.Fatalf("Validate() error = %v, want ttl upper bound", err)
	}
}

func TestSpeechConfigValidatesJigasiIntegrationSecret(t *testing.T) {
	invalid := SpeechConfig{
		Provider: "disabled",
		Jigasi:   JigasiSpeechConfig{Enabled: true, SharedSecret: "change_me"},
	}
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "sharedSecret") {
		t.Fatalf("Validate() error = %v, want shared secret validation", err)
	}

	valid := invalid
	valid.Jigasi.SharedSecret = strings.Repeat("s", 48)
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() disabled provider with protected gateway error = %v", err)
	}

	valid.Jigasi.MaxParticipantStreams = 65
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "maxParticipantStreams") {
		t.Fatalf("Validate() error = %v, want participant limit validation", err)
	}

	valid.Jigasi.MaxParticipantStreams = 8
	valid.Jigasi.MaxConcurrentStreams = 4
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "maxConcurrentStreams") {
		t.Fatalf("Validate() error = %v, want concurrent stream validation", err)
	}
}

func TestSpeechConfigRequiresXfyunServerCredentials(t *testing.T) {
	cfg := SpeechConfig{Provider: "xfyun"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "appId") {
		t.Fatalf("Validate() error = %v, want xfyun credential validation", err)
	}

	cfg.Xfyun = XfyunSpeechConfig{
		AppID:    "app-id",
		APIKey:   "api-key",
		Endpoint: "wss://rtasr.xfyun.cn/v1/ws",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() configured xfyun error = %v", err)
	}
}

func TestSpeechConfigTranscriptionEnabled(t *testing.T) {
	cfg := SpeechConfig{
		Provider: "xfyun",
		Xfyun: XfyunSpeechConfig{
			AppID:  "app-id",
			APIKey: "api-key",
		},
		Jigasi: JigasiSpeechConfig{Enabled: true},
	}
	if !cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = false, want true")
	}

	cfg.Xfyun.APIKey = ""
	if cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = true without API key")
	}

	cfg = SpeechConfig{Provider: "disabled", Jigasi: JigasiSpeechConfig{Enabled: true}}
	if cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = true for disabled provider")
	}
	cfg.Jigasi.Enabled = false
	if cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = true without Jigasi")
	}

	cfg = SpeechConfig{
		Provider: "custom_provider",
		Jigasi: JigasiSpeechConfig{
			Enabled: true, SharedSecret: strings.Repeat("s", 48),
		},
	}
	if !cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = false for registered-provider configuration")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() custom provider error = %v", err)
	}

	cfg = SpeechConfig{
		Provider: "mock",
		Mock:     MockSpeechConfig{SegmentDurationMS: 500},
		Jigasi:   JigasiSpeechConfig{Enabled: true, SharedSecret: strings.Repeat("s", 48)},
	}
	if !cfg.TranscriptionEnabled() {
		t.Fatal("TranscriptionEnabled() = false for mock provider")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() mock provider error = %v", err)
	}
	cfg.Mock.SegmentDurationMS = 100
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "segmentDurationMs") {
		t.Fatalf("Validate() error = %v, want mock segment duration validation", err)
	}
}
