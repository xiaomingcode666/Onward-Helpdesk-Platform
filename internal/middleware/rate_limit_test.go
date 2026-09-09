package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/openidentity"

	"github.com/gin-gonic/gin"
)

func TestMemoryRateLimiterRejectsAndResets(t *testing.T) {
	limiter := newMemoryRateLimiter()
	now := time.Now()
	keys := []rateLimitKey{{key: "ip:127.0.0.1", limit: 2, name: "ip"}}

	allowed, remaining, _, limit := limiter.check(keys, now, time.Minute)
	if !allowed || remaining != 1 || limit != 2 {
		t.Fatalf("first request = allowed %v remaining %d limit %d", allowed, remaining, limit)
	}
	allowed, remaining, _, _ = limiter.check(keys, now.Add(time.Second), time.Minute)
	if !allowed || remaining != 0 {
		t.Fatalf("second request = allowed %v remaining %d", allowed, remaining)
	}
	allowed, _, resetAt, _ := limiter.check(keys, now.Add(2*time.Second), time.Minute)
	if allowed || resetAt <= now.UnixMilli() {
		t.Fatalf("third request = allowed %v resetAt %d", allowed, resetAt)
	}
	allowed, remaining, _, _ = limiter.check(keys, now.Add(time.Minute), time.Minute)
	if !allowed || remaining != 1 {
		t.Fatalf("request after reset = allowed %v remaining %d", allowed, remaining)
	}
}

func TestMemoryRateLimiterRejectedActorDoesNotConsumeSharedQuota(t *testing.T) {
	limiter := newMemoryRateLimiter()
	now := time.Now()
	window := time.Minute
	global := rateLimitKey{key: "global", limit: 10, name: "global"}
	actorA := rateLimitKey{key: "actor:a", limit: 1, name: "actor"}
	actorB := rateLimitKey{key: "actor:b", limit: 10, name: "actor"}

	allowed, _, _, _ := limiter.check([]rateLimitKey{global, actorA}, now, window)
	if !allowed {
		t.Fatal("first actor A request should be allowed")
	}
	allowed, _, _, _ = limiter.check([]rateLimitKey{global, actorA}, now.Add(time.Second), window)
	if allowed {
		t.Fatal("second actor A request should be rejected")
	}
	allowed, _, _, _ = limiter.check([]rateLimitKey{global, actorB}, now.Add(2*time.Second), window)
	if !allowed {
		t.Fatal("actor B request should be allowed")
	}

	if got := limiter.entries[global.key].count; got != 2 {
		t.Fatalf("shared quota count=%d, want 2; rejected request must not consume quota", got)
	}
}

func TestBuildRateLimitKeysUsesAuthenticatedCustomerScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://example.test/api/message/list", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:3210"
	httpx.SetCustomerSessionAccess(ctx, httpx.CustomerSessionAccess{
		EntrySessionID: 44,
		TenantID:       7,
	})

	keys := buildRateLimitKeys(ctx, RateLimitConfig{
		GlobalLimit: 1000,
		TenantLimit: 200,
		ActorLimit:  50,
		IPLimit:     400,
	}, "customer-read", time.Minute.Milliseconds())

	if len(keys) != 4 {
		t.Fatalf("keys=%+v, want global, tenant, actor and IP scopes", keys)
	}
	joined := make([]string, 0, len(keys))
	for _, key := range keys {
		joined = append(joined, key.key)
	}
	all := strings.Join(joined, "\n")
	for _, expected := range []string{"global", "tenant:60000:7", "actor:60000:entry:44", "ip:60000:203.0.113.9"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("keys=%s, missing %s", all, expected)
		}
	}
}

func TestGetActorIDHashesExternalIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	httpx.SetExternalUser(ctx, &openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "visitor-private-id",
	})

	actorID := getActorID(ctx)
	if !strings.HasPrefix(actorID, "external:") || strings.Contains(actorID, "visitor-private-id") {
		t.Fatalf("actor id must be stable and privacy-preserving, got %q", actorID)
	}
}

func TestBuildRateLimitKeysAuthenticatedUserOmitsSharedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://example.test/api/customer/v1/tickets", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:3210"
	httpx.SetCustomerSessionAccess(ctx, httpx.CustomerSessionAccess{TenantID: 7})
	httpx.SetExternalUser(ctx, &openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "31",
	})

	keys := buildRateLimitKeys(ctx, RateLimitConfig{
		GlobalLimit: 1000,
		TenantLimit: 200,
		ActorLimit:  50,
		IPLimit:     400,
	}, "customer-portal", time.Minute.Milliseconds())

	if len(keys) != 3 {
		t.Fatalf("keys=%+v, want global, tenant and actor scopes", keys)
	}
	joined := make([]string, 0, len(keys))
	for _, key := range keys {
		joined = append(joined, key.key)
	}
	all := strings.Join(joined, "\n")
	for _, expected := range []string{"global", "tenant:60000:7", "actor:60000:external:"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("keys=%s, missing %s", all, expected)
		}
	}
	if strings.Contains(all, "ip:60000:") {
		t.Fatalf("authenticated customer must not consume a shared IP quota: %s", all)
	}
}

func TestBuildRateLimitKeysAuthenticatedPrincipalOmitsSharedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://example.test/api/auth/profile", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:3210"
	ctx.Set(middlewareAuthPrincipalKey, &dto.AuthPrincipal{UserID: 31, TenantID: 7})
	ctx.Set(tenantContextKey, &TenantContext{TenantID: 7, UserID: 31, UserType: UserTypeCustomer})

	keys := buildRateLimitKeys(ctx, RateLimitConfig{
		GlobalLimit: 1000,
		TenantLimit: 200,
		ActorLimit:  50,
		IPLimit:     400,
	}, "authenticated-session", time.Minute.Milliseconds())

	if len(keys) != 3 {
		t.Fatalf("keys=%+v, want global, tenant and actor scopes", keys)
	}
	joined := make([]string, 0, len(keys))
	for _, key := range keys {
		joined = append(joined, key.key)
	}
	all := strings.Join(joined, "\n")
	for _, expected := range []string{"global", "tenant:60000:7", "actor:60000:user:"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("keys=%s, missing %s", all, expected)
		}
	}
	if strings.Contains(all, "ip:60000:") {
		t.Fatalf("authenticated principal must not consume a shared IP quota: %s", all)
	}
}
