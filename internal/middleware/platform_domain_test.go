package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func TestPlatformOnlyMiddlewareAllowsPlatformRoles(t *testing.T) {
	for _, role := range []string{"super_admin", "platform_admin", "platform_staff"} {
		t.Run(role, func(t *testing.T) {
			called, status := runPlatformOnlyMiddleware(&dto.AuthPrincipal{Roles: []string{role}})
			if !called || status != http.StatusNoContent {
				t.Fatalf("called = %v status = %d, want true and %d", called, status, http.StatusNoContent)
			}
		})
	}
}

func TestPlatformOnlyMiddlewareRejectsEnterprisePrincipal(t *testing.T) {
	called, status := runPlatformOnlyMiddleware(&dto.AuthPrincipal{DomainType: UserTypeEnterprise, Roles: []string{"enterprise_admin"}})
	if called || status != http.StatusOK {
		t.Fatalf("called = %v status = %d, want false and %d", called, status, http.StatusOK)
	}
}

func TestDomainMiddlewareReturnsForbiddenBusinessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(middlewareAuthPrincipalKey, &dto.AuthPrincipal{DomainType: UserTypeEnterprise})
		ctx.Next()
	})
	engine.Use(PlatformOnlyMiddleware())
	engine.GET("/platform", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/platform", nil))
	var result struct {
		Success   bool `json:"success"`
		ErrorCode int  `json:"errorCode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Success || result.ErrorCode != 3001 {
		t.Fatalf("result = %+v, want forbidden error code 3001", result)
	}
}

func TestEnterpriseOnlyMiddlewareRejectsPartnerPrincipal(t *testing.T) {
	called, status := runDomainMiddleware(
		EnterpriseOnlyMiddleware(),
		&dto.AuthPrincipal{DomainType: UserTypePartner, PartnerAccountID: 9},
	)
	if called || status != http.StatusOK {
		t.Fatalf("called = %v status = %d, want false and %d", called, status, http.StatusOK)
	}
}

func TestPartnerOnlyMiddlewareRequiresResolvedPartnerAccount(t *testing.T) {
	allowed, allowedStatus := runDomainMiddleware(
		PartnerOnlyMiddleware(),
		&dto.AuthPrincipal{DomainType: UserTypePartner, PartnerAccountID: 9},
	)
	if !allowed || allowedStatus != http.StatusNoContent {
		t.Fatalf("allowed = %v status = %d, want true and %d", allowed, allowedStatus, http.StatusNoContent)
	}
	rejected, rejectedStatus := runDomainMiddleware(
		PartnerOnlyMiddleware(),
		&dto.AuthPrincipal{DomainType: UserTypeEnterprise},
	)
	if rejected || rejectedStatus != http.StatusOK {
		t.Fatalf("rejected = %v status = %d, want false and %d", rejected, rejectedStatus, http.StatusOK)
	}
}

func runPlatformOnlyMiddleware(principal *dto.AuthPrincipal) (bool, int) {
	return runDomainMiddleware(PlatformOnlyMiddleware(), principal)
}

func runDomainMiddleware(domainMiddleware gin.HandlerFunc, principal *dto.AuthPrincipal) (bool, int) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(middlewareAuthPrincipalKey, principal)
		ctx.Next()
	})
	engine.Use(domainMiddleware)
	called := false
	engine.GET("/platform", func(ctx *gin.Context) {
		called = true
		ctx.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/platform", nil)
	engine.ServeHTTP(recorder, request)
	return called, recorder.Code
}
