package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupPlatformIAMTestDB(t *testing.T) (*gorm.DB, *dto.AuthPrincipal) {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(
		&models.User{},
		&models.Tenant{},
		&models.TenantBranding{},
		&models.Asset{},
		&models.TenantIntegrationConfig{},
		&models.TenantPlan{},
		&models.TenantSubscription{},
		&models.TenantQuotaOverride{},
		&models.Sub2APITenantAccount{},
		&models.Department{},
		&models.AgentTeam{},
		&models.TenantMember{},
		&models.AuthRole{},
		&models.AuthRoleBinding{},
		&models.AuthRolePermission{},
		&models.AuthSession{},
		&models.AuthAuditLog{},
		&models.Device{},
		&models.PlatformStaffProfile{},
		&models.PlatformTenantGrant{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	admin := &models.User{
		Username: "platform-admin",
		Nickname: "Platform Admin",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create admin user error = %v", err)
	}

	return db, &dto.AuthPrincipal{
		UserID:      admin.ID,
		Username:    admin.Username,
		DomainType:  models.DomainTypePlatform,
		Domain:      models.DomainTypePlatform,
		SubjectType: models.SubjectTypePlatformStaff,
		SubjectID:   admin.ID,
		Status:      enums.StatusOk,
	}
}

func platformIAMStringPtr(value string) *string {
	return &value
}

func platformIAMIntPtr(value int) *int {
	return &value
}

func TestTenantPortalSettingsPersistsUploadedLogoAsset(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	tenant := &models.Tenant{
		Name:        "Logo Tenant",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	asset := &models.Asset{
		AssetID:    "tenant-logo-public-id",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "tenant-branding/logo/tenant-logo-public-id.png",
		Filename:   "logo.png",
		FileSize:   16,
		MimeType:   "image/png",
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := db.Create(asset).Error; err != nil {
		t.Fatalf("create logo asset: %v", err)
	}
	if resolved := TenantPortalSettingsService.ResolvePublicLogoAssetDB(db, asset.AssetID); resolved != nil {
		t.Fatalf("unbound logo asset must not be public: %+v", resolved)
	}

	branding, err := TenantPortalSettingsService.SaveBrandingDB(
		db, tenant.ID, tenant.Name, asset.ID, "", "logo.example.com", models.TenantCustomerThemeDefault, "en-US", operator,
	)
	if err != nil {
		t.Fatalf("SaveBrandingDB() error = %v", err)
	}
	if branding == nil || branding.LogoAssetID != asset.ID || branding.LogoURL != "/api/tenant-branding/logo/tenant-logo-public-id" {
		t.Fatalf("tenant branding logo = %+v", branding)
	}
	if resolved := TenantPortalSettingsService.ResolvePublicLogoAssetDB(db, asset.AssetID); resolved == nil || resolved.ID != asset.ID {
		t.Fatalf("resolved public logo asset = %+v", resolved)
	}
}

func TestPlatformIAMCreateTenantWithInlineAdministrator(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name: "Inline Admin Tenant", AdminUsername: "tenant.owner", AdminNickname: "Tenant Owner",
		AdminEmail: "owner@example.com", AdminPassword: "InitialPass123!",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	user := repositories.UserRepository.GetByUsername(db, "tenant.owner")
	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("InitialPass123!")) != nil {
		t.Fatalf("inline tenant administrator was not created with supplied password")
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, tenant.ID, user.ID)
	if member == nil || member.MemberType != "owner" {
		t.Fatalf("tenant owner membership was not created: %+v", member)
	}
	roles, permissions, err := PlatformIAMService.ListAuthRoles(tenant.ID, models.DomainTypeEnterprise)
	if err != nil || len(roles) < 6 {
		t.Fatalf("default enterprise roles = %d, error = %v", len(roles), err)
	}
	owner := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenant.ID, models.DomainTypeEnterprise, EnterpriseRoleOwner)
	if owner == nil || len(permissions[owner.ID]) == 0 {
		t.Fatalf("tenant owner default permissions were not seeded")
	}
	technicalTeam, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
	if err != nil || technicalTeam == nil || technicalTeam.TeamType != AgentTeamTypeTechnicalRepair || !technicalTeam.SystemManaged {
		t.Fatalf("default technical repair team = %+v, error = %v", technicalTeam, err)
	}
	if technicalTeam.DepartmentID <= 0 || technicalTeam.TenantID != tenant.ID {
		t.Fatalf("default technical repair team lost tenant organization binding: %+v", technicalTeam)
	}
}

func TestPlatformIAMCreateTenantProvisionsDefaultAIAgentAndTenantKey(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.KnowledgeBase{},
		&models.KnowledgeRevision{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
		&models.SkillDefinition{},
	); err != nil {
		t.Fatalf("AutoMigrate AI defaults: %v", err)
	}
	originalProvision := provisionNewTenantDefaultAIKey
	t.Cleanup(func() { provisionNewTenantDefaultAIKey = originalProvision })
	provisionedTenantID := int64(0)
	provisionNewTenantDefaultAIKey = func(tenantID int64, _ *dto.AuthPrincipal) error {
		provisionedTenantID = tenantID
		return nil
	}

	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Default AI Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if provisionedTenantID != tenant.ID {
		t.Fatalf("default key provision tenant = %d, want %d", provisionedTenantID, tenant.ID)
	}
	if tenant.EffectiveServiceScene() != models.TenantServiceSceneEquipmentAfterSales {
		t.Fatalf("default tenant service scene = %q", tenant.EffectiveServiceScene())
	}
	agent := repositories.AIAgentRepository.Take(db,
		"tenant_id = ? AND product_id = 0 AND source = ? AND status <> ?",
		tenant.ID, TenantDefaultAIAgentSource, enums.StatusDeleted)
	if agent == nil || agent.Status != enums.StatusOk || agent.ActiveReleaseID <= 0 {
		t.Fatalf("default tenant AI agent = %+v", agent)
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenant.ID).
		Eq("remark", tenantDefaultKnowledgeBaseRemark).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).
		NotEq("status", enums.StatusDeleted))
	if knowledgeBase == nil || knowledgeBase.Name != "企业通用知识库" {
		t.Fatalf("default tenant knowledge base = %+v", knowledgeBase)
	}
	if !tenantDefaultContainsInt64(utils.SplitInt64s(agent.KnowledgeIDs), knowledgeBase.ID) {
		t.Fatalf("default agent knowledge IDs = %q, want %d", agent.KnowledgeIDs, knowledgeBase.ID)
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if release == nil {
		t.Fatalf("default tenant AI release not found: agent=%+v", agent)
	}
	var snapshot dto.AIAgentReleaseConfigSnapshot
	if err := json.Unmarshal([]byte(release.AgentConfigSnapshot), &snapshot); err != nil {
		t.Fatalf("decode default tenant AI release snapshot: %v", err)
	}
	if !tenantDefaultContainsInt64(snapshot.KnowledgeIDs, knowledgeBase.ID) {
		t.Fatalf("default release knowledge IDs = %+v, want %d", snapshot.KnowledgeIDs, knowledgeBase.ID)
	}
	if snapshot.ServiceMode != enums.IMConversationServiceModeAIOnly || len(snapshot.TeamIDs) != 0 {
		t.Fatalf("default release retained assisted-service settings: %+v", snapshot)
	}
	workflow := repositories.AIWorkflowRepository.Get(db, agent.WorkflowID)
	if workflow == nil || workflow.Code != PlatformDeviceAIOnlyWorkflowCode || workflow.CurrentStableVersionID != agent.WorkflowVersionID || agent.ServiceMode != enums.IMConversationServiceModeAIOnly || agent.TeamIDs != "" {
		t.Fatalf("default tenant workflow = agent %+v workflow %+v", agent, workflow)
	}
	if AIWorkflowService.AgentAllowsHumanHandoff(agent) || AIWorkflowService.AgentAllowsTicketCreation(agent) {
		t.Fatal("tenant default production release must not expose handoff or ticket creation")
	}
}

func TestPlatformIAMCreateKnowledgeSupportTenantBindsKnowledgeWorkflow(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.KnowledgeBase{},
		&models.KnowledgeRevision{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
		&models.SkillDefinition{},
	); err != nil {
		t.Fatalf("AutoMigrate AI defaults: %v", err)
	}
	originalProvision := provisionNewTenantDefaultAIKey
	t.Cleanup(func() { provisionNewTenantDefaultAIKey = originalProvision })
	provisionNewTenantDefaultAIKey = func(int64, *dto.AuthPrincipal) error { return nil }

	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name:                 "Onward Digital Technology L.L.C.",
		ServiceScene:         models.TenantServiceSceneKnowledgeSupport,
		BrandName:            "Onward Digital Technology L.L.C.",
		LogoURL:              "/images/tenants/odt/onward-logo-web.png",
		CustomDomain:         "www.odt.ae",
		CustomerTheme:        models.TenantCustomerThemeODTIntelligence,
		ServerConsoleName:    "1Panel",
		ServerConsoleURL:     "http://panel.example.com/security-entry",
		ServerConsoleMode:    TenantExternalPortalModeIframe,
		ServerConsoleEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if !tenant.IsKnowledgeSupportScene() {
		t.Fatalf("tenant service scene = %q", tenant.EffectiveServiceScene())
	}
	agent := repositories.AIAgentRepository.Take(db,
		"tenant_id = ? AND product_id = 0 AND source = ? AND status <> ?",
		tenant.ID, TenantDefaultAIAgentSource, enums.StatusDeleted)
	if agent == nil || agent.ActiveReleaseID <= 0 {
		t.Fatalf("knowledge tenant default agent = %+v", agent)
	}
	technicalTeam := repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenant.ID).
		Eq("team_type", AgentTeamTypeTechnicalRepair).
		NotEq("status", enums.StatusDeleted))
	if technicalTeam == nil || technicalTeam.Name != TechnicalMaintenanceTeamName || technicalTeam.ProductID != 0 {
		t.Fatalf("knowledge tenant maintenance team = %+v", technicalTeam)
	}
	if agent.TeamIDs != utils.JoinInt64s([]int64{technicalTeam.ID}) || agent.HandoffMode != enums.AIAgentHandoffModeDefaultTeamPool {
		t.Fatalf("knowledge tenant handoff target = agent %+v team %+v", agent, technicalTeam)
	}
	workflow := repositories.AIWorkflowRepository.Get(db, agent.WorkflowID)
	if workflow == nil || workflow.Code != PlatformKnowledgeSupportWorkflowCode || workflow.CurrentStableVersionID != agent.WorkflowVersionID {
		t.Fatalf("knowledge tenant workflow = agent %+v workflow %+v", agent, workflow)
	}
	if strings.Contains(agent.WelcomeMessage, "设备") || !strings.Contains(agent.WelcomeMessage, "知识服务助手") {
		t.Fatalf("knowledge tenant welcome message = %q", agent.WelcomeMessage)
	}
	if agent.ServiceMode != enums.IMConversationServiceModeAIFirst || !AIWorkflowService.AgentAllowsHumanHandoff(agent) || AIWorkflowService.AgentAllowsTicketCreation(agent) {
		t.Fatal("knowledge tenant workflow must allow handoff without direct customer ticket creation")
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	activeSnapshot := dto.AIAgentReleaseConfigSnapshot{}
	if release == nil || json.Unmarshal([]byte(release.AgentConfigSnapshot), &activeSnapshot) != nil ||
		activeSnapshot.HandoffMode != enums.AIAgentHandoffModeDefaultTeamPool ||
		len(activeSnapshot.TeamIDs) != 1 || activeSnapshot.TeamIDs[0] != technicalTeam.ID {
		t.Fatalf("knowledge tenant release handoff target = release %+v snapshot %+v", release, activeSnapshot)
	}
	branding, serverConsole := TenantPortalSettingsService.GetDB(db, tenant.ID)
	if branding == nil || branding.BrandName != "Onward Digital Technology L.L.C." || branding.LogoURL != "/images/tenants/odt/onward-logo-web.png" || branding.CustomDomain != "www.odt.ae" {
		t.Fatalf("knowledge tenant branding = %+v", branding)
	}
	if branding.EffectiveCustomerTheme() != models.TenantCustomerThemeODTIntelligence {
		t.Fatalf("knowledge tenant customer theme = %q", branding.EffectiveCustomerTheme())
	}
	if serverConsole == nil || !serverConsole.Enabled || serverConsole.BaseURL != "http://panel.example.com/security-entry" {
		t.Fatalf("knowledge tenant server console = %+v", serverConsole)
	}
	if metadata := TenantExternalPortalMetadata(serverConsole); metadata.EmbedMode != TenantExternalPortalModeExternal {
		t.Fatalf("HTTP server console embed mode = %q, want external", metadata.EmbedMode)
	}
}

