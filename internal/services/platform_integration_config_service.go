package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var PlatformIntegrationConfigService = &platformIntegrationConfigService{}

const (
	platformJitsiRuntimeConfigKey = "platform.jitsi_runtime.v1"
	platformARRuntimeConfigKey    = "platform.meeting_ar_runtime.v1"
	platformSMTPRuntimeConfigKey  = "platform.smtp_runtime.v1"
	integrationProbeTimeout       = 8 * time.Second
)

type platformIntegrationConfigService struct {
	mu sync.Mutex
}

type PlatformJitsiRuntimeSettings struct {
	URL                     string
	JitsiDomain             string
	RequireAuth             bool
	AppID                   string
	AppSecretConfigured     bool
	WebhookSecretConfigured bool
	TokenTTLMinutes         int
	Timeout                 string
	MaxRetries              int
	UpdatedAt               *time.Time
}

type PlatformSpeechRuntimeSettings struct {
	Provider                     string
	Configured                   bool
	XfyunAppID                   string
	XfyunAPIKeyConfigured        bool
	XfyunEndpoint                string
	XfyunDomain                  string
	AliyunAPIKeyConfigured       bool
	AliyunWorkspaceID            string
	AliyunEndpoint               string
	AliyunModel                  string
	JigasiEnabled                bool
	JigasiSharedSecretConfigured bool
	JigasiMaxParticipantStreams  int
	JigasiMaxConcurrentStreams   int
	TranslationEnabled           bool
	TranslationConfigured        bool
	TranslationEndpoint          string
	TranslationTargetLanguage    string
}

type PlatformARRuntimeSettings struct {
	Provider         string
	Configured       bool
	Endpoint         string
	APIKeyConfigured bool
	HealthPath       string
	UpdatedAt        *time.Time
}

type PlatformSub2APIRuntimeSettings struct {
	Host                         string
	AdminAPIKeyConfigured        bool
	AdminAPIKeyMasked            string
	AdminAPIKeyFingerprint       string
	TranslationAPIKeyConfigured  bool
	TranslationAPIKeyMasked      string
	TranslationAPIKeyFingerprint string
	DefaultLLMModel              string
	UpdatedAt                    *time.Time
}

type PlatformSMTPRuntimeSettings struct {
	SMTPHost           string
	SMTPPort           int
	Username           string
	PasswordConfigured bool
	FromAddress        string
	FromName           string
	UseTLS             bool
	Configured         bool
	UpdatedAt          *time.Time
}

type PlatformIntegrationSettingsAggregate struct {
	CanUpdate bool
	Jitsi     PlatformJitsiRuntimeSettings
	Speech    PlatformSpeechRuntimeSettings
	AR        PlatformARRuntimeSettings
	Sub2API   PlatformSub2APIRuntimeSettings
	SMTP      PlatformSMTPRuntimeSettings
}

type PlatformIntegrationTestAggregate struct {
	Integration string
	Success     bool
	Message     string
	CheckedAt   time.Time
	Latency     time.Duration
	Details     map[string]any
}

type persistedPlatformJitsiRuntime struct {
	Version                int    `json:"version"`
	URL                    string `json:"url"`
	JitsiDomain            string `json:"jitsiDomain,omitempty"`
	RequireAuth            bool   `json:"requireAuth"`
	AppID                  string `json:"appId,omitempty"`
	AppSecretEncrypted     string `json:"appSecretEncrypted,omitempty"`
	WebhookSecretEncrypted string `json:"webhookSecretEncrypted,omitempty"`
	TokenTTLMinutes        int    `json:"tokenTtlMinutes,omitempty"`
	Timeout                string `json:"timeout,omitempty"`
	MaxRetries             int    `json:"maxRetries,omitempty"`
	UpdatedAt              string `json:"updatedAt"`
}

type persistedPlatformARRuntime struct {
	Version         int    `json:"version"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint,omitempty"`
	APIKeyEncrypted string `json:"apiKeyEncrypted,omitempty"`
	HealthPath      string `json:"healthPath,omitempty"`
	UpdatedAt       string `json:"updatedAt"`
}

type persistedPlatformSMTPRuntime struct {
	Version           int    `json:"version"`
	SMTPHost          string `json:"smtpHost,omitempty"`
	SMTPPort          int    `json:"smtpPort,omitempty"`
	Username          string `json:"username,omitempty"`
	PasswordEncrypted string `json:"passwordEncrypted,omitempty"`
	FromAddress       string `json:"fromAddress,omitempty"`
	FromName          string `json:"fromName,omitempty"`
	UseTLS            bool   `json:"useTls"`
	UpdatedAt         string `json:"updatedAt"`
}

