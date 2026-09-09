package response

type PlatformJitsiRuntimeSettingsResponse struct {
	URL                     string `json:"url"`
	JitsiDomain             string `json:"jitsiDomain"`
	RequireAuth             bool   `json:"requireAuth"`
	AppID                   string `json:"appId"`
	AppSecretConfigured     bool   `json:"appSecretConfigured"`
	WebhookSecretConfigured bool   `json:"webhookSecretConfigured"`
	TokenTTLMinutes         int    `json:"tokenTtlMinutes"`
	Timeout                 string `json:"timeout"`
	MaxRetries              int    `json:"maxRetries"`
	UpdatedAt               string `json:"updatedAt"`
}

type PlatformSpeechRuntimeSettingsResponse struct {
	Provider                     string `json:"provider"`
	Configured                   bool   `json:"configured"`
	XfyunAppID                   string `json:"xfyunAppId"`
	XfyunAPIKeyConfigured        bool   `json:"xfyunApiKeyConfigured"`
	XfyunEndpoint                string `json:"xfyunEndpoint"`
	XfyunDomain                  string `json:"xfyunDomain"`
	AliyunAPIKeyConfigured       bool   `json:"aliyunApiKeyConfigured"`
	AliyunWorkspaceID            string `json:"aliyunWorkspaceId"`
	AliyunEndpoint               string `json:"aliyunEndpoint"`
	AliyunModel                  string `json:"aliyunModel"`
	JigasiEnabled                bool   `json:"jigasiEnabled"`
	JigasiSharedSecretConfigured bool   `json:"jigasiSharedSecretConfigured"`
	JigasiMaxParticipantStreams  int    `json:"jigasiMaxParticipantStreams"`
	JigasiMaxConcurrentStreams   int    `json:"jigasiMaxConcurrentStreams"`
	TranslationEnabled           bool   `json:"translationEnabled"`
	TranslationConfigured        bool   `json:"translationConfigured"`
	TranslationEndpoint          string `json:"translationEndpoint"`
	TranslationTargetLanguage    string `json:"translationTargetLanguage"`
}

type PlatformARRuntimeSettingsResponse struct {
	Provider         string `json:"provider"`
	Configured       bool   `json:"configured"`
	Endpoint         string `json:"endpoint"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
	HealthPath       string `json:"healthPath"`
	UpdatedAt        string `json:"updatedAt"`
}

type PlatformSub2APIRuntimeSettingsResponse struct {
	Host                         string `json:"host"`
	AdminAPIKeyConfigured        bool   `json:"adminApiKeyConfigured"`
	AdminAPIKeyMasked            string `json:"adminApiKeyMasked"`
	AdminAPIKeyFingerprint       string `json:"adminApiKeyFingerprint"`
	TranslationAPIKeyConfigured  bool   `json:"translationApiKeyConfigured"`
	TranslationAPIKeyMasked      string `json:"translationApiKeyMasked"`
	TranslationAPIKeyFingerprint string `json:"translationApiKeyFingerprint"`
	DefaultLLMModel              string `json:"defaultLlmModel"`
	UpdatedAt                    string `json:"updatedAt"`
}

type PlatformSMTPRuntimeSettingsResponse struct {
	SMTPHost           string `json:"smtpHost"`
	SMTPPort           int    `json:"smtpPort"`
	Username           string `json:"username"`
	PasswordConfigured bool   `json:"passwordConfigured"`
	FromAddress        string `json:"fromAddress"`
	FromName           string `json:"fromName"`
	UseTLS             bool   `json:"useTls"`
	Configured         bool   `json:"configured"`
	UpdatedAt          string `json:"updatedAt"`
}

type PlatformIntegrationSettingsResponse struct {
	CanUpdate bool                                   `json:"canUpdate"`
	Jitsi     PlatformJitsiRuntimeSettingsResponse   `json:"jitsi"`
	Speech    PlatformSpeechRuntimeSettingsResponse  `json:"speech"`
	AR        PlatformARRuntimeSettingsResponse      `json:"ar"`
	Sub2API   PlatformSub2APIRuntimeSettingsResponse `json:"sub2api"`
	SMTP      PlatformSMTPRuntimeSettingsResponse    `json:"smtp"`
}

type PlatformIntegrationTestResponse struct {
	Integration string         `json:"integration"`
	Success     bool           `json:"success"`
	Message     string         `json:"message"`
	CheckedAt   string         `json:"checkedAt"`
	LatencyMs   int64          `json:"latencyMs"`
	Details     map[string]any `json:"details,omitempty"`
}
