package migration

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNormalizeTechnicalAfterSalesOrganizationNames(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "technical-after-sales.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AgentTeam{}, &models.Department{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	systemKey := "technical-repair"
	rows := []any{
		&models.AgentTeam{
			TenantID:      1,
			TeamType:      services.AgentTeamTypeTechnicalRepair,
			SystemKey:     &systemKey,
			SystemManaged: true,
			Name:          legacyTechnicalRepairTeamName,
			Status:        enums.StatusOk,
			AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		&models.Department{
			TenantID:       1,
			DepartmentCode: "technical-repair",
			Name:           legacyTechnicalRepairTeamName,
			Path:           legacyTechnicalRepairTeamPath,
			Depth:          1,
			Status:         enums.StatusOk,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		&models.Department{
			TenantID:       1,
			DepartmentCode: "product-7",
			Name:           "产品 A",
			Path:           legacyTechnicalRepairTeamPath + "/产品 A",
			Depth:          2,
			Status:         enums.StatusOk,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}

	if err := normalizeTechnicalAfterSalesOrganizationNames(db); err != nil {
		t.Fatalf("normalizeTechnicalAfterSalesOrganizationNames() error = %v", err)
	}
	if err := normalizeTechnicalAfterSalesOrganizationNames(db); err != nil {
		t.Fatalf("idempotent normalizeTechnicalAfterSalesOrganizationNames() error = %v", err)
	}

	var team models.AgentTeam
	if err := db.First(&team, "team_type = ?", services.AgentTeamTypeTechnicalRepair).Error; err != nil {
		t.Fatalf("load team: %v", err)
	}
	if team.Name != services.TechnicalAfterSalesTeamName {
		t.Fatalf("team name = %q, want %q", team.Name, services.TechnicalAfterSalesTeamName)
	}
	var rootDepartment models.Department
	if err := db.First(&rootDepartment, "department_code = ?", "technical-repair").Error; err != nil {
		t.Fatalf("load root department: %v", err)
	}
	if rootDepartment.Name != services.TechnicalAfterSalesTeamName || rootDepartment.Path != services.TechnicalAfterSalesTeamPath {
		t.Fatalf("root department = %+v", rootDepartment)
	}
	var productDepartment models.Department
	if err := db.First(&productDepartment, "department_code = ?", "product-7").Error; err != nil {
		t.Fatalf("load product department: %v", err)
	}
	if productDepartment.Path != services.TechnicalAfterSalesTeamPath+"/产品 A" {
		t.Fatalf("product department path = %q", productDepartment.Path)
	}
}
