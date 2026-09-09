package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

// ---------- Request / Response 数据类型 ----------

// ChatMessage OpenAI 兼容的消息格式
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest LLM 聊天补全请求参数
type ChatRequest struct {
	Model       string         `json:"model"`
	Messages    []ChatMessage  `json:"messages"`
	Temperature *float64       `json:"temperature,omitempty"`
	TopP        *float64       `json:"top_p,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ChatChoice 聊天补全返回的选择结果
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// TokenUsage token 用量统计
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse LLM 聊天补全响应
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *TokenUsage  `json:"usage,omitempty"`
}

// EmbeddingRequest 文本向量化请求参数
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingData 向量数据
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// EmbeddingResponse 文本向量化响应
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *TokenUsage     `json:"usage,omitempty"`
}

type Sub2APIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type Sub2APIModelListResponse struct {
	Object string         `json:"object"`
	Data   []Sub2APIModel `json:"data"`
}

// QuotaInfo Sub2API 配额信息
type QuotaInfo struct {
	TotalQuota     int64 `json:"total_quota"`
	UsedQuota      int64 `json:"used_quota"`
	RemainingQuota int64 `json:"remaining_quota"`
}

type Sub2APIAdminCreateUserRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	Username    string  `json:"username"`
	Notes       string  `json:"notes,omitempty"`
	Role        string  `json:"role,omitempty"`
	Concurrency int64   `json:"concurrency,omitempty"`
	RPMLimit    int64   `json:"rpm_limit,omitempty"`
	Balance     float64 `json:"balance,omitempty"`
}

type Sub2APIAdminUser struct {
	ID                         int64   `json:"id"`
	Email                      string  `json:"email"`
	Username                   string  `json:"username"`
	Role                       string  `json:"role"`
	Balance                    float64 `json:"balance"`
	FrozenBalance              float64 `json:"frozen_balance"`
	Concurrency                int64   `json:"concurrency"`
	Status                     string  `json:"status"`
	AllowedGroups              any     `json:"allowed_groups"`
	LastActiveAt               string  `json:"last_active_at"`
	CreatedAt                  string  `json:"created_at"`
	UpdatedAt                  string  `json:"updated_at"`
	BalanceNotifyEnabled       bool    `json:"balance_notify_enabled"`
	BalanceNotifyThresholdType string  `json:"balance_notify_threshold_type"`
	BalanceNotifyThreshold     any     `json:"balance_notify_threshold"`
	BalanceNotifyExtraEmails   any     `json:"balance_notify_extra_emails"`
	TotalRecharged             float64 `json:"total_recharged"`
	RPMLimit                   int64   `json:"rpm_limit"`
	Notes                      string  `json:"notes"`
	LastUsedAt                 *string `json:"last_used_at"`
	CurrentConcurrency         int64   `json:"current_concurrency"`
}

type Sub2APIAdminCreateUserResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    Sub2APIAdminUser `json:"data"`
}

type Sub2APIAdminListUsersQuery struct {
	Page                 int
	PageSize             int
	Status               string
	Role                 string
	Search               string
	IncludeSubscriptions bool
	SortBy               string
	SortOrder            string
	Timezone             string
}

type Sub2APIAdminListUsersData struct {
	Items    []Sub2APIAdminUser `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Pages    int                `json:"pages"`
}

type Sub2APIAdminListUsersResponse struct {
	Code    int                       `json:"code"`
	Message string                    `json:"message"`
	Data    Sub2APIAdminListUsersData `json:"data"`
}

type Sub2APIAdminUpdateBalanceRequest struct {
	Balance   float64 `json:"balance"`
	Operation string  `json:"operation"`
	Notes     string  `json:"notes"`
}

type Sub2APIAdminUpdateBalanceResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    Sub2APIAdminUser `json:"data"`
}

