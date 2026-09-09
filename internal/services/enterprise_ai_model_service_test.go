package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
)

func TestEnterpriseAIModelSyncProductKeySnapshotsUpdatesOnlyRequestedProducts(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	if err := db.AutoMigrate(&models.ProductAIUsageCredential{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	credentials := []models.ProductAIUsageCredential{
		{
			TenantID: fixture.Tenant.ID, ProductID: 101, Sub2APIKeyID: "501", KeyName: "old-a",
			QuotaLimit: 100, QuotaUsed: 10, Currency: "USD", Status: enums.StatusOk,
		},
		{
			TenantID: fixture.Tenant.ID, ProductID: 102, Sub2APIKeyID: "502", KeyName: "old-b",
			QuotaLimit: 100, QuotaUsed: 20, Currency: "USD", Status: enums.StatusOk,
		},
	}
	if err := db.Create(&credentials).Error; err != nil {
		t.Fatalf("create product key credentials: %v", err)
	}

	err := EnterpriseAIModelService.syncProductKeySnapshots(fixture.Tenant.ID, []int64{101}, []providers.Sub2APIKey{
		{ID: 501, Name: "product-a", Quota: 200, QuotaUsed: 190},
		{ID: 502, Name: "product-b", Quota: 300, QuotaUsed: 250},
	})
	if err != nil {
		t.Fatalf("syncProductKeySnapshots() error = %v", err)
	}

	var updatedA, untouchedB models.ProductAIUsageCredential
	if err := db.First(&updatedA, credentials[0].ID).Error; err != nil {
		t.Fatalf("load updated credential: %v", err)
	}
	if err := db.First(&untouchedB, credentials[1].ID).Error; err != nil {
		t.Fatalf("load untouched credential: %v", err)
	}
	if updatedA.KeyName != "product-a" || updatedA.QuotaLimit != 200 || updatedA.QuotaUsed != 190 || updatedA.LastSyncedAt == nil {
		t.Fatalf("requested product snapshot not updated: %+v", updatedA)
	}
	if !strings.Contains(updatedA.QuotaPolicyJSON, `"quota_used":190`) {
		t.Fatalf("quota policy snapshot not refreshed: %s", updatedA.QuotaPolicyJSON)
	}
	if untouchedB.KeyName != "old-b" || untouchedB.QuotaLimit != 100 || untouchedB.QuotaUsed != 20 || untouchedB.LastSyncedAt != nil {
		t.Fatalf("out-of-scope product snapshot changed: %+v", untouchedB)
	}
}

func TestProductKeySnapshotsAreFresh(t *testing.T) {
	now := time.Now()
	recent := now.Add(-30 * time.Second)
	stale := now.Add(-2 * time.Minute)

	if !productKeySnapshotsAreFresh([]models.ProductAIUsageCredential{{LastSyncedAt: &recent}}, now) {
		t.Fatal("recent product key snapshot should be fresh")
	}
	if productKeySnapshotsAreFresh([]models.ProductAIUsageCredential{{LastSyncedAt: &stale}}, now) {
		t.Fatal("old product key snapshot should be stale")
	}
	if productKeySnapshotsAreFresh([]models.ProductAIUsageCredential{{}}, now) {
		t.Fatal("missing product key snapshot timestamp should be stale")
	}
}

func TestParseProductIDFromSub2APIKeyName(t *testing.T) {
	tests := []struct {
		name string
		want int64
	}{
		{name: "RHD-PRODUCT-15", want: 15},
		{name: " rhd-product-2 ", want: 2},
		{name: "Default Tenant", want: 0},
		{name: "RHD-PRODUCT-15-backup", want: 0},
	}

	for _, tt := range tests {
		if got := parseProductIDFromSub2APIKeyName(tt.name); got != tt.want {
			t.Fatalf("parseProductIDFromSub2APIKeyName(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}
