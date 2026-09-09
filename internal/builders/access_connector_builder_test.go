package builders

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
)

func TestBuildAccessConnectorListUsesFrontendContractWithoutCredentials(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	result := BuildAccessConnectorList([]*models.AccessConnector{{
		ID: 4, TenantID: 8, Name: "ERP", ConnectorType: "api", BaseURL: "https://erp.example.com",
		AuthType: "bearer", AuthConfig: "secret", Status: "active", HealthStatus: "healthy", LastTestedAt: &now, CreatedAt: now,
	}})
	if len(result) != 1 {
		t.Fatalf("result length = %d", len(result))
	}
	item := result[0]
	if item.ID != 4 || item.Name != "ERP" || !item.Active || item.HealthStatus != "healthy" || item.LastHealthCheckAt == "" {
		t.Fatalf("unexpected connector DTO: %+v", item)
	}
}
