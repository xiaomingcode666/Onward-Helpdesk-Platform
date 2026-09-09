package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func TestIndustrySolutionPackRoutesRequireProductPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/enterprise/v1")
	group.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", &dto.AuthPrincipal{TenantID: 1, UserID: 99, DomainType: "enterprise"})
		ctx.Next()
	})
	registerEnterpriseProductRoutes(group)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/enterprise/v1/industry-solution-packs", ""},
		{http.MethodPost, "/api/enterprise/v1/industry-solution-packs/import", `{}`},
		{http.MethodPut, "/api/enterprise/v1/industry-solution-packs/1/draft", `{}`},
		{http.MethodPost, "/api/enterprise/v1/industry-solution-packs/1/_validate", ""},
		{http.MethodPost, "/api/enterprise/v1/industry-solution-packs/1/_publish", ""},
		{http.MethodPost, "/api/enterprise/v1/industry-solution-packs/1/_dry-run", `{}`},
		{http.MethodPost, "/api/enterprise/v1/industry-solution-packs/1/_apply", `{}`},
		{http.MethodGet, "/api/enterprise/v1/industry-solution-packs/applications/1", ""},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			engine.ServeHTTP(recorder, request)
			var response struct {
				Success   bool `json:"success"`
				ErrorCode int  `json:"errorCode"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v, body=%s", err, recorder.Body.String())
			}
			if response.Success || response.ErrorCode != 3001 {
				t.Fatalf("response = %+v, want permission denied", response)
			}
		})
	}
}