func TestEnsureTenantTechnicalRepairTeamReusesLegacyTypeOnlyTeam(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Legacy Technical Team Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if err := db.Where("tenant_id = ? AND team_type = ?", tenant.ID, AgentTeamTypeTechnicalRepair).Delete(&models.AgentTeam{}).Error; err != nil {
		t.Fatalf("delete seeded technical team: %v", err)
	}
	if err := db.Where("tenant_id = ? AND department_code = ?", tenant.ID, "technical-repair").Delete(&models.Department{}).Error; err != nil {
		t.Fatalf("delete seeded technical department: %v", err)
	}
	legacy := &models.AgentTeam{
		TenantID:      tenant.ID,
		TeamType:      AgentTeamTypeTechnicalRepair,
		SystemManaged: true,
		Name:          "技术维修组",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy technical team: %v", err)
	}

	team, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
	if err != nil {
		t.Fatalf("EnsureTenantTechnicalRepairTeamDB() error = %v", err)
	}
	if team == nil || team.ID != legacy.ID {
		t.Fatalf("technical team was duplicated: got %+v legacy=%+v", team, legacy)
	}
	if team.Name != TechnicalAfterSalesTeamName || team.SystemKey == nil || *team.SystemKey != "technical-repair" {
		t.Fatalf("legacy team was not normalized: %+v", team)
	}
}