func (s *platformIntegrationConfigService) GetSettings(operator *dto.AuthPrincipal) (*PlatformIntegrationSettingsAggregate, error) {
	cfg := config.CurrentOrDefault()
	_, speechProvider := providers.CurrentSpeechRuntime()
	translationProvider := providers.CurrentTextTranslationProvider()
	arProvider := providers.CurrentMeetingARDetectionProvider()
	host, adminKey, fingerprint, sub2UpdatedAt, err := PlatformAIModelService.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	translationKey, translationKeyFingerprint, translationUpdatedAt, err := resolvePlatformSub2APITranslationKeyConfig()
	if err != nil {
		return nil, err
	}
	sub2UpdatedAt = latestTimePtr(sub2UpdatedAt, translationUpdatedAt)
	return &PlatformIntegrationSettingsAggregate{
		CanUpdate: canUpdatePlatformIntegration(operator),
		Jitsi: PlatformJitsiRuntimeSettings{
			URL: strings.TrimSpace(cfg.Jitsi.URL), JitsiDomain: strings.TrimSpace(cfg.Jitsi.JitsiDomain),
			RequireAuth: cfg.Jitsi.RequireAuth, AppID: strings.TrimSpace(cfg.Jitsi.AppID),
			AppSecretConfigured:     strings.TrimSpace(cfg.Jitsi.AppSecret) != "",
			WebhookSecretConfigured: strings.TrimSpace(cfg.Jitsi.WebhookSecret) != "",
			TokenTTLMinutes:         effectiveJitsiTTL(cfg.Jitsi), Timeout: effectiveJitsiTimeout(cfg.Jitsi),
			MaxRetries: effectiveJitsiRetries(cfg.Jitsi), UpdatedAt: systemConfigUpdatedAt(platformJitsiRuntimeConfigKey),
		},
		Speech: PlatformSpeechRuntimeSettings{
			Provider: cfg.Speech.ProviderName(), Configured: speechProvider != nil && speechProvider.Configured(),
			XfyunAppID:            strings.TrimSpace(cfg.Speech.Xfyun.AppID),
			XfyunAPIKeyConfigured: strings.TrimSpace(cfg.Speech.Xfyun.APIKey) != "",
			XfyunEndpoint:         strings.TrimSpace(cfg.Speech.Xfyun.Endpoint), XfyunDomain: strings.TrimSpace(cfg.Speech.Xfyun.Domain),
			AliyunAPIKeyConfigured:       strings.TrimSpace(cfg.Speech.Aliyun.APIKey) != "",
			AliyunWorkspaceID:            strings.TrimSpace(cfg.Speech.Aliyun.WorkspaceID),
			AliyunEndpoint:               strings.TrimSpace(cfg.Speech.Aliyun.Endpoint),
			AliyunModel:                  strings.TrimSpace(cfg.Speech.Aliyun.Model),
			JigasiEnabled:                cfg.Speech.Jigasi.Enabled,
			JigasiSharedSecretConfigured: strings.TrimSpace(cfg.Speech.Jigasi.SharedSecret) != "",
			JigasiMaxParticipantStreams:  cfg.Speech.Jigasi.MaxStreams(),
			JigasiMaxConcurrentStreams:   cfg.Speech.Jigasi.MaxConcurrent(),
			TranslationEnabled:           cfg.Speech.Xfyun.TranslationEnabled,
			TranslationConfigured:        translationProvider != nil && translationProvider.Configured(),
			TranslationEndpoint:          strings.TrimSpace(cfg.Speech.Xfyun.TranslationEndpoint),
			TranslationTargetLanguage:    strings.TrimSpace(cfg.Speech.Xfyun.TranslationTargetLanguage),
		},
		AR: PlatformARRuntimeSettings{
			Provider: cfg.MeetingAR.ProviderName(), Configured: arProvider != nil && arProvider.Configured(),
			Endpoint:         meetingARConfigOption(cfg.MeetingAR, "endpoint"),
			APIKeyConfigured: meetingARConfigOption(cfg.MeetingAR, "apiKey") != "",
			HealthPath:       meetingARConfigOption(cfg.MeetingAR, "healthPath"),
			UpdatedAt:        systemConfigUpdatedAt(platformARRuntimeConfigKey),
		},
		Sub2API: PlatformSub2APIRuntimeSettings{
			Host: host, AdminAPIKeyConfigured: strings.TrimSpace(adminKey) != "",
			AdminAPIKeyMasked:      maskedIntegrationSecret(adminKey),
			AdminAPIKeyFingerprint: fingerprint, DefaultLLMModel: PlatformAIModelService.resolveDefaultLLMModel(),
			TranslationAPIKeyConfigured:  strings.TrimSpace(translationKey) != "",
			TranslationAPIKeyMasked:      maskedIntegrationSecret(translationKey),
			TranslationAPIKeyFingerprint: translationKeyFingerprint,
			UpdatedAt:                    sub2UpdatedAt,
		},
		SMTP: PlatformSMTPRuntimeSettings{
			SMTPHost: strings.TrimSpace(cfg.Email.SMTPHost), SMTPPort: cfg.Email.SMTPPort,
			Username:           strings.TrimSpace(cfg.Email.Username),
			PasswordConfigured: strings.TrimSpace(cfg.Email.Password) != "",
			FromAddress:        strings.TrimSpace(cfg.Email.FromAddress), FromName: strings.TrimSpace(cfg.Email.FromName),
			UseTLS: cfg.Email.UseTLS, Configured: smtpRuntimeConfigured(cfg.Email),
			UpdatedAt: systemConfigUpdatedAt(platformSMTPRuntimeConfigKey),
		},
	}, nil
}

func (s *platformIntegrationConfigService) Update(input request.PlatformIntegrationUpdateRequest, operator *dto.AuthPrincipal) (*PlatformIntegrationSettingsAggregate, error) {
	if !canUpdatePlatformIntegration(operator) {
		return nil, errorsx.Forbidden("only platform operators can update shared integrations")
	}
	integration := normalizePlatformIntegration(input.Integration)
	switch integration {
	case "jitsi":
		if input.Jitsi == nil {
			return nil, errorsx.InvalidParam("jitsi settings are required")
		}
		if err := s.updateJitsi(*input.Jitsi, operator); err != nil {
			return nil, err
		}
	case "speech", "translation":
		if input.Speech == nil {
			return nil, errorsx.InvalidParam("speech settings are required")
		}
		if _, err := SpeechRuntimeService.Update(*input.Speech, operator); err != nil {
			return nil, err
		}
	case "ar":
		if input.AR == nil {
			return nil, errorsx.InvalidParam("AR settings are required")
		}
		if err := s.updateAR(*input.AR, operator); err != nil {
			return nil, err
		}
	case "sub2api":
		if input.Sub2API == nil {
			return nil, errorsx.InvalidParam("Sub2API settings are required")
		}
		before, _ := s.GetSettings(operator)
		if err := PlatformAIModelService.UpdateProvider(request.PlatformAIProviderSaveRequest{
			Host: input.Sub2API.Host, AdminAPIKey: input.Sub2API.AdminAPIKey,
			DefaultLLMModel: input.Sub2API.DefaultLLMModel, TranslationAPIKey: input.Sub2API.TranslationAPIKey,
		}, operator); err != nil {
			return nil, err
		}
		current := config.CurrentOrDefault()
		current.Sub2API.BaseURL = strings.TrimSpace(input.Sub2API.Host)
		config.SetCurrent(&current)
		after, _ := s.GetSettings(operator)
		s.recordUpdateAudit(operator, "sub2api", sanitizedSub2APIAuditState(before), sanitizedSub2APIAuditState(after))
	case "smtp", "email":
		if input.SMTP == nil {
			return nil, errorsx.InvalidParam("SMTP settings are required")
		}
		if err := s.updateSMTP(*input.SMTP, operator); err != nil {
			return nil, err
		}
	default:
		return nil, errorsx.InvalidParam("unsupported platform integration")
	}
	return s.GetSettings(operator)
}

func (s *platformIntegrationConfigService) Test(input request.PlatformIntegrationTestRequest, operator *dto.AuthPrincipal) *PlatformIntegrationTestAggregate {
	startedAt := time.Now()
	integration := normalizePlatformIntegration(input.Integration)
	result := &PlatformIntegrationTestAggregate{Integration: integration, CheckedAt: startedAt}
	if !canUpdatePlatformIntegration(operator) {
		result.Message = "only platform operators can test shared integrations"
		result.Latency = time.Since(startedAt)
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), integrationProbeTimeout)
	defer cancel()

	var err error
	switch integration {
	case "jitsi":
		var candidate config.JitsiConfig
		candidate, err = mergeJitsiRuntimeInput(config.CurrentOrDefault().Jitsi, input.Jitsi)
		if err == nil {
			health := providers.NewJitsiClient(&candidate).CheckHealth(ctx)
			result.Details = map[string]any{
				"status": health.Status, "serviceUrl": health.ServiceURL, "probeUrl": health.ProbeURL,
				"httpStatus": health.HTTPStatus,
			}
			if health.Status != "healthy" {
				err = errors.New(defaultString(health.Error, "Jitsi service is not ready"))
			}
		}
	case "speech":
		err = testSpeechRuntime(ctx, input.Speech)
	case "translation":
		var translated string
		translated, err = testTranslationRuntime(ctx, input.Speech)
		if translated != "" {
			result.Details = map[string]any{"sample": translated}
		}
	case "ar":
		err = testARRuntime(ctx, input.AR)
	case "sub2api":
		err = testSub2APIRuntime(ctx, input.Sub2API)
	case "smtp", "email":
		err = testSMTPRuntime(ctx, input.SMTP)
	default:
		err = errors.New("unsupported platform integration")
	}
	result.Latency = time.Since(startedAt)
	result.Success = err == nil
	if err != nil {
		result.Message = err.Error()
	} else {
		result.Message = "连接测试通过"
	}
	s.recordTestAudit(operator, result)
	return result
}

