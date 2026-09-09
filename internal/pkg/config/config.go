package config

import (
	"fmt"
	"net/url"
	"os"
	"remotehelpdesk/internal/pkg/enums"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var environmentBindings = map[string][]string{
	"encryptionKey":             {"RHD_ENCRYPTIONKEY", "ENCRYPTION_KEY"},
	"encryptionKeyFallbacks":    {"RHD_ENCRYPTIONKEY_FALLBACKS", "ENCRYPTION_KEY_FALLBACKS"},
	"public.baseUrl":            {"RHD_PUBLIC_BASEURL", "RHD_PUBLIC_URL", "PUBLIC_APP_URL", "APP_PUBLIC_URL", "NEXT_PUBLIC_APP_BASE_URL", "SERVER_PUBLIC_URL"},
	"mcp.serverToken":           {"RHD_MCP_SERVERTOKEN", "MCP_SERVER_TOKEN"},
	"jitsi.url":                 {"RHD_JITSI_URL", "JITSI_URL"},
	"jitsi.appId":               {"RHD_JITSI_APP_ID", "JITSI_APP_ID"},
	"jitsi.appSecret":           {"RHD_JITSI_APP_SECRET", "JITSI_APP_SECRET"},
	"jitsi.webhookSecret":       {"RHD_JITSI_WEBHOOK_SECRET", "JITSI_WEBHOOK_SECRET"},
	"jitsi.jitsiDomain":         {"RHD_JITSI_DOMAIN", "JITSI_DOMAIN"},
	"jitsi.tokenTTLMinutes":     {"RHD_JITSI_TOKEN_TTL_MINUTES", "JITSI_TOKEN_TTL_MINUTES"},
	"jitsi.requireAuth":         {"RHD_JITSI_REQUIRE_AUTH", "JITSI_REQUIRE_AUTH"},
	"speech.provider":           {"RHD_SPEECH_PROVIDER", "SPEECH_PROVIDER"},
	"speech.xfyun.appId":        {"RHD_XFYUN_RTASR_APP_ID", "XFYUN_RTASR_APP_ID"},
	"speech.xfyun.apiKey":       {"RHD_XFYUN_RTASR_API_KEY", "XFYUN_RTASR_API_KEY"},
	"speech.xfyun.endpoint":     {"RHD_XFYUN_RTASR_ENDPOINT", "XFYUN_RTASR_ENDPOINT"},
	"speech.xfyun.domain":       {"RHD_XFYUN_RTASR_DOMAIN", "XFYUN_RTASR_DOMAIN"},
	"speech.aliyun.apiKey":      {"RHD_ALIYUN_ASR_API_KEY", "ALIYUN_ASR_API_KEY", "DASHSCOPE_API_KEY"},
	"speech.aliyun.workspaceId": {"RHD_ALIYUN_ASR_WORKSPACE_ID", "ALIYUN_ASR_WORKSPACE_ID"},
	"speech.aliyun.endpoint":    {"RHD_ALIYUN_ASR_ENDPOINT", "ALIYUN_ASR_ENDPOINT"},
	"speech.aliyun.model":       {"RHD_ALIYUN_ASR_MODEL", "ALIYUN_ASR_MODEL"},
	"speech.mock.segmentDurationMs": {
		"RHD_SPEECH_MOCK_SEGMENT_DURATION_MS", "SPEECH_MOCK_SEGMENT_DURATION_MS",
	},
	"speech.xfyun.translationEnabled": {
		"RHD_XFYUN_TRANSLATION_ENABLED", "XFYUN_TRANSLATION_ENABLED",
	},
	"speech.xfyun.translationApiSecret": {
		"RHD_XFYUN_TRANSLATION_API_SECRET", "XFYUN_TRANSLATION_API_SECRET",
	},
	"speech.xfyun.translationEndpoint": {
		"RHD_XFYUN_TRANSLATION_ENDPOINT", "XFYUN_TRANSLATION_ENDPOINT",
	},
	"speech.xfyun.translationTargetLanguage": {
		"RHD_XFYUN_TRANSLATION_TARGET_LANGUAGE", "XFYUN_TRANSLATION_TARGET_LANGUAGE",
	},
	"speech.jigasi.enabled": {"RHD_SPEECH_JIGASI_ENABLED", "SPEECH_JIGASI_ENABLED"},
	"speech.jigasi.sharedSecret": {
		"RHD_SPEECH_JIGASI_SHARED_SECRET", "SPEECH_JIGASI_SHARED_SECRET",
	},
	"speech.jigasi.maxParticipantStreams": {
		"RHD_SPEECH_JIGASI_MAX_PARTICIPANT_STREAMS", "SPEECH_JIGASI_MAX_PARTICIPANT_STREAMS",
	},
	"speech.jigasi.maxConcurrentStreams": {
		"RHD_SPEECH_JIGASI_MAX_CONCURRENT_STREAMS", "SPEECH_JIGASI_MAX_CONCURRENT_STREAMS",
	},
	"meetingAR.provider": {"RHD_MEETING_AR_PROVIDER", "MEETING_AR_PROVIDER"},
	"meetingAR.providerOptions.endpoint": {
		"RHD_MEETING_AR_ENDPOINT", "MEETING_AR_ENDPOINT",
	},
	"meetingAR.providerOptions.apiKey": {
		"RHD_MEETING_AR_API_KEY", "MEETING_AR_API_KEY",
	},
	"meetingAR.providerOptions.healthPath": {
		"RHD_MEETING_AR_HEALTH_PATH", "MEETING_AR_HEALTH_PATH",
	},
	"sub2api.base_url": {
		"RHD_SUB2API_BASE_URL", "SUB2API_BASE_URL",
	},
	"sub2api.default_key": {
		"RHD_SUB2API_DEFAULT_KEY", "SUB2API_DEFAULT_KEY",
	},
	"sub2api.admin_api_key": {
		"RHD_SUB2API_ADMIN_API_KEY", "SUB2API_ADMIN_API_KEY", "RHD_SUB2API_DEFAULT_KEY", "SUB2API_DEFAULT_KEY",
	},
	"sub2api.default_model": {
		"RHD_SUB2API_DEFAULT_MODEL", "SUB2API_DEFAULT_MODEL", "SUB2API_LLM_MODEL",
	},
	"email.smtpHost":                 {"RHD_EMAIL_SMTP_HOST", "EMAIL_SMTP_HOST", "SMTP_HOST"},
	"email.smtpPort":                 {"RHD_EMAIL_SMTP_PORT", "EMAIL_SMTP_PORT", "SMTP_PORT"},
	"email.username":                 {"RHD_EMAIL_USERNAME", "EMAIL_USERNAME", "SMTP_USERNAME"},
	"email.password":                 {"RHD_EMAIL_PASSWORD", "EMAIL_PASSWORD", "SMTP_PASSWORD"},
	"email.fromAddress":              {"RHD_EMAIL_FROM_ADDRESS", "EMAIL_FROM_ADDRESS", "SMTP_FROM_ADDRESS"},
	"email.fromName":                 {"RHD_EMAIL_FROM_NAME", "EMAIL_FROM_NAME", "SMTP_FROM_NAME"},
	"email.useTls":                   {"RHD_EMAIL_USE_TLS", "EMAIL_USE_TLS", "SMTP_USE_TLS"},
	"marketing.demoRequestRecipient": {"RHD_MARKETING_DEMO_REQUEST_RECIPIENT", "DEMO_REQUEST_RECIPIENT"},
	"mobilePush.enabled":             {"RHD_MOBILE_PUSH_ENABLED", "MOBILE_PUSH_ENABLED"},
	"mobilePush.fcm.projectId":       {"RHD_FCM_PROJECT_ID", "FCM_PROJECT_ID"},
	"mobilePush.fcm.credentialsJson": {"RHD_FCM_CREDENTIALS_JSON", "FCM_CREDENTIALS_JSON"},
	"mobilePush.apns.teamId":         {"RHD_APNS_TEAM_ID", "APNS_TEAM_ID"},
	"mobilePush.apns.keyId":          {"RHD_APNS_KEY_ID", "APNS_KEY_ID"},
	"mobilePush.apns.privateKey":     {"RHD_APNS_PRIVATE_KEY", "APNS_PRIVATE_KEY"},
	"mobilePush.apns.bundleId":       {"RHD_APNS_BUNDLE_ID", "APNS_BUNDLE_ID"},
	"mobilePush.apns.production":     {"RHD_APNS_PRODUCTION", "APNS_PRODUCTION"},
}

// HasEnvironmentOverride reports whether a bound config namespace contains a
// non-empty environment value. Runtime services use it to seed an empty DB once.
func HasEnvironmentOverride(namespace string) bool {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return false
	}
	for key, names := range environmentBindings {
		if key != namespace && !strings.HasPrefix(key, namespace+".") {
			continue
		}
		for _, name := range remoteHelpDeskEnvironmentNames(names) {
			if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
				return true
			}
		}
	}
	return false
}

