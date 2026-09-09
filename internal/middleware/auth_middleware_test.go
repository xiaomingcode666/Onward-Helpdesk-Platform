package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func TestAuthMiddlewareReturnsUnauthorizedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/enterprise/v1/products", nil)

	AuthMiddleware(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var result web.JsonResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.ErrorCode != errorsx.CodeAuthUnauthorized {
		t.Fatalf("errorCode = %d, want %d", result.ErrorCode, errorsx.CodeAuthUnauthorized)
	}
}