func (s *platformIntegrationConfigService) LoadPersisted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncEnvironmentDefaults(); err != nil {
		return err
	}
	if err := s.loadPersistedJitsi(); err != nil {
		return err
	}
	if err := s.loadPersistedAR(); err != nil {
		return err
	}
	if err := s.loadPersistedSMTP(); err != nil {
		return err
	}
	if host, _, _, _, err := PlatformAIModelService.resolveProviderConfig(); err != nil {
		return err
	} else if strings.TrimSpace(host) != "" {
		current := config.CurrentOrDefault()
		current.Sub2API.BaseURL = strings.TrimSpace(host)
		config.SetCurrent(&current)
	}
	return nil
}

func (s *platformIntegrationConfigService) syncEnvironmentDefaults() error {
	cfg := config.CurrentOrDefault()
	if config.HasEnvironmentOverride("jitsi") {
		appSecretEncrypted, err := secretstore.Encrypt(cfg.Jitsi.AppSecret)
		if err != nil {
			return err
		}
		webhookSecretEncrypted, err := secretstore.Encrypt(cfg.Jitsi.WebhookSecret)
		if err != nil {
			return err
		}
		stored := persistedPlatformJitsiRuntime{
			Version: 1, URL: strings.TrimSpace(cfg.Jitsi.URL), JitsiDomain: strings.TrimSpace(cfg.Jitsi.JitsiDomain),
			RequireAuth: cfg.Jitsi.RequireAuth, AppID: strings.TrimSpace(cfg.Jitsi.AppID),
			AppSecretEncrypted: appSecretEncrypted, WebhookSecretEncrypted: webhookSecretEncrypted,
			TokenTTLMinutes: cfg.Jitsi.TokenTTLMinutes, Timeout: strings.TrimSpace(cfg.Jitsi.Timeout),
			MaxRetries: cfg.Jitsi.MaxRetries, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.saveRuntimeConfig(platformJitsiRuntimeConfigKey, "Jitsi video runtime", stored, nil,
			"platform_integration.environment_synced", nil, sanitizedJitsiAuditState(cfg.Jitsi)); err != nil {
			return err
		}
	}
	if config.HasEnvironmentOverride("meetingAR") {
		apiKeyEncrypted, err := secretstore.Encrypt(meetingARConfigOption(cfg.MeetingAR, "apiKey"))
		if err != nil {
			return err
		}
		stored := persistedPlatformARRuntime{
			Version: 1, Provider: cfg.MeetingAR.ProviderName(), Endpoint: meetingARConfigOption(cfg.MeetingAR, "endpoint"),
			APIKeyEncrypted: apiKeyEncrypted, HealthPath: meetingARConfigOption(cfg.MeetingAR, "healthPath"),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.saveRuntimeConfig(platformARRuntimeConfigKey, "Meeting AR runtime", stored, nil,
			"platform_integration.environment_synced", nil, sanitizedARAuditState(cfg.MeetingAR)); err != nil {
			return err
		}
	}
	if config.HasEnvironmentOverride("email") {
		passwordEncrypted, err := secretstore.Encrypt(cfg.Email.Password)
		if err != nil {
			return err
		}
		stored := persistedPlatformSMTPRuntime{
			Version: 1, SMTPHost: strings.TrimSpace(cfg.Email.SMTPHost), SMTPPort: cfg.Email.SMTPPort,
			Username: strings.TrimSpace(cfg.Email.Username), PasswordEncrypted: passwordEncrypted,
			FromAddress: strings.TrimSpace(cfg.Email.FromAddress), FromName: strings.TrimSpace(cfg.Email.FromName),
			UseTLS: cfg.Email.UseTLS, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.saveRuntimeConfig(platformSMTPRuntimeConfigKey, "SMTP email runtime", stored, nil,
			"platform_integration.environment_synced", nil, sanitizedSMTPAuditState(cfg.Email)); err != nil {
			return err
		}
	}
	if config.HasEnvironmentOverride("sub2api") {
		if strings.TrimSpace(cfg.Sub2API.BaseURL) != "" {
			if err := saveEnvironmentSystemConfig(platformSub2APIHostConfigKey, strings.TrimSpace(cfg.Sub2API.BaseURL),
				"platform_ai", "Sub2API Host", "Environment-seeded platform Sub2API host"); err != nil {
				return err
			}
		}
		adminAPIKey := firstNonBlank(cfg.Sub2API.AdminAPIKey, cfg.Sub2API.DefaultKey)
		if adminAPIKey != "" {
			if err := PlatformAIModelService.saveAdminAPIKey(adminAPIKey, nil); err != nil {
				return err
			}
		} else if err := saveEnvironmentSystemConfig(platformSub2APIAdminKeyConfigKey, "",
			"platform_ai", "Sub2API Admin API Key", "Environment-cleared platform Sub2API admin API key"); err != nil {
			return err
		}
		if strings.TrimSpace(cfg.Sub2API.DefaultModel) != "" {
			if err := saveEnvironmentSystemConfig(platformSub2APILLMModelConfigKey, strings.TrimSpace(cfg.Sub2API.DefaultModel),
				"platform_ai", "Sub2API Default LLM Model", "Environment-seeded default chat model"); err != nil {
				return err
			}
		}
		if err := s.syncEnvironmentSub2APIAIConfigs(cfg); err != nil {
			return err
		}
	}
	if err := s.syncEnvironmentOpenAICompatibleAIConfigs(); err != nil {
		return err
	}
	return nil
}

func (s *platformIntegrationConfigService) syncEnvironmentSub2APIAIConfigs(cfg config.Config) error {
	baseURL := strings.TrimSpace(cfg.Sub2API.BaseURL)
	apiKey := strings.TrimSpace(cfg.Sub2API.DefaultKey)
	if baseURL == "" || apiKey == "" {
		return nil
	}

	if shouldSeedSub2APILLMConfig() {
		modelName := firstNonBlank(os.Getenv("SUB2API_LLM_MODEL"), cfg.Sub2API.DefaultModel)
		if modelName == "" {
			if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APILLMModelConfigKey); item != nil {
				modelName = strings.TrimSpace(item.ConfigValue)
			}
		}
		if modelName != "" {
			if err := syncEnvironmentSub2APIAIConfig(enums.AIModelTypeLLM, modelName, 0, baseURL, apiKey, cfg.Sub2API); err != nil {
				return err
			}
		}
	}

	return nil
}

func shouldSeedSub2APILLMConfig() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LLM_SOURCE"))) {
	case "", "auto", "sub2api":
		return true
	default:
		return false
	}
}

