package services

import (
	"encoding/json"
	"errors"
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
	if err := db.Create(&models.Tenant{ID: 91, Name: "Owner fixture", Status: enums.StatusOk}).Error; err != nil {
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

func caseOwnerPrincipal(user models.User) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: user.ID, Username: user.Username, TenantID: 91, DomainType: models.DomainTypeEnterprise,
		Permissions: []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code}}
}

func TestCaseOwnerCandidatesRequireActiveInternalPermission(t *testing.T) {
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
			t.Fatalf("ineligible owner %s was allowed", user.Username)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ValidateCaseOwnerDB(tx, 91, valid.ID) }); err != nil {
		t.Fatalf("eligible owner rejected: %v", err)
	}
	candidates, err := TicketCaseOwnerService.ListCandidates(91, caseOwnerPrincipal(valid))
	if err != nil || len(candidates) != 1 || candidates[0].UserID != valid.ID {
		t.Fatalf("candidates = %+v, err = %v", candidates, err)
	}
	if _, err := TicketCaseOwnerService.ListCandidates(92, caseOwnerPrincipal(valid)); err == nil {
		t.Fatal("cross-tenant candidate listing succeeded")
	}
}

func TestCaseOwnerTransferPreservesTechnicalAssignmentAndAudits(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketAssign.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(owner)
	op.Permissions = nil // Current owner may hand over without becoming a manager.
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "客服换班，后续由接班客服跟进", op); err != nil {
		t.Fatal(err)
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CaseOwnerID != next.ID || stored.CaseRevision != 4 || stored.CurrentAssigneeID != ticket.CurrentAssigneeID || stored.CurrentTeamID != ticket.CurrentTeamID || stored.CaseStatus != ticket.CaseStatus {
		t.Fatalf("transfer changed technical workflow or lost owner: %+v", stored)
	}
	var progress models.TicketProgress
	if err := db.Where("ticket_id = ?", ticket.ID).First(&progress).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(progress.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if progress.VisibleToCustomer || progress.EventType != "case_owner_transferred" || progress.AuthorID != owner.ID || metadata["fromOwnerId"] != float64(owner.ID) || metadata["toOwnerId"] != float64(next.ID) {
		t.Fatalf("invalid handover audit: %+v, %+v", progress, metadata)
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "旧页面再次提交", caseOwnerPrincipal(owner)); err == nil {
		t.Fatal("stale owner transfer should fail")
	}
}

func TestCaseOwnerTransferRejectsInvalidRequestsWithoutChangingTicket(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketUpdate.Code)
	foreign, _, _ := seedCaseOwnerMember(t, db, 92, "foreign", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(owner)
	foreignOp := *op
	foreignOp.TenantID = 92
	unrelatedOp := *op
	unrelatedOp.UserID = 500
	unrelatedOp.Permissions = nil
	for _, test := range []struct {
		name   string
		next   int64
		reason string
		op     *dto.AuthPrincipal
	}{
		{"foreign_operator", next.ID, "正常交接", &foreignOp},
		{"no_authority", next.ID, "正常交接", &unrelatedOp},
		{"foreign_owner", foreign.ID, "正常交接", op},
		{"clear_owner", 0, "正常交接", op},
		{"missing_reason", next.ID, " ", op},
	} {
		if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, test.next, test.reason, test.op); err == nil {
			t.Errorf("%s succeeded", test.name)
		}
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.TicketProgress{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CaseOwnerID != owner.ID || stored.CaseRevision != ticket.CaseRevision || count != 0 {
		t.Fatalf("rejected request changed owner or audit: %+v, progress=%d", stored, count)
	}
	for _, status := range []string{"new"} {
		if err := db.Model(&ticket).Update("case_status", status).Error; err != nil {
			t.Fatal(err)
		}
		if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "正常交接", op); err == nil {
			t.Errorf("status %s allowed transfer", status)
		}
	}
}

func TestCaseOwnerClosedCaseCanExplicitlyReplaceDisabledOwnerBeforeReopening(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(next)
	if err := db.Model(&ticket).Update("case_status", "closed").Error; err != nil {
		t.Fatal(err)
	}
	if err := UserService.UpdateStatus(owner.ID, int(enums.StatusDisabled), op); err != nil {
		t.Fatal(err)
	}
	if err := RequireTicketCaseOwnerDB(db, &ticket); err == nil {
		t.Fatal("disabled historical owner cannot own a reopened case")
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "原客服已离职，重新打开前改由新客服跟进", op); err != nil {
		t.Fatalf("explicit reassignment should recover the closed case: %v", err)
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CaseStatus != "closed" || stored.CaseOwnerID != next.ID {
		t.Fatalf("reassignment should retain closure until explicit reopen: %+v", stored)
	}
	if err := RequireTicketCaseOwnerDB(db, &stored); err != nil {
		t.Fatalf("owner must now meet the reopening guard: %v", err)
	}
}

func TestCaseOwnerAuditFailureRollsBackTransfer(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	if err := db.Migrator().DropTable(&models.TicketProgress{}); err != nil {
		t.Fatal(err)
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "正常交接", caseOwnerPrincipal(owner)); err == nil {
		t.Fatal("missing audit storage should reject transfer")
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CaseOwnerID != owner.ID || stored.CaseRevision != ticket.CaseRevision {
		t.Fatal("audit failure did not roll back owner change")
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
		if err := operation(); err == nil || !strings.Contains(err.Error(), "交接") {
			t.Fatalf("active case owner mutation should require handover, got %v", err)
		}
	}
	var stored models.User
	if err := db.First(&stored, owner.ID).Error; err != nil || stored.Status != enums.StatusOk || stored.Username != owner.Username {
		t.Fatalf("blocked account mutation changed user: %+v, %v", stored, err)
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "停用前交接", op); err != nil {
		t.Fatal(err)
	}
	if err := UserService.UpdateStatus(owner.ID, int(enums.StatusDisabled), op); err != nil {
		t.Fatalf("owner already handed over should be disableable: %v", err)
	}
	if err := db.Model(&ticket).Update("case_status", "closed").Error; err != nil {
		t.Fatal(err)
	}
	if err := UserService.DeleteUser(next.ID, op); err != nil {
		t.Fatalf("completed ownership history should permit account deletion: %v", err)
	}
	var closed models.Ticket
	if err := db.First(&closed, ticket.ID).Error; err != nil || closed.CaseOwnerID != next.ID {
		t.Fatalf("account deletion lost historical owner: %+v, %v", closed, err)
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
		t.Fatal("changing active owner to viewer should fail")
	}
	if _, _, err := PlatformIAMService.SaveAuthPolicy(request.PlatformAuthPolicySaveRequest{TenantID: 91, RoleID: role.ID, Effect: "allow", PermissionCodes: []string{constants.PermissionTicketView.Code}}, op); err == nil {
		t.Fatal("removing last processing permission should fail")
	}
	status := int(enums.StatusDisabled)
	if _, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{ID: role.ID, TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: role.Code, Name: role.Name, Status: &status}, op); err == nil {
		t.Fatal("disabling active owner's role should fail")
	}
	if err := PlatformIAMService.DeleteAuthRole(request.PlatformAuthRoleDeleteRequest{RoleID: role.ID}, op); err == nil {
		t.Fatal("deleting active owner's only role should fail")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ValidateCaseOwnerDB(tx, 91, owner.ID) }); err != nil {
		t.Fatalf("rejected edits must leave original owner permissions valid: %v", err)
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
		t.Fatal("missing owner must require explicit acceptance")
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, 0, owner.ID, "正常交接", caseOwnerPrincipal(owner)); err == nil {
		t.Fatal("ownerless historical ticket must be adopted, not silently transferred")
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil || stored.CaseOwnerID != 0 {
		t.Fatalf("historical owner was invented: %+v, %v", stored, err)
	}
}

func TestCaseOwnerDefaultRoleRefreshCannotRemoveLastProcessingPermission(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, member, viewer := seedCaseOwnerMember(t, db, 91, EnterpriseRoleViewer, constants.PermissionTicketUpdate.Code)
	next, _, custom := seedCaseOwnerMember(t, db, 91, "custom-service", constants.PermissionTicketChangeStatus.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(next)
	// A different builtin role is processed earlier than the viewer. Its name
	// must also roll back if a later role would invalidate active ownership.
	engineer := models.AuthRole{TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: EnterpriseRoleEngineer,
		Name: "Existing engineer name", Status: enums.StatusOk, IsBuiltin: true}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&viewer).Update("is_builtin", true).Error; err != nil {
		t.Fatal(err)
	}
	// No caller-owned transaction: the refresh must provide its own rollback.
	if err := EnsureTenantDefaultIAMRolesDB(db, 91, op); err == nil || !strings.Contains(err.Error(), "交接") {
		t.Fatalf("default policy refresh must reject removing an active owner's last permission: %v", err)
	}
	if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
		t.Fatalf("failed refresh removed owner permissions: %v", err)
	}
	var storedEngineer models.AuthRole
	if err := db.First(&storedEngineer, engineer.ID).Error; err != nil || storedEngineer.Name != engineer.Name {
		t.Fatalf("failed batch partially changed an earlier role: %+v, %v", storedEngineer, err)
	}
	var inserted int64
	if err := db.Model(&models.AuthRole{}).Where("tenant_id = ? AND code = ?", 91, EnterpriseRoleOwner).Count(&inserted).Error; err != nil || inserted != 0 {
		t.Fatalf("failed refresh retained a newly created role: %d, %v", inserted, err)
	}
	// Once responsibility is explicitly handed over, the same refresh can run
	// inside a caller transaction without touching custom role permissions.
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, next.ID, "角色恢复默认前完成客服交接", op); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return EnsureTenantDefaultIAMRolesDB(tx, 91, op) }); err != nil {
		t.Fatalf("refresh after handover should succeed, including nested transaction use: %v", err)
	}
	permissions, err := caseOwnerPermissionsDB(db, 91, member.ID, owner.ID)
	if err != nil || hasCaseOwnerPermissions(permissions) {
		t.Fatalf("viewer should now have default read-only permissions: %+v, %v", permissions, err)
	}
	if err := ValidateCaseOwnerDB(db, 91, next.ID); err != nil {
		t.Fatalf("custom role owner was affected by builtin refresh: %v", err)
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
		if err := operation(); err == nil || !strings.Contains(err.Error(), "交接") {
			t.Fatalf("legacy permissions cannot be revoked before handover: %v", err)
		}
		if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
			t.Fatalf("rejected legacy mutation must roll back permissions: %v", err)
		}
	}
}

