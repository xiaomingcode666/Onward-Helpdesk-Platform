package services

import (
	"context"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestAccessConnectorEncryptsAndRedactsCredential(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:access_connector_security?mode=memory&cache=shared"), &gorm.Config{})
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

	connector := &models.AccessConnector{
		TenantID: 1, Name: "secured connector", ConnectorType: "openapi",
		BaseURL: "https://example.com", AuthType: "bearer", AuthConfig: "connector-secret-value",
	}
	if err := AccessConnectorService.CreateConnector(context.Background(), connector); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	var persisted models.AccessConnector
	if err := db.First(&persisted, connector.ID).Error; err != nil {
		t.Fatalf("reload connector: %v", err)
	}
	if persisted.AuthConfig == "connector-secret-value" || !strings.HasPrefix(persisted.AuthConfig, "enc:v1:") {
		t.Fatalf("credential was not encrypted: %q", persisted.AuthConfig)
	}
	headers, err := AccessConnectorService.buildAuthHeaders(&persisted)
	if err != nil {
		t.Fatalf("decrypt connector credential: %v", err)
	}
	if headers["Authorization"] != "Bearer connector-secret-value" {
		t.Fatalf("authorization header = %q", headers["Authorization"])
	}
	listed, err := AccessConnectorService.ListConnectors(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("list connectors: %v", err)
	}
	if len(listed) != 1 || listed[0].AuthConfig != "" {
		t.Fatalf("connector credential leaked in list: %+v", listed)
	}
	if _, err := AccessConnectorService.GetConnectorStatus(context.Background(), 2, connector.ID); err == nil {
		t.Fatal("expected cross-tenant connector read to fail")
	}
	blocked := &models.AccessConnector{
		TenantID: 1, Name: "blocked connector", ConnectorType: "openapi",
		BaseURL: "http://127.0.0.1:8080", AuthType: "bearer", AuthConfig: "secret",
	}
	if err := AccessConnectorService.CreateConnector(context.Background(), blocked); err == nil {
		t.Fatal("expected loopback connector URL to be rejected")
	}
	if err := db.Model(&models.AccessConnector{}).Where("id = ?", connector.ID).Update("status", "testing").Error; err != nil {
		t.Fatalf("activate connector: %v", err)
	}
	if _, err := AccessConnectorService.CallConnector(context.Background(), 1, connector.ID, ConnectorRequest{
		Method: "GET", Path: "/health", Headers: map[string]string{"Authorization": "override"},
	}); err == nil {
		t.Fatal("expected caller auth header override to be rejected")
	}
}
