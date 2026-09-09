package migration

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCorrectODTPortalBrandNameUsesExactDomain(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open(filepath.Join(t.TempDir(), "odt-brand-name.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.TenantBranding{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	brandings := []models.TenantBranding{
		{TenantID: 15, BrandName: "ODE", CustomDomain: "https://www.odt.ae/", Status: enums.StatusOk},
		{TenantID: 16, BrandName: "Other", CustomDomain: "https://odt.ae.example.com", Status: enums.StatusOk},
		{TenantID: 17, BrandName: "Deleted ODE", CustomDomain: "odt.ae", Status: enums.StatusDeleted},
	}
	if err := db.Create(&brandings).Error; err != nil {
		t.Fatalf("create brandings: %v", err)
	}

	if err := correctODTPortalBrandName(db); err != nil {
		t.Fatalf("correct ODT brand name: %v", err)
	}
	var stored []models.TenantBranding
	if err := db.Order("tenant_id ASC").Find(&stored).Error; err != nil {
		t.Fatalf("reload brandings: %v", err)
	}
	if stored[0].BrandName != "ODT" {
		t.Fatalf("ODT brand name = %q, want ODT", stored[0].BrandName)
	}
	if stored[1].BrandName != "Other" {
		t.Fatalf("near-match domain was changed: %q", stored[1].BrandName)
	}
	if stored[2].BrandName != "Deleted ODE" {
		t.Fatalf("deleted branding was changed: %q", stored[2].BrandName)
	}
}