func shouldSeedOpenAICompatibleEmbeddingConfig() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("EMBEDDING_SOURCE"))) {
	case "", "auto", "aliyun":
		return true
	default:
		return false
	}
}

func resolveEnvironmentEmbeddingDimension() (int, bool, error) {
	value := firstNonBlank(os.Getenv("SUB2API_EMBEDDING_DIM"), os.Getenv("EMBEDDING_DIM"))
	if value == "" {
		return 0, false, nil
	}
	dimension, err := strconv.Atoi(value)
	if err != nil || dimension <= 0 {
		return 0, false, fmt.Errorf("environment embedding dimension must be a positive integer")
	}
	return dimension, true, nil
}

func (s *platformIntegrationConfigService) syncEnvironmentOpenAICompatibleAIConfigs() error {
	if !shouldSeedOpenAICompatibleEmbeddingConfig() {
		return nil
	}
	apiKey := firstNonBlank(os.Getenv("EMBEDDING_API_KEY"), os.Getenv("OPENAI_API_KEY"))
	baseURL := firstNonBlank(os.Getenv("EMBEDDING_BASE_URL"), os.Getenv("OPENAI_BASE_URL"))
	modelName := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	dimension, dimensionConfigured, err := resolveEnvironmentEmbeddingDimension()
	if err != nil {
		return err
	}
	source := strings.ToLower(strings.TrimSpace(os.Getenv("EMBEDDING_SOURCE")))
	if source != "aliyun" && apiKey == "" && baseURL == "" {
		return nil
	}
	missing := make([]string, 0, 4)
	if apiKey == "" {
		missing = append(missing, "EMBEDDING_API_KEY")
	}
	if baseURL == "" {
		missing = append(missing, "EMBEDDING_BASE_URL")
	}
	if modelName == "" {
		missing = append(missing, "EMBEDDING_MODEL")
	}
	if !dimensionConfigured {
		missing = append(missing, "EMBEDDING_DIM")
	}
	if len(missing) > 0 {
		return fmt.Errorf("environment embedding AI config is incomplete: missing %s", strings.Join(missing, ", "))
	}
	return syncEnvironmentOpenAICompatibleEmbeddingAIConfig(modelName, dimension, baseURL, apiKey)
}

func syncEnvironmentOpenAICompatibleEmbeddingAIConfig(modelName string, dimension int, baseURL string, apiKey string) error {
	db := sqls.DB()
	var items []models.AIConfig
	if err := db.Where("model_type = ? AND status <> ?", enums.AIModelTypeEmbedding, enums.StatusDeleted).
		Order("status ASC, sort_no DESC, id DESC").
		Find(&items).Error; err != nil {
		return err
	}

	var targetID int64
	for i := range items {
		if isReplaceableEnvironmentAIConfig(items[i]) {
			targetID = items[i].ID
			break
		}
	}
	if targetID == 0 && len(items) > 0 {
		return nil
	}

	timeout := 60 * time.Second
	now := time.Now()
	columns := map[string]any{
		"name":             environmentOpenAICompatibleAIConfigName(enums.AIModelTypeEmbedding),
		"provider":         enums.AIProviderOpenAI,
		"base_url":         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		"api_key":          apiKey,
		"model_type":       enums.AIModelTypeEmbedding,
		"model_name":       strings.TrimSpace(modelName),
		"dimension":        dimension,
		"timeout_ms":       int(timeout.Milliseconds()),
		"max_retry_count":  2,
		"status":           enums.StatusOk,
		"sort_no":          20000,
		"remark":           "Environment-seeded OpenAI-compatible embedding configuration.",
		"updated_at":       now,
		"update_user_id":   0,
		"update_user_name": "platform-integration-env",
	}
	if targetID == 0 {
		item := &models.AIConfig{
			Name:          environmentOpenAICompatibleAIConfigName(enums.AIModelTypeEmbedding),
			Provider:      enums.AIProviderOpenAI,
			BaseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
			APIKey:        apiKey,
			ModelType:     enums.AIModelTypeEmbedding,
			ModelName:     strings.TrimSpace(modelName),
			Dimension:     dimension,
			TimeoutMS:     int(timeout.Milliseconds()),
			MaxRetryCount: 2,
			Status:        enums.StatusOk,
			SortNo:        20000,
			Remark:        "Environment-seeded OpenAI-compatible embedding configuration.",
			AuditFields:   utils.BuildAuditFields(&dto.AuthPrincipal{Username: "platform-integration-env"}),
		}
		if err := repositories.AIConfigRepository.Create(db, item); err != nil {
			return err
		}
		targetID = item.ID
	} else if err := repositories.AIConfigRepository.Updates(db, targetID, columns); err != nil {
		return err
	}

	return db.Model(&models.AIConfig{}).
		Where("model_type = ? AND id <> ? AND status = ?", enums.AIModelTypeEmbedding, targetID, enums.StatusOk).
		Updates(map[string]any{
			"status":           enums.StatusDisabled,
			"updated_at":       now,
			"update_user_id":   0,
			"update_user_name": "platform-integration-env",
		}).Error
}