type Config struct {
	Language               string                `yaml:"language"`
	EncryptionKey          string                `yaml:"encryptionKey"`
	EncryptionKeyFallbacks []string              `yaml:"encryptionKeyFallbacks" mapstructure:"encryptionKeyFallbacks"`
	Server                 ServerConfig          `yaml:"server"`
	Public                 PublicConfig          `yaml:"public" mapstructure:"public"`
	DB                     DBConfig              `yaml:"db"`
	Logger                 LoggerConfig          `yaml:"logger"`
	Auth                   AuthConfig            `yaml:"auth"`
	Storage                StorageConfig         `yaml:"storage"`
	VectorDB               VectorDBConfig        `yaml:"vectorDB"`
	RAG                    RAGConfig             `yaml:"rag"`
	MCP                    MCPConfig             `yaml:"mcp"`
	WxWork                 WxWorkConfig          `yaml:"wxWork"`
	OIDC                   OIDCConfig            `yaml:"oidc"`
	CustomerSession        CustomerSessionConfig `yaml:"customerSession"`
	Jitsi                  JitsiConfig           `yaml:"jitsi"`
	Speech                 SpeechConfig          `yaml:"speech"`
	MeetingAR              MeetingARConfig       `yaml:"meetingAR"`
	Redis                  RedisConfig           `yaml:"redis"`
	Sub2API                Sub2APIConfig         `yaml:"sub2api"`
	Email                  EmailConfig           `yaml:"email"`
	Marketing              MarketingConfig       `yaml:"marketing" mapstructure:"marketing"`
	MobilePush             MobilePushConfig      `yaml:"mobilePush" mapstructure:"mobilePush"`
	TicketDispatch         TicketDispatchConfig  `yaml:"ticketDispatch"`
}

func (c Config) LanguageOrDefault() string {
	switch strings.ToLower(strings.TrimSpace(c.Language)) {
	case "zh", "zh-cn", "zh_cn", "zh-hans":
		return "zh-CN"
	case "en", "en-us", "en_us":
		return "en-US"
	default:
		return "zh-CN"
	}
}

type WxWorkNotifyConfig struct {
	Enabled                bool    `yaml:"enabled"`
	ToUsers                []int64 `yaml:"toUsers"`
	Safe                   bool    `yaml:"safe"`
	EnableDuplicateCheck   bool    `yaml:"enableDuplicateCheck"`
	DuplicateCheckInterval int     `yaml:"duplicateCheckInterval"`
}

type ServerConfig struct {
	Port int        `yaml:"port" mapstructure:"port"`
	CORS CORSConfig `yaml:"cors" mapstructure:"cors"`
}

func (s ServerConfig) Address() string {
	if s.Port <= 0 {
		return ":8080"
	}
	return fmt.Sprintf(":%d", s.Port)
}

type CORSConfig struct {
	// AllowedOrigins 是允许浏览器跨域访问的 Origin 白名单，必须包含协议和域名。
	// 留空表示不允许跨域请求；同源请求通常不会携带 Origin，不受影响。
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

type PublicConfig struct {
	BaseURL string `yaml:"baseUrl" mapstructure:"baseUrl"`
}

func (c PublicConfig) NormalizedBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
}