func TestPlatformIAMInviteStaffCreatesAccountAndRoleBinding(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	staff, err := PlatformIAMService.InvitePlatformStaff(request.PlatformStaffInviteRequest{
		Username: "platform.ops", Nickname: "Platform Ops", Password: "InitialPass123!",
		RoleCodes: []string{PlatformRoleOperations}, TeamCode: "operations",
	}, operator)
	if err != nil {
		t.Fatalf("InvitePlatformStaff() error = %v", err)
	}
	if staff.UserID <= 0 {
		t.Fatalf("staff user ID = %d", staff.UserID)
	}
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, staff.ID)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("platform role bindings = %d, error = %v", len(bindings), err)
	}
}

func TestPlatformIAMUpdateAndDeletePlatformStaff(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	staff, err := PlatformIAMService.InvitePlatformStaff(request.PlatformStaffInviteRequest{
		Username: "platform.editor", Nickname: "Platform Editor", Password: "InitialPass123!",
		Email: "editor@example.com", Mobile: "+1 555 0100", RoleCodes: []string{PlatformRoleOperations},
		TeamCode: "operations", JobTitle: "Ops", SupportLevel: "administrator", EmploymentType: "employee",
	}, operator)
	if err != nil {
		t.Fatalf("InvitePlatformStaff() error = %v", err)
	}

	updated, err := PlatformIAMService.UpdatePlatformStaff(request.PlatformStaffUpdateRequest{
		PlatformStaffID: staff.ID,
		Nickname:        platformIAMStringPtr("Platform Auditor"),
		Email:           platformIAMStringPtr("auditor@example.com"),
		Mobile:          platformIAMStringPtr("+1 555 0200"),
		TeamCode:        platformIAMStringPtr("audit"),
		JobTitle:        platformIAMStringPtr("Compliance Auditor"),
		SupportLevel:    platformIAMStringPtr("audit"),
		EmploymentType:  platformIAMStringPtr("auditor"),
		Status:          platformIAMIntPtr(int(enums.StatusOk)),
	}, operator)
	if err != nil {
		t.Fatalf("UpdatePlatformStaff() error = %v", err)
	}
	if updated.TeamCode != "audit" || updated.JobTitle != "Compliance Auditor" || updated.SupportLevel != "audit" || updated.EmploymentType != "auditor" {
		t.Fatalf("updated platform staff profile = %+v", updated)
	}
	user := repositories.UserRepository.Get(db, staff.UserID)
	if user == nil || user.Nickname != "Platform Auditor" || user.Email == nil || *user.Email != "auditor@example.com" || user.Mobile == nil || *user.Mobile != "+1 555 0200" {
		t.Fatalf("updated platform staff user = %+v", user)
	}

	if err := PlatformIAMService.DeletePlatformStaff(request.PlatformStaffDeleteRequest{PlatformStaffID: staff.ID, Reason: "left company"}, operator); err != nil {
		t.Fatalf("DeletePlatformStaff() error = %v", err)
	}
	deletedStaff := repositories.PlatformIAMRepository.GetPlatformStaff(db, staff.ID)
	if deletedStaff == nil || deletedStaff.Status != enums.StatusDeleted {
		t.Fatalf("deleted platform staff = %+v", deletedStaff)
	}
	var binding models.AuthRoleBinding
	if err := db.First(&binding, "tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?", 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, staff.ID).Error; err != nil {
		t.Fatalf("platform staff role binding missing: %v", err)
	}
	if binding.Status != enums.StatusDeleted {
		t.Fatalf("platform staff role binding status = %d, want deleted", binding.Status)
	}
	var deletedUser models.User
	if err := db.Unscoped().First(&deletedUser, "id = ?", staff.UserID).Error; err != nil {
		t.Fatalf("deleted platform staff user missing: %v", err)
	}
	if !deletedUser.DeletedAt.Valid || deletedUser.Status != enums.StatusDisabled {
		t.Fatalf("deleted platform staff user = %+v", deletedUser)
	}
}

