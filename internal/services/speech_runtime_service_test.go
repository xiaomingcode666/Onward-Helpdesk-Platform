package services

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestSpeechRuntimeServiceSwitchesProviderAndRollsBackInvalidReplacement(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &models.AuthAuditLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	operator := &dto.AuthPrincipal{
		DomainType: models.DomainTypePlatform,
		UserID:     1,
		Username:   "platform-admin",
	}
	previousConfig := config.CurrentOrDefault()
	previousSpeech := providers.CurrentSpeechConfig()
	t.Cleanup(func() {
		config.SetCurrent(&previousConfig)
		if err := providers.InitSpeech(&previousSpeech); err != nil {
			t.Fatalf("restore speech provider: %v", err)
		}
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	initial := config.SpeechConfig{
		Provider: "disabled",
		Jigasi: config.JigasiSpeechConfig{
			Enabled:      true,
			SharedSecret: "0123456789abcdef0123456789abcdef",
		},
	}
	if err := providers.InitSpeech(&initial); err != nil {
		t.Fatalf("initialize speech runtime: %v", err)
	}
	config.SetCurrent(&config.Config{
		EncryptionKey: "speech-runtime-test-key-0123456789",
		Speech:        initial,
	})

	status, err := SpeechRuntimeService.Update(request.SpeechRuntimeUpdateRequest{
		Provider:    "xfyun",
		XfyunAppID:  "runtime-app",
		XfyunAPIKey: "runtime-key",
	}, operator)
	if err != nil {
		t.Fatalf("switch to xfyun: %v", err)
	}
	if status.Provider != "xfyun" || !status.Configured || !status.Enabled {
		t.Fatalf("xfyun status=%+v", status)
	}
	if !status.XfyunCredentialsConfigured || status.XfyunTranslationEnabled {
		t.Fatalf("speech runtime metadata=%+v", status)
	}
	if !status.CanUpdate {
		t.Fatalf("platform runtime status should be editable: %+v", status)
	}
	runtimeConfig, runtimeProvider := providers.CurrentSpeechRuntime()
	if runtimeConfig.ProviderName() != "xfyun" || runtimeProvider.Name() != "xfyun_rtasr" {
		t.Fatalf("runtime config=%q provider=%q", runtimeConfig.ProviderName(), runtimeProvider.Name())
	}
	if current := config.Current().Speech; current.Xfyun.AppID != "runtime-app" || current.Xfyun.APIKey != "runtime-key" {
		t.Fatalf("global speech config was not updated: %+v", current)
	}
	stored := repositories.SystemConfigRepository.FindByKey(db, speechRuntimeConfigKey)
	if stored == nil {
		t.Fatal("speech runtime config was not persisted")
	}
	if strings.Contains(stored.ConfigValue, "runtime-key") || !strings.Contains(stored.ConfigValue, "enc:v1:") {
		t.Fatalf("persisted speech config did not protect credentials: %s", stored.ConfigValue)
	}
	var auditCount int64
	if err := db.Model(&models.AuthAuditLog{}).Where("action = ?", "speech_runtime.updated").Count(&auditCount).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("speech runtime audit count = %d, want 1", auditCount)
	}

	translationEnabled := true
	if _, err := SpeechRuntimeService.Update(request.SpeechRuntimeUpdateRequest{
		Provider:                "xfyun",
		XfyunTranslationEnabled: &translationEnabled,
	}, operator); err == nil {
		t.Fatal("invalid replacement unexpectedly succeeded")
	}
	runtimeConfig, runtimeProvider = providers.CurrentSpeechRuntime()
	if runtimeConfig.ProviderName() != "xfyun" || runtimeProvider.Name() != "xfyun_rtasr" {
		t.Fatalf("failed update changed runtime config=%q provider=%q", runtimeConfig.ProviderName(), runtimeProvider.Name())
	}

	disabled := initial
	disabled.Provider = "disabled"
	if err := providers.InitSpeech(&disabled); err != nil {
		t.Fatalf("disable runtime before reload: %v", err)
	}
	currentConfig := config.CurrentOrDefault()
	currentConfig.Speech = disabled
	config.SetCurrent(&currentConfig)
	if err := SpeechRuntimeService.LoadPersisted(); err != nil {
		t.Fatalf("load persisted speech runtime: %v", err)
	}
	runtimeConfig, runtimeProvider = providers.CurrentSpeechRuntime()
	if runtimeConfig.ProviderName() != "xfyun" || runtimeConfig.Xfyun.APIKey != "runtime-key" || runtimeProvider.Name() != "xfyun_rtasr" {
		t.Fatalf("persisted runtime was not restored: config=%+v provider=%q", runtimeConfig, runtimeProvider.Name())
	}
}

func TestSpeechRuntimeServiceSeedsEnvironmentOverDatabase(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &models.AuthAuditLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	previousConfig := config.CurrentOrDefault()
	previousSpeech := providers.CurrentSpeechConfig()
	t.Cleanup(func() {
		config.SetCurrent(&previousConfig)
		if err := providers.InitSpeech(&previousSpeech); err != nil {
			t.Fatalf("restore speech provider: %v", err)
		}
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	const jigasiSecret = "environment-jigasi-secret-0123456789"
	environmentSpeech := config.SpeechConfig{
		Provider: "mock",
		Mock: config.MockSpeechConfig{
			SegmentDurationMS: 1500,
		},
		Jigasi: config.JigasiSpeechConfig{
			Enabled:               true,
			SharedSecret:          jigasiSecret,
			MaxParticipantStreams: 4,
			MaxConcurrentStreams:  8,
		},
	}
	config.SetCurrent(&config.Config{
		EncryptionKey: "speech-environment-seed-key-0123456789",
		Speech:        environmentSpeech,
	})
	if err := providers.InitSpeech(&environmentSpeech); err != nil {
		t.Fatalf("initialize environment speech runtime: %v", err)
	}
	t.Setenv("SPEECH_PROVIDER", "mock")
	t.Setenv("SPEECH_JIGASI_ENABLED", "true")
	t.Setenv("SPEECH_JIGASI_SHARED_SECRET", jigasiSecret)

	if err := SpeechRuntimeService.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted() seed error = %v", err)
	}
	stored := repositories.SystemConfigRepository.FindByKey(db, speechRuntimeConfigKey)
	if stored == nil {
		t.Fatal("speech environment config was not seeded")
	}
	if strings.Contains(stored.ConfigValue, jigasiSecret) || !strings.Contains(stored.ConfigValue, "enc:v1:") {
		t.Fatalf("seeded speech credential was not protected: %s", stored.ConfigValue)
	}
	runtimeConfig := providers.CurrentSpeechConfig()
	if runtimeConfig.ProviderName() != "mock" || !runtimeConfig.Jigasi.Enabled || runtimeConfig.Jigasi.SharedSecret != jigasiSecret {
		t.Fatalf("speech environment config was not loaded: %+v", runtimeConfig)
	}

	configuredSpeech := environmentSpeech
	configuredSpeech.Mock.SegmentDurationMS = 3200
	if err := SpeechRuntimeService.persistAndAudit(configuredSpeech, environmentSpeech, nil); err != nil {
		t.Fatalf("persist database speech override: %v", err)
	}
	environmentSpeech.Mock.SegmentDurationMS = 9000
	config.SetCurrent(&config.Config{
		EncryptionKey: "speech-environment-seed-key-0123456789",
		Speech:        environmentSpeech,
	})
	if err := providers.InitSpeech(&environmentSpeech); err != nil {
		t.Fatalf("initialize changed environment speech runtime: %v", err)
	}
	if err := SpeechRuntimeService.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted() environment precedence error = %v", err)
	}
	if got := providers.CurrentSpeechConfig().Mock.SegmentDurationMS; got != 9000 {
		t.Fatalf("environment speech config did not overwrite database: got %d, want 9000", got)
	}
}

func TestSpeechRuntimeServiceRejectsTenantUpdate(t *testing.T) {
	_, err := SpeechRuntimeService.Update(request.SpeechRuntimeUpdateRequest{Provider: "mock"}, &dto.AuthPrincipal{
		DomainType: models.DomainTypeEnterprise,
		TenantID:   42,
		UserID:     9,
	})
	if err == nil {
		t.Fatal("tenant operator unexpectedly changed shared speech runtime")
	}
}

func TestSpeechRuntimeServiceReadinessMapsProviderErrors(t *testing.T) {
	readiness := buildSpeechProviderReadiness(nil)
	if !readiness.Ready || readiness.Code != "" || readiness.Message != "" {
		t.Fatalf("successful readiness = %+v", readiness)
	}

	readiness = buildSpeechProviderReadiness(&providers.XfyunRTASRError{
		Code: "10105", Description: "illegal access", SID: "test-sid",
	})
	if readiness.Ready || readiness.Code != "10105" || !strings.Contains(readiness.Message, "APPID") {
		t.Fatalf("xfyun readiness = %+v", readiness)
	}

	readiness = buildSpeechProviderReadiness(&providers.XfyunRTASRError{
		Code: "10800", Description: "over max connect limit", SID: "test-sid",
	})
	if readiness.Ready || readiness.Code != "10800" || !strings.Contains(readiness.Message, "并发路数已满") {
		t.Fatalf("xfyun concurrency readiness = %+v", readiness)
	}
}
