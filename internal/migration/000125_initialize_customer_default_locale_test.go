package migration

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestInitializeCustomerDefaultLocaleCopiesTenantLocale(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open(filepath.Join(t.TempDir(), "customer-default-locale.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Tenant{}, &models.TenantBranding{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	tenant := &models.Tenant{Name: "Existing Tenant", DefaultLocale: "zh-CN", Status: enums.StatusOk}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	branding := &models.TenantBranding{TenantID: tenant.ID, DefaultLocale: "en-US", Status: enums.StatusOk}
	if err := db.Create(branding).Error; err != nil {
		t.Fatalf("create branding: %v", err)
	}

	if err := initializeCustomerDefaultLocale(db); err != nil {
		t.Fatalf("initialize customer default locale: %v", err)
	}
	if err := db.First(branding, "id = ?", branding.ID).Error; err != nil {
		t.Fatalf("reload branding: %v", err)
	}
	if branding.DefaultLocale != "zh-CN" {
		t.Fatalf("customer default locale = %q, want zh-CN", branding.DefaultLocale)
	}
}
