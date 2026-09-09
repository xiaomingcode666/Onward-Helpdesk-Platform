package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	simpleweb "github.com/mlogclub/simple/web"
)

var (
	rdb                *redis.Client
	rdbOnce            sync.Once
	rateLimitLuaScript = redis.NewScript(`
local window = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local windowStart = now - window
local counts = {}
local remaining = 2147483647
local responseLimit = 0
local responseResetAt = now + window

for index, key in ipairs(KEYS) do
	local limit = tonumber(ARGV[index + 2])
	redis.call('ZREMRANGEBYSCORE', key, 0, windowStart)
	local count = redis.call('ZCARD', key)
	local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
	local resetAt = now + window
	if oldest[2] then
		resetAt = tonumber(oldest[2]) + window
	end
	if count >= limit then
		return {0, 0, resetAt, limit}
	end
	counts[index] = count
	local candidateRemaining = limit - count - 1
	if candidateRemaining < remaining then
		remaining = candidateRemaining
		responseLimit = limit
		responseResetAt = resetAt
	end
end

for index, key in ipairs(KEYS) do
	redis.call('ZADD', key, now, now .. ':' .. index .. ':' .. math.random())
	redis.call('EXPIRE', key, math.ceil(window / 1000) + 1)
end

if #KEYS == 0 then
	return {1, 0, now + window, 0}
end
return {1, remaining, responseResetAt, responseLimit}
`)
	fallbackRateLimiter = newMemoryRateLimiter()
)

// getRedisClient 获取全局 Redis 客户端实例（单例）
func getRedisClient() *redis.Client {
	rdbOnce.Do(func() {
		cfg := config.Current().Redis
		addr := cfg.Addr
		if addr == "" {
			addr = ":6379"
		}
		rdb = redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			MaxRetries:   1,
			DialTimeout:  500 * time.Millisecond,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			slog.Warn("rate limit redis not available, rate limiting will be skipped", "error", err)
		} else {
			slog.Info("rate limit redis client initialized", "addr", addr)
		}
	})
	return rdb
}

type rateLimitKey struct {
	key   string
	limit int
	name  string
}

type memoryRateLimitEntry struct {
	windowStart time.Time
	count       int
}

type memoryRateLimiter struct {
	mu      sync.Mutex
	entries map[string]memoryRateLimitEntry
}

func newMemoryRateLimiter() *memoryRateLimiter {
	return &memoryRateLimiter{entries: make(map[string]memoryRateLimitEntry)}
}

func (m *memoryRateLimiter) check(keys []rateLimitKey, now time.Time, window time.Duration) (bool, int, int64, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	remaining := int(^uint(0) >> 1)
	resetAt := now.Add(window)
	limitValue := 0
	normalizedEntries := make(map[string]memoryRateLimitEntry, len(keys))
	for _, item := range keys {
		entry := m.entries[item.key]
		if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= window {
			entry = memoryRateLimitEntry{windowStart: now}
		}
		itemResetAt := entry.windowStart.Add(window)
		if entry.count >= item.limit {
			return false, 0, itemResetAt.UnixMilli(), item.limit
		}
		normalizedEntries[item.key] = entry
	}
	for _, item := range keys {
		entry := normalizedEntries[item.key]
		entry.count++
		m.entries[item.key] = entry
		itemResetAt := entry.windowStart.Add(window)
		itemRemaining := item.limit - entry.count
		if itemRemaining < remaining {
			remaining = itemRemaining
			limitValue = item.limit
			resetAt = itemResetAt
		}
	}
	if len(m.entries) > 10000 {
		for key, entry := range m.entries {
			if now.Sub(entry.windowStart) >= window {
				delete(m.entries, key)
			}
		}
	}
	return true, remaining, resetAt.UnixMilli(), limitValue
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Namespace   string
	GlobalLimit int
	TenantLimit int
	ActorLimit  int
	IPLimit     int
	Window      time.Duration
}

// DefaultRateLimitConfig 默认限流配置
var DefaultRateLimitConfig = RateLimitConfig{
	Namespace:   "api",
	GlobalLimit: 100,
	TenantLimit: 50,
	IPLimit:     30,
	Window:      1 * time.Second,
}