func (c PublicConfig) Validate() error {
	baseURL := c.NormalizedBaseURL()
	if baseURL == "" {
		return nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("public.baseUrl must be an absolute HTTP(S) URL")
	}
	return nil
}

type DBConfig struct {
	Type                   string `yaml:"type"`
	DSN                    string `yaml:"dsn"`
	AutoMigrate            *bool  `yaml:"autoMigrate"`
	MaxIdleConns           int    `yaml:"maxIdleConns"`
	MaxOpenConns           int    `yaml:"maxOpenConns"`
	ConnMaxIdleTimeSeconds int    `yaml:"connMaxIdleTimeSeconds"`
	ConnMaxLifetimeSeconds int    `yaml:"connMaxLifetimeSeconds"`
}

func (c DBConfig) AutoMigrateEnabled() bool {
	return c.AutoMigrate == nil || *c.AutoMigrate
}

type LoggerConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	AddSource bool   `yaml:"addSource"`
}

type AuthConfig struct {
	TokenTTLHours        int `yaml:"tokenTTLHours"`
	MaxFailedAttempts    int `yaml:"maxFailedAttempts"`
	CredentialLockMinute int `yaml:"credentialLockMinute"`
}

type CustomerSessionConfig struct {
	Secret                  string `yaml:"secret"`
	TTLMinutes              int    `yaml:"ttlMinutes"`
	RefreshThresholdMinutes int    `yaml:"refreshThresholdMinutes"`
}

func (c CustomerSessionConfig) TTL() int {
	if c.TTLMinutes <= 0 {
		return 120
	}
	return c.TTLMinutes
}

func (c CustomerSessionConfig) RefreshThreshold() int {
	if c.RefreshThresholdMinutes <= 0 {
		return 30
	}
	return c.RefreshThresholdMinutes
}

type StorageConfig struct {
	Default         enums.AssetProvider  `yaml:"default"`
	MaxUploadSizeMB int64                `yaml:"maxUploadSizeMB"`
	UploadSecurity  UploadSecurityConfig `yaml:"uploadSecurity"`
	Local           LocalStorageConfig   `yaml:"local"`
	OSS             OSSStorageConfig     `yaml:"oss"`
	MinIO           MinIOStorageConfig   `yaml:"minio"`
}

type UploadSecurityConfig struct {
	// Enabled defaults to true when omitted. Set it explicitly to false only for
	// isolated development environments.
	Enabled                    *bool                `yaml:"enabled"`
	BlockedExtensions          []string             `yaml:"blockedExtensions"`
	BlockedMIMETypes           []string             `yaml:"blockedMimeTypes"`
	MaxArchiveEntries          int                  `yaml:"maxArchiveEntries"`
	MaxArchiveExpandedSizeMB   int64                `yaml:"maxArchiveExpandedSizeMB"`
	MaxArchiveCompressionRatio int64                `yaml:"maxArchiveCompressionRatio"`
	ClamAV                     ClamAVSecurityConfig `yaml:"clamav"`
}

func (c UploadSecurityConfig) EnabledOrDefault() bool {
	return c.Enabled == nil || *c.Enabled
}

func (c UploadSecurityConfig) ArchiveEntriesLimit() int {
	if c.MaxArchiveEntries <= 0 {
		return 1000
	}
	return c.MaxArchiveEntries
}

func (c UploadSecurityConfig) ArchiveExpandedSizeLimit() int64 {
	if c.MaxArchiveExpandedSizeMB <= 0 {
		return 100 << 20
	}
	return c.MaxArchiveExpandedSizeMB << 20
}

func (c UploadSecurityConfig) ArchiveCompressionRatioLimit() int64 {
	if c.MaxArchiveCompressionRatio <= 0 {
		return 100
	}
	return c.MaxArchiveCompressionRatio
}

type ClamAVSecurityConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Address        string `yaml:"address"`
	TimeoutSeconds int    `yaml:"timeoutSeconds"`
	FailClosed     *bool  `yaml:"failClosed"`
}

func (c ClamAVSecurityConfig) AddressOrDefault() string {
	if value := strings.TrimSpace(c.Address); value != "" {
		return value
	}
	return "127.0.0.1:3310"
}

func (c ClamAVSecurityConfig) TimeoutOrDefault() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c ClamAVSecurityConfig) FailClosedOrDefault() bool {
	return c.FailClosed == nil || *c.FailClosed
}

func (s StorageConfig) MaxUploadSizeBytes() int64 {
	if s.MaxUploadSizeMB <= 0 {
		return 5 << 20
	}
	return s.MaxUploadSizeMB << 20
}

func (s StorageConfig) MaxRequestBodySizeBytes() int64 {
	limit := s.MaxUploadSizeBytes()
	return limit + (1 << 20)
}

type LocalStorageConfig struct {
	Root    string `yaml:"root"`
	BaseURL string `yaml:"baseUrl"`
}

type OSSStorageConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"accessKeyId"`
	AccessKeySecret string `yaml:"accessKeySecret"`
	BaseURL         string `yaml:"baseUrl"`
	Private         bool   `yaml:"private"`
	SignedURLExpire int    `yaml:"signedUrlExpireSeconds"`
}

type MinIOStorageConfig struct {
	Endpoint        string `yaml:"endpoint"`
	PublicEndpoint  string `yaml:"publicEndpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"accessKeyId"`
	AccessKeySecret string `yaml:"accessKeySecret"`
	BaseURL         string `yaml:"baseUrl"`
	Region          string `yaml:"region"`
	UseSSL          bool   `yaml:"useSsl"`
	Private         bool   `yaml:"private"`
	SignedURLExpire int    `yaml:"signedUrlExpireSeconds"`
}

type VectorDBConfig struct {
	Type    string                `yaml:"type"`
	Qdrant  QdrantVectorDBConfig  `yaml:"qdrant"`
	LanceDB LanceDBVectorDBConfig `yaml:"lancedb"`
}

type QdrantVectorDBConfig struct {
	Host     string `yaml:"host"`
	GrpcPort int    `yaml:"grpcPort"`
	APIKey   string `yaml:"apiKey"`
	UseTLS   bool   `yaml:"useTls"`
}

