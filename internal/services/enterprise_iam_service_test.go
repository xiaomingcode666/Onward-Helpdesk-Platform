package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestEnterpriseIAMInvitationWorkflowsCreateDomainIdentities(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.EngineerProfile{}, &models.CustomerOrg{}, &models.CustomerUser{},
		&models.Customer{}, &models.CustomerIdentity{}, &models.PartnerCompany{}, &models.PartnerAccount{},
		&models.CustomerRegistrationGrant{}, &models.AgentProfile{},
	); err != nil {
		t.Fatalf("AutoMigrate enterprise identities: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Enterprise IAM Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	member, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "field.engineer", DisplayName: "Field Engineer", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if member.Engineer == nil || !member.Engineer.DispatchEnabled {
		t.Fatalf("dispatch engineer profile was not created")
	}
	agentProfile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", member.User.ID))
	if agentProfile == nil || agentProfile.TenantID != tenant.ID || agentProfile.TeamID <= 0 || !agentProfile.AutoAssignEnabled {
		t.Fatalf("dispatch agent profile was not created: %+v", agentProfile)
	}
	team := repositories.AgentTeamRepository.Get(db, agentProfile.TeamID)
	if team == nil || team.TeamType != AgentTeamTypeTechnicalRepair {
		t.Fatalf("engineer was not placed in the technical repair pool: %+v", team)
	}
	storedMember := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, tenant.ID, member.User.ID)
	if storedMember == nil || storedMember.DepartmentID != team.DepartmentID {
		t.Fatalf("engineer organization was not synchronized: %+v", storedMember)
	}
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, member.Member.ID, EnterpriseRoleEngineer)

	customer, err := EnterpriseIAMService.AuthorizeCustomerUser(tenant.ID, request.EnterpriseCustomerUserAuthorizeRequest{
		Username: "customer.user", DisplayName: "Customer User", Password: "InitialPass123!",
		CustomerOrg: "Customer Factory", RoleCodes: []string{CustomerRoleUser},
	}, operator)
	if err != nil {
		t.Fatalf("AuthorizeCustomerUser() error = %v", err)
	}
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customer.CustomerUser.ID, CustomerRoleUser)
	if identity := repositories.CustomerIdentityRepository.GetBy(db, enums.ExternalSourceUser, strconv.FormatInt(customer.CustomerUser.UserID, 10)); identity == nil {
		t.Fatalf("legacy customer portal identity was not created")
	}

	partner, err := EnterpriseIAMService.InvitePartnerAdmin(tenant.ID, request.EnterprisePartnerAdminInviteRequest{
		PartnerName: "Authorized Supplier", DisplayName: "Supplier Admin", Email: "supplier.admin@example.com",
	}, operator)
	if err != nil {
		t.Fatalf("InvitePartnerAdmin() error = %v", err)
	}
	partnerAccount := acceptPartnerInvitationForTest(t, db, partner, "supplier.admin")
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypePartner, models.SubjectTypePartnerAccount, partnerAccount.ID, PartnerRoleAdmin)
}

