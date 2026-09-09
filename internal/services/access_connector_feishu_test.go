package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupFeishuConnectorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:access_connector_feishu?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AccessConnector{}); err != nil {
		t.Fatalf("migrate connector: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)
	return db
}

func TestFeishuConnectorStoresEncryptedCredentialAndBuildsOfficialAuthRequest(t *testing.T) {
	db := setupFeishuConnectorTestDB(t)
	credential := `{"appId":"cli_test","appSecret":"secret-value"}`
	connector := &models.AccessConnector{
		TenantID: 8, Name: "飞书通知", ConnectorType: models.ConnectorTypeFeishu,
		BaseURL: "https://open.feishu.cn/open-apis", AuthType: "app_credentials", AuthConfig: credential,
	}
	if err := AccessConnectorService.CreateConnector(context.Background(), connector); err != nil {
		t.Fatalf("create feishu connector: %v", err)
	}
	var persisted models.AccessConnector
	if err := db.First(&persisted, connector.ID).Error; err != nil {
		t.Fatalf("reload connector: %v", err)
	}
	if persisted.AuthConfig == credential || !strings.HasPrefix(persisted.AuthConfig, "enc:v1:") {
		t.Fatalf("credential was not encrypted: %q", persisted.AuthConfig)
	}

	request, err := AccessConnectorService.buildTestRequest(&persisted)
	if err != nil {
		t.Fatalf("build feishu test request: %v", err)
	}
	if request.Method != "POST" || request.URL != "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" {
		t.Fatalf("unexpected feishu test request: %+v", request)
	}
	var body map[string]string
	if err := json.Unmarshal(request.Data, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body["app_id"] != "cli_test" || body["app_secret"] != "secret-value" {
		t.Fatalf("unexpected request body: %+v", body)
	}
	if err := validateConnectorTestResponse(&persisted, 200, []byte(`{"code":0,"msg":"ok","tenant_access_token":"token","expire":7200}`)); err != nil {
		t.Fatalf("validate successful response: %v", err)
	}
	if err := validateConnectorTestResponse(&persisted, 200, []byte(`{"code":10003,"msg":"invalid app secret"}`)); err == nil {
		t.Fatal("expected invalid credential response to fail")
	}
}

func TestFeishuConnectorRejectsUnofficialEndpointAndResetsHealthOnUpdate(t *testing.T) {
	db := setupFeishuConnectorTestDB(t)
	credential := `{"appId":"cli_test","appSecret":"secret-value"}`
	blocked := &models.AccessConnector{
		TenantID: 8, Name: "飞书通知", ConnectorType: models.ConnectorTypeFeishu,
		BaseURL: "https://example.com/open-apis", AuthType: "app_credentials", AuthConfig: credential,
	}
	if err := AccessConnectorService.CreateConnector(context.Background(), blocked); err == nil {
		t.Fatal("expected unofficial feishu endpoint to be rejected")
	}

	connector := &models.AccessConnector{
		TenantID: 8, Name: "飞书通知", ConnectorType: models.ConnectorTypeFeishu,
		BaseURL: "https://open.feishu.cn/open-apis", AuthType: "app_credentials", AuthConfig: credential,
		Status: "active", HealthStatus: "healthy",
	}
	if err := AccessConnectorService.CreateConnector(context.Background(), connector); err != nil {
		t.Fatalf("create feishu connector: %v", err)
	}
	updatedCredential := `{"appId":"cli_updated","appSecret":"updated-secret"}`
	if err := AccessConnectorService.UpdateConnector(context.Background(), &models.AccessConnector{
		ID: connector.ID, TenantID: 8, Name: "飞书国际通知", ConnectorType: models.ConnectorTypeFeishu,
		BaseURL: "https://open.larksuite.com/open-apis", AuthType: "app_credentials", AuthConfig: updatedCredential,
	}); err != nil {
		t.Fatalf("update feishu connector: %v", err)
	}
	var persisted models.AccessConnector
	if err := db.First(&persisted, connector.ID).Error; err != nil {
		t.Fatalf("reload updated connector: %v", err)
	}
	plain, err := secretstore.Decrypt(persisted.AuthConfig)
	if err != nil {
		t.Fatalf("decrypt updated credential: %v", err)
	}
	if plain != updatedCredential || persisted.Status != "inactive" || persisted.HealthStatus != "unknown" || persisted.LastTestedAt != nil {
		t.Fatalf("unexpected updated connector: %+v, credential=%q", persisted, plain)
	}
	if err := AccessConnectorService.UpdateConnector(context.Background(), &models.AccessConnector{
		ID: connector.ID, TenantID: 9, Name: "cross tenant", ConnectorType: models.ConnectorTypeFeishu,
		BaseURL: "https://open.feishu.cn/open-apis", AuthType: "app_credentials", AuthConfig: credential,
	}); err == nil {
		t.Fatal("expected cross-tenant update to fail")
	}
}
