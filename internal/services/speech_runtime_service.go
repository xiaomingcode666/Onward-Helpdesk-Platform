package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var SpeechRuntimeService = &speechRuntimeService{}

type speechRuntimeService struct {
	mu          sync.Mutex
	readinessMu sync.Mutex
	readiness   cachedSpeechProviderReadiness
}

const (
	speechProviderProbeTimeout = 5 * time.Second
	speechReadinessCacheTTL    = 30 * time.Second
	speechReadinessSuccessTTL  = 5 * time.Minute
	speechRuntimeConfigKey     = "platform.speech_runtime.v1"
)

type persistedSpeechRuntimeConfig struct {
	Version                            int    `json:"version"`
	Provider                           string `json:"provider"`
	MockSegmentDurationMS              int    `json:"mockSegmentDurationMs,omitempty"`
	XfyunAppID                         string `json:"xfyunAppId,omitempty"`
	XfyunAPIKeyEncrypted               string `json:"xfyunApiKeyEncrypted,omitempty"`
	XfyunEndpoint                      string `json:"xfyunEndpoint,omitempty"`
	XfyunDomain                        string `json:"xfyunDomain,omitempty"`
	XfyunTranslationEnabled            bool   `json:"xfyunTranslationEnabled,omitempty"`
	XfyunTranslationAPISecretEncrypted string `json:"xfyunTranslationApiSecretEncrypted,omitempty"`
	XfyunTranslationEndpoint           string `json:"xfyunTranslationEndpoint,omitempty"`
	XfyunTranslationTargetLanguage     string `json:"xfyunTranslationTargetLanguage,omitempty"`
	AliyunAPIKeyEncrypted              string `json:"aliyunApiKeyEncrypted,omitempty"`
	AliyunWorkspaceID                  string `json:"aliyunWorkspaceId,omitempty"`
	AliyunEndpoint                     string `json:"aliyunEndpoint,omitempty"`
	AliyunModel                        string `json:"aliyunModel,omitempty"`
	JigasiEnabled                      bool   `json:"jigasiEnabled,omitempty"`
	JigasiSharedSecretEncrypted        string `json:"jigasiSharedSecretEncrypted,omitempty"`
	JigasiMaxParticipantStreams        int    `json:"jigasiMaxParticipantStreams,omitempty"`
	JigasiMaxConcurrentStreams         int    `json:"jigasiMaxConcurrentStreams,omitempty"`
	UpdatedAt                          string `json:"updatedAt"`
}

type speechProviderReadiness struct {
	Ready   bool
	Code    string
	Message string
}

type cachedSpeechProviderReadiness struct {
	key       string
	checkedAt time.Time
	result    speechProviderReadiness
}

func (s *speechRuntimeService) Status() response.SpeechRuntimeResponse {
	speechConfig, provider := providers.CurrentSpeechRuntime()
	return response.SpeechRuntimeResponse{
		Provider:                   speechConfig.ProviderName(),
		Configured:                 provider != nil && provider.Configured(),
		Enabled:                    speechConfig.TranscriptionEnabled(),
		XfyunCredentialsConfigured: strings.TrimSpace(speechConfig.Xfyun.AppID) != "" && strings.TrimSpace(speechConfig.Xfyun.APIKey) != "",
		XfyunEndpoint:              strings.TrimSpace(speechConfig.Xfyun.Endpoint),
		XfyunDomain:                strings.TrimSpace(speechConfig.Xfyun.Domain),
		XfyunTranslationEnabled:    speechConfig.Xfyun.TranslationEnabled,
		XfyunTranslationConfigured: strings.TrimSpace(speechConfig.Xfyun.AppID) != "" &&
			strings.TrimSpace(speechConfig.Xfyun.APIKey) != "" && strings.TrimSpace(speechConfig.Xfyun.TranslationAPISecret) != "",
		XfyunTranslationEndpoint:       strings.TrimSpace(speechConfig.Xfyun.TranslationEndpoint),
		XfyunTranslationTargetLanguage: strings.TrimSpace(speechConfig.Xfyun.TranslationTargetLanguage),
		AliyunAPIKeyConfigured:         strings.TrimSpace(speechConfig.Aliyun.APIKey) != "",
		AliyunWorkspaceID:              strings.TrimSpace(speechConfig.Aliyun.WorkspaceID),
		AliyunEndpoint:                 strings.TrimSpace(speechConfig.Aliyun.Endpoint),
		AliyunModel:                    strings.TrimSpace(speechConfig.Aliyun.Model),
		MockSegmentDurationMS:          int(speechConfig.Mock.SegmentDuration().Milliseconds()),
		JigasiEnabled:                  speechConfig.Jigasi.Enabled,
		JigasiSharedSecretConfigured:   strings.TrimSpace(speechConfig.Jigasi.SharedSecret) != "",
		JigasiMaxParticipantStreams:    speechConfig.Jigasi.MaxStreams(),
		JigasiMaxConcurrentStreams:     speechConfig.Jigasi.MaxConcurrent(),
	}
}

