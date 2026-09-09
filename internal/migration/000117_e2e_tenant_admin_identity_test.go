package migration

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupE2ETenantAdminIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Tenant{},
		&models.Department{},
		&models.AgentTeam{},
		&models.TenantMember{},
		&models.AuthRole{},
		&models.AuthRolePermission{},
		&models.AuthRoleBinding{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&models.Tenant{ID: 1, Name: "Default Tenant", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return db
}

func TestRepairE2ETenantAdminIdentityCreatesEnterpriseOwnerIdentity(t *testing.T) {
	db := setupE2ETenantAdminIdentityTestDB(t)
	user := &models.User{
		Username: e2eTenantAdminUsername,
		Nickname: "E2E Admin",
		Password: "existing-hash",
		Status:   enums.StatusDisabled,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := repairE2ETenantAdminIdentity(db); err != nil {
		t.Fatalf("repair identity: %v", err)
	}

	storedUser := repositories.UserRepository.Get(db, user.ID)
	if storedUser == nil || storedUser.Status != enums.StatusOk || storedUser.Password != "existing-hash" {
		t.Fatalf("user was not safely restored without password mutation: %+v", storedUser)
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, 1, user.ID)
	if member == nil || member.Status != enums.StatusOk || member.MemberType != "owner" || member.DepartmentID <= 0 {
		t.Fatalf("tenant member was not repaired: %+v", member)
	}
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, 1, models.DomainTypeEnterprise, services.EnterpriseRoleOwner)
	if role == nil {
		t.Fatalf("tenant owner role was not seeded")
	}
	var binding models.AuthRoleBinding
	if err := db.Where(
		"tenant_id = ? AND domain_type = ? AND role_id = ? AND subject_type = ? AND subject_id = ? AND status = ?",
		int64(1), models.DomainTypeEnterprise, role.ID, models.SubjectTypeTenantMember, member.ID, enums.StatusOk,
	).First(&binding).Error; err != nil {
		t.Fatalf("tenant owner binding was not repaired: %v", err)
	}
}

func TestRepairE2ETenantAdminIdentitySkipsMissingUser(t *testing.T) {
	db := setupE2ETenantAdminIdentityTestDB(t)

	if err := repairE2ETenantAdminIdentity(db); err != nil {
		t.Fatalf("repair identity: %v", err)
	}

	var count int64
	db.Model(&models.TenantMember{}).Count(&count)
	if count != 0 {
		t.Fatalf("missing E2E admin should not create a member, got %d", count)
	}
}
