package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"

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