func syncEnvironmentSub2APIAIConfig(modelType enums.AIModelType, modelName string, dimension int, baseURL string, apiKey string, cfg config.Sub2APIConfig) error {
	db := sqls.DB()
	var items []models.AIConfig
	if err := db.Where("model_type = ? AND status <> ?", modelType, enums.StatusDeleted).
		Order("status ASC, sort_no DESC, id DESC").
		Find(&items).Error; err != nil {
		return err
	}

	var targetID int64
	for i := range items {
		if isReplaceableEnvironmentAIConfig(items[i]) {
			targetID = items[i].ID
			break
		}
	}
	if targetID == 0 {
		for i := range items {
			if items[i].Status == enums.StatusOk {
				targetID = items[i].ID
				break
			}
		}
	}
	if targetID == 0 && len(items) > 0 {
		targetID = items[0].ID
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}
	now := time.Now()
	columns := map[string]any{
		"name":             environmentSub2APIAIConfigName(modelType),
		"provider":         enums.AIProviderSub2API,
		"base_url":         openAICompatibleIntegrationBaseURL(baseURL),
		"api_key":          apiKey,
		"model_type":       modelType,
		"model_name":       strings.TrimSpace(modelName),
		"dimension":        dimension,
		"timeout_ms":       int(timeout.Milliseconds()),
		"max_retry_count":  maxRetries,
		"status":           enums.StatusOk,
		"sort_no":          20000,
		"remark":           "Environment-seeded Sub2API model configuration.",
		"updated_at":       now,
		"update_user_id":   0,
		"update_user_name": "platform-integration-env",
	}
	if targetID == 0 {
		item := &models.AIConfig{
			Name:          environmentSub2APIAIConfigName(modelType),
			Provider:      enums.AIProviderSub2API,
			BaseURL:       openAICompatibleIntegrationBaseURL(baseURL),
			APIKey:        apiKey,
			ModelType:     modelType,
			ModelName:     strings.TrimSpace(modelName),
			Dimension:     dimension,
			TimeoutMS:     int(timeout.Milliseconds()),
			MaxRetryCount: maxRetries,
			Status:        enums.StatusOk,
			SortNo:        20000,
			Remark:        "Environment-seeded Sub2API model configuration.",
			AuditFields:   utils.BuildAuditFields(&dto.AuthPrincipal{Username: "platform-integration-env"}),
		}
		if err := repositories.AIConfigRepository.Create(db, item); err != nil {
			return err
		}
		targetID = item.ID
	} else if err := repositories.AIConfigRepository.Updates(db, targetID, columns); err != nil {
		return err
	}

	return db.Model(&models.AIConfig{}).
		Where("model_type = ? AND id <> ? AND status = ?", modelType, targetID, enums.StatusOk).
		Updates(map[string]any{
			"status":           enums.StatusDisabled,
			"updated_at":       now,
			"update_user_id":   0,
			"update_user_name": "platform-integration-env",
		}).Error
}

func isReplaceableEnvironmentAIConfig(item models.AIConfig) bool {
	baseURL := strings.ToLower(strings.TrimRight(strings.TrimSpace(item.BaseURL), "/"))
	if baseURL == "" || strings.Contains(baseURL, "127.0.0.1:18099") || strings.Contains(baseURL, "localhost:18099") {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(item.Name))
	if strings.Contains(name, "environment sub2api") || strings.Contains(name, "environment openai-compatible") {
		return true
	}
	model := strings.ToLower(strings.TrimSpace(item.ModelName))
	return strings.Contains(name, "deterministic") || strings.HasPrefix(model, "e2e-")
}

func environmentSub2APIAIConfigName(modelType enums.AIModelType) string {
	switch modelType {
	case enums.AIModelTypeEmbedding:
		return "Environment Sub2API Embedding"
	default:
		return "Environment Sub2API LLM"
	}
}

func environmentOpenAICompatibleAIConfigName(modelType enums.AIModelType) string {
	switch modelType {
	case enums.AIModelTypeEmbedding:
		return "Environment OpenAI-compatible Embedding"
	default:
		return "Environment OpenAI-compatible LLM"
	}
}

func openAICompatibleIntegrationBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		return baseURL
	}
	return baseURL + "/v1"
}

func systemConfigNeedsEnvironmentSeed(key string) bool {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), key)
	if item == nil {
		return true
	}
	return item.Status != enums.StatusDeleted && strings.TrimSpace(item.ConfigValue) == ""
}

func saveEnvironmentSystemConfig(key, value, groupCode, title, description string) error {
	return repositories.SystemConfigRepository.SaveByKey(sqls.DB(), &models.SystemConfig{
		ConfigKey: key, ConfigValue: value, GroupCode: groupCode, Title: title,
		Description: description, Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(nil),
	})
}

func resolvePlatformSub2APITranslationKeyConfig() (string, string, *time.Time, error) {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APITranslationKeyConfigKey)
	if item == nil || item.Status == enums.StatusDeleted || strings.TrimSpace(item.ConfigValue) == "" {
		return "", "", nil, nil
	}
	apiKey, fingerprint, err := decryptPlatformSecretConfigValue(item.ConfigValue)
	if err != nil {
		return "", "", nil, err
	}
	if fingerprint == "" && apiKey != "" {
		fingerprint = secretstore.Fingerprint(apiKey)
	}
	var updatedAt *time.Time
	if !item.UpdatedAt.IsZero() {
		value := item.UpdatedAt
		updatedAt = &value
	}
	return apiKey, fingerprint, updatedAt, nil
}

func decryptPlatformSecretConfigValue(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", nil
	}
	payload := platformSecretValue{}
	if err := json.Unmarshal([]byte(value), &payload); err == nil && strings.TrimSpace(payload.Ciphertext) != "" {
		plain, err := secretstore.Decrypt(payload.Ciphertext)
		if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(plain), strings.TrimSpace(payload.Fingerprint), nil
	}
	plain, err := secretstore.Decrypt(value)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(plain), "", nil
}

func latestTimePtr(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || !right.After(*left) {
		return left
	}
	return right
}

func maskedIntegrationSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "********"
	}
	return "********" + value[len(value)-4:]
}

