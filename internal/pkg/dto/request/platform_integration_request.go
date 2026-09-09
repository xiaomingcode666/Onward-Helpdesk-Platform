package request

type PlatformJitsiRuntimeUpdateRequest struct {
	URL             string `json:"url"`
	JitsiDomain     string `json:"jitsiDomain"`
	RequireAuth     bool   `json:"requireAuth"`
	AppID           string `json:"appId"`
	AppSecret       string `json:"appSecret"`
	WebhookSecret   string `json:"webhookSecret"`
	TokenTTLMinutes int    `json:"tokenTtlMinutes"`
	Timeout         string `json:"timeout"`
	MaxRetries      int    `json:"maxRetries"`
}

type PlatformARRuntimeUpdateRequest struct {
	Provider   string `json:"provider"`
	Endpoint   string `json:"endpoint"`
	APIKey     string `json:"apiKey"`
	HealthPath string `json:"healthPath"`
}

type PlatformSub2APIRuntimeUpdateRequest struct {
	Host              string `json:"host"`
	AdminAPIKey       string `json:"adminApiKey"`
	DefaultLLMModel   string `json:"defaultLlmModel"`
	TranslationAPIKey string `json:"translationApiKey"`
}

type PlatformSMTPRuntimeUpdateRequest struct {
	SMTPHost    string `json:"smtpHost"`
	SMTPPort    int    `json:"smtpPort"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	UseTLS      bool   `json:"useTls"`
}

type PlatformIntegrationUpdateRequest struct {
	Integration string                               `json:"integration" binding:"required"`
	Jitsi       *PlatformJitsiRuntimeUpdateRequest   `json:"jitsi,omitempty"`
	Speech      *SpeechRuntimeUpdateRequest          `json:"speech,omitempty"`
	AR          *PlatformARRuntimeUpdateRequest      `json:"ar,omitempty"`
	Sub2API     *PlatformSub2APIRuntimeUpdateRequest `json:"sub2api,omitempty"`
	SMTP        *PlatformSMTPRuntimeUpdateRequest    `json:"smtp,omitempty"`
}

type PlatformIntegrationTestRequest = PlatformIntegrationUpdateRequest
