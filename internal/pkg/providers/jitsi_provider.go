package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/golang-jwt/jwt/v5"
)

// --- 数据类型 ---

// JitsiUserInfo Jitsi JWT 中的用户信息
type JitsiUserInfo struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	IsModerator bool   `json:"moderator"`
}

// RoomOptions 创建 Jitsi 房间的选项
type RoomOptions struct {
	StartAudioMuted  int  `json:"startAudioMuted,omitempty"`  // 0/1
	StartVideoMuted  int  `json:"startVideoMuted,omitempty"`  // 0/1
	MaxParticipants  int  `json:"maxParticipants,omitempty"`  // 最大参会人数
	EnableRecording  bool `json:"enableRecording,omitempty"`  // 是否允许录制
	EnableLivestream bool `json:"enableLivestream,omitempty"` // 是否允许直播
}

// Room 表示 Jitsi 房间信息
type Room struct {
	RoomName  string `json:"roomName"`
	MeetingID string `json:"meetingId,omitempty"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// RoomInfo Jitsi 房间详细信息
type RoomInfo struct {
	RoomName         string `json:"roomName"`
	ParticipantCount int    `json:"participantCount"`
	IsActive         bool   `json:"isActive"`
	CreatedAt        string `json:"createdAt,omitempty"`
}

// JitsiServiceHealth is the result of checking the public Jitsi web endpoint
// required by the iframe client. It intentionally excludes meeting activity.
type JitsiServiceHealth struct {
	Status     string
	ServiceURL string
	ProbeURL   string
	HTTPStatus int
	Latency    time.Duration
	CheckedAt  time.Time
	Error      string
}

// --- JitsiClient 实现 ---

// JitsiProvider Jitsi 服务接口
type JitsiProvider interface {
	GenerateToken(roomName string, userInfo JitsiUserInfo) (string, error)
	GenerateJoinToken(roomName string, user JitsiUserInfo) (string, error)
	CreateRoom(ctx context.Context, roomName string, options RoomOptions) (*Room, error)
	CreateMeeting(ctx context.Context, roomName string, options RoomOptions, hostInfo JitsiUserInfo) (*Room, string, error)
	GetRoomInfo(ctx context.Context, roomName string) (*RoomInfo, error)
	DeleteRoom(ctx context.Context, roomName string) error
	GetJitsiURL() string
	GetJitsiPublicURL() string
	GetJitsiDomain() string
	CheckHealth(ctx context.Context) JitsiServiceHealth
	ValidateWebhookSignature(payload []byte, signature string) bool
}

// JitsiClient 是 Jitsi Meet 服务的 API 客户端。
// Jitsi 由其他团队自建部署，本项目只通过 REST API 调用。
type JitsiClient struct {
	baseURL       string // Jitsi 服务地址，如 https://jitsi.example.com
	appID         string
	appSecret     string        // 用于 JWT 签名（HS256）
	webhookSecret string        // Webhook 签名密钥
	jitsiDomain   string        // Jitsi 前端域，用于前端嵌入 iframe
	tokenTTL      time.Duration // JWT token 有效期
	httpClient    *http.Client
	maxRetries    int
	requireAuth   bool
}

// JitsiClientOption 用于配置 JitsiClient 的选项
type JitsiClientOption func(*JitsiClient)

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) JitsiClientOption {
	return func(c *JitsiClient) {
		c.httpClient = client
	}
}

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(maxRetries int) JitsiClientOption {
	return func(c *JitsiClient) {
		c.maxRetries = maxRetries
	}
}

// NewJitsiClient 创建 JitsiClient 实例
func NewJitsiClient(cfg *config.JitsiConfig, opts ...JitsiClientOption) *JitsiClient {
	timeout := 10 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil && d > 0 {
			timeout = d
		}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	client := &JitsiClient{
		baseURL:       cfg.URL,
		appID:         cfg.AppID,
		appSecret:     cfg.AppSecret,
		webhookSecret: cfg.WebhookSecret,
		jitsiDomain:   cfg.JitsiDomain,
		tokenTTL:      cfg.TokenExpiry(),
		httpClient:    &http.Client{Timeout: timeout},
		maxRetries:    maxRetries,
		requireAuth:   cfg.RequireAuth,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// DefaultJitsiClient 默认全局 JitsiClient 实例
var DefaultJitsiClient *JitsiClient

// DefaultJitsiProvider 默认全局 JitsiProvider 接口实例（兼容旧引用）
var DefaultJitsiProvider JitsiProvider

func init() {
	cfg := config.CurrentOrDefault()
	InitJitsi(&cfg.Jitsi)
}

// InitJitsi refreshes the global provider after application configuration has
// been loaded. Package init runs before bootstrap loads config.yaml, so startup
// must call this again with the final configuration.
func InitJitsi(cfg *config.JitsiConfig) {
	if cfg == nil {
		cfg = &config.JitsiConfig{}
	}
	DefaultJitsiClient = NewJitsiClient(cfg)
	DefaultJitsiProvider = DefaultJitsiClient
}

// --- JWT Token 生成 ---

// jitsiCustomClaims JWT 自定义 Claims
type jitsiCustomClaims struct {
	Context  map[string]any `json:"context"`
	Room     string         `json:"room"`
	Audience string         `json:"aud"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 Jitsi JWT token（HS256）
func (c *JitsiClient) GenerateToken(roomName string, userInfo JitsiUserInfo) (string, error) {
	appID := strings.TrimSpace(c.appID)
	appSecret := strings.TrimSpace(c.appSecret)
	if appSecret == "" {
		if appID != "" || c.requireAuth {
			return "", fmt.Errorf("jitsi: app secret is required when app id is configured")
		}
		// Public and anonymous self-hosted Jitsi deployments do not require JWT.
		return "", nil
	}
	if appID == "" {
		return "", fmt.Errorf("jitsi: app id is required when app secret is configured")
	}

	now := time.Now()
	affiliation := "member"
	if userInfo.IsModerator {
		affiliation = "owner"
	}
	claims := &jitsiCustomClaims{
		Context: map[string]any{
			"user": map[string]any{
				"id":          userInfo.UserID,
				"name":        userInfo.Name,
				"email":       userInfo.Email,
				"avatar":      userInfo.Avatar,
				"moderator":   userInfo.IsModerator,
				"affiliation": affiliation,
			},
			"features": map[string]any{
				"livestreaming":     false,
				"recording":         false,
				"outbound-call":     false,
				"transcription":     true,
				"sip-outbound-call": false,
			},
			"group": userInfo.UserID,
		},
		Room:     roomName,
		Audience: "jitsi",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    appID,
			Subject:   c.GetJitsiDomain(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.tokenTTL)),
			ID:        fmt.Sprintf("%s-%d", userInfo.UserID, now.UnixMilli()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(appSecret))
}

// GenerateJoinToken 生成入会 JWT，是 GenerateToken 的别名，
// 语义上更清楚地表明用于加入会议。
func (c *JitsiClient) GenerateJoinToken(roomName string, user JitsiUserInfo) (string, error) {
	return c.GenerateToken(roomName, user)
}

// --- HTTP API 方法 ---

// CreateRoom 创建 Jitsi 房间
func (c *JitsiClient) CreateRoom(ctx context.Context, roomName string, options RoomOptions) (*Room, error) {
	body := map[string]any{
		"roomName": roomName,
	}
	if options.StartAudioMuted > 0 {
		body["startAudioMuted"] = options.StartAudioMuted
	}
	if options.StartVideoMuted > 0 {
		body["startVideoMuted"] = options.StartVideoMuted
	}
	if options.MaxParticipants > 0 {
		body["maxParticipants"] = options.MaxParticipants
	}
	// 注意: enableRecording 和 enableLivestream 需要 Jitsi 服务端支持，
	// 如果 Jitsi 版本不支持这些参数，服务端会忽略它们。

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("jitsi: marshal create room request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/rooms", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("jitsi: create room request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var respData struct {
		Room *Room `json:"room"`
	}
	if err := c.doWithRetry(ctx, req, &respData); err != nil {
		return nil, err
	}
	if respData.Room == nil {
		return nil, fmt.Errorf("jitsi: create room returned nil room")
	}
	respData.Room.URL = c.joinURL(roomName)
	return respData.Room, nil
}

// GetRoomInfo 获取 Jitsi 房间信息
func (c *JitsiClient) GetRoomInfo(ctx context.Context, roomName string) (*RoomInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/rooms/"+roomName, nil)
	if err != nil {
		return nil, fmt.Errorf("jitsi: get room info request: %w", err)
	}

	var respData struct {
		RoomInfo *RoomInfo `json:"roomInfo"`
	}
	if err := c.doWithRetry(ctx, req, &respData); err != nil {
		return nil, err
	}
	if respData.RoomInfo == nil {
		return nil, fmt.Errorf("jitsi: room not found: %s", roomName)
	}
	return respData.RoomInfo, nil
}

// DeleteRoom 删除 Jitsi 房间
func (c *JitsiClient) DeleteRoom(ctx context.Context, roomName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/rooms/"+roomName, nil)
	if err != nil {
		return fmt.Errorf("jitsi: delete room request: %w", err)
	}

	return c.doWithRetry(ctx, req, nil)
}

// --- 工具方法 ---

// GetJitsiURL 获取 Jitsi 服务基础地址
func (c *JitsiClient) GetJitsiURL() string {
	if strings.TrimSpace(c.baseURL) == "" {
		return "https://meet.jit.si"
	}
	return strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
}

// GetJitsiPublicURL 获取面向浏览器的 Jitsi 公网地址。
func (c *JitsiClient) GetJitsiPublicURL() string {
	if domain := normalizeJitsiDomain(c.jitsiDomain); domain != "" {
		return "https://" + domain
	}
	return c.GetJitsiURL()
}

// GetJitsiDomain 获取 Jitsi 前端域
func (c *JitsiClient) GetJitsiDomain() string {
	if domain := normalizeJitsiDomain(c.jitsiDomain); domain != "" {
		return domain
	}
	return normalizeJitsiDomain(c.GetJitsiURL())
}

// CheckHealth verifies the Jitsi config asset used by the iframe bootstrap.
// Unlike probing the root page, this rejects a generic reverse-proxy response
// that is not a usable Jitsi Meet deployment.
func (c *JitsiClient) CheckHealth(ctx context.Context) JitsiServiceHealth {
	checkedAt := time.Now()
	serviceURL := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	result := JitsiServiceHealth{
		Status:     "unconfigured",
		ServiceURL: serviceURL,
		CheckedAt:  checkedAt,
	}
	if serviceURL == "" {
		return result
	}

	result.Status = "unhealthy"
	result.ProbeURL = serviceURL + "/config.js"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.ProbeURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Accept", "application/javascript, text/javascript;q=0.9, */*;q=0.1")

	startedAt := time.Now()
	resp, err := c.httpClient.Do(req)
	result.Latency = time.Since(startedAt)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode

	bytesRead, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if readErr != nil {
		result.Error = fmt.Sprintf("read health response: %v", readErr)
		return result
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		result.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
		return result
	}
	if bytesRead == 0 {
		result.Error = "empty Jitsi config response"
		return result
	}
	result.Status = "healthy"
	return result
}

func normalizeJitsiDomain(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "//" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// CreateMeeting 创建会议并返回房间信息和主持人 token。
// 这是 CreateRoom 和 GenerateToken 的便捷组合方法。
func (c *JitsiClient) CreateMeeting(ctx context.Context, roomName string, options RoomOptions, hostInfo JitsiUserInfo) (*Room, string, error) {
	// 创建房间
	room, err := c.CreateRoom(ctx, roomName, options)
	if err != nil {
		return nil, "", fmt.Errorf("jitsi: create meeting room failed: %w", err)
	}

	// 生成主持人 JWT token
	hostInfo.IsModerator = true
	token, err := c.GenerateToken(roomName, hostInfo)
	if err != nil {
		return nil, "", fmt.Errorf("jitsi: generate meeting token failed: %w", err)
	}

	return room, token, nil
}

// ValidateWebhookSignature 校验 Webhook 签名
// Jitsi webhook 使用 HMAC-SHA256 对 payload 进行签名，
// 签名值放在 X-Hub-Signature 或 X-Jitsi-Signature 请求头中。
// 当 Jitsi 不可用时，webhook 回调不会触发，因此无需降级处理。
func (c *JitsiClient) ValidateWebhookSignature(payload []byte, signature string) bool {
	if c.webhookSecret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// joinURL 构建房间加入 URL
func (c *JitsiClient) joinURL(roomName string) string {
	return c.GetJitsiPublicURL() + "/" + strings.TrimLeft(roomName, "/")
}

// --- HTTP 请求与重试 ---

// jitsiAPIError 表示 Jitsi API 返回的错误
type jitsiAPIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (e *jitsiAPIError) Error() string {
	return fmt.Sprintf("jitsi: api error [%d] %s: %s", e.StatusCode, e.Code, e.Message)
}

// doWithRetry 执行 HTTP 请求并解析 JSON 响应，包含重试逻辑。
//
// 降级策略：
//   - 网络超时/连接失败：最多重试 maxRetries 次，均失败则返回 JitsiUnavailableError。
//   - 4xx 错误（认证/参数错误）：不重试，直接返回业务错误。
//   - 5xx 错误：重试，均失败则返回 JitsiUnavailableError。
//   - 调用方应根据错误类型决定是否降级：若 Jitsi 不可用，可跳过视频会议创建，
//     仅返回入会链接（由前端直接拉起 Jitsi 客户端）。
func (c *JitsiClient) doWithRetry(ctx context.Context, req *http.Request, result any) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}

		lastErr = c.doRequest(req, result)
		if lastErr == nil {
			return nil
		}

		// 判断是否应该重试
		if !isRetryable(lastErr) {
			return lastErr
		}
	}

	return fmt.Errorf("jitsi: %w after %d retries", lastErr, c.maxRetries)
}

func (c *JitsiClient) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &JitsiUnavailableError{Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("jitsi: read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &jitsiAPIError{StatusCode: resp.StatusCode}
		// 尝试解析错误响应体
		if err := json.Unmarshal(body, apiErr); err != nil {
			apiErr.Message = string(body)
		}
		return apiErr
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("jitsi: unmarshal response: %w", err)
		}
	}
	return nil
}

// isRetryable 判断错误是否可重试
func isRetryable(err error) bool {
	switch e := err.(type) {
	case *JitsiUnavailableError:
		return true
	case *jitsiAPIError:
		// 5xx 服务端错误可重试
		return e.StatusCode >= 500
	default:
		return false
	}
}

// backoff 计算第 n 次重试的等待时间（指数退避 + 随机抖动）
func backoff(attempt int) time.Duration {
	base := time.Duration(100*attempt) * time.Millisecond
	if base > 2*time.Second {
		base = 2 * time.Second
	}
	return base
}

// --- 自定义错误类型 ---

// JitsiUnavailableError Jitsi 服务不可用错误
type JitsiUnavailableError struct {
	Err error
}

func (e *JitsiUnavailableError) Error() string {
	return fmt.Sprintf("jitsi: service unavailable: %v", e.Err)
}

func (e *JitsiUnavailableError) Unwrap() error {
	return e.Err
}

// IsJitsiUnavailable 判断是否为 Jitsi 不可用错误
func IsJitsiUnavailable(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*JitsiUnavailableError)
	return ok
}