type Sub2APILoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Sub2APIUser struct {
	ID                         int64   `json:"id"`
	Email                      string  `json:"email"`
	Username                   string  `json:"username"`
	Role                       string  `json:"role"`
	Balance                    float64 `json:"balance"`
	FrozenBalance              float64 `json:"frozen_balance"`
	Concurrency                int64   `json:"concurrency"`
	Status                     string  `json:"status"`
	AllowedGroups              any     `json:"allowed_groups"`
	LastActiveAt               string  `json:"last_active_at"`
	CreatedAt                  string  `json:"created_at"`
	UpdatedAt                  string  `json:"updated_at"`
	BalanceNotifyEnabled       bool    `json:"balance_notify_enabled"`
	BalanceNotifyThresholdType string  `json:"balance_notify_threshold_type"`
	BalanceNotifyThreshold     any     `json:"balance_notify_threshold"`
	BalanceNotifyExtraEmails   any     `json:"balance_notify_extra_emails"`
	TotalRecharged             float64 `json:"total_recharged"`
	RPMLimit                   int64   `json:"rpm_limit"`
	CurrentConcurrency         int64   `json:"current_concurrency"`
}

type Sub2APILoginData struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	TokenType    string      `json:"token_type"`
	User         Sub2APIUser `json:"user"`
}

type Sub2APILoginResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    Sub2APILoginData `json:"data"`
}

type Sub2APIAuthMeResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    Sub2APIUser `json:"data"`
}

type Sub2APICreateKeyRequest struct {
	Name      string  `json:"name"`
	GroupID   int64   `json:"group_id"`
	Quota     float64 `json:"quota"`
	ExpiresAt string  `json:"expires_at,omitempty"`
}

type Sub2APIKey struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"user_id"`
	Key                string  `json:"key"`
	Name               string  `json:"name"`
	GroupID            int64   `json:"group_id"`
	Status             string  `json:"status"`
	IPWhitelist        any     `json:"ip_whitelist"`
	IPBlacklist        any     `json:"ip_blacklist"`
	LastUsedAt         *string `json:"last_used_at"`
	LastUsedIP         string  `json:"last_used_ip"`
	Quota              float64 `json:"quota"`
	QuotaUsed          float64 `json:"quota_used"`
	ExpiresAt          *string `json:"expires_at"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CurrentConcurrency int64   `json:"current_concurrency"`
	RateLimit5h        int64   `json:"rate_limit_5h"`
	RateLimit1d        int64   `json:"rate_limit_1d"`
	RateLimit7d        int64   `json:"rate_limit_7d"`
	Usage5h            int64   `json:"usage_5h"`
	Usage1d            int64   `json:"usage_1d"`
	Usage7d            int64   `json:"usage_7d"`
	Window5hStart      *string `json:"window_5h_start"`
	Window1dStart      *string `json:"window_1d_start"`
	Window7dStart      *string `json:"window_7d_start"`
	Group              *struct {
		ID                       int64    `json:"id"`
		Name                     string   `json:"name"`
		Description              string   `json:"description"`
		Platform                 string   `json:"platform"`
		RateMultiplier           float64  `json:"rate_multiplier"`
		Status                   string   `json:"status"`
		SubscriptionType         string   `json:"subscription_type"`
		DailyLimitUSD            float64  `json:"daily_limit_usd"`
		WeeklyLimitUSD           float64  `json:"weekly_limit_usd"`
		MonthlyLimitUSD          float64  `json:"monthly_limit_usd"`
		AllowImageGeneration     bool     `json:"allow_image_generation"`
		AllowMessagesDispatch    bool     `json:"allow_messages_dispatch"`
		RPMLimit                 int64    `json:"rpm_limit"`
		FallbackGroupID          *int64   `json:"fallback_group_id"`
		FallbackGroupIDOnInvalid *int64   `json:"fallback_group_id_on_invalid_request"`
		CreatedAt                string   `json:"created_at"`
		UpdatedAt                string   `json:"updated_at"`
		ImagePrice1K             *float64 `json:"image_price_1k"`
		ImagePrice2K             *float64 `json:"image_price_2k"`
		ImagePrice4K             *float64 `json:"image_price_4k"`
	} `json:"group"`
}

type Sub2APICreateKeyResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    Sub2APIKey `json:"data"`
}

type Sub2APIListKeysQuery struct {
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
	Timezone  string
}