type LanceDBVectorDBConfig struct {
	Path string `yaml:"path"`
}

type RAGConfig struct {
	Enabled               bool          `yaml:"enabled"`
	RuntimeEnabled        bool          `yaml:"runtimeEnabled"`
	IngestionEnabled      bool          `yaml:"ingestionEnabled"`
	HybridEnabled         bool          `yaml:"hybridEnabled"`
	RetrievalMode         string        `yaml:"retrievalMode"`
	ActiveCollectionAlias string        `yaml:"activeCollectionAlias"`
	CollectionPrefix      string        `yaml:"collectionPrefix"`
	SchemaVersion         int           `yaml:"schemaVersion"`
	TaskBatchSize         int           `yaml:"taskBatchSize"`
	TaskInterval          time.Duration `yaml:"taskInterval"`
	MaxRetries            int           `yaml:"maxRetries"`
	ContextMaxTokens      int           `yaml:"contextMaxTokens"`
	MaxContextItems       int           `yaml:"maxContextItems"`
	DenseTopK             int           `yaml:"denseTopK"`
	LexicalTopK           int           `yaml:"lexicalTopK"`
	HybridDenseMinHits    int           `yaml:"hybridDenseMinHits"`
	HybridDenseMinScore   float64       `yaml:"hybridDenseMinScore"`
	RRFK                  int           `yaml:"rrfK"`
	RerankTopN            int           `yaml:"rerankTopN"`
	RerankMaxCandidates   int           `yaml:"rerankMaxCandidates"`
	RerankSkipTopScore    float64       `yaml:"rerankSkipTopScore"`
}

func (c RAGConfig) Normalized() RAGConfig {
	if strings.TrimSpace(c.RetrievalMode) == "" {
		c.RetrievalMode = "auto"
	}
	if strings.TrimSpace(c.ActiveCollectionAlias) == "" {
		c.ActiveCollectionAlias = "knowledge_chunks_active"
	}
	if strings.TrimSpace(c.CollectionPrefix) == "" {
		c.CollectionPrefix = "knowledge_chunks"
	}
	if c.SchemaVersion <= 0 {
		c.SchemaVersion = 2
	}
	if c.TaskBatchSize <= 0 {
		c.TaskBatchSize = 20
	}
	if c.TaskInterval <= 0 {
		c.TaskInterval = 5 * time.Second
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 5
	}
	if c.ContextMaxTokens <= 0 {
		c.ContextMaxTokens = 4000
	}
	if c.MaxContextItems <= 0 {
		c.MaxContextItems = 5
	}
	if c.DenseTopK <= 0 {
		c.DenseTopK = 30
	}
	if c.LexicalTopK <= 0 {
		c.LexicalTopK = 12
	}
	if c.HybridDenseMinHits <= 0 {
		c.HybridDenseMinHits = 3
	}
	if c.HybridDenseMinScore <= 0 {
		c.HybridDenseMinScore = 0.55
	}
	if c.RRFK <= 0 {
		c.RRFK = 60
	}
	if c.RerankTopN <= 0 {
		c.RerankTopN = 5
	}
	if c.RerankMaxCandidates <= 0 {
		c.RerankMaxCandidates = 12
	}
	if c.RerankSkipTopScore <= 0 {
		c.RerankSkipTopScore = 0.82
	}
	return c
}

func (c RAGConfig) Validate() error {
	switch strings.TrimSpace(c.RetrievalMode) {
	case "", "auto", "dense_only", "hybrid_rrf", "hybrid_rrf_rerank":
	default:
		return fmt.Errorf("rag.retrievalMode must be one of auto,dense_only,hybrid_rrf,hybrid_rrf_rerank")
	}
	if strings.TrimSpace(c.ActiveCollectionAlias) == "" {
		return fmt.Errorf("rag.activeCollectionAlias is required")
	}
	if strings.TrimSpace(c.CollectionPrefix) == "" {
		return fmt.Errorf("rag.collectionPrefix is required")
	}
	if c.SchemaVersion < 0 {
		return fmt.Errorf("rag.schemaVersion must be >= 0")
	}
	if c.TaskBatchSize < 0 {
		return fmt.Errorf("rag.taskBatchSize must be >= 0")
	}
	if c.TaskInterval < 0 {
		return fmt.Errorf("rag.taskInterval must be >= 0")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("rag.maxRetries must be >= 0")
	}
	if c.ContextMaxTokens < 0 {
		return fmt.Errorf("rag.contextMaxTokens must be >= 0")
	}
	if c.MaxContextItems < 0 {
		return fmt.Errorf("rag.maxContextItems must be >= 0")
	}
	if c.DenseTopK < 0 {
		return fmt.Errorf("rag.denseTopK must be >= 0")
	}
	if c.LexicalTopK < 0 {
		return fmt.Errorf("rag.lexicalTopK must be >= 0")
	}
	if c.HybridDenseMinHits < 0 {
		return fmt.Errorf("rag.hybridDenseMinHits must be >= 0")
	}
	if c.HybridDenseMinScore < 0 {
		return fmt.Errorf("rag.hybridDenseMinScore must be >= 0")
	}
	if c.RRFK < 0 {
		return fmt.Errorf("rag.rrfK must be >= 0")
	}
	if c.RerankTopN < 0 {
		return fmt.Errorf("rag.rerankTopN must be >= 0")
	}
	if c.RerankMaxCandidates < 0 {
		return fmt.Errorf("rag.rerankMaxCandidates must be >= 0")
	}
	if c.RerankSkipTopScore < 0 {
		return fmt.Errorf("rag.rerankSkipTopScore must be >= 0")
	}
	return nil
}

type MCPConfig struct {
	Enabled     bool                       `yaml:"enabled"`
	ServerToken string                     `yaml:"serverToken"`
	Servers     map[string]MCPServerConfig `yaml:"servers"`
}

type MCPServerConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Endpoint  string            `yaml:"endpoint"`
	TimeoutMS int               `yaml:"timeoutMs"`
	Headers   map[string]string `yaml:"headers"`
}

type OIDCConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"clientId"`
	ClientSecret string   `yaml:"clientSecret"`
	RedirectURL  string   `yaml:"redirectUrl"`
	StateSecret  string   `yaml:"stateSecret"`
	Scopes       []string `yaml:"scopes"`
}

// WxWorkConfig 定义企业微信接入配置。
//
// 当前主要用于后台管理台的企业微信登录流程：
// 1. /api/auth/wxwork/login 生成企业微信授权地址
// 2. 企业微信回调到 OAuthRedirect
// 3. 后端通过 code 换取企业成员身份并完成系统登录
//
// 其中 OAuthRedirect、CorpID、CorpSecret、AgentID 为登录流程核心配置。
type WxWorkConfig struct {
	// Enabled 表示是否启用企业微信登录能力。
	// false 时不会初始化企业微信 SDK，相关登录接口不可用。
	Enabled bool `yaml:"enabled"`
	// CorpID 为企业微信公司 ID，例如 wwxxxxxxxxxxxxxxxx。
	CorpID string `yaml:"corpId"`
	// CorpSecret 为企业微信应用 Secret，用于换取 access_token。
	CorpSecret string `yaml:"corpSecret"`
	// AgentID 为企业微信自建应用 AgentID。
	AgentID string `yaml:"agentId"`
	// OAuthRedirect 为企业微信网页授权回调地址。
	// 必须填写完整 URL，且通常指向后端接口 /api/auth/wxwork/callback。
	OAuthRedirect string `yaml:"oauthRedirect"`
	// StateSecret 为登录 state 的签名密钥，用于防止篡改和重放。
	// 建议填写独立随机字符串；留空时业务代码会退回使用 CorpSecret。
	StateSecret string `yaml:"stateSecret"`
	// RSAPrivateKey 为企业微信回调解密私钥。
	// 当前登录流程未使用，保留给消息回调等场景。
	RSAPrivateKey string `yaml:"rsaPrivateKey"`
	// Token 为企业微信回调 Token。
	// 当前登录流程未使用，保留给消息回调等场景。
	Token string `yaml:"token"`
	// EncodingAESKey 为企业微信消息加解密密钥。
	// 当前登录流程未使用，保留给消息回调等场景。
	EncodingAESKey string `yaml:"encodingAESKey"`
	// Notify 为企业微信应用消息通知配置。
	Notify WxWorkNotifyConfig `yaml:"notify"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("RHD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	for _, key := range v.AllKeys() {
		args := append([]string{key}, configEnvironmentNames(key)...)
		if err := v.BindEnv(args...); err != nil {
			return nil, err
		}
	}
	if err := v.BindEnv("redis.password", "RHD_REDIS_PASSWORD", "AGENT_DESK_REDIS_PASSWORD", "REDIS_PASSWORD"); err != nil {
		return nil, err
	}
	for key, names := range environmentBindings {
		args := append([]string{key}, remoteHelpDeskEnvironmentNames(names)...)
		if err := v.BindEnv(args...); err != nil {
			return nil, err
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	// Keep runtime speech integration settings aligned with deployment env vars.
	// These values are security-sensitive and must not silently fall back to the
	// placeholder YAML when a production container supplies them explicitly.
	if raw, ok := lookupEnvironment("speech.jigasi.enabled"); ok {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.Speech.Jigasi.Enabled = enabled
		}
	}
	if raw, ok := lookupEnvironment("speech.jigasi.sharedSecret"); ok {
		cfg.Speech.Jigasi.SharedSecret = raw
	}
	if raw, ok := lookupEnvironment("speech.jigasi.maxParticipantStreams"); ok {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.Speech.Jigasi.MaxParticipantStreams = value
		}
	}
	if raw, ok := lookupEnvironment("speech.jigasi.maxConcurrentStreams"); ok {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.Speech.Jigasi.MaxConcurrentStreams = value
		}
	}
	if raw := strings.TrimSpace(v.GetString("encryptionKeyFallbacks")); raw != "" {
		cfg.EncryptionKeyFallbacks = splitConfigList(raw)
	}
	if cfg.MeetingAR.ProviderOptions == nil {
		cfg.MeetingAR.ProviderOptions = map[string]any{}
	}
	for option, key := range map[string]string{
		"endpoint":   "meetingAR.providerOptions.endpoint",
		"apiKey":     "meetingAR.providerOptions.apiKey",
		"healthPath": "meetingAR.providerOptions.healthPath",
	} {
		if value := strings.TrimSpace(v.GetString(key)); value != "" {
			cfg.MeetingAR.ProviderOptions[option] = value
		}
	}
	cfg.RAG = cfg.RAG.Normalized()
	cfg.TicketDispatch = cfg.TicketDispatch.Normalized()
	if err := cfg.RAG.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Jitsi.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Speech.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Public.Validate(); err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(cfg.EncryptionKey); key != "" && len(key) < 32 {
		return nil, fmt.Errorf("encryptionKey must contain at least 32 characters when configured")
	}
	if token := strings.TrimSpace(cfg.MCP.ServerToken); token != "" && len(token) < 32 {
		return nil, fmt.Errorf("mcp.serverToken must contain at least 32 characters when configured")
	}
	return cfg, nil
}

func configEnvironmentNames(key string) []string {
	suffix := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
	return []string{"RHD_" + suffix, "AGENT_DESK_" + suffix}
}

func lookupEnvironment(namespace string) (string, bool) {
	for key, names := range environmentBindings {
		if key != namespace {
			continue
		}
		for _, name := range remoteHelpDeskEnvironmentNames(names) {
			if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value), true
			}
		}
	}
	return "", false
}

func remoteHelpDeskEnvironmentNames(names []string) []string {
	result := make([]string, 0, len(names)*2)
	for _, name := range names {
		result = append(result, name)
		if strings.HasPrefix(name, "RHD_") {
			result = append(result, "AGENT_DESK_"+strings.TrimPrefix(name, "RHD_"))
		}
	}
	return result
}

