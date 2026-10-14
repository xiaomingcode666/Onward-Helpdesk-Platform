package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func TestTenantContextAllowsPendingCustomerWithoutTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", &dto.AuthPrincipal{
			UserID:      42,
			DomainType:  UserTypeCustomer,
			SubjectType: models.SubjectTypePendingCustomer,
			SubjectID:   42,
		})
		ctx.Next()
	})
	engine.Use(TenantContextMiddleware())
	engine.GET("/profile", func(ctx *gin.Context) {
		tenantContext := GetTenantContext(ctx)
		if tenantContext == nil {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"tenantId": tenantContext.TenantID,
			"userType": tenantContext.UserType,
		})
	})

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/profile", nil)
	engine.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected pending customer request to pass, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestTenantContextRejectsFormalCustomerWithoutTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", &dto.AuthPrincipal{
			UserID:      42,
			DomainType:  UserTypeCustomer,
			SubjectType: models.SubjectTypeCustomerUser,
			SubjectID:   7,
		})
		ctx.Next()
	})
	engine.Use(TenantContextMiddleware())
	engine.GET("/profile", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/profile", nil)
	engine.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected formal customer without tenant to be rejected, got %d", responseRecorder.Code)
	}
}

func TestTenantContextUsesExplicitRequestLocale(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		header string
		stored string
		want   string
	}{
		{header: "en-US", stored: "zh-CN", want: "en-US"},
		{header: "es", stored: "zh-CN", want: "es-ES"},
		{header: "", stored: "es-ES", want: i18nx.DefaultLocale},
		{header: "fr-FR", stored: "es-ES", want: i18nx.DefaultLocale},
	}
	for _, tc := range cases {
		engine := gin.New()
		engine.Use(func(ctx *gin.Context) {
			ctx.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:     7,
				TenantID:   3,
				DomainType: UserTypeEnterprise,
				Locale:     tc.stored,
			})
			ctx.Next()
		})
		engine.Use(i18nx.Middleware())
		engine.Use(TenantContextMiddleware())
		engine.GET("/locale", func(ctx *gin.Context) {
			if principalLocale, handlerLocale := services.AuthService.GetAuthPrincipal(ctx).Locale, i18nx.Locale(ctx); principalLocale != handlerLocale {
				ctx.String(http.StatusConflict, principalLocale+" != "+handlerLocale)
				return
			}
			ctx.String(http.StatusOK, services.AuthService.GetAuthPrincipal(ctx).Locale)
		})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/locale", nil)
		request.Header.Set("X-Locale", tc.header)
		engine.ServeHTTP(recorder, request)

		if got := recorder.Body.String(); got != tc.want {
			t.Fatalf("header %q stored %q: principal locale = %q, want %q", tc.header, tc.stored, got, tc.want)
		}
	}
}