func TestEnsureBootstrapPlatformAdministratorCreatesProfileAndRoleBinding(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	bootstrap := &models.User{
		Username: constants.BootstrapAdminUsername,
		Nickname: constants.BootstrapAdminNickname,
		Status:   enums.StatusOk,
	}
	if err := db.Create(bootstrap).Error; err != nil {
		t.Fatalf("create bootstrap administrator: %v", err)
	}
	if err := EnsureBootstrapPlatformAdministratorDB(db, operator); err != nil {
		t.Fatalf("EnsureBootstrapPlatformAdministratorDB() error = %v", err)
	}
	profile := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, bootstrap.ID)
	if profile == nil || profile.Status != enums.StatusOk {
		t.Fatalf("bootstrap platform profile = %+v", profile)
	}
	assertIAMRoleBinding(t, db, 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, profile.ID, PlatformRoleAdmin)
}

func TestPlatformIAMCreateTenantInitializesSubscriptionMemberAndAudit(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)

	plan, err := PlatformIAMService.SavePlan(request.PlatformPlanSaveRequest{
		Code:            "enterprise",
		Name:            "Enterprise",
		PlanType:        "enterprise",
		OverageStrategy: "metered",
	}, operator)
	if err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name:                  "Atlas Machines",
		Industry:              "machinery",
		CountryRegion:         "DE",
		DefaultLocale:         "en-US",
		CustomerDefaultLocale: "de-DE",
		SupportedLocales:      []string{"de-DE", "en-US", "de-DE"},
		Timezone:              "Europe/Berlin",
		SupportedTimezones:    []string{"UTC", "Europe/Berlin", "UTC"},
		DataRegion:            "EU",
		PlanID:                plan.ID,
		AdminUserID:           operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if tenant.ID <= 0 {
		t.Fatalf("tenant ID = %d, want persisted tenant", tenant.ID)
	}
	if got := utils.ParseStringListJSON(tenant.SupportedLocalesJSON); strings.Join(got, ",") != "en-US,de-DE" {
		t.Fatalf("supported locales = %q", tenant.SupportedLocalesJSON)
	}
	if got := utils.ParseStringListJSON(tenant.SupportedTimezonesJSON); strings.Join(got, ",") != "Europe/Berlin,UTC" {
		t.Fatalf("supported timezones = %q", tenant.SupportedTimezonesJSON)
	}
	branding := repositories.PlatformIAMRepository.GetTenantBranding(db, tenant.ID)
	if branding == nil || branding.DefaultLocale != "de-DE" {
		t.Fatalf("customer default locale = %+v, want de-DE", branding)
	}

	var subscription models.TenantSubscription
	if err := db.First(&subscription, "tenant_id = ? AND plan_id = ? AND status = ?", tenant.ID, plan.ID, enums.StatusOk).Error; err != nil {
		t.Fatalf("expected active subscription: %v", err)
	}

	var member models.TenantMember
	if err := db.First(&member, "tenant_id = ? AND user_id = ?", tenant.ID, operator.UserID).Error; err != nil {
		t.Fatalf("expected tenant member: %v", err)
	}
	if member.MemberType != "owner" {
		t.Fatalf("member type = %q, want owner", member.MemberType)
	}

	var binding models.AuthRoleBinding
	if err := db.First(&binding, "tenant_id = ? AND subject_type = ? AND subject_id = ?", tenant.ID, models.SubjectTypeTenantMember, member.ID).Error; err != nil {
		t.Fatalf("expected tenant owner role binding: %v", err)
	}

	var audit models.AuthAuditLog
	if err := db.First(&audit, "tenant_id = ? AND action = ?", tenant.ID, models.AuditActionTenantCreated).Error; err != nil {
		t.Fatalf("expected tenant creation audit: %v", err)
	}
	if audit.RiskLevel != models.RiskLevelMedium {
		t.Fatalf("audit risk = %q, want medium", audit.RiskLevel)
	}
}