// RateLimitMiddleware 基于 Redis 滑动窗口的 API 限流中间件。
// 支持全局、租户、访问者和 IP 四层粒度。
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	cfg := DefaultRateLimitConfig
	if limit > 0 {
		cfg.GlobalLimit = limit
	}
	if window > 0 {
		cfg.Window = window
	}
	return rateLimitMiddlewareWithConfig(cfg)
}

func CustomerEntryRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{Namespace: "customer-entry", GlobalLimit: 5000, TenantLimit: 600, IPLimit: 120, Window: time.Minute})
}

func CustomerPortalRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "customer-portal",
		GlobalLimit: 30000,
		TenantLimit: 12000,
		ActorLimit:  600,
		IPLimit:     600,
		Window:      time.Minute,
	})
}

// CustomerConversationReadRateLimitMiddleware protects authenticated polling
// and conversation reconciliation separately from customer write actions.
func CustomerConversationReadRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "customer-conversation-read",
		GlobalLimit: 30000,
		TenantLimit: 6000,
		ActorLimit:  600,
		IPLimit:     1800,
		Window:      time.Minute,
	})
}

func CustomerConversationMutationRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "customer-conversation-mutation",
		GlobalLimit: 6000,
		TenantLimit: 1200,
		ActorLimit:  120,
		IPLimit:     600,
		Window:      time.Minute,
	})
}

func CustomerMessageRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "customer-message",
		GlobalLimit: 6000,
		TenantLimit: 1200,
		ActorLimit:  90,
		IPLimit:     600,
		Window:      time.Minute,
	})
}

func AuthenticationRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{Namespace: "authentication", GlobalLimit: 1000, TenantLimit: 120, IPLimit: 30, Window: time.Minute})
}

// MarketingDemoRequestRateLimitMiddleware protects the public email delivery
// endpoint from being used as a spam relay.
func MarketingDemoRequestRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{Namespace: "marketing-demo-request", GlobalLimit: 300, IPLimit: 5, Window: time.Minute})
}

// AuthenticatedEndpointPreAuthRateLimitMiddleware bounds invalid-token traffic
// before authentication reaches the session store. The wider IP allowance is
// deliberate because many legitimate enterprise users may share one NAT IP;
// successful requests are constrained again per tenant and actor afterwards.
func AuthenticatedEndpointPreAuthRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "authenticated-preauth",
		GlobalLimit: 60000,
		IPLimit:     1200,
		Window:      time.Minute,
	})
}

func AuthenticatedSessionRateLimitMiddleware() gin.HandlerFunc {
	return rateLimitMiddlewareWithConfig(RateLimitConfig{
		Namespace:   "authenticated-session",
		GlobalLimit: 30000,
		TenantLimit: 6000,
		ActorLimit:  300,
		IPLimit:     600,
		Window:      time.Minute,
	})
}

// rateLimitMiddlewareWithConfig 使用完整配置创建限流中间件
func rateLimitMiddlewareWithConfig(cfg RateLimitConfig) gin.HandlerFunc {
	redisClient := getRedisClient()

	return func(ctx *gin.Context) {
		if redisClient == nil {
			ctx.Next()
			return
		}

		now := time.Now().UnixMilli()
		windowMs := cfg.Window.Milliseconds()
		namespace := strings.TrimSpace(cfg.Namespace)
		if namespace == "" {
			namespace = "api"
		}

		keys := buildRateLimitKeys(ctx, cfg, namespace, windowMs)

		allowed := true
		remaining := cfg.GlobalLimit
		resetTime := now + windowMs
		responseLimit := cfg.GlobalLimit
		redisFailed := false

		redisKeys := make([]string, 0, len(keys))
		scriptArgs := make([]any, 0, len(keys)+2)
		scriptArgs = append(scriptArgs, windowMs, now)
		for _, key := range keys {
			redisKeys = append(redisKeys, key.key)
			scriptArgs = append(scriptArgs, key.limit)
		}
		raw, err := rateLimitLuaScript.Run(ctx, redisClient, redisKeys, scriptArgs...).Result()
		if err != nil {
			slog.Warn("rate limit redis check failed, using local fallback", "namespace", namespace, "error", err)
			redisFailed = true
		} else if parts, ok := raw.([]interface{}); !ok || len(parts) < 4 {
			redisFailed = true
		} else {
			allowed = toInt64(parts[0]) == 1
			remaining = int(toInt64(parts[1]))
			resetTime = toInt64(parts[2])
			responseLimit = int(toInt64(parts[3]))
		}
		if redisFailed {
			allowed, remaining, resetTime, responseLimit = fallbackRateLimiter.check(keys, time.Now(), cfg.Window)
		}

		// 设置标准响应头
		ctx.Header("X-RateLimit-Limit", strconv.Itoa(responseLimit))
		ctx.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		ctx.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime/1000, 10))

		if !allowed {
			retryAfter := int((resetTime - now) / 1000)
			if retryAfter <= 0 {
				retryAfter = 1
			}
			ctx.Header("Retry-After", strconv.Itoa(retryAfter))
			ctx.Abort()
			ctx.JSON(http.StatusTooManyRequests, simpleweb.JsonErrorData(
				2090,
				i18nx.T(ctx, "error.rateLimit.exceeded"),
				gin.H{
					"retryAfter": retryAfter,
					"limit":      responseLimit,
					"remaining":  0,
					"resetTime":  resetTime / 1000,
				},
			))
			return
		}

		ctx.Next()
	}
}