func TestCaseOwnerTransferRejectsEngineerOutsideProductScope(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	if err := db.AutoMigrate(&models.AgentTeamMember{}, &models.Product{}); err != nil {
		t.Fatal(err)
	}
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	engineer, _, _ := seedCaseOwnerMember(t, db, 91, "restricted-engineer", constants.PermissionTicketUpdate.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	if err := db.Model(&ticket).Update("product_id", int64(802)).Error; err != nil {
		t.Fatal(err)
	}
	team := models.AgentTeam{TenantID: 91, ProductID: 801, Name: "Own product", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentTeamMember{TenantID: 91, TeamID: team.ID, UserID: engineer.ID, Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	op := caseOwnerPrincipal(engineer)
	op.Roles = []string{EnterpriseRoleEngineer}
	if canManageTicketCaseOwner(&ticket, op) {
		t.Fatal("outside product scope must not expose handover action")
	}
	if err := TicketCaseOwnerService.Transfer(ticket.ID, owner.ID, engineer.ID, "尝试越权认领其他产品的工单", op); err == nil {
		t.Fatal("ticket.update must not bypass product scope")
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil || stored.CaseOwnerID != owner.ID || stored.CaseRevision != ticket.CaseRevision {
		t.Fatalf("out-of-scope handover changed the case: %+v, %v", stored, err)
	}
}

func TestCaseOwnerKeyedHandoverReplaysStableReceiptWithoutAuthorityReuse(t *testing.T) {
	db := setupCaseOwnerTestDB(t)
	owner, _, _ := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketChangeStatus.Code)
	next, _, _ := seedCaseOwnerMember(t, db, 91, "next", constants.PermissionTicketChangeStatus.Code)
	third, _, _ := seedCaseOwnerMember(t, db, 91, "third", constants.PermissionTicketChangeStatus.Code)
	ticket := seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(owner)
	op.Permissions = []string{constants.PermissionTicketView.Code}
	const reason = "客服换班"
	if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, owner.ID, next.ID, reason, "handover-1", op); err != nil {
		t.Fatal(err)
	}
	nextOp := caseOwnerPrincipal(next)
	nextOp.Permissions = []string{constants.PermissionTicketView.Code}
	if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, next.ID, third.ID, "再次换班", "handover-2", nextOp); err != nil {
		t.Fatal(err)
	}
	if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, owner.ID, next.ID, reason, "handover-1", op); err != nil {
		t.Fatalf("original owner must be able to retry a completed handover: %v", err)
	}
	result, err := GetTicketCaseCommandResult(91, ticket.ID, "handover-1")
	if err != nil || result.Revision != ticket.CaseRevision+1 || result.Status != ticket.CaseStatus {
		t.Fatalf("replay receipt changed after subsequent handover: %+v, %v", result, err)
	}
	for _, test := range []struct {
		key, reason string
		op          *dto.AuthPrincipal
	}{
		{"handover-1", "篡改原来的原因", op},
		{"handover-1", reason, nextOp},
	} {
		if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, owner.ID, next.ID, test.reason, test.key, test.op); !errors.Is(err, ErrTicketCaseConflict) {
			t.Fatalf("payload or actor changed under the same key: %v", err)
		}
	}
	if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, third.ID, next.ID, reason, "", caseOwnerPrincipal(third)); err == nil {
		t.Fatal("HTTP-oriented keyed method must reject a missing key")
	}
	var stored models.Ticket
	if err := db.First(&stored, ticket.ID).Error; err != nil || stored.CaseOwnerID != third.ID || stored.CaseRevision != ticket.CaseRevision+2 {
		t.Fatalf("replay or conflicting request changed latest owner: %+v, %v", stored, err)
	}
	var count int64
	if err := db.Model(&models.TicketProgress{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, "case_owner_transferred").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("retries must not duplicate audit entries: %d, %v", count, err)
	}
}