type Sub2APIListKeysData struct {
	Items    []Sub2APIKey `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Pages    int          `json:"pages"`
}

type Sub2APIListKeysResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    Sub2APIListKeysData `json:"data"`
}

type Sub2APIUpdateKeyRequest struct {
	Name        string    `json:"name,omitempty"`
	GroupID     int64     `json:"group_id,omitempty"`
	IPWhitelist *[]string `json:"ip_whitelist,omitempty"`
	IPBlacklist *[]string `json:"ip_blacklist,omitempty"`
	Quota       *float64  `json:"quota,omitempty"`
	ExpiresAt   string    `json:"expires_at,omitempty"`
	RateLimit5h int64     `json:"rate_limit_5h,omitempty"`
	RateLimit1d int64     `json:"rate_limit_1d,omitempty"`
	RateLimit7d int64     `json:"rate_limit_7d,omitempty"`
	Status      string    `json:"status,omitempty"`
	ResetQuota  bool      `json:"reset_quota,omitempty"`
}

type Sub2APIUpdateKeyResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    Sub2APIKey `json:"data"`
}

type Sub2APISubscriptionGroup struct {
	ID                           int64   `json:"id"`
	Name                         string  `json:"name"`
	Description                  string  `json:"description"`
	Platform                     string  `json:"platform"`
	RateMultiplier               float64 `json:"rate_multiplier"`
	IsExclusive                  bool    `json:"is_exclusive"`
	Status                       string  `json:"status"`
	SubscriptionType             string  `json:"subscription_type"`
	DailyLimitUSD                float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD               float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD              float64 `json:"monthly_limit_usd"`
	AllowImageGeneration         bool    `json:"allow_image_generation"`
	AllowBatchImageGeneration    bool    `json:"allow_batch_image_generation"`
	ImageRateIndependent         bool    `json:"image_rate_independent"`
	ImageRateMultiplier          float64 `json:"image_rate_multiplier"`
	BatchImageDiscountMultiplier float64 `json:"batch_image_discount_multiplier"`
	BatchImageHoldMultiplier     float64 `json:"batch_image_hold_multiplier"`
	VideoRateIndependent         bool    `json:"video_rate_independent"`
	VideoRateMultiplier          float64 `json:"video_rate_multiplier"`
	PeakRateEnabled              bool    `json:"peak_rate_enabled"`
	PeakStart                    string  `json:"peak_start"`
	PeakEnd                      string  `json:"peak_end"`
	PeakRateMultiplier           float64 `json:"peak_rate_multiplier"`
	ClaudeCodeOnly               bool    `json:"claude_code_only"`
	AllowMessagesDispatch        bool    `json:"allow_messages_dispatch"`
	RequireOAuthOnly             bool    `json:"require_oauth_only"`
	RequirePrivacySet            bool    `json:"require_privacy_set"`
	RPMLimit                     int64   `json:"rpm_limit"`
	CreatedAt                    string  `json:"created_at"`
	UpdatedAt                    string  `json:"updated_at"`
}

type Sub2APISubscription struct {
	ID                 int64                     `json:"id"`
	UserID             int64                     `json:"user_id"`
	GroupID            int64                     `json:"group_id"`
	StartsAt           string                    `json:"starts_at"`
	ExpiresAt          string                    `json:"expires_at"`
	Status             string                    `json:"status"`
	DailyWindowStart   *string                   `json:"daily_window_start"`
	WeeklyWindowStart  *string                   `json:"weekly_window_start"`
	MonthlyWindowStart *string                   `json:"monthly_window_start"`
	DailyUsageUSD      float64                   `json:"daily_usage_usd"`
	WeeklyUsageUSD     float64                   `json:"weekly_usage_usd"`
	MonthlyUsageUSD    float64                   `json:"monthly_usage_usd"`
	CreatedAt          string                    `json:"created_at"`
	UpdatedAt          string                    `json:"updated_at"`
	Group              *Sub2APISubscriptionGroup `json:"group"`
}

type Sub2APIListSubscriptionsResponse struct {
	Code    int                   `json:"code"`
	Message string                `json:"message"`
	Data    []Sub2APISubscription `json:"data"`
}

type Sub2APIUsageQuery struct {
	StartDate   string
	EndDate     string
	ModelSource string
	Timezone    string
	Granularity string
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
}

type Sub2APIUsageDashboardModel struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actual_cost"`
}

type Sub2APIUsageDashboardModelsData struct {
	EndDate   string                       `json:"end_date"`
	Models    []Sub2APIUsageDashboardModel `json:"models"`
	StartDate string                       `json:"start_date"`
}