func splitConfigList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// JitsiConfig Jitsi 视频会议配置
type JitsiConfig struct {
	// RequireAuth requires private JWT rooms and signed webhooks. Production
	// deployments should enable it; local demonstrations may leave it disabled.
	RequireAuth bool `yaml:"requireAuth" mapstructure:"requireAuth"`
	// URL Jitsi Meet 服务地址，例如 https://meet.example.com
	URL string `yaml:"url" mapstructure:"url"`
	// AppID Jitsi JWT 签发者标识
	AppID string `yaml:"appId" mapstructure:"appId"`
	// AppSecret Jitsi JWT 签名密钥（HS256）
	AppSecret string `yaml:"appSecret" mapstructure:"appSecret"`
	// WebhookSecret Jitsi Webhook 校验密钥
	WebhookSecret string `yaml:"webhookSecret" mapstructure:"webhookSecret"`
	// JitsiDomain Jitsi 前端域，用于前端嵌入 iframe 时指定 domain
	JitsiDomain string `yaml:"jitsiDomain" mapstructure:"jitsiDomain"`
	// TokenTTLMinutes JWT token 有效时长（分钟）
	TokenTTLMinutes int `yaml:"tokenTTLMinutes" mapstructure:"tokenTTLMinutes"`
	// Timeout HTTP 请求超时时间（例如 10s）
	Timeout string `yaml:"timeout" mapstructure:"timeout"`
	// MaxRetries HTTP 请求最大重试次数
	MaxRetries int `yaml:"maxRetries" mapstructure:"maxRetries"`
}

func (c JitsiConfig) TokenExpiry() time.Duration {
	if c.TokenTTLMinutes <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(c.TokenTTLMinutes) * time.Minute
}

func (c JitsiConfig) Validate() error {
	baseURL := strings.TrimSpace(c.URL)
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("jitsi.url must be an absolute HTTP(S) URL")
		}
		if c.RequireAuth && parsed.Scheme != "https" && !isLocalJitsiHost(parsed.Hostname()) {
			return fmt.Errorf("jitsi.url must use HTTPS when jitsi.requireAuth is enabled")
		}
	}
	appID := strings.TrimSpace(c.AppID)
	appSecret := strings.TrimSpace(c.AppSecret)
	if (appID == "") != (appSecret == "") {
		return fmt.Errorf("jitsi.appId and jitsi.appSecret must be configured together")
	}
	if c.RequireAuth {
		missing := make([]string, 0, 4)
		if baseURL == "" {
			missing = append(missing, "jitsi.url")
		}
		if appID == "" {
			missing = append(missing, "jitsi.appId")
		}
		if appSecret == "" {
			missing = append(missing, "jitsi.appSecret")
		}
		if strings.TrimSpace(c.WebhookSecret) == "" {
			missing = append(missing, "jitsi.webhookSecret")
		}
		if len(missing) > 0 {
			return fmt.Errorf("authenticated Jitsi requires %s", strings.Join(missing, ", "))
		}
		if err := validateJitsiSecret("jitsi.appSecret", appSecret); err != nil {
			return err
		}
		if err := validateJitsiSecret("jitsi.webhookSecret", strings.TrimSpace(c.WebhookSecret)); err != nil {
			return err
		}
	}
	if c.TokenTTLMinutes < 0 || c.TokenTTLMinutes > 7200 {
		return fmt.Errorf("jitsi.tokenTTLMinutes must be between 0 and 7200 when configured")
	}
	if value := strings.TrimSpace(c.Timeout); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("jitsi.timeout must be a positive duration")
		}
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("jitsi.maxRetries must be >= 0")
	}
	return nil
}

func isLocalJitsiHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// SpeechConfig configures meeting speech-to-text. Provider credentials remain
// on the server because they grant direct API access to external services.
type SpeechConfig struct {
	Provider        string             `yaml:"provider" mapstructure:"provider"`
	ProviderOptions map[string]any     `yaml:"providerOptions" mapstructure:"providerOptions"`
	Xfyun           XfyunSpeechConfig  `yaml:"xfyun" mapstructure:"xfyun"`
	Aliyun          AliyunSpeechConfig `yaml:"aliyun" mapstructure:"aliyun"`
	Mock            MockSpeechConfig   `yaml:"mock" mapstructure:"mock"`
	Jigasi          JigasiSpeechConfig `yaml:"jigasi" mapstructure:"jigasi"`
}

type XfyunSpeechConfig struct {
	AppID                     string `yaml:"appId" mapstructure:"appId"`
	APIKey                    string `yaml:"apiKey" mapstructure:"apiKey"`
	Endpoint                  string `yaml:"endpoint" mapstructure:"endpoint"`
	Domain                    string `yaml:"domain" mapstructure:"domain"`
	TranslationEnabled        bool   `yaml:"translationEnabled" mapstructure:"translationEnabled"`
	TranslationAPISecret      string `yaml:"translationApiSecret" mapstructure:"translationApiSecret"`
	TranslationEndpoint       string `yaml:"translationEndpoint" mapstructure:"translationEndpoint"`
	TranslationTargetLanguage string `yaml:"translationTargetLanguage" mapstructure:"translationTargetLanguage"`
}

type AliyunSpeechConfig struct {
	APIKey      string `yaml:"apiKey" mapstructure:"apiKey"`
	WorkspaceID string `yaml:"workspaceId" mapstructure:"workspaceId"`
	Endpoint    string `yaml:"endpoint" mapstructure:"endpoint"`
	Model       string `yaml:"model" mapstructure:"model"`
}

type MeetingARConfig struct {
	Provider        string         `yaml:"provider" mapstructure:"provider"`
	ProviderOptions map[string]any `yaml:"providerOptions" mapstructure:"providerOptions"`
}

type MockSpeechConfig struct {
	SegmentDurationMS int `yaml:"segmentDurationMs" mapstructure:"segmentDurationMs"`
}

func (c MockSpeechConfig) SegmentDuration() time.Duration {
	if c.SegmentDurationMS <= 0 {
		return 2 * time.Second
	}
	return time.Duration(c.SegmentDurationMS) * time.Millisecond
}