func (s *speechRuntimeService) StatusForPrincipal(operator *dto.AuthPrincipal) response.SpeechRuntimeResponse {
	status := s.Status()
	status.CanUpdate = operator != nil && operator.IsPlatform() && operator.EffectiveTenantID() == 0
	return status
}

func (s *speechRuntimeService) Readiness(ctx context.Context) speechProviderReadiness {
	speechConfig, provider := providers.CurrentSpeechRuntime()
	if !speechConfig.TranscriptionEnabled() || provider == nil || !provider.Configured() {
		return speechProviderReadiness{
			Code:    "not_configured",
			Message: "实时字幕服务尚未完成配置",
		}
	}

	key := speechReadinessKey(speechConfig)
	s.readinessMu.Lock()
	defer s.readinessMu.Unlock()
	if s.readiness.key == key {
		cacheTTL := speechReadinessCacheTTL
		if s.readiness.result.Ready {
			cacheTTL = speechReadinessSuccessTTL
		}
		if time.Since(s.readiness.checkedAt) < cacheTTL {
			return s.readiness.result
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, speechProviderProbeTimeout)
	defer cancel()
	stream, err := provider.Open(probeCtx, providers.SpeechTranscriptionOptions{
		Language:    "zh-CN",
		AudioFormat: providers.DefaultSpeechAudioFormat(),
	})
	if err == nil {
		err = stream.Close(probeCtx)
	}
	result := buildSpeechProviderReadiness(err)
	s.readiness = cachedSpeechProviderReadiness{
		key:       key,
		checkedAt: time.Now(),
		result:    result,
	}
	return result
}

func speechReadinessKey(speechConfig config.SpeechConfig) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		speechConfig.ProviderName(),
		strings.TrimSpace(speechConfig.Xfyun.AppID),
		strings.TrimSpace(speechConfig.Xfyun.APIKey),
		strings.TrimSpace(speechConfig.Xfyun.Endpoint),
		strings.TrimSpace(speechConfig.Xfyun.Domain),
		strings.TrimSpace(speechConfig.Aliyun.APIKey),
		strings.TrimSpace(speechConfig.Aliyun.WorkspaceID),
		strings.TrimSpace(speechConfig.Aliyun.Endpoint),
		strings.TrimSpace(speechConfig.Aliyun.Model),
	}, "\x00")))
	return fmt.Sprintf("%x", digest[:])
}

