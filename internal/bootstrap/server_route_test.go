package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"remotehelpdesk/internal/pkg/config"
)

func TestNewServerRegistersGinRoutes(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	routes := make(map[string]bool)
	for _, route := range app.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	expected := []string{
		http.MethodGet + " /metrics",
		http.MethodPost + " /api/auth/login",
		http.MethodGet + " /api/config",
		http.MethodGet + " /api/tenant-branding/logo/:assetId",
		http.MethodGet + " /api/health",
		http.MethodGet + " /api/auth/oidc_login",
		http.MethodGet + " /api/auth/oidc_callback",
		http.MethodPost + " /api/auth/oidc_exchange",
		http.MethodGet + " /api/auth/profile",
		http.MethodPost + " /api/auth/profile/update",
		http.MethodGet + " /api/customer/service-code/resolve",
		http.MethodGet + " /api/customer/service-code/qr-image",
		http.MethodGet + " /api/customer/v1/me",
		http.MethodPost + " /api/customer/v1/presence/_heartbeat",
		http.MethodGet + " /api/customer/v1/account-deletion",
		http.MethodPost + " /api/customer/v1/account-deletion",
		http.MethodGet + " /api/customer/v1/home",
		http.MethodGet + " /api/customer/v1/devices/page",
		http.MethodGet + " /api/customer/v1/devices",
		http.MethodPost + " /api/customer/v1/devices/bind",
		http.MethodGet + " /api/customer/v1/devices/:id/access",
		http.MethodGet + " /api/customer/v1/devices/:id/manuals",
		http.MethodGet + " /api/customer/v1/conversations/page",
		http.MethodGet + " /api/customer/v1/conversations",
		http.MethodPost + " /api/customer/v1/conversations/:id/_translate",
		http.MethodPost + " /api/customer/v1/notifications/push-tokens",
		http.MethodPost + " /api/customer/v1/notifications/push-tokens/:id/_revoke",
		http.MethodGet + " /api/customer/v1/tickets/page",
		http.MethodGet + " /api/customer/v1/tickets",
		http.MethodGet + " /api/customer/v1/tickets/:id",
		http.MethodGet + " /api/customer/v1/meetings/page",
		http.MethodGet + " /api/customer/v1/meetings",
		http.MethodGet + " /api/partner/v1/conversations",
		http.MethodGet + " /api/partner/v1/tickets",
		http.MethodGet + " /api/partner/v1/tickets/:id",
		http.MethodPost + " /api/partner/v1/tickets/:id/progress",
		http.MethodPost + " /api/partner/v1/tickets/:id/upload_image",
		http.MethodPost + " /api/partner/v1/tickets/:id/upload_audio",
		http.MethodPost + " /api/partner/v1/tickets/:id/upload_attachment",
		http.MethodGet + " /api/partner/v1/profile",
		http.MethodGet + " /api/partner/v1/accounts",
		http.MethodPost + " /api/partner/v1/accounts",
		http.MethodPut + " /api/partner/v1/accounts/:accountId",
		http.MethodGet + " /api/partner/v1/meetings",
		http.MethodPost + " /api/partner/v1/tickets/:id/_accept",
		http.MethodPost + " /api/partner/v1/tickets/:id/_assign",
		http.MethodPost + " /api/partner/v1/tickets/:id/participants",
		http.MethodPost + " /api/partner/v1/tickets/:id/participants/:accountId/_remove",
		http.MethodPost + " /api/partner/v1/tickets/:id/_resolve",
		http.MethodGet + " /api/partner/v1/tickets/:id/meetings",
		http.MethodGet + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/join",
		http.MethodPost + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/_joined",
		http.MethodPost + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/_left",
		http.MethodPost + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/_heartbeat",
		http.MethodGet + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/status",
		http.MethodGet + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/transcripts",
		http.MethodGet + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/transcripts/page",
		http.MethodPost + " /api/partner/v1/supplier-collaborations/:id/meetings/:meetingId/transcripts",
		http.MethodGet + " /api/partner/v1/tickets/:id/meetings/:meetingId/join",
		http.MethodGet + " /api/customer/v1/meetings/:id/join",
		http.MethodGet + " /api/customer/v1/meetings/:id/transcripts/page",
		http.MethodPost + " /api/message/upload_audio",
		http.MethodGet + " /api/message/media/:assetId",
		http.MethodGet + " /api/media/:assetId",
		http.MethodGet + " /api/dashboard/user/list",
		http.MethodGet + " /api/dashboard/user/:id",
		http.MethodPost + " /api/dashboard/user/create",
		http.MethodGet + " /api/dashboard/product/list",
		http.MethodPost + " /api/dashboard/product/create",
		http.MethodGet + " /api/dashboard/product-service-profile/list",
		http.MethodGet + " /api/dashboard/product-service-profile/:id",
		http.MethodPost + " /api/dashboard/product-service-profile/create",
		http.MethodPost + " /api/dashboard/product-service-profile/update",
		http.MethodPost + " /api/dashboard/product-service-profile/delete",
		http.MethodPost + " /api/dashboard/product-service-profile/update_status",
		http.MethodGet + " /api/dashboard/tenant-integration-config/list",
		http.MethodGet + " /api/dashboard/tenant-integration-config/:id",
		http.MethodPost + " /api/dashboard/tenant-integration-config/create",
		http.MethodPost + " /api/dashboard/tenant-integration-config/update",
		http.MethodPost + " /api/dashboard/tenant-integration-config/delete",
		http.MethodPost + " /api/dashboard/tenant-integration-config/update_status",
		http.MethodGet + " /api/dashboard/product-ai-usage-credential/list",
		http.MethodGet + " /api/dashboard/product-ai-usage-credential/:id",
		http.MethodPost + " /api/dashboard/product-ai-usage-credential/create",
		http.MethodPost + " /api/dashboard/product-ai-usage-credential/update",
		http.MethodPost + " /api/dashboard/product-ai-usage-credential/delete",
		http.MethodPost + " /api/dashboard/product-ai-usage-credential/update_status",
		http.MethodPost + " /api/dashboard/meeting/preview-create",
		http.MethodPost + " /api/dashboard/conversation/upload_audio",
		http.MethodGet + " /api/dashboard/agent/self/work-schedule",
		http.MethodPost + " /api/dashboard/agent-team/member/delete",
		http.MethodPost + " /api/platform/tenant/logo/upload",
		http.MethodGet + " /api/dashboard/product-knowledge-binding/list",
		http.MethodGet + " /api/dashboard/product-knowledge-binding/:id",
		http.MethodPost + " /api/dashboard/product-knowledge-binding/create",
		http.MethodPost + " /api/dashboard/product-knowledge-binding/update",
		http.MethodPost + " /api/dashboard/product-knowledge-binding/delete",
		http.MethodPost + " /api/dashboard/product-knowledge-binding/update_status",
		http.MethodPost + " /api/dashboard/product-knowledge-binding/resolve",
		http.MethodGet + " /api/dashboard/product-model/list",
		http.MethodPost + " /api/dashboard/product-model/create",
		http.MethodGet + " /api/dashboard/device/list",
		http.MethodGet + " /api/dashboard/ticket/repair-history/list",
		http.MethodPost + " /api/dashboard/ticket/:id/takeover",
		http.MethodPost + " /api/dashboard/tickets/:id/takeover",
		http.MethodPost + " /api/dashboard/device/create",
		http.MethodGet + " /api/dashboard/service-code-batch/list",
		http.MethodPost + " /api/dashboard/service-code-batch/create",
		http.MethodGet + " /api/dashboard/service-code/list",
		http.MethodPost + " /api/dashboard/service-code/generate",
		http.MethodPost + " /api/dashboard/service-code/revoke",
		http.MethodPost + " /api/dashboard/conversation/send_message",
		http.MethodPost + " /api/dashboard/conversation/forward_image",
		http.MethodGet + " /api/dashboard/ai-config/speech_runtime",
		http.MethodPost + " /api/dashboard/ai-config/speech_runtime/update",
		http.MethodGet + " /api/dashboard/ai-workflow/default-definition",
		http.MethodGet + " /api/dashboard/ai-workflow/run/list",
		http.MethodGet + " /api/dashboard/ai-workflow/run/:id",
		http.MethodPost + " /api/dashboard/knowledge-index-generation/prepare",
		http.MethodGet + " /api/enterprise/v1/models/workspace",
		http.MethodGet + " /api/enterprise/v1/models/capabilities",
		http.MethodPut + " /api/enterprise/v1/models/default-llm-model",
		http.MethodPut + " /api/enterprise/v1/models/keys/:keyId/quota",
		http.MethodPost + " /api/enterprise/v1/models/keys/:keyId/reset-quota",
		http.MethodGet + " /api/enterprise/v1/ai-agents/summary",
		http.MethodGet + " /api/enterprise/v1/ai-agents",
		http.MethodPost + " /api/enterprise/v1/ai-agents/_ensure-product",
		http.MethodPost + " /api/enterprise/v1/ai-agents/_ensure-tenant-default",
		http.MethodGet + " /api/enterprise/v1/industry-solution-packs",
		http.MethodPost + " /api/enterprise/v1/industry-solution-packs/import",
		http.MethodPut + " /api/enterprise/v1/industry-solution-packs/:packId/draft",
		http.MethodPost + " /api/enterprise/v1/industry-solution-packs/:packId/_validate",
		http.MethodPost + " /api/enterprise/v1/industry-solution-packs/:packId/_publish",
		http.MethodPost + " /api/enterprise/v1/industry-solution-packs/:packId/_dry-run",
		http.MethodPost + " /api/enterprise/v1/industry-solution-packs/:packId/_apply",
		http.MethodGet + " /api/enterprise/v1/industry-solution-packs/applications/:applicationId",
		http.MethodGet + " /api/enterprise/v1/ai-workflows/summary",
		http.MethodGet + " /api/enterprise/v1/ai-workflows/adoption",
		http.MethodGet + " /api/enterprise/v1/ai-workflows",
		http.MethodGet + " /api/enterprise/v1/ai-workflows/:id",
		http.MethodGet + " /api/enterprise/v1/ai-workflows/:id/versions",
		http.MethodPost + " /api/enterprise/v1/ai-workflows",
		http.MethodPatch + " /api/enterprise/v1/ai-workflows/:id/draft",
		http.MethodDelete + " /api/enterprise/v1/ai-workflows/:id",
		http.MethodPost + " /api/enterprise/v1/ai-workflows/:id/_publish",
		http.MethodPost + " /api/enterprise/v1/ai-workflows/:id/versions/:versionId/_rollback",
		http.MethodPost + " /api/enterprise/v1/ai-workflows/:id/_test",
		http.MethodPatch + " /api/enterprise/v1/ai-agents/:id/workflow-binding",
		http.MethodGet + " /api/enterprise/v1/products/:id/models",
		http.MethodPost + " /api/enterprise/v1/products/:id/models",
		http.MethodPatch + " /api/enterprise/v1/products/:id/models/:modelId",
		http.MethodPost + " /api/enterprise/v1/products/:id/modules",
		http.MethodPatch + " /api/enterprise/v1/products/:id/modules/:moduleId",
		http.MethodGet + " /api/enterprise/v1/products/:id/manual-files",
		http.MethodGet + " /api/enterprise/v1/products/:id/manual-files/:manualFileId/content",
		http.MethodPost + " /api/enterprise/v1/products/:id/manual-files/_upload",
		http.MethodPatch + " /api/enterprise/v1/products/:id/manual-files/:manualFileId",
		http.MethodDelete + " /api/enterprise/v1/products/:id/manual-files/:manualFileId",
		http.MethodGet + " /api/enterprise/v1/products/:id/knowledge-documents",
		http.MethodPost + " /api/enterprise/v1/products/:id/knowledge-documents/_upload",
		http.MethodPost + " /api/enterprise/v1/products/:id/knowledge-documents/:documentId/_reprocess",
		http.MethodDelete + " /api/enterprise/v1/products/:id/knowledge-documents/:documentId",
		http.MethodGet + " /api/enterprise/v1/knowledge-bases/:knowledgeBaseId/documents",
		http.MethodPost + " /api/enterprise/v1/knowledge-bases/:knowledgeBaseId/documents/_upload",
		http.MethodPost + " /api/enterprise/v1/knowledge-bases/:knowledgeBaseId/documents/:documentId/_reprocess",
		http.MethodDelete + " /api/enterprise/v1/knowledge-bases/:knowledgeBaseId/documents/:documentId",
		http.MethodGet + " /api/enterprise/v1/products/:id/manuals",
		http.MethodPost + " /api/enterprise/v1/products/:id/manuals/_link",
		http.MethodPost + " /api/enterprise/v1/products/:id/quality-signals",
		http.MethodPatch + " /api/enterprise/v1/products/:id/quality-signals/:signalId",
		http.MethodPost + " /api/enterprise/v1/products/:id/quality-signals/:signalId/_resolve",
		http.MethodGet + " /api/enterprise/v1/diagnosis/fault-tree-nodes",
		http.MethodPost + " /api/enterprise/v1/diagnosis/fault-tree-nodes",
		http.MethodPatch + " /api/enterprise/v1/diagnosis/fault-tree-nodes/:nodeId",
		http.MethodGet + " /api/enterprise/v1/workbench/overview",
		http.MethodGet + " /api/enterprise/v1/workbench/core",
		http.MethodGet + " /api/enterprise/v1/workbench/collaboration",
		http.MethodGet + " /api/enterprise/v1/workbench/notifications",
		http.MethodGet + " /api/enterprise/v1/workbench/resources",
		http.MethodGet + " /api/enterprise/v1/workbench/queue",
		http.MethodGet + " /api/enterprise/v1/reminders/poll",
		http.MethodGet + " /api/enterprise/v1/reports/supplier-performance",
		http.MethodPost + " /api/enterprise/v1/conversations/:id/_translate",
		http.MethodGet + " /api/enterprise/v1/tickets/summary",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/progress",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/repair",
		http.MethodGet + " /api/enterprise/v1/tickets/:id/knowledge-candidates",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/knowledge-candidates",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/_takeover",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/takeover",
		http.MethodGet + " /api/enterprise/v1/tickets/:id/supplier-options",
		http.MethodGet + " /api/enterprise/v1/tickets/:id/supplier-collaborations/:collaborationId",
		http.MethodPost + " /api/enterprise/v1/tickets/:id/supplier-collaborations/:collaborationId/progress",
		http.MethodGet + " /api/enterprise/v1/devices",
		http.MethodPost + " /api/enterprise/v1/devices",
		http.MethodPost + " /api/enterprise/v1/devices/_batch-import",
		http.MethodGet + " /api/enterprise/v1/devices/:id",
		http.MethodPost + " /api/enterprise/v1/devices/:id",
		http.MethodGet + " /api/enterprise/v1/meetings",
		http.MethodGet + " /api/enterprise/v1/meetings/:id/join",
		http.MethodGet + " /api/enterprise/v1/meetings/:id/transcripts/page",
		http.MethodPost + " /api/enterprise/v1/meetings/:id/_end",
		http.MethodGet + " /api/enterprise/v1/notifications",
		http.MethodPost + " /api/enterprise/v1/notifications/push-tokens",
		http.MethodPost + " /api/enterprise/v1/notifications/push-tokens/:id/_revoke",
		http.MethodGet + " /api/enterprise/v1/iam/audit/export",
		http.MethodPost + " /api/enterprise/v1/notifications/:id/_read",
		http.MethodGet + " /api/enterprise/v1/iam/members",
		http.MethodPost + " /api/enterprise/v1/iam/members/invite",
		http.MethodPost + " /api/enterprise/v1/iam/members/:id/update",
		http.MethodPost + " /api/enterprise/v1/iam/members/:id/_status",
		http.MethodPost + " /api/enterprise/v1/iam/members/:id/portal-session",
		http.MethodGet + " /api/enterprise/v1/iam/customer-users",
		http.MethodPost + " /api/enterprise/v1/iam/customer-users/authorize",
		http.MethodPost + " /api/enterprise/v1/iam/customer-users/invite",
		http.MethodPost + " /api/enterprise/v1/iam/customer-users/:id/update",
		http.MethodPost + " /api/enterprise/v1/iam/customer-users/:id/_status",
		http.MethodPost + " /api/enterprise/v1/iam/customer-users/:id/portal-session",
		http.MethodGet + " /api/enterprise/v1/iam/partners",
		http.MethodPost + " /api/enterprise/v1/iam/partners/invite-admin",
		http.MethodPost + " /api/enterprise/v1/iam/partners/:id/update",
		http.MethodPost + " /api/enterprise/v1/iam/partners/:id/_status",
		http.MethodGet + " /api/enterprise/v1/iam/departments",
		http.MethodPost + " /api/enterprise/v1/iam/departments/create",
		http.MethodGet + " /api/enterprise/v1/iam/roles",
		http.MethodPost + " /api/enterprise/v1/iam/roles/save",
		http.MethodPost + " /api/enterprise/v1/iam/policy/save",
		http.MethodGet + " /api/enterprise/v1/iam/audit",
		http.MethodGet + " /api/platform/overview",
		http.MethodGet + " /api/platform/overview/metrics",
		http.MethodGet + " /api/platform/overview/top-tenants",
		http.MethodGet + " /api/platform/overview/usage-trend",
		http.MethodGet + " /api/platform/overview/events",
		http.MethodGet + " /api/platform/tenant/list",
		http.MethodPost + " /api/platform/tenant/create",
		http.MethodPost + " /api/platform/staff/invite",
		http.MethodPost + " /api/platform/staff/update",
		http.MethodPost + " /api/platform/staff/delete",
		http.MethodPost + " /api/platform/permission/role/delete",
		http.MethodGet + " /api/platform/usage/overview",
		http.MethodGet + " /api/platform/usage/sub2api/users",
		http.MethodGet + " /api/platform/usage/sub2api/users/:userId/api-keys",
		http.MethodGet + " /api/platform/ops/overview",
		http.MethodGet + " /api/platform/ops/runtime",
		http.MethodGet + " /api/platform/ops/database",
		http.MethodGet + " /api/platform/ops/infrastructure",
		http.MethodGet + " /api/platform/ops/jitsi",
		http.MethodGet + " /api/platform/ops/speech",
		http.MethodGet + " /api/platform/ops/translation",
		http.MethodGet + " /api/platform/ops/ar",
		http.MethodGet + " /api/platform/ops/sub2api",
		http.MethodGet + " /api/platform/ops/access",
		http.MethodGet + " /api/platform/ops/sync-queue",
		http.MethodGet + " /api/platform/ops/knowledge-queue",
		http.MethodGet + " /api/platform/ops/notification-queue",
		http.MethodGet + " /api/platform/integrations/config",
		http.MethodPost + " /api/platform/integrations/update",
		http.MethodPost + " /api/platform/integrations/test",
		http.MethodGet + " /api/ws/dashboard",
		http.MethodGet + " /api/ws/open",
	}
	for _, route := range expected {
		if !routes[route] {
			t.Fatalf("expected route %s to be registered", route)
		}
	}
}