func (c MeetingARConfig) ProviderName() string {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return "disabled"
	}
	return provider
}

type JigasiSpeechConfig struct {
	Enabled               bool   `yaml:"enabled" mapstructure:"enabled"`
	SharedSecret          string `yaml:"sharedSecret" mapstructure:"sharedSecret"`
	MaxParticipantStreams int    `yaml:"maxParticipantStreams" mapstructure:"maxParticipantStreams"`
	MaxConcurrentStreams  int    `yaml:"maxConcurrentStreams" mapstructure:"maxConcurrentStreams"`
}

func (c SpeechConfig) TranscriptionEnabled() bool {
	if !c.Jigasi.Enabled {
		return false
	}
	switch c.ProviderName() {
	case "disabled":
		return false
	case "mock":
		return true
	case "xfyun":
		return strings.TrimSpace(c.Xfyun.AppID) != "" && strings.TrimSpace(c.Xfyun.APIKey) != ""
	case "aliyun":
		return strings.TrimSpace(c.Aliyun.APIKey) != ""
	default:
		return true
	}
}

func (c SpeechConfig) ProviderName() string {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return "disabled"
	}
	return provider
}

func (c JigasiSpeechConfig) MaxStreams() int {
	if c.MaxParticipantStreams <= 0 {
		return 8
	}
	return c.MaxParticipantStreams
}

func (c JigasiSpeechConfig) MaxConcurrent() int {
	if c.MaxConcurrentStreams > 0 {
		return c.MaxConcurrentStreams
	}
	if maxStreams := c.MaxStreams(); maxStreams > 32 {
		return maxStreams
	}
	return 32
}

func (c SpeechConfig) Validate() error {
	if c.Jigasi.Enabled {
		secret := strings.TrimSpace(c.Jigasi.SharedSecret)
		if len(secret) < 32 || strings.Contains(strings.ToLower(secret), "change_me") {
			return fmt.Errorf("speech.jigasi.sharedSecret must be a non-placeholder secret of at least 32 characters")
		}
		if c.Jigasi.MaxParticipantStreams < 0 {
			return fmt.Errorf("speech.jigasi.maxParticipantStreams must be >= 0")
		}
		if c.Jigasi.MaxStreams() > 64 {
			return fmt.Errorf("speech.jigasi.maxParticipantStreams must not exceed 64")
		}
		if c.Jigasi.MaxConcurrentStreams < 0 {
			return fmt.Errorf("speech.jigasi.maxConcurrentStreams must be >= 0")
		}
		if c.Jigasi.MaxConcurrent() < c.Jigasi.MaxStreams() {
			return fmt.Errorf("speech.jigasi.maxConcurrentStreams must be >= maxParticipantStreams")
		}
		if c.Jigasi.MaxConcurrent() > 512 {
			return fmt.Errorf("speech.jigasi.maxConcurrentStreams must not exceed 512")
		}
	}
	if err := c.validateXfyunTranslation(); err != nil {
		return err
	}
	provider := c.ProviderName()
	switch provider {
	case "disabled":
		return nil
	case "mock":
		if duration := c.Mock.SegmentDuration(); duration < 500*time.Millisecond || duration > 30*time.Second {
			return fmt.Errorf("speech.mock.segmentDurationMs must be between 500 and 30000")
		}
		return nil
	case "xfyun":
		if strings.TrimSpace(c.Xfyun.AppID) == "" || strings.TrimSpace(c.Xfyun.APIKey) == "" {
			return fmt.Errorf("speech.xfyun.appId and speech.xfyun.apiKey are required when speech.provider=xfyun")
		}
		if endpoint := strings.TrimSpace(c.Xfyun.Endpoint); endpoint != "" {
			parsed, err := url.Parse(endpoint)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss") {
				return fmt.Errorf("speech.xfyun.endpoint must be an absolute WS(S) URL")
			}
			if parsed.Scheme != "wss" && !isLocalJitsiHost(parsed.Hostname()) {
				return fmt.Errorf("speech.xfyun.endpoint must use WSS outside local development")
			}
		}
		return nil
	case "aliyun":
		if strings.TrimSpace(c.Aliyun.APIKey) == "" {
			return fmt.Errorf("speech.aliyun.apiKey is required when speech.provider=aliyun")
		}
		if endpoint := strings.TrimSpace(c.Aliyun.Endpoint); endpoint != "" {
			parsed, err := url.Parse(endpoint)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss" && parsed.Scheme != "http" && parsed.Scheme != "https") {
				return fmt.Errorf("speech.aliyun.endpoint must be an absolute WS(S) or HTTP(S) URL")
			}
			if parsed.Scheme != "wss" && parsed.Scheme != "https" && !isLocalJitsiHost(parsed.Hostname()) {
				return fmt.Errorf("speech.aliyun.endpoint must use WSS or HTTPS outside local development")
			}
		}
		return nil
	default:
		// Registered providers validate their own providerOptions at startup.
		return nil
	}
}

func (c SpeechConfig) validateXfyunTranslation() error {
	if !c.Xfyun.TranslationEnabled {
		return nil
	}
	if strings.TrimSpace(c.Xfyun.AppID) == "" || strings.TrimSpace(c.Xfyun.APIKey) == "" {
		return fmt.Errorf("speech.xfyun.appId and speech.xfyun.apiKey are required when translation is enabled")
	}
	if strings.TrimSpace(c.Xfyun.TranslationAPISecret) == "" {
		return fmt.Errorf("speech.xfyun.translationApiSecret is required when translation is enabled")
	}
	if endpoint := strings.TrimSpace(c.Xfyun.TranslationEndpoint); endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("speech.xfyun.translationEndpoint must be an absolute HTTP(S) URL")
		}
		if parsed.Scheme != "https" && !isLocalJitsiHost(parsed.Hostname()) {
			return fmt.Errorf("speech.xfyun.translationEndpoint must use HTTPS outside local development")
		}
	}
	return nil
}

func validateJitsiSecret(name, value string) error {
	if len(value) < 32 || strings.Contains(strings.ToLower(value), "change_me") {
		return fmt.Errorf("%s must be a non-placeholder secret of at least 32 characters", name)
	}
	return nil
}