type Sub2APIUsageDashboardModelsResponse struct {
	Code    int                             `json:"code"`
	Message string                          `json:"message"`
	Data    Sub2APIUsageDashboardModelsData `json:"data"`
}

type Sub2APIUsageEndpoint struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actual_cost"`
}

type Sub2APIUsageStatsData struct {
	TotalRequests            int64                  `json:"total_requests"`
	TotalInputTokens         int64                  `json:"total_input_tokens"`
	TotalOutputTokens        int64                  `json:"total_output_tokens"`
	TotalCacheTokens         int64                  `json:"total_cache_tokens"`
	TotalCacheCreationTokens int64                  `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64                  `json:"total_cache_read_tokens"`
	TotalTokens              int64                  `json:"total_tokens"`
	TotalCost                float64                `json:"total_cost"`
	TotalActualCost          float64                `json:"total_actual_cost"`
	AverageDurationMS        float64                `json:"average_duration_ms"`
	Endpoints                []Sub2APIUsageEndpoint `json:"endpoints"`
}

type Sub2APIUsageStatsResponse struct {
	Code    int                   `json:"code"`
	Message string                `json:"message"`
	Data    Sub2APIUsageStatsData `json:"data"`
}

type Sub2APIUsageListResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type Sub2APIUsageDashboardTrendPoint struct {
	Date                string  `json:"date"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actual_cost"`
}

type Sub2APIUsageDashboardTrendData struct {
	EndDate     string                            `json:"end_date"`
	Granularity string                            `json:"granularity"`
	StartDate   string                            `json:"start_date"`
	Trend       []Sub2APIUsageDashboardTrendPoint `json:"trend"`
}

type Sub2APIUsageDashboardTrendResponse struct {
	Code    int                            `json:"code"`
	Message string                         `json:"message"`
	Data    Sub2APIUsageDashboardTrendData `json:"data"`
}

type Sub2APIUsagePlatformStats struct {
	Platform        string  `json:"platform"`
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalActualCost float64 `json:"total_actual_cost"`
	TodayRequests   int64   `json:"today_requests"`
	TodayTokens     int64   `json:"today_tokens"`
	TodayActualCost float64 `json:"today_actual_cost"`
}