func (s *platformIntegrationConfigService) updateJitsi(input request.PlatformJitsiRuntimeUpdateRequest, operator *dto.AuthPrincipal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := config.CurrentOrDefault().Jitsi
	candidate, err := mergeJitsiRuntimeInput(previous, &input)
	if err != nil {
		return err
	}
	appSecretEncrypted, err := secretstore.Encrypt(candidate.AppSecret)
	if err != nil {
		return err
	}
	webhookSecretEncrypted, err := secretstore.Encrypt(candidate.WebhookSecret)
	if err != nil {
		return err
	}
	stored := persistedPlatformJitsiRuntime{
		Version: 1, URL: strings.TrimSpace(candidate.URL), JitsiDomain: strings.TrimSpace(candidate.JitsiDomain),
		RequireAuth: candidate.RequireAuth, AppID: strings.TrimSpace(candidate.AppID),
		AppSecretEncrypted: appSecretEncrypted, WebhookSecretEncrypted: webhookSecretEncrypted,
		TokenTTLMinutes: candidate.TokenTTLMinutes, Timeout: strings.TrimSpace(candidate.Timeout),
		MaxRetries: candidate.MaxRetries, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.saveRuntimeConfig(platformJitsiRuntimeConfigKey, "Jitsi video runtime", stored, operator,
		"jitsi_runtime.updated", sanitizedJitsiAuditState(previous), sanitizedJitsiAuditState(candidate)); err != nil {
		return err
	}
	providers.InitJitsi(&candidate)
	current := config.CurrentOrDefault()
	current.Jitsi = candidate
	config.SetCurrent(&current)
	return nil
}

func (s *platformIntegrationConfigService) updateAR(input request.PlatformARRuntimeUpdateRequest, operator *dto.AuthPrincipal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := config.CurrentOrDefault().MeetingAR
	candidate := mergeARRuntimeInput(previous, &input)
	if _, err := providers.NewMeetingARDetectionProvider(candidate); err != nil {
		return err
	}
	apiKeyEncrypted, err := secretstore.Encrypt(meetingARConfigOption(candidate, "apiKey"))
	if err != nil {
		return err
	}
	stored := persistedPlatformARRuntime{
		Version: 1, Provider: candidate.ProviderName(), Endpoint: meetingARConfigOption(candidate, "endpoint"),
		APIKeyEncrypted: apiKeyEncrypted, HealthPath: meetingARConfigOption(candidate, "healthPath"),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.saveRuntimeConfig(platformARRuntimeConfigKey, "Meeting AR runtime", stored, operator,
		"meeting_ar_runtime.updated", sanitizedARAuditState(previous), sanitizedARAuditState(candidate)); err != nil {
		return err
	}
	if err := providers.InitMeetingARDetection(candidate); err != nil {
		return err
	}
	current := config.CurrentOrDefault()
	current.MeetingAR = candidate
	config.SetCurrent(&current)
	return nil
}

func (s *platformIntegrationConfigService) updateSMTP(input request.PlatformSMTPRuntimeUpdateRequest, operator *dto.AuthPrincipal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := config.CurrentOrDefault().Email
	candidate, err := mergeSMTPRuntimeInput(previous, &input)
	if err != nil {
		return err
	}
	passwordEncrypted, err := secretstore.Encrypt(candidate.Password)
	if err != nil {
		return err
	}
	stored := persistedPlatformSMTPRuntime{
		Version: 1, SMTPHost: strings.TrimSpace(candidate.SMTPHost), SMTPPort: candidate.SMTPPort,
		Username: strings.TrimSpace(candidate.Username), PasswordEncrypted: passwordEncrypted,
		FromAddress: strings.TrimSpace(candidate.FromAddress), FromName: strings.TrimSpace(candidate.FromName),
		UseTLS: candidate.UseTLS, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.saveRuntimeConfig(platformSMTPRuntimeConfigKey, "SMTP email runtime", stored, operator,
		"smtp_runtime.updated", sanitizedSMTPAuditState(previous), sanitizedSMTPAuditState(candidate)); err != nil {
		return err
	}
	current := config.CurrentOrDefault()
	current.Email = candidate
	config.SetCurrent(&current)
	return nil
}

func (s *platformIntegrationConfigService) saveRuntimeConfig(key, title string, value any, operator *dto.AuthPrincipal, action string, before, after any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.SystemConfigRepository.SaveByKey(ctx.Tx, &models.SystemConfig{
			ConfigKey: key, ConfigValue: string(payload), GroupCode: "platform_integrations", Title: title,
			Description: "Encrypted platform integration runtime settings", Status: enums.StatusOk,
			AuditFields: utils.BuildAuditFields(operator),
		}); err != nil {
			return err
		}
		return PlatformIAMService.recordAuthAuditTx(ctx, operator, 0, models.DomainTypePlatform,
			"platform_integration", strings.TrimPrefix(key, "platform."), action, before, after, models.RiskLevelHigh, "")
	})
}

func (s *platformIntegrationConfigService) loadPersistedJitsi() error {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformJitsiRuntimeConfigKey)
	if item == nil || item.Status == enums.StatusDeleted || strings.TrimSpace(item.ConfigValue) == "" {
		return nil
	}
	var stored persistedPlatformJitsiRuntime
	if err := json.Unmarshal([]byte(item.ConfigValue), &stored); err != nil {
		return fmt.Errorf("decode persisted Jitsi runtime: %w", err)
	}
	appSecret, err := secretstore.Decrypt(stored.AppSecretEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted Jitsi app secret: %w", err)
	}
	webhookSecret, err := secretstore.Decrypt(stored.WebhookSecretEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted Jitsi webhook secret: %w", err)
	}
	candidate := config.JitsiConfig{
		URL: stored.URL, JitsiDomain: stored.JitsiDomain, RequireAuth: stored.RequireAuth,
		AppID: stored.AppID, AppSecret: appSecret, WebhookSecret: webhookSecret,
		TokenTTLMinutes: stored.TokenTTLMinutes, Timeout: stored.Timeout, MaxRetries: stored.MaxRetries,
	}
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("validate persisted Jitsi runtime: %w", err)
	}
	providers.InitJitsi(&candidate)
	current := config.CurrentOrDefault()
	current.Jitsi = candidate
	config.SetCurrent(&current)
	return nil
}

func (s *platformIntegrationConfigService) loadPersistedAR() error {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformARRuntimeConfigKey)
	if item == nil || item.Status == enums.StatusDeleted || strings.TrimSpace(item.ConfigValue) == "" {
		return nil
	}
	var stored persistedPlatformARRuntime
	if err := json.Unmarshal([]byte(item.ConfigValue), &stored); err != nil {
		return fmt.Errorf("decode persisted AR runtime: %w", err)
	}
	apiKey, err := secretstore.Decrypt(stored.APIKeyEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted AR API key: %w", err)
	}
	candidate := config.MeetingARConfig{Provider: stored.Provider, ProviderOptions: map[string]any{
		"endpoint": stored.Endpoint, "apiKey": apiKey, "healthPath": stored.HealthPath,
	}}
	if err := providers.InitMeetingARDetection(candidate); err != nil {
		return fmt.Errorf("initialize persisted AR runtime: %w", err)
	}
	current := config.CurrentOrDefault()
	current.MeetingAR = candidate
	config.SetCurrent(&current)
	return nil
}

func (s *platformIntegrationConfigService) loadPersistedSMTP() error {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSMTPRuntimeConfigKey)
	if item == nil || item.Status == enums.StatusDeleted || strings.TrimSpace(item.ConfigValue) == "" {
		return nil
	}
	var stored persistedPlatformSMTPRuntime
	if err := json.Unmarshal([]byte(item.ConfigValue), &stored); err != nil {
		return fmt.Errorf("decode persisted SMTP runtime: %w", err)
	}
	password, err := secretstore.Decrypt(stored.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted SMTP password: %w", err)
	}
	candidate := config.EmailConfig{
		SMTPHost: strings.TrimSpace(stored.SMTPHost), SMTPPort: stored.SMTPPort,
		Username: strings.TrimSpace(stored.Username), Password: password,
		FromAddress: strings.TrimSpace(stored.FromAddress), FromName: strings.TrimSpace(stored.FromName),
		UseTLS: stored.UseTLS,
	}
	if err := validateSMTPRuntimeConfig(candidate); err != nil {
		return fmt.Errorf("validate persisted SMTP runtime: %w", err)
	}
	current := config.CurrentOrDefault()
	current.Email = candidate
	config.SetCurrent(&current)
	return nil
}