func buildSpeechProviderReadiness(err error) speechProviderReadiness {
	if err == nil {
		return speechProviderReadiness{Ready: true}
	}
	var xfyunErr *providers.XfyunRTASRError
	if errors.As(err, &xfyunErr) {
		switch xfyunErr.Code {
		case "10105":
			return speechProviderReadiness{
				Code:    xfyunErr.Code,
				Message: "讯飞实时语音转写鉴权失败（10105）：请检查同一应用下的 APPID、APIKey、服务授权和 IP 白名单",
			}
		case "10110":
			return speechProviderReadiness{
				Code:    xfyunErr.Code,
				Message: "讯飞实时语音转写不可用（10110）：请检查授权有效期、剩余时长和并发路数",
			}
		case "10800":
			return speechProviderReadiness{
				Code:    xfyunErr.Code,
				Message: "讯飞实时语音转写并发路数已满（10800）",
			}
		default:
			return speechProviderReadiness{
				Code:    xfyunErr.Code,
				Message: fmt.Sprintf("讯飞实时语音转写连接失败（%s）", xfyunErr.Code),
			}
		}
	}
	var aliyunErr *providers.AliyunASRError
	if errors.As(err, &aliyunErr) {
		if aliyunErr.Code != "" {
			return speechProviderReadiness{
				Code:    aliyunErr.Code,
				Message: fmt.Sprintf("阿里云实时语音转写连接失败（%s）", aliyunErr.Code),
			}
		}
		return speechProviderReadiness{Code: "aliyun_error", Message: "阿里云实时语音转写连接失败，请检查 API Key、模型和服务地址"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return speechProviderReadiness{Code: "timeout", Message: "实时字幕服务连接超时"}
	}
	if errors.Is(err, providers.ErrSpeechStreamLimitExceeded) {
		return speechProviderReadiness{Code: "stream_limit", Message: "实时字幕并发路数已满"}
	}
	return speechProviderReadiness{Code: "unavailable", Message: "实时字幕服务当前不可用"}
}

func (s *speechRuntimeService) Update(input request.SpeechRuntimeUpdateRequest, operator *dto.AuthPrincipal) (response.SpeechRuntimeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if operator == nil || !operator.IsPlatform() || operator.EffectiveTenantID() != 0 {
		return response.SpeechRuntimeResponse{}, errorsx.Forbidden("only platform operators can update the shared speech runtime")
	}

	previous := providers.CurrentSpeechConfig()
	candidate, err := mergeSpeechRuntimeInput(previous, input)
	if err != nil {
		return response.SpeechRuntimeResponse{}, err
	}
	if err := providers.InitSpeech(&candidate); err != nil {
		return response.SpeechRuntimeResponse{}, err
	}
	if err := s.persistAndAudit(candidate, previous, operator); err != nil {
		if rollbackErr := providers.InitSpeech(&previous); rollbackErr != nil {
			slog.Error("rollback speech runtime after persistence failure failed", "error", rollbackErr)
		}
		return response.SpeechRuntimeResponse{}, err
	}
	currentConfig := config.CurrentOrDefault()
	currentConfig.Speech = candidate
	config.SetCurrent(&currentConfig)
	s.InvalidateReadiness()
	return s.StatusForPrincipal(operator), nil
}

func mergeSpeechRuntimeInput(previous config.SpeechConfig, input request.SpeechRuntimeUpdateRequest) (config.SpeechConfig, error) {
	candidate := previous
	candidate.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if candidate.Provider == "" {
		return config.SpeechConfig{}, errorsx.InvalidParam("speech provider is required")
	}
	if value := strings.TrimSpace(input.XfyunAppID); value != "" {
		candidate.Xfyun.AppID = value
	}
	if value := strings.TrimSpace(input.XfyunAPIKey); value != "" {
		candidate.Xfyun.APIKey = value
	}
	if input.XfyunEndpoint != nil {
		candidate.Xfyun.Endpoint = strings.TrimSpace(*input.XfyunEndpoint)
	}
	if input.XfyunDomain != nil {
		candidate.Xfyun.Domain = strings.TrimSpace(*input.XfyunDomain)
	}
	if input.XfyunTranslationEnabled != nil {
		candidate.Xfyun.TranslationEnabled = *input.XfyunTranslationEnabled
	}
	if value := strings.TrimSpace(input.XfyunTranslationAPISecret); value != "" {
		candidate.Xfyun.TranslationAPISecret = value
	}
	if input.XfyunTranslationEndpoint != nil {
		candidate.Xfyun.TranslationEndpoint = strings.TrimSpace(*input.XfyunTranslationEndpoint)
	}
	if input.XfyunTranslationTargetLanguage != nil {
		candidate.Xfyun.TranslationTargetLanguage = strings.TrimSpace(*input.XfyunTranslationTargetLanguage)
	}
	if value := strings.TrimSpace(input.AliyunAPIKey); value != "" {
		candidate.Aliyun.APIKey = value
	}
	if input.AliyunWorkspaceID != nil {
		candidate.Aliyun.WorkspaceID = strings.TrimSpace(*input.AliyunWorkspaceID)
	}
	if input.AliyunEndpoint != nil {
		candidate.Aliyun.Endpoint = strings.TrimSpace(*input.AliyunEndpoint)
	}
	if input.AliyunModel != nil {
		candidate.Aliyun.Model = strings.TrimSpace(*input.AliyunModel)
	}
	if input.MockSegmentDurationMS > 0 {
		candidate.Mock.SegmentDurationMS = input.MockSegmentDurationMS
	}
	if input.JigasiEnabled != nil {
		candidate.Jigasi.Enabled = *input.JigasiEnabled
	}
	if value := strings.TrimSpace(input.JigasiSharedSecret); value != "" {
		candidate.Jigasi.SharedSecret = value
	}
	if input.JigasiMaxParticipantStreams != nil {
		candidate.Jigasi.MaxParticipantStreams = *input.JigasiMaxParticipantStreams
	}
	if input.JigasiMaxConcurrentStreams != nil {
		candidate.Jigasi.MaxConcurrentStreams = *input.JigasiMaxConcurrentStreams
	}
	return candidate, nil
}

func (s *speechRuntimeService) InvalidateReadiness() {
	s.readinessMu.Lock()
	s.readiness = cachedSpeechProviderReadiness{}
	s.readinessMu.Unlock()
}

func (s *speechRuntimeService) LoadPersisted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), speechRuntimeConfigKey)
	if config.HasEnvironmentOverride("speech") {
		environmentConfig := providers.CurrentSpeechConfig()
		if err := s.persistAndAudit(environmentConfig, config.SpeechConfig{}, nil); err != nil {
			return fmt.Errorf("seed speech runtime from environment: %w", err)
		}
		item = repositories.SystemConfigRepository.FindByKey(sqls.DB(), speechRuntimeConfigKey)
	}
	if item == nil || strings.TrimSpace(item.ConfigValue) == "" || item.Status == enums.StatusDeleted {
		return nil
	}
	var stored persistedSpeechRuntimeConfig
	if err := json.Unmarshal([]byte(item.ConfigValue), &stored); err != nil {
		return fmt.Errorf("decode persisted speech runtime: %w", err)
	}
	apiKey, err := secretstore.Decrypt(stored.XfyunAPIKeyEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted xfyun API key: %w", err)
	}
	translationSecret, err := secretstore.Decrypt(stored.XfyunTranslationAPISecretEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted xfyun translation secret: %w", err)
	}
	jigasiSecret, err := secretstore.Decrypt(stored.JigasiSharedSecretEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted jigasi shared secret: %w", err)
	}
	aliyunAPIKey, err := secretstore.Decrypt(stored.AliyunAPIKeyEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt persisted aliyun api key: %w", err)
	}
	candidate := providers.CurrentSpeechConfig()
	candidate.Provider = strings.TrimSpace(stored.Provider)
	candidate.Mock.SegmentDurationMS = stored.MockSegmentDurationMS
	candidate.Xfyun.AppID = strings.TrimSpace(stored.XfyunAppID)
	candidate.Xfyun.APIKey = strings.TrimSpace(apiKey)
	candidate.Xfyun.Endpoint = strings.TrimSpace(stored.XfyunEndpoint)
	candidate.Xfyun.Domain = strings.TrimSpace(stored.XfyunDomain)
	candidate.Xfyun.TranslationEnabled = stored.XfyunTranslationEnabled
	candidate.Xfyun.TranslationAPISecret = strings.TrimSpace(translationSecret)
	candidate.Xfyun.TranslationEndpoint = strings.TrimSpace(stored.XfyunTranslationEndpoint)
	candidate.Xfyun.TranslationTargetLanguage = strings.TrimSpace(stored.XfyunTranslationTargetLanguage)
	candidate.Aliyun.APIKey = strings.TrimSpace(aliyunAPIKey)
	candidate.Aliyun.WorkspaceID = strings.TrimSpace(stored.AliyunWorkspaceID)
	candidate.Aliyun.Endpoint = strings.TrimSpace(stored.AliyunEndpoint)
	candidate.Aliyun.Model = strings.TrimSpace(stored.AliyunModel)
	if stored.Version >= 2 {
		candidate.Jigasi.Enabled = stored.JigasiEnabled
	}
	if strings.TrimSpace(jigasiSecret) != "" {
		candidate.Jigasi.SharedSecret = strings.TrimSpace(jigasiSecret)
	}
	if stored.JigasiMaxParticipantStreams > 0 {
		candidate.Jigasi.MaxParticipantStreams = stored.JigasiMaxParticipantStreams
	}
	if stored.JigasiMaxConcurrentStreams > 0 {
		candidate.Jigasi.MaxConcurrentStreams = stored.JigasiMaxConcurrentStreams
	}
	if err := providers.InitSpeech(&candidate); err != nil {
		return fmt.Errorf("initialize persisted speech runtime: %w", err)
	}
	currentConfig := config.CurrentOrDefault()
	currentConfig.Speech = candidate
	config.SetCurrent(&currentConfig)
	s.InvalidateReadiness()
	return nil
}