func TestEnterpriseIAMUpdateAndDisableWorkflows(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.EngineerProfile{}, &models.CustomerOrg{}, &models.CustomerUser{},
		&models.Customer{}, &models.CustomerIdentity{}, &models.PartnerCompany{}, &models.PartnerAccount{},
		&models.PartnerContract{}, &models.PartnerAuthorizationScope{}, &models.AgentProfile{}, &models.AgentTeamMember{},
		&models.CustomerRegistrationGrant{}, &models.LoginSession{}, &models.Ticket{}, &models.Conversation{},
	); err != nil {
		t.Fatalf("AutoMigrate enterprise identities: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Enterprise IAM Update Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	member, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "edit.engineer", DisplayName: "Edit Engineer", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	memberName := "Senior Engineer"
	memberEmail := "senior.engineer@example.com"
	memberMobile := "+1 555 0101"
	memberStatus := int(enums.StatusDisabled)
	updatedMember, err := EnterpriseIAMService.UpdateMember(tenant.ID, member.Member.ID, request.EnterpriseMemberUpdateRequest{
		DisplayName:     &memberName,
		Email:           &memberEmail,
		Mobile:          &memberMobile,
		RoleCodes:       []string{EnterpriseRoleViewer},
		DispatchEnabled: boolPtr(false),
		Status:          &memberStatus,
	}, operator)
	if err != nil {
		t.Fatalf("UpdateMember() error = %v", err)
	}
	if updatedMember.Member.DisplayName != memberName || updatedMember.Member.Status != enums.StatusDisabled || updatedMember.User.Status != enums.StatusDisabled {
		t.Fatalf("member was not disabled and renamed: member=%+v user=%+v", updatedMember.Member, updatedMember.User)
	}
	if updatedMember.Engineer == nil || updatedMember.Engineer.DispatchEnabled || updatedMember.Engineer.Status != enums.StatusDisabled {
		t.Fatalf("engineer dispatch profile was not disabled: %+v", updatedMember.Engineer)
	}
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, member.Member.ID, EnterpriseRoleViewer)

	customer, err := EnterpriseIAMService.AuthorizeCustomerUser(tenant.ID, request.EnterpriseCustomerUserAuthorizeRequest{
		Username: "edit.customer", DisplayName: "Edit Customer", Password: "InitialPass123!",
		CustomerOrg: "Original Customer Org", RoleCodes: []string{CustomerRoleUser},
	}, operator)
	if err != nil {
		t.Fatalf("AuthorizeCustomerUser() error = %v", err)
	}
	customerName := "Customer Manager"
	customerOrg := "Target Customer Org"
	customerMobile := "+44 20 0101"
	customerStatus := int(enums.StatusDisabled)
	updatedCustomer, err := EnterpriseIAMService.UpdateCustomerUser(tenant.ID, customer.CustomerUser.ID, request.EnterpriseCustomerUserUpdateRequest{
		DisplayName: &customerName,
		Mobile:      &customerMobile,
		CustomerOrg: &customerOrg,
		RoleCodes:   []string{CustomerRoleAdmin},
		Status:      &customerStatus,
	}, operator)
	if err != nil {
		t.Fatalf("UpdateCustomerUser() error = %v", err)
	}
	if updatedCustomer.CustomerUser.DisplayName != customerName || updatedCustomer.CustomerUser.Phone != customerMobile || updatedCustomer.CustomerUser.Status != enums.StatusDisabled {
		t.Fatalf("customer user was not updated: %+v", updatedCustomer.CustomerUser)
	}
	if updatedCustomer.CustomerOrg == nil || updatedCustomer.CustomerOrg.Name != customerOrg || updatedCustomer.CustomerUser.CustomerOrgID != updatedCustomer.CustomerOrg.ID {
		t.Fatalf("customer user was not moved to target org: user=%+v org=%+v", updatedCustomer.CustomerUser, updatedCustomer.CustomerOrg)
	}
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customer.CustomerUser.ID, CustomerRoleAdmin)

	partner, err := EnterpriseIAMService.InvitePartnerAdmin(tenant.ID, request.EnterprisePartnerAdminInviteRequest{
		PartnerName: "Update Supplier", DisplayName: "Supplier Admin", Email: "update.supplier.admin@example.com",
	}, operator)
	if err != nil {
		t.Fatalf("InvitePartnerAdmin() error = %v", err)
	}
	partnerAccount := acceptPartnerInvitationForTest(t, db, partner, "update.supplier.admin")
	if err := db.Create(&models.PartnerAuthorizationScope{
		TenantID: tenant.ID, PartnerAccountID: partnerAccount.ID, ResourceType: "ticket", ResourceID: "T-100",
		Status: enums.StatusOk, AuditFields: models.AuditFields{},
	}).Error; err != nil {
		t.Fatalf("create partner scope: %v", err)
	}
	partnerName := "Disabled Supplier"
	partnerStatus := int(enums.StatusDisabled)
	updatedPartner, err := EnterpriseIAMService.UpdatePartner(tenant.ID, partner.PartnerCompany.ID, request.EnterprisePartnerUpdateRequest{
		PartnerName: &partnerName,
		Status:      &partnerStatus,
	}, operator)
	if err != nil {
		t.Fatalf("UpdatePartner() error = %v", err)
	}
	if updatedPartner.PartnerCompany.Name != partnerName || updatedPartner.PartnerCompany.Status != enums.StatusDisabled {
		t.Fatalf("partner company was not updated: %+v", updatedPartner.PartnerCompany)
	}
	var storedPartnerAccount models.PartnerAccount
	if err := db.First(&storedPartnerAccount, "id = ?", partnerAccount.ID).Error; err != nil {
		t.Fatalf("reload partner account: %v", err)
	}
	if storedPartnerAccount.Status != enums.StatusDisabled {
		t.Fatalf("partner account was not disabled with company: %+v", storedPartnerAccount)
	}
	var scope models.PartnerAuthorizationScope
	if err := db.First(&scope, "partner_account_id = ?", partnerAccount.ID).Error; err != nil {
		t.Fatalf("reload partner scope: %v", err)
	}
	if scope.Status != enums.StatusDisabled {
		t.Fatalf("partner scope was not disabled with company: %+v", scope)
	}
}