func TestPlatformIAMFreezeAndUnfreezeTenant(t *testing.T) {
	_, operator := setupPlatformIAMTestDB(t)

	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "BlueForge"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	frozen, err := PlatformIAMService.FreezeTenant(request.PlatformTenantStateRequest{TenantID: tenant.ID, Reason: "quota exceeded"}, operator)
	if err != nil {
		t.Fatalf("FreezeTenant() error = %v", err)
	}
	if frozen.Status != enums.StatusDisabled || frozen.FrozenReason != "quota exceeded" {
		t.Fatalf("frozen tenant = status %d reason %q", frozen.Status, frozen.FrozenReason)
	}

	active, err := PlatformIAMService.UnfreezeTenant(request.PlatformTenantStateRequest{TenantID: tenant.ID}, operator)
	if err != nil {
		t.Fatalf("UnfreezeTenant() error = %v", err)
	}
	if active.Status != enums.StatusOk || active.FrozenReason != "" {
		t.Fatalf("active tenant = status %d reason %q", active.Status, active.FrozenReason)
	}
}

func TestPlatformIAMDeleteTenantRequiresDecommissionedTenant(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	now := time.Now()
	active := &models.Tenant{Name: "Active Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	retired := &models.Tenant{Name: "Retired Tenant", Status: enums.StatusDisabled, DecommissionedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create active tenant: %v", err)
	}
	if err := db.Create(retired).Error; err != nil {
		t.Fatalf("create retired tenant: %v", err)
	}

	deleted, err := PlatformIAMService.DeleteTenant(request.PlatformTenantDeleteRequest{TenantID: retired.ID, Reason: "cleanup"}, operator)
	if err != nil {
		t.Fatalf("DeleteTenant() error = %v", err)
	}
	if deleted.Status != enums.StatusDeleted || deleted.DecommissionedAt == nil {
		t.Fatalf("deleted tenant = %+v, want deleted status with decommission timestamp", deleted)
	}
	var audit models.AuthAuditLog
	if err := db.First(&audit, "tenant_id = ? AND action = ?", retired.ID, "tenant.deleted").Error; err != nil {
		t.Fatalf("expected tenant deletion audit: %v", err)
	}
	if _, err := PlatformIAMService.DeleteTenant(request.PlatformTenantDeleteRequest{TenantID: active.ID}, operator); err == nil {
		t.Fatal("DeleteTenant() error = nil for active tenant, want decommissioned tenant rejection")
	}
}