func mergeJitsiRuntimeInput(previous config.JitsiConfig, input *request.PlatformJitsiRuntimeUpdateRequest) (config.JitsiConfig, error) {
	if input == nil {
		if err := previous.Validate(); err != nil {
			return config.JitsiConfig{}, err
		}
		return previous, nil
	}
	candidate := previous
	candidate.URL = strings.TrimSpace(input.URL)
	candidate.JitsiDomain = strings.TrimSpace(input.JitsiDomain)
	candidate.RequireAuth = input.RequireAuth
	candidate.AppID = strings.TrimSpace(input.AppID)
	if value := strings.TrimSpace(input.AppSecret); value != "" {
		candidate.AppSecret = value
	}
	if value := strings.TrimSpace(input.WebhookSecret); value != "" {
		candidate.WebhookSecret = value
	}
	candidate.TokenTTLMinutes = input.TokenTTLMinutes
	candidate.Timeout = strings.TrimSpace(input.Timeout)
	candidate.MaxRetries = input.MaxRetries
	if err := candidate.Validate(); err != nil {
		return config.JitsiConfig{}, err
	}
	return candidate, nil
}

func mergeARRuntimeInput(previous config.MeetingARConfig, input *request.PlatformARRuntimeUpdateRequest) config.MeetingARConfig {
	if input == nil {
		return previous
	}
	apiKey := meetingARConfigOption(previous, "apiKey")
	if value := strings.TrimSpace(input.APIKey); value != "" {
		apiKey = value
	}
	return config.MeetingARConfig{
		Provider: strings.ToLower(strings.TrimSpace(input.Provider)),
		ProviderOptions: map[string]any{
			"endpoint": strings.TrimSpace(input.Endpoint), "apiKey": apiKey,
			"healthPath": strings.TrimSpace(input.HealthPath),
		},
	}
}

func mergeSMTPRuntimeInput(previous config.EmailConfig, input *request.PlatformSMTPRuntimeUpdateRequest) (config.EmailConfig, error) {
	if input == nil {
		if err := validateSMTPRuntimeConfig(previous); err != nil {
			return config.EmailConfig{}, err
		}
		return previous, nil
	}
	candidate := previous
	host, port, err := normalizeSMTPHostPort(input.SMTPHost, input.SMTPPort, input.UseTLS)
	if err != nil {
		return config.EmailConfig{}, err
	}
	candidate.SMTPHost = host
	candidate.SMTPPort = port
	candidate.Username = strings.TrimSpace(input.Username)
	if value := strings.TrimSpace(input.Password); value != "" {
		candidate.Password = value
	}
	candidate.FromAddress = strings.TrimSpace(input.FromAddress)
	candidate.FromName = strings.TrimSpace(input.FromName)
	candidate.UseTLS = input.UseTLS
	if strings.TrimSpace(candidate.SMTPHost) == "" {
		candidate.Password = ""
	}
	if err := validateSMTPRuntimeConfig(candidate); err != nil {
		return config.EmailConfig{}, err
	}
	return candidate, nil
}

func normalizeSMTPHostPort(host string, port int, useTLS bool) (string, int, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", 0, nil
	}
	if strings.Contains(host, "://") {
		return "", 0, errors.New("SMTP host should not include protocol")
	}
	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		host = strings.TrimSpace(parsedHost)
		if port <= 0 {
			value, convErr := strconv.Atoi(parsedPort)
			if convErr != nil {
				return "", 0, errors.New("invalid SMTP port")
			}
			port = value
		}
	}
	if port <= 0 {
		port = defaultSMTPPort(useTLS)
	}
	return host, port, nil
}

func validateSMTPRuntimeConfig(cfg config.EmailConfig) error {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return nil
	}
	if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
		return errors.New("invalid SMTP port")
	}
	if strings.TrimSpace(cfg.FromAddress) == "" {
		return errors.New("SMTP sender address is required")
	}
	if strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) == "" {
		return errors.New("SMTP password is required when username is set")
	}
	return nil
}

func defaultSMTPPort(useTLS bool) int {
	if useTLS {
		return 465
	}
	return 25
}

func testSpeechRuntime(ctx context.Context, input *request.SpeechRuntimeUpdateRequest) error {
	if input == nil {
		return errors.New("speech settings are required")
	}
	candidate, err := mergeSpeechRuntimeInput(providers.CurrentSpeechConfig(), *input)
	if err != nil {
		return err
	}
	provider, err := providers.NewSpeechTranscriptionProvider(&candidate)
	if err != nil {
		return err
	}
	if provider == nil || !provider.Configured() {
		return errors.New("实时语音转写尚未配置")
	}
	stream, err := provider.Open(ctx, providers.SpeechTranscriptionOptions{Language: "zh-CN", AudioFormat: providers.DefaultSpeechAudioFormat()})
	if err == nil {
		err = stream.Close(ctx)
	}
	readiness := buildSpeechProviderReadiness(err)
	if !readiness.Ready {
		return errors.New(defaultString(readiness.Message, "实时语音转写连接失败"))
	}
	return nil
}

func testTranslationRuntime(ctx context.Context, input *request.SpeechRuntimeUpdateRequest) (string, error) {
	if input == nil {
		return "", errors.New("translation settings are required")
	}
	candidate, err := mergeSpeechRuntimeInput(providers.CurrentSpeechConfig(), *input)
	if err != nil {
		return "", err
	}
	if !candidate.Xfyun.TranslationEnabled {
		return "", errors.New("实时翻译尚未启用")
	}
	provider := providers.NewXfyunTextTranslationProvider(candidate.Xfyun)
	result, err := provider.Translate(ctx, providers.TextTranslationRequest{
		Text: "连接测试", SourceLanguage: "cn", TargetLanguage: candidate.Xfyun.TranslationTargetLanguage,
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func testARRuntime(ctx context.Context, input *request.PlatformARRuntimeUpdateRequest) error {
	candidate := mergeARRuntimeInput(config.CurrentOrDefault().MeetingAR, input)
	provider, err := providers.NewMeetingARDetectionProvider(candidate)
	if err != nil {
		return err
	}
	checker, ok := provider.(providers.MeetingARDetectionHealthChecker)
	if !ok {
		return errors.New("当前 AR Provider 不支持连接测试")
	}
	return checker.CheckHealth(ctx)
}

func testSub2APIRuntime(ctx context.Context, input *request.PlatformSub2APIRuntimeUpdateRequest) error {
	host, adminKey, _, _, err := PlatformAIModelService.resolveProviderConfig()
	if err != nil {
		return err
	}
	if input != nil {
		host = strings.TrimSpace(input.Host)
		if value := strings.TrimSpace(input.AdminAPIKey); value != "" {
			adminKey = value
		}
	}
	if host == "" {
		return errors.New("Sub2API 服务地址未配置")
	}
	if adminKey == "" {
		return errors.New("Sub2API 管理凭据未配置")
	}
	provider := PlatformAIModelService.providerFromConfig(host, adminKey)
	if err := provider.HealthCheck(ctx); err != nil {
		return err
	}
	_, err = provider.AdminListUsers(ctx, adminKey, providers.Sub2APIAdminListUsersQuery{
		Page: 1, PageSize: 1, SortBy: "created_at", SortOrder: "desc", Timezone: "Asia/Shanghai",
	})
	return err
}

func testSMTPRuntime(ctx context.Context, input *request.PlatformSMTPRuntimeUpdateRequest) error {
	candidate, err := mergeSMTPRuntimeInput(config.CurrentOrDefault().Email, input)
	if err != nil {
		return err
	}
	if strings.TrimSpace(candidate.SMTPHost) == "" {
		return errors.New("SMTP Server 未配置")
	}
	return probeSMTPRuntime(ctx, candidate)
}

func probeSMTPRuntime(ctx context.Context, cfg config.EmailConfig) error {
	host := strings.TrimSpace(cfg.SMTPHost)
	addr := net.JoinHostPort(host, strconv.Itoa(cfg.SMTPPort))
	dialer := &net.Dialer{Timeout: integrationProbeTimeout}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	}
	var (
		conn net.Conn
		err  error
	)
	if cfg.UseTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: host, MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("SMTP connect failed: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("SMTP handshake failed: %w", err)
	}
	defer client.Close()

	if !cfg.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("SMTP STARTTLS failed: %w", err)
			}
		}
	}
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	if username != "" {
		if password == "" {
			return errors.New("SMTP password is required when username is set")
		}
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}
	if err := client.Noop(); err != nil {
		return fmt.Errorf("SMTP NOOP failed: %w", err)
	}
	_ = client.Quit()
	return nil
}