func acceptPartnerInvitationForTest(t *testing.T, db *gorm.DB, invitation *EnterpriseIAMPartnerAdminInviteResult, username string) *models.PartnerAccount {
	t.Helper()
	if invitation == nil || invitation.PartnerCompany == nil || invitation.Grant == nil || invitation.InviteCode == "" {
		t.Fatalf("incomplete partner invitation: %+v", invitation)
	}
	now := time.Now()
	user := &models.User{
		Username: username,
		Nickname: invitation.Grant.DisplayName,
		Email:    &invitation.Grant.Email,
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create invited partner user: %v", err)
	}
	registration, err := CustomerRegistrationService.resolveInviteContext(db, invitation.InviteCode, false)
	if err != nil {
		t.Fatalf("resolve partner invitation: %v", err)
	}
	if err := CustomerRegistrationService.bindPartnerInvitationAccountDB(db, registration, user, invitation.Grant.DisplayName); err != nil {
		t.Fatalf("accept partner invitation: %v", err)
	}
	if err := CustomerRegistrationService.consumeInvitationGrantDB(db, invitation.Grant, user.ID); err != nil {
		t.Fatalf("consume partner invitation: %v", err)
	}
	account := repositories.EnterpriseIAMRepository.FindPartnerAccountByUser(db, invitation.Grant.TenantID, invitation.PartnerCompany.ID, user.ID)
	if account == nil {
		t.Fatal("accepted partner account was not created")
	}
	return account
}

