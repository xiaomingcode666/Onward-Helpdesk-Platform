package enterprise

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func TestProjectConfigurationHTTPContract(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	db := setupEnterpriseContractDB(t)
	if err := db.AutoMigrate(&models.DomainEvent{}, &models.OutboxRecord{}, &services.SLAPolicy{}, &services.ServiceCalendar{}, &models.TenantBranding{}, &models.TenantIntegrationConfig{}, &models.DataRegionPolicy{}, &models.TenantMailSetting{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Tenant{ID: 8001, Name: "Synthetic configuration tenant", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 8001, Environment: "development", Projects: []projectconfig.Project{{Key: "support", Name: "知识服务"}}, SecretRefs: []string{}}
	input := services.ProjectConfigDraft{Document: doc, Note: "test", RequestKey: "http-configuration-draft"}
	// A previously accepted draft with omitted/null arrays must not be persisted.
	for _, rawDocument := range []string{
		`{"schema_version":1,"tenant_id":8001,"environment":"development"}`,
		`{"schema_version":1,"tenant_id":8001,"environment":"development","projects":null,"secret_refs":[]}`,
	} {
		ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/configuration/drafts", 8001, `{"base_version_id":0,"request_key":"invalid-draft-arrays","note":"test","document":`+rawDocument+`}`)
		ProjectConfigurationDraft(ctx)
		if decodeEnterpriseEnvelope(t, rec).Success {
			t.Fatal("incomplete draft structure accepted")
		}
	}
	var drafts int64
	if err := db.Model(&models.ProjectConfigurationVersion{}).Count(&drafts).Error; err != nil || drafts != 0 {
		t.Fatalf("invalid draft persisted: %d, %v", drafts, err)
	}
	b, _ := json.Marshal(input)
	ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/configuration/drafts", 8001, string(b))
	ProjectConfigurationDraft(ctx)
	var draft services.ProjectConfigVersion
	decodeEnterpriseData(t, rec, &draft)
	ctx, rec = newEnterpriseContractJSONContext(http.MethodPost, "/configuration/apply", 8001, `{}`, gin.Param{Key: "version", Value: fmt.Sprint(draft.ID)})
	ProjectConfigurationApply(ctx)
	decodeEnterpriseData(t, rec, &draft)
	input.Note = "conflicting payload"
	b, _ = json.Marshal(input)
	ctx, rec = newEnterpriseContractJSONContext(http.MethodPost, "/configuration/drafts", 8001, string(b))
	ProjectConfigurationDraft(ctx)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", rec.Code)
	}
	for _, body := range []string{`{"document":{"password":"not-to-be-stored"}}`, `{} {}`, strings.Repeat("x", 310*1024)} {
		ctx, rec = newEnterpriseContractJSONContext(http.MethodPost, "/configuration/drafts", 8001, body)
		ProjectConfigurationDraft(ctx)
		if decodeEnterpriseEnvelope(t, rec).Success {
			t.Fatal("invalid payload accepted")
		}
	}
	for _, handler := range []gin.HandlerFunc{ProjectConfigurationGet, ProjectConfigurationDraft, ProjectConfigurationValidate, ProjectConfigurationApply} {
		ctx, rec = newEnterpriseContractJSONContext(http.MethodPost, "/configuration", 8001, `{}`)
		ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).Permissions = nil
		handler(ctx)
		if decodeEnterpriseEnvelope(t, rec).Success {
			t.Fatal("missing permission accepted")
		}
	}
	ctx, rec = newEnterpriseContractJSONContext(http.MethodPost, "/configuration/apply", 8002, `{}`, gin.Param{Key: "version", Value: fmt.Sprint(draft.ID)})
	ProjectConfigurationApply(ctx)
	if decodeEnterpriseEnvelope(t, rec).Success {
		t.Fatal("cross tenant activation accepted")
	}
}

// Opt-in local browser fixture: real configuration handlers and an isolated
// temporary SQLite DB. Authentication here is synthetic, not an IAM acceptance.
// This endpoint is compiled only by go test, never into the product server.
func TestProjectConfigurationBrowserFixture(t *testing.T) {
	address := os.Getenv("RHD_CONFIG_BROWSER_ADDRESS")
	if address == "" {
		t.Skip("opt-in browser fixture")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("fixture must bind to IPv4 loopback")
	}
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	secretRoot := t.TempDir()
	t.Setenv("RHD_PROJECT_SECRET_DIR", secretRoot)
	secretScope := filepath.Join(secretRoot, "8001", "development")
	if err := os.MkdirAll(secretScope, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config-shared", "config-mail", "config-app-key"} {
		if err := os.WriteFile(filepath.Join(secretScope, name), []byte("synthetic-browser-fixture-secret"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	db := setupEnterpriseContractDB(t)
	if err := db.AutoMigrate(&models.DomainEvent{}, &models.OutboxRecord{}, &services.SLAPolicy{}, &services.ServiceCalendar{}, &models.TenantBranding{}, &models.TenantIntegrationConfig{}, &models.DataRegionPolicy{}, &models.TenantMailSetting{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Tenant{ID: 8001, Name: "Synthetic configuration tenant", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	done := make(chan struct{}, 1)
	router.Use(func(ctx *gin.Context) {
		if ctx.GetHeader("Authorization") != "Bearer config-browser-fixture" {
			ctx.AbortWithStatus(401)
			return
		}
		fixture, _ := newEnterpriseContractContext(ctx.Request.Method, ctx.Request.URL.Path, 8001)
		for _, key := range []string{"tenantId", "authPrincipal", "middlewareAuthPrincipal"} {
			ctx.Set(key, fixture.MustGet(key))
		}
		ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).UserID = 9001
		ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).Roles = []string{services.EnterpriseRoleAdmin}
		ctx.Next()
	})
	router.GET("/api/enterprise/v1/ticket-settings/configuration", ProjectConfigurationGet)
	router.GET("/api/enterprise/v1/ticket-settings/configuration/upgrade", ProjectConfigurationUpgrade)
	router.POST("/api/enterprise/v1/ticket-settings/configuration/drafts", ProjectConfigurationDraft)
	router.POST("/api/enterprise/v1/ticket-settings/configuration/validate", ProjectConfigurationValidate)
	router.POST("/api/enterprise/v1/ticket-settings/configuration/:version/apply", ProjectConfigurationApply)
	router.POST("/__test/stop", func(ctx *gin.Context) {
		ctx.Status(204)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(router)
	server.Listener = listener
	server.Start()
	defer server.Close()
	t.Logf("configuration browser fixture listening at %s", server.URL)
	select {
	case <-done:
	case <-time.After(10 * time.Minute):
		t.Fatal("browser fixture timed out")
	}
}
