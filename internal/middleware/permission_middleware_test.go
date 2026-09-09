package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func TestRequirePermissionMiddlewareRejectsMissingPermission(t *testing.T) {
	called, result := runPermissionMiddleware(&dto.AuthPrincipal{
		Permissions: []string{constants.PermissionDeviceView.Code},
	}, constants.PermissionProductView)
	if called {
		t.Fatal("protected handler was called without product.view")
	}
	if result.ErrorCode != 3001 || result.Success {
		t.Fatalf("result = %+v, want forbidden error code 3001", result)
	}
}

func TestRequirePermissionMiddlewareAllowsMatchingPermission(t *testing.T) {
	called, result := runPermissionMiddleware(&dto.AuthPrincipal{
		Permissions: []string{constants.PermissionProductView.Code},
	}, constants.PermissionProductView)
	if !called || !result.Success {
		t.Fatalf("called = %v result = %+v, want allowed", called, result)
	}
}

func runPermissionMiddleware(principal *dto.AuthPrincipal, permission constants.Permission) (bool, struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"errorCode"`
}) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", principal)
		ctx.Next()
	})
	engine.Use(RequirePermissionMiddleware(permission))
	called := false
	engine.GET("/protected", func(ctx *gin.Context) {
		called = true
		ctx.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	var result struct {
		Success   bool `json:"success"`
		ErrorCode int  `json:"errorCode"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &result)
	return called, result
}