type Sub2APIUsageDashboardStatsData struct {
	TotalAPIKeys             int64                       `json:"total_api_keys"`
	ActiveAPIKeys            int64                       `json:"active_api_keys"`
	TotalRequests            int64                       `json:"total_requests"`
	TotalInputTokens         int64                       `json:"total_input_tokens"`
	TotalOutputTokens        int64                       `json:"total_output_tokens"`
	TotalCacheCreationTokens int64                       `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64                       `json:"total_cache_read_tokens"`
	TotalTokens              int64                       `json:"total_tokens"`
	TotalCost                float64                     `json:"total_cost"`
	TotalActualCost          float64                     `json:"total_actual_cost"`
	TodayRequests            int64                       `json:"today_requests"`
	TodayInputTokens         int64                       `json:"today_input_tokens"`
	TodayOutputTokens        int64                       `json:"today_output_tokens"`
	TodayCacheCreationTokens int64                       `json:"today_cache_creation_tokens"`
	TodayCacheReadTokens     int64                       `json:"today_cache_read_tokens"`
	TodayTokens              int64                       `json:"today_tokens"`
	TodayCost                float64                     `json:"today_cost"`
	TodayActualCost          float64                     `json:"today_actual_cost"`
	AverageDurationMS        float64                     `json:"average_duration_ms"`
	RPM                      int64                       `json:"rpm"`
	TPM                      int64                       `json:"tpm"`
	ByPlatform               []Sub2APIUsagePlatformStats `json:"by_platform"`
}

type Sub2APIUsageDashboardStatsResponse struct {
	Code    int                            `json:"code"`
	Message string                         `json:"message"`
	Data    Sub2APIUsageDashboardStatsData `json:"data"`
}

// ---------- Provider 接口与实现 ----------

// Sub2APIProvider 是外部 AI LLM Gateway 的客户端接口
//
// Sub2API 由其他团队自建部署，本项目只通过 API 调用。
// 接口设计兼容 OpenAI 格式，支持按 X-Api-Key 头做产品级路由。
type Sub2APIProvider interface {
	// ChatCompletion 调用 LLM 聊天补全
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Embedding 获取文本向量
	Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// ListModels returns the models exposed to the supplied tenant key.
	ListModels(ctx context.Context, apiKey string) (*Sub2APIModelListResponse, error)

	// GetQuota 查询租户指定 API Key 的剩余配额
	GetQuota(ctx context.Context, tenantID, apiKeyID string) (*QuotaInfo, error)

	AdminCreateUser(ctx context.Context, apiKey string, req Sub2APIAdminCreateUserRequest) (*Sub2APIAdminCreateUserResponse, error)
	AdminListUsers(ctx context.Context, apiKey string, query Sub2APIAdminListUsersQuery) (*Sub2APIAdminListUsersResponse, error)
	AdminListUserKeys(ctx context.Context, apiKey string, userID int64, timezone string) (*Sub2APIListKeysResponse, error)
	AdminUpdateUserBalance(ctx context.Context, apiKey string, userID int64, req Sub2APIAdminUpdateBalanceRequest) (*Sub2APIAdminUpdateBalanceResponse, error)
	Login(ctx context.Context, req Sub2APILoginRequest) (*Sub2APILoginResponse, error)
	AuthMe(ctx context.Context, bearerToken string, timezone string) (*Sub2APIAuthMeResponse, error)
	CreateKey(ctx context.Context, bearerToken string, req Sub2APICreateKeyRequest) (*Sub2APICreateKeyResponse, error)
	ListKeys(ctx context.Context, bearerToken string, query Sub2APIListKeysQuery) (*Sub2APIListKeysResponse, error)
	UpdateKey(ctx context.Context, bearerToken string, keyID int64, req Sub2APIUpdateKeyRequest) (*Sub2APIUpdateKeyResponse, error)
	ListSubscriptions(ctx context.Context, bearerToken string, timezone string) (*Sub2APIListSubscriptionsResponse, error)
	UsageDashboardModels(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageDashboardModelsResponse, error)
	UsageDashboardTrend(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageDashboardTrendResponse, error)
	UsageDashboardStats(ctx context.Context, bearerToken string, timezone string) (*Sub2APIUsageDashboardStatsResponse, error)
	UsageStats(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageStatsResponse, error)
	UsageList(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageListResponse, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// sub2APIProvider Sub2APIProvider 的默认 HTTP 实现
type sub2APIProvider struct {
	cfg    *config.Sub2APIConfig
	client *http.Client
	mu     sync.RWMutex
}

// NewSub2APIProvider 创建 Sub2APIProvider 实例
func NewSub2APIProvider(cfg *config.Sub2APIConfig) Sub2APIProvider {
	return &sub2APIProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
	}
}

// ---------- HTTP 请求封装 ----------

// Sub2APIClientError is a structured upstream 4xx response. Callers use the
// status and reason to decide whether an operation is safe to reconcile.
type Sub2APIClientError struct {
	StatusCode int
	Code       string
	Message    string
	Reason     string
}

func (e *Sub2APIClientError) Error() string {
	parts := []string{fmt.Sprintf("sub2api: client error: status=%d", e.StatusCode)}
	if e.Message != "" {
		parts = append(parts, "message="+e.Message)
	}
	if e.Reason != "" {
		parts = append(parts, "reason="+e.Reason)
	}
	return strings.Join(parts, " ")
}

func parseSub2APIClientError(statusCode int, body []byte) error {
	var envelope struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Reason  string          `json:"reason"`
		Error   struct {
			Code    json.RawMessage `json:"code"`
			Message string          `json:"message"`
			Type    string          `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &Sub2APIClientError{StatusCode: statusCode, Message: http.StatusText(statusCode)}
	}
	message := strings.TrimSpace(envelope.Message)
	code := normalizeSub2APIErrorCode(envelope.Code)
	if message == "" {
		message = strings.TrimSpace(envelope.Error.Message)
	}
	if code == "" {
		code = normalizeSub2APIErrorCode(envelope.Error.Code)
	}
	reason := strings.TrimSpace(envelope.Reason)
	if reason == "" {
		reason = strings.TrimSpace(envelope.Error.Type)
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return &Sub2APIClientError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Reason:     reason,
	}
}