func TestDashboardAIAgentStaticRoutesTakePriorityOverIDRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerDashboardAIAgentRoutes(engine.Group("/api/dashboard/ai-agent"))

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard/ai-agent/list_all", nil))

	if rec.Code == http.StatusBadRequest && strings.Contains(rec.Body.String(), "路径参数错误") {
		t.Fatalf("list_all was handled as an ai-agent id route: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewServerExposesPrometheusMetrics(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{Root: "storage", BaseURL: "/storage"},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health/live", nil))

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status=%d want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, metric := range []string{
		`http_requests_total{method="GET",route="/api/health/live",status="200"}`,
		"http_request_duration_seconds_bucket",
		"db_connection_pool_active",
		`application_dependency_up{dependency="redis"}`,
		"go_goroutines",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics response missing %q", metric)
		}
	}
}

func TestRedactSensitiveRequestPath(t *testing.T) {
	tests := map[string]string{
		"/api/third/jitsi/transcription/ws/super-secret/connection-7": "/api/third/jitsi/transcription/ws/<redacted>/connection-7",
		"/api/third/jitsi/transcription/events/super-secret":          "/api/third/jitsi/transcription/events/<redacted>",
		"/api/health/live": "/api/health/live",
	}
	for input, want := range tests {
		if got := redactSensitiveRequestPath(input); got != want {
			t.Fatalf("redactSensitiveRequestPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNewServerRegistersMCPOnlyWhenEnabledAndProtectsIt(t *testing.T) {
	config.SetCurrent(&config.Config{
		MCP: config.MCPConfig{Enabled: false},
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{Root: "storage", BaseURL: "/storage"},
		},
	})
	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	for _, route := range app.Routes() {
		if route.Path == "/api/mcp" {
			t.Fatal("MCP route must not be registered while MCP is disabled")
		}
	}

	config.SetCurrent(&config.Config{
		MCP: config.MCPConfig{Enabled: true, ServerToken: "mcp-test-token"},
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{Root: "storage", BaseURL: "/storage"},
		},
	})
	app, err = NewServer()
	if err != nil {
		t.Fatalf("NewServer() with MCP enabled error = %v", err)
	}
	registered := false
	for _, route := range app.Routes() {
		if route.Path == "/api/mcp" {
			registered = true
			break
		}
	}
	if !registered {
		t.Fatal("MCP route was not registered while MCP is enabled")
	}

	request := httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unauthenticated MCP status=%d want 404", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("Authorization", "Bearer mcp-test-token")
	recorder = httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusNotFound {
		t.Fatalf("authenticated MCP request was rejected: status=%d", recorder.Code)
	}
}

func TestNewServerHealthEndpointIsPublic(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("success=false, body=%s", rec.Body.String())
	}
	if body.Data.Status != "healthy" {
		t.Fatalf("status=%q want healthy", body.Data.Status)
	}
}