func TestSyncEngineerAgentProfilesRepairsMissingEngineerRoleBinding(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(&models.EngineerProfile{}, &models.AgentProfile{}); err != nil {
		t.Fatalf("AutoMigrate engineer identities: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Legacy Engineer Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	result, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "legacy.engineer", DisplayName: "Legacy Engineer", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	if err := db.Where(
		"tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?",
		tenant.ID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, result.Member.ID,
	).Delete(&models.AuthRoleBinding{}).Error; err != nil {
		t.Fatalf("delete legacy role binding: %v", err)
	}

	if err := ProductSupportOrganizationService.SyncEngineerAgentProfilesDB(db, operator); err != nil {
		t.Fatalf("SyncEngineerAgentProfilesDB() error = %v", err)
	}
	assertIAMRoleBinding(t, db, tenant.ID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, result.Member.ID, EnterpriseRoleEngineer)
}

func TestSyncEngineerAgentProfilesPreservesTeamMemberDispatchSettings(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(&models.EngineerProfile{}, &models.AgentProfile{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("AutoMigrate engineer dispatch tables: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Engineer Dispatch Preserve Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	result, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "preserve.dispatch.engineer", DisplayName: "Preserve Dispatch Engineer", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", result.User.ID))
	if profile == nil || profile.TeamID <= 0 {
		t.Fatalf("agent profile was not created: %+v", profile)
	}
	var member models.AgentTeamMember
	if err := db.First(&member, "tenant_id = ? AND team_id = ? AND user_id = ?", tenant.ID, profile.TeamID, result.User.ID).Error; err != nil {
		t.Fatalf("membership was not created: %v", err)
	}
	if err := db.Model(&models.AgentTeamMember{}).Where("id = ?", member.ID).Updates(map[string]any{
		"dispatch_enabled": false,
		"dispatch_weight":  7,
	}).Error; err != nil {
		t.Fatalf("disable team dispatch: %v", err)
	}

	if err := ProductSupportOrganizationService.SyncEngineerAgentProfilesDB(db, operator); err != nil {
		t.Fatalf("SyncEngineerAgentProfilesDB() error = %v", err)
	}
	var refreshed models.AgentTeamMember
	if err := db.First(&refreshed, member.ID).Error; err != nil {
		t.Fatalf("reload membership: %v", err)
	}
	if refreshed.DispatchEnabled || refreshed.DispatchWeight != 7 {
		t.Fatalf("engineer sync clobbered team dispatch settings: %+v", refreshed)
	}
}

func TestSyncEngineerAgentProfilesSkipsDisabledTeamMembershipRebuild(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(&models.EngineerProfile{}, &models.AgentProfile{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("AutoMigrate engineer dispatch tables: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Disabled Dispatch Team Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	result, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "disabled.sync.engineer", DisplayName: "Disabled Sync Engineer", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", result.User.ID))
	if profile == nil || profile.TeamID <= 0 {
		t.Fatalf("agent profile was not created: %+v", profile)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", profile.TeamID).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatalf("disable team: %v", err)
	}
	if err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ?", tenant.ID, profile.TeamID, result.User.ID).
		Delete(&models.AgentTeamMember{}).Error; err != nil {
		t.Fatalf("delete stale membership: %v", err)
	}

	if err := ProductSupportOrganizationService.SyncEngineerAgentProfilesDB(db, operator); err != nil {
		t.Fatalf("SyncEngineerAgentProfilesDB() error = %v", err)
	}
	var rebuilt int64
	if err := db.Model(&models.AgentTeamMember{}).
		Where("tenant_id = ? AND team_id = ? AND user_id = ?", tenant.ID, profile.TeamID, result.User.ID).
		Count(&rebuilt).Error; err != nil {
		t.Fatalf("count rebuilt membership: %v", err)
	}
	if rebuilt != 0 {
		t.Fatalf("disabled team membership should not be rebuilt, got %d", rebuilt)
	}
}

func TestEnterpriseIAMMemberProductGroupsOnlyIncludeActiveDispatchProducts(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("AutoMigrate product group dispatch tables: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "IAM Product Group Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	result, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "product.group.member", DisplayName: "Product Group Member", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleViewer},
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}
	activeProduct := &models.Product{TenantID: tenant.ID, Code: "ACTIVE-GROUP", Name: "Active Group Product", Status: enums.StatusOk}
	disabledProduct := &models.Product{TenantID: tenant.ID, Code: "DISABLED-GROUP", Name: "Disabled Group Product", Status: enums.StatusDisabled}
	if err := db.Create(activeProduct).Error; err != nil {
		t.Fatalf("create active product: %v", err)
	}
	if err := db.Create(disabledProduct).Error; err != nil {
		t.Fatalf("create disabled product: %v", err)
	}
	activeTeam := &models.AgentTeam{
		TenantID: tenant.ID, ProductID: activeProduct.ID, TeamType: AgentTeamTypeProductRepair,
		Name: "Active Product Team", AssignmentMode: AgentTeamAssignmentModeBalanced, Status: enums.StatusOk,
	}
	disabledTeam := &models.AgentTeam{
		TenantID: tenant.ID, ProductID: activeProduct.ID, TeamType: AgentTeamTypeProductRepair,
		Name: "Disabled Product Team", AssignmentMode: AgentTeamAssignmentModeBalanced, Status: enums.StatusDisabled,
	}
	staleProductTeam := &models.AgentTeam{
		TenantID: tenant.ID, ProductID: disabledProduct.ID, TeamType: AgentTeamTypeProductRepair,
		Name: "Stale Disabled Product Team", AssignmentMode: AgentTeamAssignmentModeBalanced, Status: enums.StatusOk,
	}
	for _, team := range []*models.AgentTeam{activeTeam, disabledTeam, staleProductTeam} {
		if err := db.Create(team).Error; err != nil {
			t.Fatalf("create team %s: %v", team.Name, err)
		}
		if err := db.Create(&models.AgentTeamMember{
			TenantID: tenant.ID, TeamID: team.ID, UserID: result.User.ID, MemberID: result.Member.ID,
			DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
		}).Error; err != nil {
			t.Fatalf("create membership for %s: %v", team.Name, err)
		}
	}

	groups := EnterpriseIAMService.memberProductGroups(tenant.ID, []models.TenantMember{*result.Member})[result.Member.ID]
	if len(groups) != 1 || groups[0].TeamID != activeTeam.ID || groups[0].ProductID != activeProduct.ID {
		t.Fatalf("member product groups should only include active product teams, got %+v", groups)
	}
}

func TestEnterpriseIAMStartsAuditedMemberPortalSupportSession(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.LoginSession{}, &models.TemporaryAccessGrant{},
		&models.EngineerProfile{}, &models.AgentProfile{},
	); err != nil {
		t.Fatalf("AutoMigrate employee support session tables: %v", err)
	}
	for table, model := range map[string]any{
		"t_role":            &models.Role{},
		"t_permission":      &models.Permission{},
		"t_user_role":       &models.UserRole{},
		"t_role_permission": &models.RolePermission{},
		"t_user_permission": &models.UserPermission{},
	} {
		if err := db.Table(table).AutoMigrate(model); err != nil {
			t.Fatalf("AutoMigrate %s: %v", table, err)
		}
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Employee Portal Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise
	operator.Domain = models.DomainTypeEnterprise
	operator.SubjectType = models.SubjectTypeTenantMember
	operator.SubjectID = operator.UserID
	target, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: "support.employee", DisplayName: "Support Employee", Password: "InitialPass123!",
		RoleCodes: []string{EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}

	if _, err := EnterpriseIAMService.StartMemberPortalSupportSession(tenant.ID+1, target.Member.ID, operator, "127.0.0.1", "go-test", config.AuthConfig{TokenTTLHours: 12}); err == nil {
		t.Fatal("expected cross-tenant employee support session to be rejected")
	}
	login, err := EnterpriseIAMService.StartMemberPortalSupportSession(tenant.ID, target.Member.ID, operator, "127.0.0.1", "go-test", config.AuthConfig{TokenTTLHours: 12})
	if err != nil {
		t.Fatalf("StartMemberPortalSupportSession() error = %v", err)
	}
	if login.User.ID != target.User.ID || login.SubjectID != target.Member.ID || login.SupportMode != models.SupportModeEmployeePortal {
		t.Fatalf("unexpected employee portal login: %+v", login)
	}
	var grant models.TemporaryAccessGrant
	if err := db.First(&grant, login.SupportGrantID).Error; err != nil {
		t.Fatalf("query employee temporary grant: %v", err)
	}
	if grant.ResourceType != models.SubjectTypeTenantMember || grant.ResourceID != strconv.FormatInt(target.Member.ID, 10) || grant.TenantID != tenant.ID {
		t.Fatalf("unexpected employee temporary grant: %+v", grant)
	}
	var audit models.AuthAuditLog
	if err := db.Where("tenant_id = ? AND action = ? AND target_id = ?", tenant.ID, "tenant_member.support_session_started", strconv.FormatInt(target.Member.ID, 10)).First(&audit).Error; err != nil {
		t.Fatalf("query employee support audit: %v", err)
	}
	if audit.SupportGrantID != grant.ID || audit.RiskLevel != models.RiskLevelHigh {
		t.Fatalf("unexpected employee support audit: %+v", audit)
	}
}

func TestEnterpriseIAMCreateDepartmentProtectsSystemSupportBranch(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Department Tree Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	root := repositories.EnterpriseIAMRepository.FindRootDepartment(db, tenant.ID)
	if root == nil {
		t.Fatalf("root department was not provisioned")
	}
	ordinary, err := EnterpriseIAMService.CreateDepartment(tenant.ID, request.EnterpriseDepartmentCreateRequest{
		Name: "海外一区",
	}, operator)
	if err != nil {
		t.Fatalf("CreateDepartment() ordinary error = %v", err)
	}
	if ordinary.ParentID != root.ID || ordinary.Path != "/海外一区" || ordinary.Depth != 1 {
		t.Fatalf("ordinary department = %+v, root = %+v", ordinary, root)
	}
	child, err := EnterpriseIAMService.CreateDepartment(tenant.ID, request.EnterpriseDepartmentCreateRequest{
		ParentID: ordinary.ID,
		Name:     "备件支持",
	}, operator)
	if err != nil {
		t.Fatalf("CreateDepartment() child error = %v", err)
	}
	if child.ParentID != ordinary.ID || child.Path != "/海外一区/备件支持" || child.Depth != 2 {
		t.Fatalf("child department = %+v", child)
	}

	technicalTeam, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
	if err != nil {
		t.Fatalf("EnsureTenantTechnicalRepairTeamDB() error = %v", err)
	}
	_, err = EnterpriseIAMService.CreateDepartment(tenant.ID, request.EnterpriseDepartmentCreateRequest{
		ParentID: technicalTeam.DepartmentID,
		Name:     "错误的产品外子组",
	}, operator)
	if err == nil || !strings.Contains(err.Error(), "技术售后组") {
		t.Fatalf("expected technical branch protection error, got %v", err)
	}
}

func assertIAMRoleBinding(t *testing.T, db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64, roleCode string) {
	t.Helper()
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenantID, domainType, roleCode)
	if role == nil {
		t.Fatalf("role %s was not created", roleCode)
	}
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, tenantID, domainType, subjectType, subjectID)
	if err != nil || len(bindings) != 1 || bindings[0].RoleID != role.ID {
		t.Fatalf("binding for %s = %+v, error = %v", roleCode, bindings, err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
