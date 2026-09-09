package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestEnterpriseSearchHonorsPermissionsAndEngineerProductScope(t *testing.T) {
	dbName := "enterprise_search_security_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.ProductLine{}, &models.AgentTeam{}, &models.AgentTeamMember{}, &models.AgentProfile{}); err != nil {
		t.Fatalf("migrate search models: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)
	now := time.Now()
	products := []models.Product{
		{TenantID: 1, Code: "PUMP-A", Name: "Pump Alpha", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, Code: "PUMP-B", Name: "Pump Beta", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&products).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}
	teams := []models.AgentTeam{
		{TenantID: 1, ProductID: products[0].ID, Name: "Alpha Team", TeamType: "product_repair", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, ProductID: products[1].ID, Name: "Beta Team", TeamType: "product_repair", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	profile := models.AgentProfile{
		TenantID: 1, UserID: 10, TeamID: teams[0].ID, AgentCode: "SEARCH-ENGINEER",
		DisplayName: "Search Engineer", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	membership := models.AgentTeamMember{
		TenantID: 1, TeamID: teams[0].ID, UserID: 10, DispatchEnabled: true, DispatchWeight: 1,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	operator := &dto.AuthPrincipal{
		TenantID: 1, UserID: 10, DomainType: models.DomainTypeEnterprise,
		Roles: []string{EnterpriseRoleEngineer}, Permissions: []string{constants.PermissionProductView.Code},
	}
	result, err := EnterpriseSearchService.Search(1, "Pump", []string{"product"}, operator)
	if err != nil {
		t.Fatalf("search products: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "product:"+formatID(products[0].ID) {
		t.Fatalf("engineer search crossed product scope: %+v", result.Items)
	}

	operator.Permissions = nil
	result, err = EnterpriseSearchService.Search(1, "Pump", []string{"product"}, operator)
	if err != nil {
		t.Fatalf("search without permission: %v", err)
	}
	if len(result.Items) != 0 || len(result.Scopes) != 0 {
		t.Fatalf("search ignored missing permission: %+v", result)
	}
}