func (s *speechRuntimeService) persistAndAudit(candidate, previous config.SpeechConfig, operator *dto.AuthPrincipal) error {
	apiKeyEncrypted, err := secretstore.Encrypt(candidate.Xfyun.APIKey)
	if err != nil {
		return err
	}
	translationSecretEncrypted, err := secretstore.Encrypt(candidate.Xfyun.TranslationAPISecret)
	if err != nil {
		return err
	}
	aliyunAPIKeyEncrypted, err := secretstore.Encrypt(candidate.Aliyun.APIKey)
	if err != nil {
		return err
	}
	jigasiSecretEncrypted, err := secretstore.Encrypt(candidate.Jigasi.SharedSecret)
	if err != nil {
		return err
	}
	stored := persistedSpeechRuntimeConfig{
		Version:                            3,
		Provider:                           candidate.ProviderName(),
		MockSegmentDurationMS:              int(candidate.Mock.SegmentDuration().Milliseconds()),
		XfyunAppID:                         strings.TrimSpace(candidate.Xfyun.AppID),
		XfyunAPIKeyEncrypted:               apiKeyEncrypted,
		XfyunEndpoint:                      strings.TrimSpace(candidate.Xfyun.Endpoint),
		XfyunDomain:                        strings.TrimSpace(candidate.Xfyun.Domain),
		XfyunTranslationEnabled:            candidate.Xfyun.TranslationEnabled,
		XfyunTranslationAPISecretEncrypted: translationSecretEncrypted,
		XfyunTranslationEndpoint:           strings.TrimSpace(candidate.Xfyun.TranslationEndpoint),
		XfyunTranslationTargetLanguage:     strings.TrimSpace(candidate.Xfyun.TranslationTargetLanguage),
		AliyunAPIKeyEncrypted:              aliyunAPIKeyEncrypted,
		AliyunWorkspaceID:                  strings.TrimSpace(candidate.Aliyun.WorkspaceID),
		AliyunEndpoint:                     strings.TrimSpace(candidate.Aliyun.Endpoint),
		AliyunModel:                        strings.TrimSpace(candidate.Aliyun.Model),
		JigasiEnabled:                      candidate.Jigasi.Enabled,
		JigasiSharedSecretEncrypted:        jigasiSecretEncrypted,
		JigasiMaxParticipantStreams:        candidate.Jigasi.MaxStreams(),
		JigasiMaxConcurrentStreams:         candidate.Jigasi.MaxConcurrent(),
		UpdatedAt:                          time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	item := &models.SystemConfig{
		ConfigKey:   speechRuntimeConfigKey,
		ConfigValue: string(payload),
		GroupCode:   "platform_integrations",
		Title:       "Shared speech runtime",
		Description: speechRuntimeCredentialFingerprint(candidate),
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.SystemConfigRepository.SaveByKey(ctx.Tx, item); err != nil {
			return err
		}
		return PlatformIAMService.recordAuthAuditTx(
			ctx,
			operator,
			0,
			models.DomainTypePlatform,
			"speech_runtime",
			"shared",
			"speech_runtime.updated",
			speechRuntimeAuditSnapshot(previous),
			speechRuntimeAuditSnapshot(candidate),
			models.RiskLevelHigh,
			"",
		)
	})
}

