package services

import (
	"bufio"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestPlatformIntegrationConfigServicePersistsAndTestsJitsiAndAR(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &models.AuthAuditLog{}, &models.AIConfig{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config.js":
			_, _ = w.Write([]byte("var config = { hosts: {} };"))
		case "/health":
			if got := r.Header.Get("Authorization"); got != "Bearer ar-runtime-secret" {
				t.Fatalf("AR authorization = %q", got)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	previous := config.CurrentOrDefault()
	t.Cleanup(func() {
		config.SetCurrent(&previous)
		providers.InitJitsi(&previous.Jitsi)
		if err := providers.InitMeetingARDetection(previous.MeetingAR); err != nil {
			_ = providers.InitMeetingARDetection(config.MeetingARConfig{})
		}
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	base := config.Config{EncryptionKey: "platform-integration-test-key-0123456789"}
	config.SetCurrent(&base)
	providers.InitJitsi(&base.Jitsi)
	if err := providers.InitMeetingARDetection(base.MeetingAR); err != nil {
		t.Fatalf("initialize disabled AR: %v", err)
	}
	operator := &dto.AuthPrincipal{DomainType: models.DomainTypePlatform, UserID: 7, Username: "platform-admin"}

	settings, err := PlatformIntegrationConfigService.Update(request.PlatformIntegrationUpdateRequest{
		Integration: "jitsi",
		Jitsi: &request.PlatformJitsiRuntimeUpdateRequest{
			URL: server.URL, JitsiDomain: strings.TrimPrefix(server.URL, "http://"),
			AppID: "remotehelpdesk", AppSecret: strings.Repeat("a", 32),
			WebhookSecret: strings.Repeat("b", 32), TokenTTLMinutes: 30, Timeout: "2s", MaxRetries: 1,
		},
	}, operator)
	if err != nil {
		t.Fatalf("update Jitsi: %v", err)
	}
	if settings.Jitsi.URL != server.URL || !settings.Jitsi.AppSecretConfigured || !settings.Jitsi.WebhookSecretConfigured {
		t.Fatalf("Jitsi settings = %+v", settings.Jitsi)
	}
	storedJitsi := repositories.SystemConfigRepository.FindByKey(db, platformJitsiRuntimeConfigKey)
	if storedJitsi == nil || strings.Contains(storedJitsi.ConfigValue, strings.Repeat("a", 32)) || !strings.Contains(storedJitsi.ConfigValue, "enc:v1:") {
		t.Fatalf("Jitsi runtime secret was not protected: %#v", storedJitsi)
	}
	jitsiTest := PlatformIntegrationConfigService.Test(request.PlatformIntegrationTestRequest{
		Integration: "jitsi",
		Jitsi: &request.PlatformJitsiRuntimeUpdateRequest{
			URL: server.URL, JitsiDomain: strings.TrimPrefix(server.URL, "http://"),
			AppID: "remotehelpdesk", TokenTTLMinutes: 30, Timeout: "2s", MaxRetries: 1,
		},
	}, operator)
	if !jitsiTest.Success {
		t.Fatalf("Jitsi connection test = %+v", jitsiTest)
	}

	settings, err = PlatformIntegrationConfigService.Update(request.PlatformIntegrationUpdateRequest{
		Integration: "ar",
		AR: &request.PlatformARRuntimeUpdateRequest{
			Provider: "http", Endpoint: server.URL + "/detect", APIKey: "ar-runtime-secret", HealthPath: "/health",
		},
	}, operator)
	if err != nil {
		t.Fatalf("update AR: %v", err)
	}
	if settings.AR.Provider != "http" || !settings.AR.Configured || !settings.AR.APIKeyConfigured {
		t.Fatalf("AR settings = %+v", settings.AR)
	}
	storedAR := repositories.SystemConfigRepository.FindByKey(db, platformARRuntimeConfigKey)
	if storedAR == nil || strings.Contains(storedAR.ConfigValue, "ar-runtime-secret") || !strings.Contains(storedAR.ConfigValue, "enc:v1:") {
		t.Fatalf("AR runtime secret was not protected: %#v", storedAR)
	}
	arTest := PlatformIntegrationConfigService.Test(request.PlatformIntegrationTestRequest{
		Integration: "ar",
		AR: &request.PlatformARRuntimeUpdateRequest{
			Provider: "http", Endpoint: server.URL + "/detect", HealthPath: "/health",
		},
	}, operator)
	if !arTest.Success {
		t.Fatalf("AR connection test = %+v", arTest)
	}

	var auditCount int64
	if err := db.Model(&models.AuthAuditLog{}).Where("action IN ?", []string{
		"jitsi_runtime.updated", "meeting_ar_runtime.updated", "platform_integration.connection_tested",
	}).Count(&auditCount).Error; err != nil {
		t.Fatalf("count integration audits: %v", err)
	}
	if auditCount != 4 {
		t.Fatalf("integration audit count = %d, want 4", auditCount)
	}
}

func TestPlatformIntegrationConfigServiceRejectsTenantOperator(t *testing.T) {
	operator := &dto.AuthPrincipal{DomainType: models.DomainTypeEnterprise, TenantID: 42, UserID: 9}
	if _, err := PlatformIntegrationConfigService.Update(request.PlatformIntegrationUpdateRequest{
		Integration: "jitsi", Jitsi: &request.PlatformJitsiRuntimeUpdateRequest{},
	}, operator); err == nil {
		t.Fatal("tenant operator unexpectedly updated platform integration")
	}
	result := PlatformIntegrationConfigService.Test(request.PlatformIntegrationTestRequest{Integration: "jitsi"}, operator)
	if result.Success || !strings.Contains(result.Message, "platform operators") {
		t.Fatalf("tenant connection test result = %+v", result)
	}
}

func TestPlatformIntegrationConfigServicePersistsAndTestsSMTP(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &models.AuthAuditLog{}, &models.AIConfig{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	smtpAddr := startFakeSMTPServer(t, "sender@example.com", "smtp-runtime-secret")

	previous := config.CurrentOrDefault()
	t.Cleanup(func() {
		config.SetCurrent(&previous)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	base := config.Config{EncryptionKey: "platform-smtp-test-key-0123456789"}
	config.SetCurrent(&base)
	operator := &dto.AuthPrincipal{DomainType: models.DomainTypePlatform, UserID: 7, Username: "platform-admin"}
	host, portText, err := net.SplitHostPort(smtpAddr)
	if err != nil {
		t.Fatalf("split SMTP address: %v", err)
	}

	settings, err := PlatformIntegrationConfigService.Update(request.PlatformIntegrationUpdateRequest{
		Integration: "smtp",
		SMTP: &request.PlatformSMTPRuntimeUpdateRequest{
			SMTPHost: host, SMTPPort: mustAtoi(t, portText), Username: "sender@example.com",
			Password: "smtp-runtime-secret", FromAddress: "sender@example.com",
			FromName: "RemoteHelpDesk", UseTLS: false,
		},
	}, operator)
	if err != nil {
		t.Fatalf("update SMTP: %v", err)
	}
	if !settings.SMTP.Configured || !settings.SMTP.PasswordConfigured || settings.SMTP.SMTPHost != host {
		t.Fatalf("SMTP settings = %+v", settings.SMTP)
	}
	storedSMTP := repositories.SystemConfigRepository.FindByKey(db, platformSMTPRuntimeConfigKey)
	if storedSMTP == nil || strings.Contains(storedSMTP.ConfigValue, "smtp-runtime-secret") || !strings.Contains(storedSMTP.ConfigValue, "enc:v1:") {
		t.Fatalf("SMTP runtime secret was not protected: %#v", storedSMTP)
	}

	smtpTest := PlatformIntegrationConfigService.Test(request.PlatformIntegrationTestRequest{
		Integration: "smtp",
		SMTP: &request.PlatformSMTPRuntimeUpdateRequest{
			SMTPHost: host, SMTPPort: mustAtoi(t, portText), Username: "sender@example.com",
			FromAddress: "sender@example.com", FromName: "RemoteHelpDesk", UseTLS: false,
		},
	}, operator)
	if !smtpTest.Success {
		t.Fatalf("SMTP connection test = %+v", smtpTest)
	}
}

func TestPlatformIntegrationConfigServiceSeedsEnvironmentOverDatabase(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &models.AuthAuditLog{}, &models.AIConfig{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	previous := config.CurrentOrDefault()
	t.Cleanup(func() {
		config.SetCurrent(&previous)
		providers.InitJitsi(&previous.Jitsi)
		if err := providers.InitMeetingARDetection(previous.MeetingAR); err != nil {
			_ = providers.InitMeetingARDetection(config.MeetingARConfig{})
		}
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	adminKey := "sub2api-environment-admin-key"
	modelKey := "sub2api-environment-model-key"
	embeddingKey := "dashscope-embedding-key"
	smtpPassword := "smtp-environment-password"
	environmentConfig := config.Config{
		EncryptionKey: "platform-environment-seed-key-0123456789",
		Jitsi: config.JitsiConfig{
			URL: "https://meet.environment.example.com", AppID: "remotehelpdesk",
			AppSecret: strings.Repeat("j", 32), WebhookSecret: strings.Repeat("w", 32),
			JitsiDomain: "meet.environment.example.com", TokenTTLMinutes: 15, Timeout: "10s", MaxRetries: 2,
		},
		MeetingAR: config.MeetingARConfig{Provider: "http", ProviderOptions: map[string]any{
			"endpoint": "https://vision.environment.example.com/v1/detect",
			"apiKey":   "ar-environment-key", "healthPath": "/health",
		}},
		Sub2API: config.Sub2APIConfig{
			BaseURL: "https://sub2api.environment.example.com", AdminAPIKey: adminKey,
			DefaultKey: modelKey, DefaultModel: "gpt-environment",
		},
		Email: config.EmailConfig{
			SMTPHost: "smtp.environment.example.com", SMTPPort: 465,
			Username: "smtp.environment@example.com", Password: smtpPassword,
			FromAddress: "smtp.environment@example.com", FromName: "RemoteHelpDesk", UseTLS: true,
		},
	}
	config.SetCurrent(&environmentConfig)
	providers.InitJitsi(&environmentConfig.Jitsi)
	if err := providers.InitMeetingARDetection(environmentConfig.MeetingAR); err != nil {
		t.Fatalf("initialize environment AR: %v", err)
	}
	t.Setenv("JITSI_URL", environmentConfig.Jitsi.URL)
	t.Setenv("MEETING_AR_PROVIDER", "http")
	t.Setenv("SUB2API_BASE_URL", environmentConfig.Sub2API.BaseURL)
	t.Setenv("SUB2API_ADMIN_API_KEY", adminKey)
	t.Setenv("SUB2API_DEFAULT_KEY", modelKey)
	t.Setenv("SUB2API_DEFAULT_MODEL", environmentConfig.Sub2API.DefaultModel)
	t.Setenv("SMTP_HOST", environmentConfig.Email.SMTPHost)
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", environmentConfig.Email.Username)
	t.Setenv("SMTP_PASSWORD", smtpPassword)
	t.Setenv("SMTP_FROM_ADDRESS", environmentConfig.Email.FromAddress)
	t.Setenv("SMTP_FROM_NAME", environmentConfig.Email.FromName)
	t.Setenv("SMTP_USE_TLS", "true")
	t.Setenv("OPENAI_API_KEY", embeddingKey)
	t.Setenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	t.Setenv("EMBEDDING_MODEL", "text-embedding-v3")
	t.Setenv("EMBEDDING_DIM", "1024")
	if err := repositories.SystemConfigRepository.SaveByKey(db, &models.SystemConfig{
		ConfigKey: platformSub2APIHostConfigKey, ConfigValue: "",
		GroupCode: "platform_ai", Title: "Sub2API Host", Status: 0,
	}); err != nil {
		t.Fatalf("save empty database config: %v", err)
	}
	if err := db.Create(&[]models.AIConfig{
		{
			Name:        "P0 deterministic LLM",
			Provider:    enums.AIProviderOpenAI,
			BaseURL:     "http://127.0.0.1:18099/v1",
			APIKey:      "local-llm-key",
			ModelType:   enums.AIModelTypeLLM,
			ModelName:   "e2e-llm",
			Status:      enums.StatusOk,
			SortNo:      10000,
			AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
		{
			Name:        "P0 deterministic embedding",
			Provider:    enums.AIProviderOpenAI,
			BaseURL:     "http://127.0.0.1:18099/v1",
			APIKey:      "local-embedding-key",
			ModelType:   enums.AIModelTypeEmbedding,
			ModelName:   "e2e-embedding",
			Dimension:   8,
			Status:      enums.StatusOk,
			SortNo:      10000,
			AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}).Error; err != nil {
		t.Fatalf("seed legacy AI configs: %v", err)
	}

	if err := PlatformIntegrationConfigService.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted() seed error = %v", err)
	}
	for _, key := range []string{
		platformJitsiRuntimeConfigKey,
		platformARRuntimeConfigKey,
		platformSMTPRuntimeConfigKey,
		platformSub2APIHostConfigKey,
		platformSub2APIAdminKeyConfigKey,
		platformSub2APILLMModelConfigKey,
	} {
		if item := repositories.SystemConfigRepository.FindByKey(db, key); item == nil || strings.TrimSpace(item.ConfigValue) == "" {
			t.Fatalf("environment config %s was not seeded", key)
		}
	}
	for _, key := range []string{platformJitsiRuntimeConfigKey, platformARRuntimeConfigKey, platformSMTPRuntimeConfigKey, platformSub2APIAdminKeyConfigKey} {
		item := repositories.SystemConfigRepository.FindByKey(db, key)
		if strings.Contains(item.ConfigValue, adminKey) || strings.Contains(item.ConfigValue, smtpPassword) || strings.Contains(item.ConfigValue, "ar-environment-key") || strings.Contains(item.ConfigValue, strings.Repeat("j", 32)) {
			t.Fatalf("environment secret leaked into %s", key)
		}
	}

	operator := &dto.AuthPrincipal{DomainType: models.DomainTypePlatform, UserID: 7, Username: "platform-admin"}
	settings, err := PlatformIntegrationConfigService.GetSettings(operator)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings.Sub2API.Host != environmentConfig.Sub2API.BaseURL || settings.Sub2API.DefaultLLMModel != "gpt-environment" {
		t.Fatalf("Sub2API environment settings = %+v", settings.Sub2API)
	}
	if settings.Sub2API.AdminAPIKeyMasked != "********-key" || strings.Contains(settings.Sub2API.AdminAPIKeyMasked, adminKey) {
		t.Fatalf("Sub2API masked API key = %q", settings.Sub2API.AdminAPIKeyMasked)
	}
	if !settings.SMTP.Configured || !settings.SMTP.PasswordConfigured || settings.SMTP.SMTPHost != environmentConfig.Email.SMTPHost {
		t.Fatalf("SMTP environment settings = %+v", settings.SMTP)
	}
	var seededLLM models.AIConfig
	if err := db.Where("model_type = ? AND status = ?", enums.AIModelTypeLLM, enums.StatusOk).First(&seededLLM).Error; err != nil {
		t.Fatalf("seeded LLM AI config not found: %v", err)
	}
	if seededLLM.Provider != enums.AIProviderSub2API || seededLLM.BaseURL != "https://sub2api.environment.example.com/v1" || seededLLM.ModelName != "gpt-environment" || seededLLM.APIKey != modelKey {
		t.Fatalf("seeded LLM AI config = %+v", seededLLM)
	}
	var seededEmbedding models.AIConfig
	if err := db.Where("model_type = ? AND status = ?", enums.AIModelTypeEmbedding, enums.StatusOk).First(&seededEmbedding).Error; err != nil {
		t.Fatalf("seeded embedding AI config not found: %v", err)
	}
	if seededEmbedding.Provider != enums.AIProviderOpenAI || seededEmbedding.BaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" || seededEmbedding.ModelName != "text-embedding-v3" || seededEmbedding.Dimension != 1024 || seededEmbedding.APIKey != embeddingKey {
		t.Fatalf("seeded embedding AI config = %+v", seededEmbedding)
	}

	if err := repositories.SystemConfigRepository.SaveByKey(db, &models.SystemConfig{
		ConfigKey: platformSub2APIHostConfigKey, ConfigValue: "https://sub2api.database.example.com",
		GroupCode: "platform_ai", Title: "Sub2API Host", Status: 0,
	}); err != nil {
		t.Fatalf("save database override: %v", err)
	}
	environmentConfig.Sub2API.BaseURL = "https://sub2api.changed-environment.example.com"
	environmentConfig.Sub2API.AdminAPIKey = "changed-environment-admin-key"
	environmentConfig.Sub2API.DefaultModel = "gpt-changed-environment"
	environmentConfig.Jitsi.URL = "https://meet.changed-environment.example.com"
	environmentConfig.MeetingAR.ProviderOptions["endpoint"] = "https://vision.changed-environment.example.com/v1/detect"
	environmentConfig.Email.SMTPHost = "smtp.changed-environment.example.com"
	config.SetCurrent(&environmentConfig)
	providers.InitJitsi(&environmentConfig.Jitsi)
	if err := providers.InitMeetingARDetection(environmentConfig.MeetingAR); err != nil {
		t.Fatalf("initialize changed environment AR: %v", err)
	}
	t.Setenv("SUB2API_BASE_URL", environmentConfig.Sub2API.BaseURL)
	t.Setenv("SUB2API_ADMIN_API_KEY", environmentConfig.Sub2API.AdminAPIKey)
	t.Setenv("SUB2API_DEFAULT_MODEL", environmentConfig.Sub2API.DefaultModel)
	t.Setenv("JITSI_URL", environmentConfig.Jitsi.URL)
	t.Setenv("MEETING_AR_ENDPOINT", meetingARConfigOption(environmentConfig.MeetingAR, "endpoint"))
	t.Setenv("SMTP_HOST", environmentConfig.Email.SMTPHost)
	if err := PlatformIntegrationConfigService.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted() environment precedence error = %v", err)
	}
	settings, err = PlatformIntegrationConfigService.GetSettings(operator)
	if err != nil {
		t.Fatalf("GetSettings() after override error = %v", err)
	}
	if settings.Sub2API.Host != environmentConfig.Sub2API.BaseURL || settings.Sub2API.AdminAPIKeyMasked != "********-key" || settings.Sub2API.DefaultLLMModel != environmentConfig.Sub2API.DefaultModel {
		t.Fatalf("environment configuration did not overwrite database: %+v", settings.Sub2API)
	}
	if settings.Jitsi.URL != environmentConfig.Jitsi.URL {
		t.Fatalf("environment Jitsi config did not overwrite database: %+v", settings.Jitsi)
	}
	if settings.AR.Endpoint != meetingARConfigOption(environmentConfig.MeetingAR, "endpoint") {
		t.Fatalf("environment AR config did not overwrite database: %+v", settings.AR)
	}
	if settings.SMTP.SMTPHost != environmentConfig.Email.SMTPHost {
		t.Fatalf("environment SMTP config did not overwrite database: %+v", settings.SMTP)
	}

	environmentConfig.Sub2API.BaseURL = "https://sub2api.url-only-environment.example.com"
	environmentConfig.Sub2API.AdminAPIKey = ""
	environmentConfig.Sub2API.DefaultKey = ""
	config.SetCurrent(&environmentConfig)
	t.Setenv("SUB2API_BASE_URL", environmentConfig.Sub2API.BaseURL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "")
	t.Setenv("SUB2API_DEFAULT_KEY", "")
	if err := PlatformIntegrationConfigService.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted() url-only environment override error = %v", err)
	}
	settings, err = PlatformIntegrationConfigService.GetSettings(operator)
	if err != nil {
		t.Fatalf("GetSettings() after url-only override error = %v", err)
	}
	if settings.Sub2API.Host != environmentConfig.Sub2API.BaseURL || settings.Sub2API.AdminAPIKeyConfigured {
		t.Fatalf("url-only environment Sub2API settings = %+v", settings.Sub2API)
	}
	storedAdminKey := repositories.SystemConfigRepository.FindByKey(db, platformSub2APIAdminKeyConfigKey)
	if storedAdminKey == nil || strings.TrimSpace(storedAdminKey.ConfigValue) != "" {
		t.Fatalf("url-only environment did not clear stale admin key: %#v", storedAdminKey)
	}
}

func startFakeSMTPServer(t *testing.T, wantUsername, wantPassword string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake SMTP: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTPConnection(t, conn, wantUsername, wantPassword)
		}
	}()
	return listener.Addr().String()
}

func handleFakeSMTPConnection(t *testing.T, conn net.Conn, wantUsername, wantPassword string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeLine := func(line string) bool {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			t.Logf("fake SMTP write %q: %v", line, err)
			return false
		}
		if err := writer.Flush(); err != nil {
			t.Logf("fake SMTP flush: %v", err)
			return false
		}
		return true
	}
	readLine := func() (string, bool) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Logf("fake SMTP read: %v", err)
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}
	if !writeLine("220 localhost ESMTP") {
		return
	}
	for {
		line, ok := readLine()
		if !ok {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			if !writeLine("250-localhost") || !writeLine("250 AUTH PLAIN") {
				return
			}
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			parts := strings.Fields(line)
			token := ""
			if len(parts) >= 3 {
				token = parts[2]
			} else {
				if !writeLine("334 ") {
					return
				}
				var ok bool
				token, ok = readLine()
				if !ok {
					return
				}
			}
			raw, err := base64.StdEncoding.DecodeString(token)
			values := strings.Split(string(raw), "\x00")
			if err != nil || len(values) < 3 || values[1] != wantUsername || values[2] != wantPassword {
				if !writeLine("535 authentication failed") {
					return
				}
				continue
			}
			if !writeLine("235 authentication successful") {
				return
			}
		case upper == "NOOP":
			if !writeLine("250 ok") {
				return
			}
		case upper == "QUIT":
			_ = writeLine("221 bye")
			return
		default:
			if !writeLine("250 ok") {
				return
			}
		}
	}
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()
	ret, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("atoi %q: %v", value, err)
	}
	return ret
}