func TestPlatformIAMUpdateTenantCanConvertTrialTenantToFormal(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)

	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name:          "Atlas Machines",
		Industry:      "machinery",
		CountryRegion: "DE",
		DefaultLocale: "en-US",
		Timezone:      "Europe/Berlin",
		DataRegion:    "EU",
		TrialEndsAt:   "2026-09-01",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if tenant.TrialEndsAt == nil {
		t.Fatalf("trial tenant missing trial end date: %+v", tenant)
	}

	updated, err := PlatformIAMService.UpdateTenant(request.PlatformTenantUpdateRequest{
		ID:            tenant.ID,
		Name:          platformIAMStringPtr("Atlas Machines GmbH"),
		Industry:      platformIAMStringPtr(""),
		CountryRegion: platformIAMStringPtr("Germany"),
		TrialEndsAt:   platformIAMStringPtr(""),
	}, operator)
	if err != nil {
		t.Fatalf("UpdateTenant() error = %v", err)
	}
	if updated.Name != "Atlas Machines GmbH" || updated.Industry != "" || updated.CountryRegion != "Germany" {
		t.Fatalf("updated tenant fields = %+v", updated)
	}
	if updated.TrialEndsAt != nil {
		t.Fatalf("trial end date = %v, want nil for formal tenant", updated.TrialEndsAt)
	}
	if updated.DefaultLocale != "en-US" || updated.Timezone != "Europe/Berlin" || updated.DataRegion != "EU" {
		t.Fatalf("omitted fields should stay unchanged: %+v", updated)
	}

	var persisted models.Tenant
	if err := db.First(&persisted, "id = ?", tenant.ID).Error; err != nil {
		t.Fatalf("reload tenant: %v", err)
	}
	if persisted.TrialEndsAt != nil {
		t.Fatalf("persisted trial end date = %v, want nil", persisted.TrialEndsAt)
	}
}