func speechRuntimeAuditSnapshot(cfg config.SpeechConfig) map[string]any {
	return map[string]any{
		"provider":                       cfg.ProviderName(),
		"mockSegmentDurationMs":          int(cfg.Mock.SegmentDuration().Milliseconds()),
		"xfyunCredentialsConfigured":     strings.TrimSpace(cfg.Xfyun.AppID) != "" && strings.TrimSpace(cfg.Xfyun.APIKey) != "",
		"xfyunCredentialFingerprint":     speechRuntimeCredentialFingerprint(cfg),
		"xfyunEndpoint":                  strings.TrimSpace(cfg.Xfyun.Endpoint),
		"xfyunDomain":                    strings.TrimSpace(cfg.Xfyun.Domain),
		"xfyunTranslationEnabled":        cfg.Xfyun.TranslationEnabled,
		"xfyunTranslationConfigured":     strings.TrimSpace(cfg.Xfyun.TranslationAPISecret) != "",
		"xfyunTranslationEndpoint":       strings.TrimSpace(cfg.Xfyun.TranslationEndpoint),
		"xfyunTranslationTargetLanguage": strings.TrimSpace(cfg.Xfyun.TranslationTargetLanguage),
		"aliyunApiKeyConfigured":         strings.TrimSpace(cfg.Aliyun.APIKey) != "",
		"aliyunWorkspaceId":              strings.TrimSpace(cfg.Aliyun.WorkspaceID),
		"aliyunEndpoint":                 strings.TrimSpace(cfg.Aliyun.Endpoint),
		"aliyunModel":                    strings.TrimSpace(cfg.Aliyun.Model),
		"jigasiEnabled":                  cfg.Jigasi.Enabled,
		"jigasiSharedSecretConfigured":   strings.TrimSpace(cfg.Jigasi.SharedSecret) != "",
		"jigasiMaxParticipantStreams":    cfg.Jigasi.MaxStreams(),
		"jigasiMaxConcurrentStreams":     cfg.Jigasi.MaxConcurrent(),
	}
}

func speechRuntimeCredentialFingerprint(cfg config.SpeechConfig) string {
	values := make([]string, 0, 3)
	if value := strings.TrimSpace(cfg.Xfyun.APIKey); value != "" {
		values = append(values, "rtasr:"+secretstore.Fingerprint(value)[:12])
	}
	if value := strings.TrimSpace(cfg.Xfyun.TranslationAPISecret); value != "" {
		values = append(values, "translation:"+secretstore.Fingerprint(value)[:12])
	}
	if value := strings.TrimSpace(cfg.Aliyun.APIKey); value != "" {
		values = append(values, "aliyun_asr:"+secretstore.Fingerprint(value)[:12])
	}
	return strings.Join(values, ",")
}