func buildRateLimitKeys(ctx *gin.Context, cfg RateLimitConfig, namespace string, windowMs int64) []rateLimitKey {
	keys := make([]rateLimitKey, 0, 4)
	appendKey := func(name, value string, limit int) {
		if limit <= 0 || strings.TrimSpace(value) == "" {
			return
		}
		keys = append(keys, rateLimitKey{
			key:   fmt.Sprintf("ratelimit:%s:%s:%d:%s", namespace, name, windowMs, value),
			limit: limit,
			name:  name,
		})
	}
	appendKey("global", "all", cfg.GlobalLimit)
	appendKey("tenant", getTenantID(ctx), cfg.TenantLimit)
	appendKey("actor", getActorID(ctx), cfg.ActorLimit)
	if !hasAuthenticatedRateLimitActor(ctx) {
		appendKey("ip", ctx.ClientIP(), cfg.IPLimit)
	}
	return keys
}

func hasAuthenticatedRateLimitActor(ctx *gin.Context) bool {
	if principal := GetAuthPrincipal(ctx); principal != nil && principal.UserID > 0 {
		return true
	}
	external := httpx.GetExternalUser(ctx)
	return external != nil && external.ExternalSource == enums.ExternalSourceUser && strings.TrimSpace(external.ExternalID) != ""
}

// getTenantID 从 Gin Context 中获取租户 ID
func getTenantID(ctx *gin.Context) string {
	tc := GetTenantContext(ctx)
	if tc != nil && tc.TenantID > 0 {
		return strconv.FormatInt(tc.TenantID, 10)
	}
	if tenantID := httpx.GetCustomerSessionAccess(ctx).TenantID; tenantID > 0 {
		return strconv.FormatInt(tenantID, 10)
	}
	if tenantID := ctx.GetHeader("X-Tenant-Id"); tenantID != "" {
		return tenantID
	}
	if tenantID := ctx.Query("tenantId"); tenantID != "" {
		return tenantID
	}
	return ""
}

func getActorID(ctx *gin.Context) string {
	access := httpx.GetCustomerSessionAccess(ctx)
	if access.EntrySessionID > 0 {
		return "entry:" + strconv.FormatInt(access.EntrySessionID, 10)
	}
	if principal := GetAuthPrincipal(ctx); principal != nil && principal.UserID > 0 {
		raw := "user:" + strconv.FormatInt(principal.UserID, 10)
		sum := sha256.Sum256([]byte(raw))
		return fmt.Sprintf("user:%x", sum[:12])
	}
	external := httpx.GetExternalUser(ctx)
	if external == nil || strings.TrimSpace(external.ExternalID) == "" {
		return ""
	}
	raw := fmt.Sprintf("%v:%s", external.ExternalSource, strings.TrimSpace(external.ExternalID))
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("external:%x", sum[:12])
}

// capitalize 首字母大写
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] = runes[0] - 32
	}
	return string(runes)
}

// toInt64 converts interface{} from Redis Lua script result to int64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}