// Sub2APIConfig Sub2API LLM Gateway 外部服务配置
type Sub2APIConfig struct {
	// BaseURL Sub2API 服务基础地址
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`
	// DefaultKey 默认 API Key，可通过 X-Api-Key 头覆盖实现产品级路由
	DefaultKey string `yaml:"default_key" mapstructure:"default_key"`
	// AdminAPIKey provisions and reconciles tenant accounts. It is seeded into
	// encrypted database configuration when that database value is absent.
	AdminAPIKey string `yaml:"admin_api_key" mapstructure:"admin_api_key"`
	// DefaultModel seeds the platform model selection for a new environment.
	DefaultModel string `yaml:"default_model" mapstructure:"default_model"`
	// Timeout HTTP 请求超时时间
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`
	// MaxRetries HTTP 请求最大重试次数
	MaxRetries int `yaml:"max_retries" mapstructure:"max_retries"`
	// RetryBackoff 重试基础等待间隔
	RetryBackoff time.Duration `yaml:"retry_backoff" mapstructure:"retry_backoff"`
}

// RedisConfig Redis 缓存配置
type RedisConfig struct {
	Addr                      string `yaml:"addr"`
	Password                  string `yaml:"password"`
	DB                        int    `yaml:"db"`
	NotificationStream        string `yaml:"notificationStream"`
	NotificationConsumerGroup string `yaml:"notificationConsumerGroup"`
}

func (c RedisConfig) NotificationStreamOrDefault() string {
	if value := strings.TrimSpace(c.NotificationStream); value != "" {
		return value
	}
	return "remotehelpdesk:notifications:v1"
}

func (c RedisConfig) NotificationConsumerGroupOrDefault() string {
	if value := strings.TrimSpace(c.NotificationConsumerGroup); value != "" {
		return value
	}
	return "remotehelpdesk-notification-workers-v1"
}

// EmailConfig SMTP 邮件发送配置。
type EmailConfig struct {
	SMTPHost    string `yaml:"smtpHost"`
	SMTPPort    int    `yaml:"smtpPort"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	FromAddress string `yaml:"fromAddress"`
	FromName    string `yaml:"fromName"`
	UseTLS      bool   `yaml:"useTls"`
}

// MarketingConfig holds public-site lead delivery settings.
type MarketingConfig struct {
	DemoRequestRecipient string `yaml:"demoRequestRecipient" mapstructure:"demoRequestRecipient"`
}

// MobilePushConfig keeps APNs and FCM credentials on the backend. Native apps
// register only opaque device tokens and never receive these provider secrets.
type MobilePushConfig struct {
	Enabled bool           `yaml:"enabled" mapstructure:"enabled"`
	FCM     FCMPushConfig  `yaml:"fcm" mapstructure:"fcm"`
	APNS    APNSPushConfig `yaml:"apns" mapstructure:"apns"`
}

type FCMPushConfig struct {
	ProjectID       string `yaml:"projectId" mapstructure:"projectId"`
	CredentialsJSON string `yaml:"credentialsJson" mapstructure:"credentialsJson"`
}

type APNSPushConfig struct {
	TeamID     string `yaml:"teamId" mapstructure:"teamId"`
	KeyID      string `yaml:"keyId" mapstructure:"keyId"`
	PrivateKey string `yaml:"privateKey" mapstructure:"privateKey"`
	BundleID   string `yaml:"bundleId" mapstructure:"bundleId"`
	Production bool   `yaml:"production" mapstructure:"production"`
}

// TicketDispatchConfig 工单响应闭环配置：自动派单、限时接单与超时回收。
type TicketDispatchConfig struct {
	// AcceptDeadlineMinutes 自动派单后工程师必须在规定分钟内确认接单，逾期回收重派。
	AcceptDeadlineMinutes int `yaml:"acceptDeadlineMinutes"`
	// OwnerReplyTakeoverMinutes 已受理工单的最新客户消息超过该时长仍无工程师回复时，允许同产品组成员接管。
	OwnerReplyTakeoverMinutes int `yaml:"ownerReplyTakeoverMinutes"`
	// UnassignedEscalationMinutes 工单持续无人可派达到该时长后，自动交由组长或服务经理兜底。
	UnassignedEscalationMinutes int `yaml:"unassignedEscalationMinutes"`
	// MaxDispatchAttempts 接单超时回收重派的最大次数，达到上限后升级主管。
	MaxDispatchAttempts int `yaml:"maxDispatchAttempts"`
	// OnlineFreshnessMinutes 跨实例派单时允许使用的最近在线时间窗口。
	OnlineFreshnessMinutes int `yaml:"onlineFreshnessMinutes"`
	// SLAWarningLeadMinutes SLA 截止前进入持续预警窗口的分钟数。
	SLAWarningLeadMinutes int `yaml:"slaWarningLeadMinutes"`
	// SLAWarningRepeatMinutes SLA 预警窗口内重复提醒的间隔分钟数。
	SLAWarningRepeatMinutes int `yaml:"slaWarningRepeatMinutes"`
}

// Normalized 返回带默认值的工单派单配置副本。
func (c TicketDispatchConfig) Normalized() TicketDispatchConfig {
	if c.AcceptDeadlineMinutes <= 0 {
		c.AcceptDeadlineMinutes = 30
	}
	if c.OwnerReplyTakeoverMinutes <= 0 {
		c.OwnerReplyTakeoverMinutes = 30
	}
	if c.UnassignedEscalationMinutes <= 0 {
		c.UnassignedEscalationMinutes = 30
	}
	if c.MaxDispatchAttempts <= 0 {
		c.MaxDispatchAttempts = 3
	}
	if c.OnlineFreshnessMinutes <= 0 {
		c.OnlineFreshnessMinutes = 2
	}
	if c.SLAWarningLeadMinutes <= 0 {
		c.SLAWarningLeadMinutes = 30
	}
	if c.SLAWarningRepeatMinutes <= 0 {
		c.SLAWarningRepeatMinutes = 15
	}
	return c
}