func TestNewServerExposesPublicConfig(t *testing.T) {
	config.SetCurrent(&config.Config{
		Language: "zh-CN",
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
		WxWork: config.WxWorkConfig{
			Enabled: true,
		},
		OIDC: config.OIDCConfig{
			Enabled:      false,
			ClientSecret: "must-not-leak",
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Language      string `json:"language"`
			WxWorkEnabled bool   `json:"wxworkEnabled"`
			OIDCEnabled   bool   `json:"oidcEnabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("success=false, body=%s", rec.Body.String())
	}
	if body.Data.Language != "zh-CN" {
		t.Fatalf("language=%q want zh-CN", body.Data.Language)
	}
	if !body.Data.WxWorkEnabled {
		t.Fatalf("wxworkEnabled=false want true")
	}
	if body.Data.OIDCEnabled {
		t.Fatalf("oidcEnabled=true want false")
	}
	if strings.Contains(rec.Body.String(), "must-not-leak") {
		t.Fatalf("response leaked sensitive OIDC config: %s", rec.Body.String())
	}
}

func TestNewServerDoesNotExposeLegacyAuthOptions(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/options", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestNewServerDoesNotServeSPA(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		path        string
		wantStatus  int
		contentType string
	}{
		{path: "/api/not-exists", wantStatus: http.StatusNotFound, contentType: "application/json"},
		{path: "/dashboard/not-exists", wantStatus: http.StatusNotFound, contentType: "application/json"},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if rec.Code != tt.wantStatus {
			t.Fatalf("%s status=%d want %d", tt.path, rec.Code, tt.wantStatus)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), tt.contentType) {
			t.Fatalf("%s Content-Type=%q want %q", tt.path, rec.Header().Get("Content-Type"), tt.contentType)
		}
	}
}

func TestNewServerAllowsConfiguredCORSOrigin(t *testing.T) {
	config.SetCurrent(&config.Config{
		Server: config.ServerConfig{
			CORS: config.CORSConfig{
				AllowedOrigins: []string{"https://console.example.com"},
			},
		},
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	req.Header.Set("Origin", "https://console.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization,x-tenant-id,idempotency-key,x-request-id")
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example.com" {
		t.Fatalf("Access-Control-Allow-Origin=%q want %q", got, "https://console.example.com")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods=%q should contain %q", got, http.MethodPost)
	}
	for _, header := range []string{"Authorization", "X-Tenant-Id", "Idempotency-Key", "X-Request-Id"} {
		if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), strings.ToLower(header)) {
			t.Fatalf("Access-Control-Allow-Headers=%q should contain %q", got, header)
		}
	}
	for _, header := range []string{"X-Request-Id", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if got := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(strings.ToLower(got), strings.ToLower(header)) {
			t.Fatalf("Access-Control-Expose-Headers=%q should contain %q", got, header)
		}
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary=%q want %q", got, "Origin")
	}
}

func TestNewServerRejectsUnconfiguredCORSOrigin(t *testing.T) {
	config.SetCurrent(&config.Config{
		Server: config.ServerConfig{
			CORS: config.CORSConfig{
				AllowedOrigins: []string{"https://console.example.com"},
			},
		},
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin=%q want empty", got)
	}
}

func TestNewServerEchoesRequestID(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/not-exists", nil)
	req.Header.Set("X-Request-Id", "trace-123")
	app.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "trace-123" {
		t.Fatalf("X-Request-Id=%q want %q", got, "trace-123")
	}
}

func TestNewServerGeneratesRequestID(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Local: config.LocalStorageConfig{
				Root:    "storage",
				BaseURL: "/storage",
			},
		},
	})

	app, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/not-exists", nil))

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatalf("X-Request-Id should be generated")
	}
}