func TestPlatformIAMRoleCodeIsUniquePerTenantDomain(t *testing.T) {
	_, operator := setupPlatformIAMTestDB(t)

	if _, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   1001,
		DomainType: models.DomainTypeEnterprise,
		Code:       "engineer",
		Name:       "Engineer",
	}, operator); err != nil {
		t.Fatalf("SaveAuthRole tenant 1001 error = %v", err)
	}
	if _, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   1002,
		DomainType: models.DomainTypeEnterprise,
		Code:       "engineer",
		Name:       "Engineer",
	}, operator); err != nil {
		t.Fatalf("same role code in another tenant should be allowed: %v", err)
	}
	if _, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   1001,
		DomainType: models.DomainTypeEnterprise,
		Code:       "engineer",
		Name:       "Engineer Duplicate",
	}, operator); err == nil {
		t.Fatalf("duplicate role code in same tenant/domain error = nil, want error")
	}
}

func TestPlatformIAMDeleteAuthRoleSoftDeletesBindingsAndPermissions(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	role, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   0,
		DomainType: models.DomainTypePlatform,
		Code:       "temporary_ops",
		Name:       "Temporary Ops",
	}, operator)
	if err != nil {
		t.Fatalf("SaveAuthRole() error = %v", err)
	}
	if _, _, err := PlatformIAMService.SaveAuthPolicy(request.PlatformAuthPolicySaveRequest{
		TenantID:        0,
		RoleID:          role.ID,
		PermissionCodes: []string{"tenant.view", "role.view"},
		Effect:          "allow",
	}, operator); err != nil {
		t.Fatalf("SaveAuthPolicy() error = %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID:    0,
		DomainType:  models.DomainTypePlatform,
		RoleID:      role.ID,
		SubjectType: models.SubjectTypePlatformStaff,
		SubjectID:   9001,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}).Error; err != nil {
		t.Fatalf("create role binding: %v", err)
	}

	if err := PlatformIAMService.DeleteAuthRole(request.PlatformAuthRoleDeleteRequest{RoleID: role.ID, Reason: "cleanup"}, operator); err != nil {
		t.Fatalf("DeleteAuthRole() error = %v", err)
	}
	deletedRole := repositories.PlatformIAMRepository.GetAuthRole(db, role.ID)
	if deletedRole == nil || deletedRole.Status != enums.StatusDeleted {
		t.Fatalf("deleted role = %+v", deletedRole)
	}
	var binding models.AuthRoleBinding
	if err := db.First(&binding, "role_id = ?", role.ID).Error; err != nil {
		t.Fatalf("role binding missing: %v", err)
	}
	if binding.Status != enums.StatusDeleted {
		t.Fatalf("role binding status = %d, want deleted", binding.Status)
	}
	var permission models.AuthRolePermission
	if err := db.First(&permission, "role_id = ?", role.ID).Error; err != nil {
		t.Fatalf("role permission missing: %v", err)
	}
	if permission.Status != enums.StatusDeleted {
		t.Fatalf("role permission status = %d, want deleted", permission.Status)
	}
}