func (s *platformIntegrationConfigService) recordUpdateAudit(operator *dto.AuthPrincipal, integration string, before, after any) {
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return PlatformIAMService.recordAuthAuditTx(ctx, operator, 0, models.DomainTypePlatform,
			"platform_integration", integration, "platform_integration.updated", before, after, models.RiskLevelHigh, "")
	})
	if err != nil {
		slog.Warn("record platform integration update audit failed", "integration", integration, "error", err)
	}
}

func (s *platformIntegrationConfigService) recordTestAudit(operator *dto.AuthPrincipal, result *PlatformIntegrationTestAggregate) {
	if operator == nil || result == nil {
		return
	}
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return PlatformIAMService.recordAuthAuditTx(ctx, operator, 0, models.DomainTypePlatform,
			"platform_integration", result.Integration, "platform_integration.connection_tested", nil,
			map[string]any{"success": result.Success, "message": result.Message, "latencyMs": result.Latency.Milliseconds()},
			models.RiskLevelMedium, "")
	})
	if err != nil {
		slog.Warn("record platform integration test audit failed", "integration", result.Integration, "error", err)
	}
}

func canUpdatePlatformIntegration(operator *dto.AuthPrincipal) bool {
	return operator != nil && operator.IsPlatform() && operator.EffectiveTenantID() == 0
}

func normalizePlatformIntegration(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "email" {
		return "smtp"
	}
	return value
}

func systemConfigUpdatedAt(key string) *time.Time {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), key)
	if item == nil || item.UpdatedAt.IsZero() {
		return nil
	}
	value := item.UpdatedAt
	return &value
}

func meetingARConfigOption(cfg config.MeetingARConfig, key string) string {
	if cfg.ProviderOptions == nil {
		return ""
	}
	value, ok := cfg.ProviderOptions[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func effectiveJitsiTTL(cfg config.JitsiConfig) int {
	if cfg.TokenTTLMinutes > 0 {
		return cfg.TokenTTLMinutes
	}
	return int(cfg.TokenExpiry() / time.Minute)
}

func effectiveJitsiTimeout(cfg config.JitsiConfig) string {
	if value := strings.TrimSpace(cfg.Timeout); value != "" {
		return value
	}
	return "10s"
}

func effectiveJitsiRetries(cfg config.JitsiConfig) int {
	if cfg.MaxRetries > 0 {
		return cfg.MaxRetries
	}
	return 3
}

func sanitizedJitsiAuditState(cfg config.JitsiConfig) map[string]any {
	return map[string]any{
		"url": strings.TrimSpace(cfg.URL), "jitsiDomain": strings.TrimSpace(cfg.JitsiDomain),
		"requireAuth": cfg.RequireAuth, "appId": strings.TrimSpace(cfg.AppID),
		"appSecretConfigured":     strings.TrimSpace(cfg.AppSecret) != "",
		"webhookSecretConfigured": strings.TrimSpace(cfg.WebhookSecret) != "",
		"tokenTtlMinutes":         effectiveJitsiTTL(cfg), "timeout": effectiveJitsiTimeout(cfg),
		"maxRetries": effectiveJitsiRetries(cfg),
	}
}

func sanitizedARAuditState(cfg config.MeetingARConfig) map[string]any {
	return map[string]any{
		"provider": cfg.ProviderName(), "endpoint": meetingARConfigOption(cfg, "endpoint"),
		"apiKeyConfigured": meetingARConfigOption(cfg, "apiKey") != "",
		"healthPath":       meetingARConfigOption(cfg, "healthPath"),
	}
}

func sanitizedSub2APIAuditState(settings *PlatformIntegrationSettingsAggregate) any {
	if settings == nil {
		return nil
	}
	return map[string]any{
		"host": settings.Sub2API.Host, "adminApiKeyConfigured": settings.Sub2API.AdminAPIKeyConfigured,
		"adminApiKeyFingerprint":       settings.Sub2API.AdminAPIKeyFingerprint,
		"translationApiKeyConfigured":  settings.Sub2API.TranslationAPIKeyConfigured,
		"translationApiKeyFingerprint": settings.Sub2API.TranslationAPIKeyFingerprint,
		"defaultLlmModel":              settings.Sub2API.DefaultLLMModel,
	}
}

func smtpRuntimeConfigured(cfg config.EmailConfig) bool {
	if strings.TrimSpace(cfg.SMTPHost) == "" || cfg.SMTPPort <= 0 || strings.TrimSpace(cfg.FromAddress) == "" {
		return false
	}
	if strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) == "" {
		return false
	}
	return true
}

func sanitizedSMTPAuditState(cfg config.EmailConfig) map[string]any {
	return map[string]any{
		"smtpHost":           strings.TrimSpace(cfg.SMTPHost),
		"smtpPort":           cfg.SMTPPort,
		"username":           strings.TrimSpace(cfg.Username),
		"passwordConfigured": strings.TrimSpace(cfg.Password) != "",
		"fromAddress":        strings.TrimSpace(cfg.FromAddress),
		"fromName":           strings.TrimSpace(cfg.FromName),
		"useTls":             cfg.UseTLS,
	}
}
