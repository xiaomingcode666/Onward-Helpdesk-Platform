package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func setupCaseOwnerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Tenant{}, &models.TenantMember{}, &models.AuthRole{}, &models.AuthRoleBinding{},
		&models.AuthRolePermission{}, &models.AuthSubjectPermissionOverride{}, &models.Ticket{}, &models.TicketProgress{}, &models.LoginSession{},
		&models.TicketCaseOperation{},
		&models.EngineerProfile{}, &models.AgentProfile{}, &models.AgentTeam{}, &models.Department{}, &models.AuthAuditLog{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if conn, err := db.DB(); err == nil {
			_ = conn.Close()
		}
	})
	if err := db.Create(&models.Tenant{ID: 91, Name: "Engineer fixture", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func seedCaseOwnerMember(t *testing.T, db *gorm.DB, tenantID int64, name string, permission string) (models.User, models.TenantMember, models.AuthRole) {
	t.Helper()
	user := models.User{Username: name, Nickname: name, Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	member := models.TenantMember{TenantID: tenantID, UserID: user.ID, DisplayName: name, MemberType: "employee", Status: enums.StatusOk}
	if err := db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	role := models.AuthRole{TenantID: tenantID, DomainType: models.DomainTypeEnterprise, Code: name, Name: name, Status: enums.StatusOk}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AuthRoleBinding{TenantID: tenantID, DomainType: models.DomainTypeEnterprise, RoleID: role.ID, SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID, Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AuthRolePermission{TenantID: tenantID, RoleID: role.ID, PermissionCode: permission, Effect: "allow", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	return user, member, role
}

func seedOwnedTicket(t *testing.T, db *gorm.DB, ownerID int64) models.Ticket {
	t.Helper()
	now := time.Now()
	ticket := models.Ticket{TenantID: 91, TicketNo: "OWNER-" + strings.ReplaceAll(t.Name(), "/", "-"), Title: "客户咨询",
		CaseOwnerID: ownerID, CaseStatus: "assigned", CaseRevision: 3, AcknowledgedAt: &now,
		Status: enums.TicketStatusAssigned, CurrentAssigneeID: 991, CurrentTeamID: 81, AssignedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	return ticket
}

// moveCaseToEngineer models an engineer taking over the case. The former
// support-agent handover endpoint was removed, so tests update the recorded
// handler directly, exactly like the dispatch/accept path does.
func moveCaseToEngineer(t *testing.T, db *gorm.DB, ticket models.Ticket, nextUserID int64, reason string) {
	t.Helper()
	if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).
		Updates(map[string]any{"case_owner_id": nextUserID, "case_revision": gorm.Expr("case_revision + 1"), "update_user_name": reason}).Error; err != nil {
		t.Fatal(err)
	}
}

func caseOwnerPrincipal(user models.User) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: user.ID, Username: user.Username, TenantID: 91, DomainType: models.DomainTypeEnterprise,
		Permissions: []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code}}
}

func TestCaseOwnerValidationRequiresActiveInternalPermission(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	valid, _, _ := seedCaseOwnerMember(t, db, 91, "valid", constants.PermissionTicketChangeStatus.Code)
	viewer, _, _ := seedCaseOwnerMember(t, db, 91, "viewer", constants.PermissionTicketView.Code)
	foreign, _, _ := seedCaseOwnerMember(t, db, 92, "foreign", constants.PermissionTicketUpdate.Code)
	disabled, _, _ := seedCaseOwnerMember(t, db, 91, "disabled", constants.PermissionTicketUpdate.Code)
	if err := db.Model(&disabled).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	denied, deniedMember, _ := seedCaseOwnerMember(t, db, 91, "denied", constants.PermissionTicketUpdate.Code)
	if err := db.Create(&models.AuthSubjectPermissionOverride{TenantID: 91, DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember,
		SubjectID: deniedMember.ID, PermissionCode: constants.PermissionTicketUpdate.Code, Effect: "deny", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	disabledRole, _, role := seedCaseOwnerMember(t, db, 91, "disabled-role", constants.PermissionTicketUpdate.Code)
	if err := db.Model(&role).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range []models.User{viewer, foreign, disabled, denied, disabledRole} {
		if err := db.Transaction(func(tx *gorm.DB) error { return ValidateCaseOwnerDB(tx, 91, user.ID) }); err == nil {
			t.Fatalf("ineligible handler %s was allowed", user.Username)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ValidateCaseOwnerDB(tx, 91, valid.ID) }); err != nil {
		t.Fatalf("eligible engineer rejected: %v", err)
	}
}

func TestCaseOwnerDisableAndDeleteRequireHandover(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, member, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(next)
	for _, operation := range []func() error{
		func() error { return UserService.UpdateStatus(owner.ID, int(enums.StatusDisabled), op) },
		func() error { return UserService.DeleteUser(owner.ID, op) },
		func() error {
			_, err := EnterpriseIAMService.UpdateMemberStatus(91, member.ID, request.EnterpriseMemberStatusRequest{Status: int(enums.StatusDisabled)}, op)
			return err
		},
		func() error { _, err := DSARService.deleteCustomerAccountDataDB(db, fmt.Sprint(owner.ID)); return err },
	} {
		if err := operation(); err == nil || !strings.Contains(err.Error(), "转派") {
			t.Fatalf("active case handler mutation should require reassignment, got %v", err)
		}
	}
	var stored models.User
	if err := db.First(&stored, owner.ID).Error; err != nil || stored.Status != enums.StatusOk || stored.Username != owner.Username {
		t.Fatalf("blocked account mutation changed user: %+v, %v", stored, err)
	}
	moveCaseToEngineer(t, db, ticket, next.ID, "转派给下一位工程师")
	if err := UserService.UpdateStatus(owner.ID, int(enums.StatusDisabled), op); err != nil {
		t.Fatalf("engineer who handed the case over should be disableable: %v", err)
	}
	if err := db.Model(&ticket).Update("case_status", "closed").Error; err != nil {
		t.Fatal(err)
	}
	if err := UserService.DeleteUser(next.ID, op); err != nil {
		t.Fatalf("completed handling history should permit account deletion: %v", err)
	}
}

func TestCaseOwnerPermissionRemovalIsRejectedAtomically(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, member, role := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	manager, _, _ := seedCaseOwnerMember(t, db, 91, "manager", constants.PermissionTicketUpdate.Code)
	seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(manager)
	if err := EnsureTenantDefaultIAMRolesDB(db, 91, op); err != nil {
		t.Fatal(err)
	}
	if _, err := EnterpriseIAMService.UpdateMember(91, member.ID, request.EnterpriseMemberUpdateRequest{RoleCodes: []string{EnterpriseRoleViewer}}, op); err == nil {
		t.Fatal("changing active handler to viewer should fail")
	}
	if _, _, err := PlatformIAMService.SaveAuthPolicy(request.PlatformAuthPolicySaveRequest{TenantID: 91, RoleID: role.ID, Effect: "allow", PermissionCodes: []string{constants.PermissionTicketView.Code}}, op); err == nil {
		t.Fatal("removing last processing permission should fail")
	}
	status := int(enums.StatusDisabled)
	if _, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{ID: role.ID, TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: role.Code, Name: role.Name, Status: &status}, op); err == nil {
		t.Fatal("disabling active handler's role should fail")
	}
	if err := PlatformIAMService.DeleteAuthRole(request.PlatformAuthRoleDeleteRequest{RoleID: role.ID}, op); err == nil {
		t.Fatal("deleting active handler's only role should fail")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ValidateCaseOwnerDB(tx, 91, owner.ID) }); err != nil {
		t.Fatalf("rejected edits must leave original handler permissions valid: %v", err)
	}
}

func TestCaseOwnerHistoricalMissingOwnerIsNeverInferred(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, 0)
	ticket.CurrentAssigneeID = owner.ID
	ticket.CreateUserID = owner.ID
	if err := db.Save(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := RequireTicketCaseOwnerDB(db, &ticket); err == nil {
		t.Fatal("missing handler must require an explicit engineer acceptance")
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil || stored.CaseOwnerID != 0 {
		t.Fatalf("historical handler was invented: %+v, %v", stored, err)
	}
}

func TestCaseOwnerDefaultRoleRefreshCannotRemoveLastProcessingPermission(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, member, viewer := seedCaseOwnerMember(t, db, 91, EnterpriseRoleViewer, constants.PermissionTicketUpdate.Code)
	next, _, custom := seedCaseOwnerMember(t, db, 91, "custom-service", constants.PermissionTicketChangeStatus.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(next)
	// A different builtin role is processed earlier than the viewer. Its name
	// must also roll back if a later role would invalidate active handling.
	engineer := models.AuthRole{TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: EnterpriseRoleEngineer,
		Name: "Existing engineer name", Status: enums.StatusOk, IsBuiltin: true}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&viewer).Update("is_builtin", true).Error; err != nil {
		t.Fatal(err)
	}
	// No caller-owned transaction: the refresh must provide its own rollback.
	if err := EnsureTenantDefaultIAMRolesDB(db, 91, op); err == nil || !strings.Contains(err.Error(), "转派") {
		t.Fatalf("default policy refresh must reject removing an active handler's last permission: %v", err)
	}
	if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
		t.Fatalf("failed refresh removed handler permissions: %v", err)
	}
	var storedEngineer models.AuthRole
	if err := db.First(&storedEngineer, engineer.ID).Error; err != nil || storedEngineer.Name != engineer.Name {
		t.Fatalf("failed batch partially changed an earlier role: %+v, %v", storedEngineer, err)
	}
	var inserted int64
	if err := db.Model(&models.AuthRole{}).Where("tenant_id = ? AND code = ?", 91, EnterpriseRoleOwner).Count(&inserted).Error; err != nil || inserted != 0 {
		t.Fatalf("failed refresh retained a newly created role: %d, %v", inserted, err)
	}
	// Once the case is moved to another engineer, the same refresh can run
	// inside a caller transaction without touching custom role permissions.
	moveCaseToEngineer(t, db, ticket, next.ID, "转派后恢复默认角色")
	if err := db.Transaction(func(tx *gorm.DB) error { return EnsureTenantDefaultIAMRolesDB(tx, 91, op) }); err != nil {
		t.Fatalf("refresh after reassignment should succeed, including nested transaction use: %v", err)
	}
	permissions, err := caseOwnerPermissionsDB(db, 91, member.ID, owner.ID)
	if err != nil || hasCaseOwnerPermissions(permissions) {
		t.Fatalf("viewer should now have default read-only permissions: %+v, %v", permissions, err)
	}
	if err := ValidateCaseOwnerDB(db, 91, next.ID); err != nil {
		t.Fatalf("custom role handler was affected by builtin refresh: %v", err)
	}
	var storedCustom models.AuthRole
	if err := db.First(&storedCustom, custom.ID).Error; err != nil || storedCustom.Name != custom.Name || storedCustom.IsBuiltin {
		t.Fatalf("builtin refresh modified a custom role: %+v, %v", storedCustom, err)
	}
}

func TestCaseOwnerLegacyPermissionRemovalAlsoRequiresHandover(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	if err := db.AutoMigrate(&models.Role{}, &models.UserRole{}, &models.Permission{}, &models.RolePermission{}); err != nil {
		t.Fatal(err)
	}
	owner := models.User{Username: "legacy-owner", Status: enums.StatusOk}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	member := models.TenantMember{TenantID: 91, UserID: owner.ID, MemberType: "employee", Status: enums.StatusOk}
	role := models.Role{TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: "legacy-service", Name: "Legacy service", Status: enums.StatusOk}
	permission := models.Permission{Code: constants.PermissionTicketUpdate.Code, Name: "Update tickets", Status: enums.StatusOk}
	for _, model := range []any{&member, &role, &permission} {
		if err := db.Create(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.UserRole{TenantID: 91, DomainType: models.DomainTypeEnterprise, UserID: owner.ID, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RolePermission{TenantID: 91, DomainType: models.DomainTypeEnterprise, RoleID: role.ID, PermissionID: permission.ID}).Error; err != nil {
		t.Fatal(err)
	}
	op := caseOwnerPrincipal(owner)
	seedOwnedTicket(t, db, owner.ID)
	if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
		t.Fatalf("legacy scoped permission should remain supported: %v", err)
	}
	for _, operation := range []func() error{
		func() error { return UserService.AssignRoles(owner.ID, nil, op) },
		func() error { return RoleService.AssignPermissions(role.ID, nil, op) },
		func() error { return RoleService.UpdateStatus(role.ID, enums.StatusDisabled, op) },
	} {
		if err := operation(); err == nil || !strings.Contains(err.Error(), "转派") {
			t.Fatalf("legacy permissions cannot be revoked before reassignment: %v", err)
		}
		if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
			t.Fatalf("rejected legacy mutation must roll back permissions: %v", err)
		}
	}
}