func TestPlatformIAMDeleteAuthRoleRejectsBuiltinRole(t *testing.T) {
	_, operator := setupPlatformIAMTestDB(t)
	role, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   0,
		DomainType: models.DomainTypePlatform,
		Code:       "builtin_guard",
		Name:       "Builtin Guard",
		IsBuiltin:  true,
	}, operator)
	if err != nil {
		t.Fatalf("SaveAuthRole() error = %v", err)
	}
	if err := PlatformIAMService.DeleteAuthRole(request.PlatformAuthRoleDeleteRequest{RoleID: role.ID}, operator); err == nil {
		t.Fatalf("DeleteAuthRole() error = nil, want builtin role rejection")
	}
}

func TestPlatformIAMPermissionCheckUsesDenyPriority(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	tenantID := int64(1001)
	subjectID := int64(3001)

	allowRole, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   tenantID,
		DomainType: models.DomainTypeEnterprise,
		Code:       "service_manager",
		Name:       "Service Manager",
	}, operator)
	if err != nil {
		t.Fatalf("create allow role: %v", err)
	}
	denyRole, err := PlatformIAMService.SaveAuthRole(request.PlatformAuthRoleSaveRequest{
		TenantID:   tenantID,
		DomainType: models.DomainTypeEnterprise,
		Code:       "restricted_engineer",
		Name:       "Restricted Engineer",
	}, operator)
	if err != nil {
		t.Fatalf("create deny role: %v", err)
	}

	now := time.Now()
	for _, role := range []*models.AuthRole{allowRole, denyRole} {
		if err := db.Create(&models.AuthRoleBinding{
			TenantID:    tenantID,
			DomainType:  models.DomainTypeEnterprise,
			RoleID:      role.ID,
			SubjectType: models.SubjectTypeTenantMember,
			SubjectID:   subjectID,
			Status:      enums.StatusOk,
			EffectiveAt: &now,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}).Error; err != nil {
			t.Fatalf("create role binding: %v", err)
		}
	}

	if _, _, err := PlatformIAMService.SaveAuthPolicy(request.PlatformAuthPolicySaveRequest{
		TenantID:        tenantID,
		RoleID:          allowRole.ID,
		PermissionCodes: []string{"api:/api/enterprise/ticket/export"},
		Effect:          "allow",
	}, operator); err != nil {
		t.Fatalf("save allow policy: %v", err)
	}
	if _, _, err := PlatformIAMService.SaveAuthPolicy(request.PlatformAuthPolicySaveRequest{
		TenantID:        tenantID,
		RoleID:          denyRole.ID,
		PermissionCodes: []string{"api:/api/enterprise/ticket/export"},
		Effect:          "deny",
	}, operator); err != nil {
		t.Fatalf("save deny policy: %v", err)
	}

	ret, err := PlatformIAMService.CheckAuthPermission(request.PlatformAuthPermissionCheckRequest{
		TenantID:       tenantID,
		DomainType:     models.DomainTypeEnterprise,
		SubjectType:    models.SubjectTypeTenantMember,
		SubjectID:      subjectID,
		PermissionCode: "api:/api/enterprise/ticket/export",
	})
	if err != nil {
		t.Fatalf("CheckAuthPermission() error = %v", err)
	}
	if ret.Allowed || ret.Effect != "deny" {
		t.Fatalf("permission check = allowed %v effect %q, want deny", ret.Allowed, ret.Effect)
	}
}