func normalizeSub2APIErrorCode(raw json.RawMessage) string {
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

// doRequest 执行 HTTP 请求并解析 JSON 响应，内置重试和指数退避
//
// apiKey 参数可覆盖默认 API Key，实现产品级路由。
// 传入空字符串时使用配置中的 DefaultKey。
func (p *sub2APIProvider) doRequest(ctx context.Context, method, path string, apiKey string, bearerToken string, bodyObj any, respObj any) error {
	baseURL := strings.TrimRight(p.cfg.BaseURL, "/")
	if baseURL == "" {
		return fmt.Errorf("sub2api: base_url is not configured")
	}
	url := baseURL + path

	key := apiKey
	if key == "" && bearerToken == "" && path != "/api/v1/auth/login" {
		key = p.cfg.DefaultKey
	}

	maxRetries := p.cfg.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避：2^(attempt-1) * baseBackoff，抖动 50%
			backoff := p.cfg.RetryBackoff * time.Duration(1<<(attempt-1))
			jitter := time.Duration(float64(backoff) * 0.5 * (float64(time.Now().UnixNano()%100) / 100))
			actualBackoff := backoff + jitter

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(actualBackoff):
			}
		}

		var reqBodyBytes []byte
		if bodyObj != nil {
			reqBodyBytes, lastErr = json.Marshal(bodyObj)
			if lastErr != nil {
				return fmt.Errorf("sub2api: request marshal error: %w", lastErr)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(reqBodyBytes))
		if err != nil {
			lastErr = fmt.Errorf("sub2api: create request error: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if key != "" {
			req.Header.Set("X-Api-Key", key)
		}
		if bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+bearerToken)
		}

		httpResp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sub2api: request failed: %w", err)
			continue
		}

		respBody, readErr := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("sub2api: read response body error: %w", readErr)
			continue
		}

		// 5xx 可重试
		if httpResp.StatusCode >= 500 {
			lastErr = fmt.Errorf("sub2api: server error: status=%d body=%s", httpResp.StatusCode, string(respBody))
			continue
		}

		// 4xx 不重试
		if httpResp.StatusCode >= 400 {
			return parseSub2APIClientError(httpResp.StatusCode, respBody)
		}

		// 成功响应
		if err := checkSub2APIBusinessError(respBody); err != nil {
			return err
		}
		if respObj != nil {
			if err := json.Unmarshal(respBody, respObj); err != nil {
				return fmt.Errorf("sub2api: response unmarshal error: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("sub2api: request failed after %d retries: %w", maxRetries, lastErr)
}

func checkSub2APIBusinessError(respBody []byte) error {
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil
	}
	if envelope.Code == 0 {
		return nil
	}
	message := strings.TrimSpace(envelope.Message)
	if message == "" {
		message = fmt.Sprintf("business code %d", envelope.Code)
	}
	return fmt.Errorf("sub2api: %s", message)
}

// ---------- 接口方法实现 ----------

// ChatCompletion 调用 LLM 聊天补全
func (p *sub2APIProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("sub2api: streaming chat completion is not supported by this provider")
	}

	var resp ChatResponse
	if err := p.doRequest(ctx, http.MethodPost, "/v1/chat/completions", "", "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Embedding 获取文本向量
func (p *sub2APIProvider) Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	var resp EmbeddingResponse
	if err := p.doRequest(ctx, http.MethodPost, "/v1/embeddings", "", "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) ListModels(ctx context.Context, apiKey string) (*Sub2APIModelListResponse, error) {
	var resp Sub2APIModelListResponse
	if err := p.doRequest(ctx, http.MethodGet, "/v1/models", apiKey, "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetQuota 查询租户指定 API Key 的剩余配额
func (p *sub2APIProvider) GetQuota(ctx context.Context, tenantID, apiKeyID string) (*QuotaInfo, error) {
	path := fmt.Sprintf("/api/v1/quota/%s/%s", tenantID, apiKeyID)
	var resp QuotaInfo
	if err := p.doRequest(ctx, http.MethodGet, path, "", "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// HealthCheck 健康检查
func (p *sub2APIProvider) HealthCheck(ctx context.Context) error {
	type healthResp struct {
		Status string `json:"status"`
	}
	var resp healthResp
	if err := p.doRequest(ctx, http.MethodGet, "/health", "", "", nil, &resp); err != nil {
		return err
	}
	return nil
}

func (p *sub2APIProvider) AdminCreateUser(ctx context.Context, apiKey string, req Sub2APIAdminCreateUserRequest) (*Sub2APIAdminCreateUserResponse, error) {
	var resp Sub2APIAdminCreateUserResponse
	if err := p.doRequest(ctx, http.MethodPost, "/api/v1/admin/users", apiKey, "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) AdminListUsers(ctx context.Context, apiKey string, query Sub2APIAdminListUsersQuery) (*Sub2APIAdminListUsersResponse, error) {
	var resp Sub2APIAdminListUsersResponse
	path := "/api/v1/admin/users" + buildAdminUsersQueryString(query)
	if err := p.doRequest(ctx, http.MethodGet, path, apiKey, "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) AdminListUserKeys(ctx context.Context, apiKey string, userID int64, timezone string) (*Sub2APIListKeysResponse, error) {
	var resp Sub2APIListKeysResponse
	values := url.Values{}
	if strings.TrimSpace(timezone) != "" {
		values.Set("timezone", strings.TrimSpace(timezone))
	}
	path := fmt.Sprintf("/api/v1/admin/users/%d/api-keys", userID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := p.doRequest(ctx, http.MethodGet, path, apiKey, "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) AdminUpdateUserBalance(ctx context.Context, apiKey string, userID int64, req Sub2APIAdminUpdateBalanceRequest) (*Sub2APIAdminUpdateBalanceResponse, error) {
	var resp Sub2APIAdminUpdateBalanceResponse
	path := fmt.Sprintf("/api/v1/admin/users/%d/balance", userID)
	if err := p.doRequest(ctx, http.MethodPost, path, apiKey, "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) Login(ctx context.Context, req Sub2APILoginRequest) (*Sub2APILoginResponse, error) {
	var resp Sub2APILoginResponse
	if err := p.doRequest(ctx, http.MethodPost, "/api/v1/auth/login", "", "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) AuthMe(ctx context.Context, bearerToken string, timezone string) (*Sub2APIAuthMeResponse, error) {
	var resp Sub2APIAuthMeResponse
	values := url.Values{}
	if strings.TrimSpace(timezone) != "" {
		values.Set("timezone", strings.TrimSpace(timezone))
	}
	path := "/api/v1/auth/me"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) CreateKey(ctx context.Context, bearerToken string, req Sub2APICreateKeyRequest) (*Sub2APICreateKeyResponse, error) {
	var resp Sub2APICreateKeyResponse
	if err := p.doRequest(ctx, http.MethodPost, "/api/v1/keys", "", bearerToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) ListKeys(ctx context.Context, bearerToken string, query Sub2APIListKeysQuery) (*Sub2APIListKeysResponse, error) {
	var resp Sub2APIListKeysResponse
	path := "/api/v1/keys" + buildKeysQueryString(query)
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UpdateKey(ctx context.Context, bearerToken string, keyID int64, req Sub2APIUpdateKeyRequest) (*Sub2APIUpdateKeyResponse, error) {
	var resp Sub2APIUpdateKeyResponse
	path := fmt.Sprintf("/api/v1/keys/%d", keyID)
	if err := p.doRequest(ctx, http.MethodPut, path, "", bearerToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) ListSubscriptions(ctx context.Context, bearerToken string, timezone string) (*Sub2APIListSubscriptionsResponse, error) {
	var resp Sub2APIListSubscriptionsResponse
	values := url.Values{}
	if strings.TrimSpace(timezone) != "" {
		values.Set("timezone", strings.TrimSpace(timezone))
	}
	path := "/api/v1/subscriptions"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UsageDashboardModels(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageDashboardModelsResponse, error) {
	var resp Sub2APIUsageDashboardModelsResponse
	path := "/api/v1/usage/dashboard/models" + buildUsageQueryString(query, false)
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UsageDashboardTrend(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageDashboardTrendResponse, error) {
	var resp Sub2APIUsageDashboardTrendResponse
	path := "/api/v1/usage/dashboard/trend" + buildTrendQueryString(query)
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UsageDashboardStats(ctx context.Context, bearerToken string, timezone string) (*Sub2APIUsageDashboardStatsResponse, error) {
	var resp Sub2APIUsageDashboardStatsResponse
	values := url.Values{}
	if strings.TrimSpace(timezone) != "" {
		values.Set("timezone", strings.TrimSpace(timezone))
	}
	path := "/api/v1/usage/dashboard/stats"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UsageStats(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageStatsResponse, error) {
	var resp Sub2APIUsageStatsResponse
	path := "/api/v1/usage/stats" + buildUsageQueryString(query, false)
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *sub2APIProvider) UsageList(ctx context.Context, bearerToken string, query Sub2APIUsageQuery) (*Sub2APIUsageListResponse, error) {
	var resp Sub2APIUsageListResponse
	path := "/api/v1/usage" + buildUsageQueryString(query, true)
	if err := p.doRequest(ctx, http.MethodGet, path, "", bearerToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func buildKeysQueryString(query Sub2APIListKeysQuery) string {
	values := url.Values{}
	if query.Page > 0 {
		values.Set("page", fmt.Sprintf("%d", query.Page))
	}
	if query.PageSize > 0 {
		values.Set("page_size", fmt.Sprintf("%d", query.PageSize))
	}
	if query.SortBy != "" {
		values.Set("sort_by", query.SortBy)
	}
	if query.SortOrder != "" {
		values.Set("sort_order", query.SortOrder)
	}
	if query.Timezone != "" {
		values.Set("timezone", query.Timezone)
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func buildTrendQueryString(query Sub2APIUsageQuery) string {
	values := url.Values{}
	if query.StartDate != "" {
		values.Set("start_date", query.StartDate)
	}
	if query.EndDate != "" {
		values.Set("end_date", query.EndDate)
	}
	if query.ModelSource != "" {
		values.Set("model_source", query.ModelSource)
	}
	if query.Timezone != "" {
		values.Set("timezone", query.Timezone)
	}
	if query.Granularity != "" {
		values.Set("granularity", query.Granularity)
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func buildAdminUsersQueryString(query Sub2APIAdminListUsersQuery) string {
	values := url.Values{}
	if query.Page > 0 {
		values.Set("page", fmt.Sprintf("%d", query.Page))
	}
	if query.PageSize > 0 {
		values.Set("page_size", fmt.Sprintf("%d", query.PageSize))
	}
	if query.Status != "" {
		values.Set("status", query.Status)
	}
	if query.Role != "" {
		values.Set("role", query.Role)
	}
	if query.Search != "" {
		values.Set("search", query.Search)
	}
	if query.IncludeSubscriptions {
		values.Set("include_subscriptions", "true")
	}
	if query.SortBy != "" {
		values.Set("sort_by", query.SortBy)
	}
	if query.SortOrder != "" {
		values.Set("sort_order", query.SortOrder)
	}
	if query.Timezone != "" {
		values.Set("timezone", query.Timezone)
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func buildUsageQueryString(query Sub2APIUsageQuery, includePaging bool) string {
	values := url.Values{}
	if query.StartDate != "" {
		values.Set("start_date", query.StartDate)
	}
	if query.EndDate != "" {
		values.Set("end_date", query.EndDate)
	}
	if query.ModelSource != "" {
		values.Set("model_source", query.ModelSource)
	}
	if query.Timezone != "" {
		values.Set("timezone", query.Timezone)
	}
	if includePaging {
		if query.Page > 0 {
			values.Set("page", fmt.Sprintf("%d", query.Page))
		}
		if query.PageSize > 0 {
			values.Set("page_size", fmt.Sprintf("%d", query.PageSize))
		}
		if query.SortBy != "" {
			values.Set("sort_by", query.SortBy)
		}
		if query.SortOrder != "" {
			values.Set("sort_order", query.SortOrder)
		}
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

// ---------- 默认全局实例 ----------

// DefaultSub2APIProvider 默认全局 Sub2APIProvider 实例
var DefaultSub2APIProvider Sub2APIProvider

func init() {
	cfg := config.CurrentOrDefault()
	DefaultSub2APIProvider = NewSub2APIProvider(&cfg.Sub2API)
}

// ensure interface compliance
var _ Sub2APIProvider = (*sub2APIProvider)(nil)
