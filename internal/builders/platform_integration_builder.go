package builders

import (
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/services"
)

func BuildPlatformIntegrationSettings(item *services.PlatformIntegrationSettingsAggregate) *response.PlatformIntegrationSettingsResponse {
	if item == nil {
		return &response.PlatformIntegrationSettingsResponse{}
	}
	return &response.PlatformIntegrationSettingsResponse{
		CanUpdate: item.CanUpdate,
		Jitsi: response.PlatformJitsiRuntimeSettingsResponse{
			URL: item.Jitsi.URL, JitsiDomain: item.Jitsi.JitsiDomain, RequireAuth: item.Jitsi.RequireAuth,
			AppID: item.Jitsi.AppID, AppSecretConfigured: item.Jitsi.AppSecretConfigured,
			WebhookSecretConfigured: item.Jitsi.WebhookSecretConfigured,
			TokenTTLMinutes:         item.Jitsi.TokenTTLMinutes, Timeout: item.Jitsi.Timeout,
			MaxRetries: item.Jitsi.MaxRetries, UpdatedAt: formatTimePtr(item.Jitsi.UpdatedAt),
		},
		Speech: response.PlatformSpeechRuntimeSettingsResponse{
			Provider: item.Speech.Provider, Configured: item.Speech.Configured,
			XfyunAppID: item.Speech.XfyunAppID, XfyunAPIKeyConfigured: item.Speech.XfyunAPIKeyConfigured,
			XfyunEndpoint: item.Speech.XfyunEndpoint, XfyunDomain: item.Speech.XfyunDomain,
			AliyunAPIKeyConfigured: item.Speech.AliyunAPIKeyConfigured, AliyunWorkspaceID: item.Speech.AliyunWorkspaceID,
			AliyunEndpoint: item.Speech.AliyunEndpoint, AliyunModel: item.Speech.AliyunModel,
			JigasiEnabled:                item.Speech.JigasiEnabled,
			JigasiSharedSecretConfigured: item.Speech.JigasiSharedSecretConfigured,
			JigasiMaxParticipantStreams:  item.Speech.JigasiMaxParticipantStreams,
			JigasiMaxConcurrentStreams:   item.Speech.JigasiMaxConcurrentStreams,
			TranslationEnabled:           item.Speech.TranslationEnabled,
			TranslationConfigured:        item.Speech.TranslationConfigured,
			TranslationEndpoint:          item.Speech.TranslationEndpoint,
			TranslationTargetLanguage:    item.Speech.TranslationTargetLanguage,
		},
		AR: response.PlatformARRuntimeSettingsResponse{
			Provider: item.AR.Provider, Configured: item.AR.Configured, Endpoint: item.AR.Endpoint,
			APIKeyConfigured: item.AR.APIKeyConfigured, HealthPath: item.AR.HealthPath,
			UpdatedAt: formatTimePtr(item.AR.UpdatedAt),
		},
		Sub2API: response.PlatformSub2APIRuntimeSettingsResponse{
			Host: item.Sub2API.Host, AdminAPIKeyConfigured: item.Sub2API.AdminAPIKeyConfigured,
			AdminAPIKeyMasked:            item.Sub2API.AdminAPIKeyMasked,
			AdminAPIKeyFingerprint:       item.Sub2API.AdminAPIKeyFingerprint,
			TranslationAPIKeyConfigured:  item.Sub2API.TranslationAPIKeyConfigured,
			TranslationAPIKeyMasked:      item.Sub2API.TranslationAPIKeyMasked,
			TranslationAPIKeyFingerprint: item.Sub2API.TranslationAPIKeyFingerprint,
			DefaultLLMModel:              item.Sub2API.DefaultLLMModel, UpdatedAt: formatTimePtr(item.Sub2API.UpdatedAt),
		},
		SMTP: response.PlatformSMTPRuntimeSettingsResponse{
			SMTPHost: item.SMTP.SMTPHost, SMTPPort: item.SMTP.SMTPPort,
			Username: item.SMTP.Username, PasswordConfigured: item.SMTP.PasswordConfigured,
			FromAddress: item.SMTP.FromAddress, FromName: item.SMTP.FromName,
			UseTLS: item.SMTP.UseTLS, Configured: item.SMTP.Configured,
			UpdatedAt: formatTimePtr(item.SMTP.UpdatedAt),
		},
	}
}

func BuildPlatformIntegrationTest(item *services.PlatformIntegrationTestAggregate) *response.PlatformIntegrationTestResponse {
	if item == nil {
		return &response.PlatformIntegrationTestResponse{}
	}
	return &response.PlatformIntegrationTestResponse{
		Integration: item.Integration, Success: item.Success, Message: item.Message,
		CheckedAt: formatTime(item.CheckedAt), LatencyMs: item.Latency.Milliseconds(), Details: item.Details,
	}
}
