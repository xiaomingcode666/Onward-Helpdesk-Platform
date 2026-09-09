package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func TestEnterpriseServiceRoutesRequireActionPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/enterprise/v1")
	group.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", &dto.AuthPrincipal{TenantID: 1, UserID: 99})
		ctx.Next()
	})
	registerEnterpriseTicketRoutes(group)
	registerEnterpriseMeetingRoutes(group)
	registerEnterpriseSLARoutes(group)
	registerEnterpriseDiagnosisRoutes(group)
	registerEnterpriseGDPRRoutes(group)
	registerEnterpriseAccessConnectorRoutes(group)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/enterprise/v1/tickets"},
		{http.MethodPost, "/api/enterprise/v1/tickets"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/progress"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/_takeover"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/_assign"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/_close"},
		{http.MethodGet, "/api/enterprise/v1/tickets/1/supplier-options"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/supplier-collaborations"},
		{http.MethodGet, "/api/enterprise/v1/meetings"},
		{http.MethodPost, "/api/enterprise/v1/tickets/1/meetings"},
		{http.MethodPost, "/api/enterprise/v1/meetings/meeting-1/_end"},
		{http.MethodGet, "/api/enterprise/v1/sla/policies"},
		{http.MethodPost, "/api/enterprise/v1/sla/policy/create"},
		{http.MethodPatch, "/api/enterprise/v1/sla/policy/policy-1"},
		{http.MethodPost, "/api/enterprise/v1/sla/pause"},
		{http.MethodGet, "/api/enterprise/v1/ticket/1/sla-timeline"},
		{http.MethodGet, "/api/enterprise/v1/diagnosis/fault-tree-nodes"},
		{http.MethodPost, "/api/enterprise/v1/diagnosis/start"},
		{http.MethodGet, "/api/enterprise/v1/diagnosis/session-1/summary"},
		{http.MethodGet, "/api/enterprise/v1/gdpr/dsar/list"},
		{http.MethodPost, "/api/enterprise/v1/gdpr/dsar/dsar-1/execute"},
		{http.MethodPost, "/api/enterprise/v1/gdpr/breach/report"},
		{http.MethodGet, "/api/enterprise/v1/gdpr/breach/list"},
		{http.MethodGet, "/api/enterprise/v1/access/connectors"},
		{http.MethodPost, "/api/enterprise/v1/access/connector/update"},
		{http.MethodPost, "/api/enterprise/v1/access/connector/1/call"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			var result struct {
				Success   bool `json:"success"`
				ErrorCode int  `json:"errorCode"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode response: %v, body=%s", err, recorder.Body.String())
			}
			if result.Success || result.ErrorCode != 3001 {
				t.Fatalf("response = %+v, want permission denied", result)
			}
		})
	}
}

func TestEnterpriseProductUpdateRequiresUpdatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/enterprise/v1")
	group.Use(func(ctx *gin.Context) {
		ctx.Set("authPrincipal", &dto.AuthPrincipal{
			TenantID: 1,
			UserID:   99,
			Permissions: []string{
				constants.PermissionProductView.Code,
			},
		})
		ctx.Next()
	})
	registerEnterpriseProductRoutes(group)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/api/enterprise/v1/products/1", nil))
	var result struct {
		Success   bool `json:"success"`
		ErrorCode int  `json:"errorCode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, recorder.Body.String())
	}
	if result.Success || result.ErrorCode != 3001 {
		t.Fatalf("response = %+v, want permission denied", result)
	}
}
