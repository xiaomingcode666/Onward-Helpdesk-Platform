package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestWebSocketProtocolCredential(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/ws/dashboard", nil)
	ctx.Request.Header.Set("Sec-WebSocket-Protocol", "other, rhd.access.ak_test, rhd.customer.jwt.test")

	if got := webSocketProtocolCredential(ctx, webSocketAccessTokenProtocolPrefix); got != "ak_test" {
		t.Fatalf("access websocket credential = %q, want %q", got, "ak_test")
	}
	if got := webSocketProtocolCredential(ctx, webSocketCustomerSessionProtocolPrefix); got != "jwt.test" {
		t.Fatalf("customer websocket credential = %q, want %q", got, "jwt.test")
	}
}

func TestWebSocketNegotiatedProtocolUsesStaticCredentialProtocol(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/ws/dashboard", nil)
	ctx.Request.Header.Set("Sec-WebSocket-Protocol", "rhd.credential, rhd.access.ak_test")

	if got := webSocketNegotiatedProtocol(ctx); got != webSocketCredentialProtocol {
		t.Fatalf("webSocketNegotiatedProtocol() = %q, want %q", got, webSocketCredentialProtocol)
	}

	ctx.Request.Header.Set("Sec-WebSocket-Protocol", "rhd.access.ak_test")
	if got := webSocketNegotiatedProtocol(ctx); got != "" {
		t.Fatalf("dynamic credential must not be echoed as negotiated protocol, got %q", got)
	}
}

func TestAuthServiceAuthenticateAcceptsWebSocketProtocolToken(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "websocket-user", "secret")
	svc := newAuthService()
	session, err := svc.Login(request.LoginRequest{
		Username: "websocket-user",
		Password: "secret",
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("login websocket user: %v", err)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/ws/dashboard", nil)
	ctx.Request.Header.Set("Sec-WebSocket-Protocol", webSocketAccessTokenProtocolPrefix+session.AccessToken)
	principal, err := svc.Authenticate(ctx)
	if err != nil {
		t.Fatalf("authenticate websocket protocol token: %v", err)
	}
	if principal == nil || principal.UserID != user.ID {
		t.Fatalf("websocket principal = %#v, want user %d", principal, user.ID)
	}
}

func TestExtractBearerToken(t *testing.T) {
	svc := newAuthService()

	if got := svc.extractBearerToken("Bearer token_123"); got != "token_123" {
		t.Fatalf("expected bearer token to be extracted, got %q", got)
	}

	if got := svc.extractBearerToken("token_123"); got != "" {
		t.Fatalf("expected raw token to be rejected by bearer extractor, got %q", got)
	}
}

func TestResolveTokenTTL(t *testing.T) {
	svc := newAuthService()

	if got := svc.resolveTokenTTL(config.AuthConfig{}); got != 72*time.Hour {
		t.Fatalf("default token TTL = %s, want 72h", got)
	}
	if got := svc.resolveTokenTTL(config.AuthConfig{TokenTTLHours: 24}); got != 24*time.Hour {
		t.Fatalf("configured token TTL = %s, want 24h", got)
	}
}

func TestGetTenantMemberPermissionsDoesNotInheritLegacyRoleFromAnotherTenant(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "legacy-cross-tenant", "secret")
	member := &models.TenantMember{
		TenantID:    1,
		UserID:      user.ID,
		DisplayName: "Legacy member",
		Status:      enums.StatusOk,
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	notificationView := &models.Permission{Name: "Notification view", Code: "notification.view", Status: enums.StatusOk}
	ticketView := &models.Permission{Name: "Ticket view", Code: "ticket.view", Status: enums.StatusOk}
	if err := db.Create([]*models.Permission{notificationView, ticketView}).Error; err != nil {
		t.Fatalf("create permissions: %v", err)
	}
	tenantOneRole := &models.Role{TenantID: 1, DomainType: models.DomainTypeEnterprise, Name: "Tenant one", Code: "tenant_one_legacy", Status: enums.StatusOk}
	tenantTwoRole := &models.Role{TenantID: 2, DomainType: models.DomainTypeEnterprise, Name: "Tenant two", Code: "tenant_two_legacy", Status: enums.StatusOk}
	if err := db.Create([]*models.Role{tenantOneRole, tenantTwoRole}).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := db.Create([]*models.RolePermission{
		{TenantID: 1, DomainType: models.DomainTypeEnterprise, RoleID: tenantOneRole.ID, PermissionID: notificationView.ID},
		{TenantID: 2, DomainType: models.DomainTypeEnterprise, RoleID: tenantTwoRole.ID, PermissionID: ticketView.ID},
	}).Error; err != nil {
		t.Fatalf("create role permissions: %v", err)
	}
	if err := db.Create([]*models.UserRole{
		{TenantID: 1, DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember, UserID: user.ID, RoleID: tenantOneRole.ID},
		{TenantID: 2, DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember, UserID: user.ID, RoleID: tenantTwoRole.ID},
	}).Error; err != nil {
		t.Fatalf("create user roles: %v", err)
	}

	permissions, err := newAuthService().GetTenantMemberPermissions(db, 1, member.ID, user.ID)
	if err != nil {
		t.Fatalf("resolve tenant permissions: %v", err)
	}
	if !slices.Contains(permissions, "notification.view") {
		t.Fatalf("permissions = %v, want tenant-one notification.view", permissions)
	}
	if slices.Contains(permissions, "ticket.view") {
		t.Fatalf("permissions = %v, inherited ticket.view from tenant two", permissions)
	}
}

func TestEnterpriseIAMBindingOverridesLegacyAdminPermissions(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "iam-viewer-overrides-legacy", "secret")
	member := &models.TenantMember{
		TenantID:    1,
		UserID:      user.ID,
		DisplayName: "IAM viewer",
		Status:      enums.StatusOk,
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	productView := &models.Permission{Name: "Product view", Code: constants.PermissionProductView.Code, Status: enums.StatusOk}
	productUpdate := &models.Permission{Name: "Product update", Code: constants.PermissionProductUpdate.Code, Status: enums.StatusOk}
	if err := db.Create([]*models.Permission{productView, productUpdate}).Error; err != nil {
		t.Fatalf("create legacy permissions: %v", err)
	}
	legacyAdmin := &models.Role{TenantID: 1, DomainType: models.DomainTypeEnterprise, Name: "Legacy admin", Code: "legacy_admin", Status: enums.StatusOk}
	if err := db.Create(legacyAdmin).Error; err != nil {
		t.Fatalf("create legacy admin role: %v", err)
	}
	if err := db.Create([]*models.RolePermission{
		{TenantID: 1, DomainType: models.DomainTypeEnterprise, RoleID: legacyAdmin.ID, PermissionID: productView.ID},
		{TenantID: 1, DomainType: models.DomainTypeEnterprise, RoleID: legacyAdmin.ID, PermissionID: productUpdate.ID},
	}).Error; err != nil {
		t.Fatalf("create legacy role permissions: %v", err)
	}
	if err := db.Create(&models.UserRole{
		TenantID: 1, DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember,
		UserID: user.ID, RoleID: legacyAdmin.ID,
	}).Error; err != nil {
		t.Fatalf("create legacy admin binding: %v", err)
	}

	viewer := &models.AuthRole{
		TenantID: 1, DomainType: models.DomainTypeEnterprise, Code: EnterpriseRoleViewer,
		Name: "Enterprise viewer", Status: enums.StatusOk,
	}
	if err := db.Create(viewer).Error; err != nil {
		t.Fatalf("create IAM viewer role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 1, DomainType: models.DomainTypeEnterprise, RoleID: viewer.ID,
		SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create IAM viewer binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 1, RoleID: viewer.ID, PermissionCode: constants.PermissionProductView.Code,
		Effect: "allow", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create IAM viewer permission: %v", err)
	}

	principal := &dto.AuthPrincipal{
		UserID: user.ID, TenantID: 1, DomainType: models.DomainTypeEnterprise,
		SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID,
		Roles: []string{legacyAdmin.Code}, Permissions: []string{constants.PermissionProductUpdate.Code},
	}
	newAuthService().mergeDomainAuthScope(db, principal)
	if !slices.Equal(principal.Roles, []string{EnterpriseRoleViewer}) {
		t.Fatalf("roles = %v, want only %s", principal.Roles, EnterpriseRoleViewer)
	}
	if !slices.Contains(principal.Permissions, constants.PermissionProductView.Code) {
		t.Fatalf("permissions = %v, want product view", principal.Permissions)
	}
	if slices.Contains(principal.Permissions, constants.PermissionProductUpdate.Code) {
		t.Fatalf("permissions = %v, legacy product update must not survive IAM binding", principal.Permissions)
	}
}

func TestAuthServiceLoginCreatesSingleAccessSession(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "admin", "secret")
	svc := newAuthService()

	ret, err := svc.Login(request.LoginRequest{
		Username: " admin ",
		Password: "secret",
	}, config.AuthConfig{TokenTTLHours: 2, MaxFailedAttempts: 5, CredentialLockMinute: 15}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if ret.AccessToken == "" || !strings.HasPrefix(ret.AccessToken, "ak_") {
		t.Fatalf("expected ak_ access token, got %q", ret.AccessToken)
	}
	if ret.ExpiresAt == "" {
		t.Fatal("expected expiresAt to be returned")
	}

	var sessions []models.LoginSession
	if err := db.Find(&sessions).Error; err != nil {
		t.Fatalf("query login sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected exactly one session, got %d", len(sessions))
	}
	if sessions[0].Token != ret.AccessToken {
		t.Fatalf("expected session token %q, got %q", ret.AccessToken, sessions[0].Token)
	}
	if sessions[0].UserID != user.ID {
		t.Fatalf("expected session user %d, got %d", user.ID, sessions[0].UserID)
	}
	if sessions[0].ClientType != "admin_web" {
		t.Fatalf("expected admin_web client type, got %q", sessions[0].ClientType)
	}

	logs := findCredentialLogs(t, db)
	if len(logs) != 1 {
		t.Fatalf("expected one credential log, got %d", len(logs))
	}
	if !logs[0].Success || logs[0].Principal != "admin" || logs[0].UserID != user.ID {
		t.Fatalf("unexpected success credential log: %+v", logs[0])
	}
}

func TestUserServiceUpdateOwnProfilePersistsContactAndLocale(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "profile-owner", "secret")
	existingEmail := "taken@example.com"
	other := createAuthTestUser(t, db, "profile-other", "secret")
	if err := repositories.UserRepository.Updates(db, other.ID, map[string]any{"email": &existingEmail}); err != nil {
		t.Fatalf("seed existing email: %v", err)
	}

	operator := &dto.AuthPrincipal{UserID: user.ID, Username: user.Username}
	email := "owner@example.com"
	mobile := "18800001111"
	updated, err := newUserService().UpdateOwnProfile(request.UpdateOwnProfileRequest{
		Nickname: "Owner Name",
		Email:    &email,
		Mobile:   &mobile,
		Locale:   "es",
	}, operator)
	if err != nil {
		t.Fatalf("update own profile: %v", err)
	}
	if updated.Nickname != "Owner Name" || updated.Email == nil || *updated.Email != email || updated.Mobile == nil || *updated.Mobile != mobile {
		t.Fatalf("updated contact profile = %+v", updated)
	}
	if updated.Locale != "es-ES" {
		t.Fatalf("updated locale = %q, want es-ES", updated.Locale)
	}

	if _, err := newUserService().UpdateOwnProfile(request.UpdateOwnProfileRequest{
		Nickname: "Owner Name",
		Email:    &existingEmail,
		Locale:   "zh-CN",
	}, operator); !hasCode(err, errorsx.CodeInvalidParam) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestAuthServiceLoginResolvesPartnerIdentity(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "supplier", "secret")
	now := time.Now()
	company := &models.PartnerCompany{
		TenantID:    27,
		PartnerNo:   "SUP-027",
		Name:        "Supplier 027",
		PartnerType: "module_supplier",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID:         company.TenantID,
		PartnerCompanyID: company.ID,
		UserID:           user.ID,
		DisplayName:      "Supplier Engineer",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}

	ret, err := newAuthService().Login(request.LoginRequest{
		Username: user.Username,
		Password: "secret",
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("partner login failed: %v", err)
	}
	if ret.TenantID != company.TenantID || ret.DomainType != models.DomainTypePartner ||
		ret.SubjectType != models.SubjectTypePartnerAccount || ret.SubjectID != account.ID || ret.PartnerAccountID != account.ID {
		t.Fatalf("unexpected partner identity response: %+v", ret)
	}
}

func TestAuthServiceLoginMergesLegacyPartnerFallbackRolePermissions(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "legacy-supplier-admin", "secret")
	now := time.Now()
	company := &models.PartnerCompany{
		TenantID:    127,
		PartnerNo:   "SUP-127",
		Name:        "Legacy Supplier 127",
		PartnerType: "module_supplier",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID:         company.TenantID,
		PartnerCompanyID: company.ID,
		UserID:           user.ID,
		DisplayName:      "Legacy Supplier Admin",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	if err := EnsureTenantDefaultIAMRolesDB(db, company.TenantID, nil); err != nil {
		t.Fatalf("ensure tenant default roles: %v", err)
	}

	ret, err := newAuthService().Login(request.LoginRequest{
		Username:   user.Username,
		Password:   "secret",
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("legacy partner admin login failed: %v", err)
	}
	if !slices.Contains(ret.Roles, PartnerRoleAdmin) {
		t.Fatalf("legacy partner admin role was not merged: roles=%v", ret.Roles)
	}
	for _, permission := range []string{
		constants.PermissionPartnerMemberView.Code,
		constants.PermissionPartnerMemberInvite.Code,
		constants.PermissionPartnerMemberUpdate.Code,
	} {
		if !slices.Contains(ret.Permissions, permission) {
			t.Fatalf("legacy partner admin missing %q: permissions=%v", permission, ret.Permissions)
		}
	}
}

func TestAuthServiceRejectsDisabledPartnerLoginAndExistingSession(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "disabled-supplier", "secret")
	now := time.Now()
	company := &models.PartnerCompany{
		TenantID:    28,
		PartnerNo:   "SUP-028",
		Name:        "Supplier 028",
		PartnerType: "module_supplier",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID:         company.TenantID,
		PartnerCompanyID: company.ID,
		UserID:           user.ID,
		DisplayName:      "Disabled Supplier Engineer",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}

	svc := newAuthService()
	activeLogin, err := svc.Login(request.LoginRequest{
		Username:   user.Username,
		Password:   "secret",
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("active partner login failed: %v", err)
	}
	if err := db.Model(account).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatalf("disable partner account: %v", err)
	}

	if _, err := svc.Login(request.LoginRequest{
		Username:   user.Username,
		Password:   "secret",
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test"); !hasCode(err, errorsx.CodeAuthInvalidAccount) {
		t.Fatalf("disabled partner login error = %v, want invalid account", err)
	}
	var sessionCount int64
	if err := db.Model(&models.LoginSession{}).Where("user_id = ?", user.ID).Count(&sessionCount).Error; err != nil {
		t.Fatalf("count partner sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("disabled partner login created a session: count=%d", sessionCount)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/partner/v1/profile", nil)
	ctx.Request.Header.Set("Authorization", "Bearer "+activeLogin.AccessToken)
	if _, err := svc.Authenticate(ctx); err == nil {
		t.Fatal("existing partner session remained valid after account disable")
	}
	principal := &dto.AuthPrincipal{UserID: user.ID}
	svc.applyIdentityContext(db, principal, models.DomainTypePartner)
	if principal.PartnerAccountID != 0 || principal.SubjectType == models.SubjectTypePartnerAccount {
		t.Fatalf("disabled partner account was restored through any-status fallback: %+v", principal)
	}
}

func TestAuthServiceLoginUsesCustomerDomainRoleBindings(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "customer-account", "secret123")
	now := time.Now()
	customerUser := &models.CustomerUser{
		TenantID: 44, CustomerOrgID: 7, UserID: user.ID, DisplayName: "Customer Account",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	role := &models.AuthRole{
		TenantID: 44, DomainType: models.DomainTypeCustomer, Code: CustomerRoleUser, Name: "Customer User",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create customer role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 44, DomainType: models.DomainTypeCustomer, RoleID: role.ID,
		SubjectType: models.SubjectTypeCustomerUser, SubjectID: customerUser.ID,
		Status: enums.StatusOk, EffectiveAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 44, RoleID: role.ID, PermissionCode: "device.view", Effect: "allow",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer role permission: %v", err)
	}

	ret, err := newAuthService().Login(request.LoginRequest{
		Username: user.Username, Password: "secret123", DomainType: models.DomainTypeCustomer,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("customer login failed: %v", err)
	}
	if ret.DomainType != models.DomainTypeCustomer || ret.TenantID != 44 || ret.SubjectID != customerUser.ID {
		t.Fatalf("unexpected customer identity response: %+v", ret)
	}
	if !slices.Contains(ret.Roles, CustomerRoleUser) || !slices.Contains(ret.Permissions, "device.view") {
		t.Fatalf("customer IAM scope was not merged: roles=%v permissions=%v", ret.Roles, ret.Permissions)
	}
	var session models.LoginSession
	if err := db.First(&session, "token = ?", ret.AccessToken).Error; err != nil || session.DomainType != models.DomainTypeCustomer {
		t.Fatalf("customer login domain was not persisted: session=%+v error=%v", session, err)
	}
}

func TestAuthServiceLoginAllowsPendingCustomerDeviceBindingIdentity(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "pending-customer", "secret123")

	ret, err := newAuthService().Login(request.LoginRequest{
		Username: user.Username, Password: "secret123", DomainType: models.DomainTypeCustomer,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("pending customer login failed: %v", err)
	}
	if ret.DomainType != models.DomainTypeCustomer || ret.TenantID != 0 || ret.SubjectType != models.SubjectTypePendingCustomer || ret.SubjectID != user.ID {
		t.Fatalf("unexpected pending customer identity: %+v", ret)
	}
	var session models.LoginSession
	if err := db.First(&session, "token = ?", ret.AccessToken).Error; err != nil {
		t.Fatalf("query pending customer session: %v", err)
	}
	if session.DomainType != models.DomainTypeCustomer || session.SubjectType != models.SubjectTypePendingCustomer {
		t.Fatalf("pending customer session domain was not persisted: %+v", session)
	}
}

func TestAuthServiceIssuesEnterpriseCustomerSupportSession(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	operatorUser := createAuthTestUser(t, db, "enterprise-agent", "secret123")
	customerAccount := createAuthTestUser(t, db, "customer-portal-account", "secret123")
	now := time.Now()
	customerUser := &models.CustomerUser{
		TenantID: 88, CustomerOrgID: 701, UserID: customerAccount.ID, DisplayName: "Customer Visitor",
		Locale: "en", Timezone: "UTC", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	role := &models.AuthRole{
		TenantID: 88, DomainType: models.DomainTypeCustomer, Code: CustomerRoleUser, Name: "Customer User",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create customer role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 88, DomainType: models.DomainTypeCustomer, RoleID: role.ID,
		SubjectType: models.SubjectTypeCustomerUser, SubjectID: customerUser.ID,
		Status: enums.StatusOk, EffectiveAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer role binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 88, RoleID: role.ID, PermissionCode: "ticket.view", Effect: "allow",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer permission: %v", err)
	}
	grant := &models.TemporaryAccessGrant{
		TenantID: 88, GranteeType: models.SubjectTypeTenantMember, GranteeID: 301,
		ResourceType: models.SubjectTypeCustomerUser, ResourceID: "1",
		Reason: "support visit", ApprovedBy: operatorUser.ID, ApprovedAt: &now,
		ExpiredAt: now.Add(time.Hour), Status: models.AccessGrantStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	grant.ResourceID = strconv.FormatInt(customerUser.ID, 10)
	if err := db.Create(grant).Error; err != nil {
		t.Fatalf("create temporary access grant: %v", err)
	}

	operator := &dto.AuthPrincipal{
		UserID: operatorUser.ID, Username: operatorUser.Username, TenantID: 88,
		DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember, SubjectID: 301,
		Status: enums.StatusOk,
	}
	ret, err := newAuthService().IssueEnterpriseCustomerSessionDB(db, operator, customerUser, grant, "127.0.0.1", "go-test", config.AuthConfig{TokenTTLHours: 12})
	if err != nil {
		t.Fatalf("IssueEnterpriseCustomerSessionDB() error = %v", err)
	}
	if ret.DomainType != models.DomainTypeCustomer || ret.TenantID != 88 || ret.SubjectID != customerUser.ID || ret.SupportGrantID != grant.ID {
		t.Fatalf("unexpected customer support identity: %+v", ret)
	}
	if ret.User.ID != customerAccount.ID || ret.ImpersonatedBy != operatorUser.Username {
		t.Fatalf("support session should expose target account and initiator: %+v", ret)
	}
	if !slices.Contains(ret.Roles, CustomerRoleUser) || !slices.Contains(ret.Permissions, "ticket.view") {
		t.Fatalf("customer support session lost customer scope: roles=%v permissions=%v", ret.Roles, ret.Permissions)
	}
	var session models.LoginSession
	if err := db.First(&session, "token = ?", ret.AccessToken).Error; err != nil {
		t.Fatalf("query customer support session: %v", err)
	}
	if session.UserID != customerAccount.ID || session.DomainType != models.DomainTypeCustomer || session.SupportGrantID != grant.ID || session.ImpersonatedBy != operatorUser.Username {
		t.Fatalf("unexpected persisted customer support session: %+v", session)
	}
}

func TestAuthServiceIssuesEnterpriseMemberSupportSession(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	operatorUser := createAuthTestUser(t, db, "tenant-admin", "secret123")
	employeeUser := createAuthTestUser(t, db, "field-engineer", "secret123")
	now := time.Now()
	member := &models.TenantMember{
		TenantID: 91, UserID: employeeUser.ID, DisplayName: "Field Engineer", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create enterprise member: %v", err)
	}
	role := &models.AuthRole{
		TenantID: 91, DomainType: models.DomainTypeEnterprise, Code: EnterpriseRoleEngineer, Name: "Service Engineer",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create enterprise role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 91, DomainType: models.DomainTypeEnterprise, RoleID: role.ID,
		SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID,
		Status: enums.StatusOk, EffectiveAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create enterprise role binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 91, RoleID: role.ID, PermissionCode: "ticket.view", Effect: "allow",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create enterprise permission: %v", err)
	}
	otherMember := &models.TenantMember{
		TenantID: 92, UserID: employeeUser.ID, DisplayName: "Other Tenant Admin", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(otherMember).Error; err != nil {
		t.Fatalf("create other-tenant member: %v", err)
	}
	otherRole := &models.AuthRole{
		TenantID: 92, DomainType: models.DomainTypeEnterprise, Code: "other_tenant_admin", Name: "Other Tenant Admin",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(otherRole).Error; err != nil {
		t.Fatalf("create other-tenant role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 92, DomainType: models.DomainTypeEnterprise, RoleID: otherRole.ID,
		SubjectType: models.SubjectTypeTenantMember, SubjectID: otherMember.ID,
		Status: enums.StatusOk, EffectiveAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other-tenant role binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 92, RoleID: otherRole.ID, PermissionCode: "user.update", Effect: "allow",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other-tenant permission: %v", err)
	}
	grant := &models.TemporaryAccessGrant{
		TenantID: 91, GranteeType: models.SubjectTypeTenantMember, GranteeID: 301,
		ResourceType: models.SubjectTypeTenantMember, ResourceID: strconv.FormatInt(member.ID, 10),
		Reason: "employee portal support", ApprovedBy: operatorUser.ID, ApprovedAt: &now,
		ExpiredAt: now.Add(time.Hour), Status: models.AccessGrantStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(grant).Error; err != nil {
		t.Fatalf("create employee access grant: %v", err)
	}
	operator := &dto.AuthPrincipal{
		UserID: operatorUser.ID, Username: operatorUser.Username, TenantID: 91,
		DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember, SubjectID: 301,
		Status: enums.StatusOk,
	}

	ret, err := newAuthService().IssueEnterpriseMemberSessionDB(db, operator, member, grant, "127.0.0.1", "go-test", config.AuthConfig{TokenTTLHours: 12})
	if err != nil {
		t.Fatalf("IssueEnterpriseMemberSessionDB() error = %v", err)
	}
	if ret.DomainType != models.DomainTypeEnterprise || ret.TenantID != 91 || ret.SubjectID != member.ID || ret.SupportGrantID != grant.ID || ret.SupportMode != models.SupportModeEmployeePortal {
		t.Fatalf("unexpected employee support identity: %+v", ret)
	}
	if ret.User.ID != employeeUser.ID || ret.ImpersonatedBy != operatorUser.Username {
		t.Fatalf("employee support session should expose target account and initiator: %+v", ret)
	}
	if !slices.Contains(ret.Roles, EnterpriseRoleEngineer) || !slices.Contains(ret.Permissions, "ticket.view") {
		t.Fatalf("employee support session lost employee scope: roles=%v permissions=%v", ret.Roles, ret.Permissions)
	}
	if slices.Contains(ret.Roles, otherRole.Code) || slices.Contains(ret.Permissions, "user.update") {
		t.Fatalf("employee support session leaked another tenant scope: roles=%v permissions=%v", ret.Roles, ret.Permissions)
	}
	var session models.LoginSession
	if err := db.First(&session, "token = ?", ret.AccessToken).Error; err != nil {
		t.Fatalf("query employee support session: %v", err)
	}
	if session.UserID != employeeUser.ID || session.SupportMode != models.SupportModeEmployeePortal || session.SupportGrantID != grant.ID || session.ImpersonatedBy != operatorUser.Username {
		t.Fatalf("unexpected persisted employee support session: %+v", session)
	}
}

func TestAuthServiceIssuesAuditedPlatformTenantSession(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "platform-support", "secret123")
	now := time.Now()
	staff := &models.PlatformStaffProfile{
		UserID: user.ID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(staff).Error; err != nil {
		t.Fatalf("create platform staff: %v", err)
	}
	member := &models.TenantMember{
		TenantID: 72, UserID: 9001, DisplayName: "Tenant Owner", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	role := &models.AuthRole{
		TenantID: 72, DomainType: models.DomainTypeEnterprise, Code: EnterpriseRoleOwner, Name: "Tenant Owner", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create tenant role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID: 72, DomainType: models.DomainTypeEnterprise, RoleID: role.ID,
		SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant role binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID: 72, RoleID: role.ID, PermissionCode: "user.create", Effect: "allow", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant permission: %v", err)
	}
	grant := &models.PlatformTenantGrant{
		PlatformStaffID: staff.ID, TenantID: 72, Reason: "support test", ApprovedBy: user.ID,
		ApprovedAt: &now, ExpiredAt: now.Add(time.Hour), Status: models.AccessGrantStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(grant).Error; err != nil {
		t.Fatalf("create platform tenant grant: %v", err)
	}
	operator := &dto.AuthPrincipal{UserID: user.ID, Username: user.Username, Status: enums.StatusOk}
	ret, err := newAuthService().IssuePlatformTenantSessionDB(db, operator, member, grant, "127.0.0.1", "go-test", config.AuthConfig{TokenTTLHours: 12})
	if err != nil {
		t.Fatalf("IssuePlatformTenantSessionDB() error = %v", err)
	}
	if ret.DomainType != models.DomainTypeEnterprise || ret.TenantID != 72 || ret.SubjectID != member.ID || ret.SupportGrantID != grant.ID {
		t.Fatalf("unexpected support session identity: %+v", ret)
	}
	if !slices.Contains(ret.Roles, EnterpriseRoleOwner) || !slices.Contains(ret.Permissions, "user.create") {
		t.Fatalf("support session lost tenant owner scope: roles=%v permissions=%v", ret.Roles, ret.Permissions)
	}
	var session models.LoginSession
	if err := db.First(&session, "token = ?", ret.AccessToken).Error; err != nil {
		t.Fatalf("query support session: %v", err)
	}
	if session.SupportGrantID != grant.ID || session.TenantID != 72 || session.ImpersonatedBy != user.Username {
		t.Fatalf("support session audit context = %+v", session)
	}
}

func TestAuthServiceLoginFailureWritesCredentialLogs(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	createAuthTestUser(t, db, "admin", "secret")
	svc := newAuthService()
	authCfg := config.AuthConfig{TokenTTLHours: 2, MaxFailedAttempts: 5, CredentialLockMinute: 15}

	if _, err := svc.Login(request.LoginRequest{Username: "missing", Password: "secret"}, authCfg, "127.0.0.1", "go-test"); !hasCode(err, errorsx.CodeAuthInvalidAccount) {
		t.Fatalf("expected invalid account for missing user, got %v", err)
	}
	if _, err := svc.Login(request.LoginRequest{Username: "admin", Password: "wrong"}, authCfg, "127.0.0.1", "go-test"); !hasCode(err, errorsx.CodeAuthInvalidAccount) {
		t.Fatalf("expected invalid account for password mismatch, got %v", err)
	}

	logs := findCredentialLogs(t, db)
	if len(logs) != 2 {
		t.Fatalf("expected two credential logs, got %d", len(logs))
	}
	if logs[0].Reason != "user not found" || logs[0].Success {
		t.Fatalf("unexpected missing-user log: %+v", logs[0])
	}
	if logs[1].Reason != "password mismatch" || logs[1].Success {
		t.Fatalf("unexpected password-mismatch log: %+v", logs[1])
	}
}

func TestAuthServiceLoginCredentialLockout(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "admin", "secret")
	now := time.Now()
	for i := 0; i < 2; i++ {
		if err := db.Create(&models.LoginCredentialLog{
			Principal: "admin",
			UserID:    user.ID,
			Success:   false,
			Reason:    "password mismatch",
			CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed credential log: %v", err)
		}
	}
	if err := db.Create(&models.LoginCredentialLog{
		Principal: "admin",
		UserID:    user.ID,
		Success:   false,
		Reason:    "password mismatch",
		CreatedAt: now.Add(-30 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed old credential log: %v", err)
	}

	svc := newAuthService()
	_, err := svc.Login(request.LoginRequest{Username: "admin", Password: "secret"}, config.AuthConfig{
		TokenTTLHours:        2,
		MaxFailedAttempts:    2,
		CredentialLockMinute: 15,
	}, "127.0.0.1", "go-test")
	if !hasCode(err, errorsx.CodeAuthCredentialLocked) {
		t.Fatalf("expected credential locked error, got %v", err)
	}

	var lockedLog models.LoginCredentialLog
	if err := db.Order("id DESC").Take(&lockedLog).Error; err != nil {
		t.Fatalf("query latest credential log: %v", err)
	}
	if lockedLog.Reason != "credential locked" || lockedLog.Success {
		t.Fatalf("unexpected locked credential log: %+v", lockedLog)
	}

	var sessionCount int64
	if err := db.Model(&models.LoginSession{}).Count(&sessionCount).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("expected no session while credential locked, got %d", sessionCount)
	}
}

func TestAuthServiceCredentialLockoutDoesNotExtendWhileLocked(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	createAuthTestUser(t, db, "admin", "secret")
	now := time.Now()
	entries := []models.LoginCredentialLog{
		{
			Principal: "admin",
			UserID:    1,
			Success:   false,
			Reason:    "password mismatch",
			CreatedAt: now.Add(-2 * time.Minute),
		},
		{
			Principal: "admin",
			UserID:    0,
			Success:   false,
			Reason:    "credential locked",
			CreatedAt: now.Add(-1 * time.Minute),
		},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("seed credential logs: %v", err)
	}

	ret, err := newAuthService().Login(request.LoginRequest{Username: "admin", Password: "secret"}, config.AuthConfig{
		TokenTTLHours:        2,
		MaxFailedAttempts:    2,
		CredentialLockMinute: 15,
	}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("expected locked attempt logs not to extend lockout, got %v", err)
	}
	if ret == nil || !strings.HasPrefix(ret.AccessToken, "ak_") {
		t.Fatalf("expected login response with ak_ token, got %+v", ret)
	}
}

func TestAuthServiceCredentialLockoutNormalizesPrincipalCase(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	createAuthTestUser(t, db, "admin", "secret")
	if err := db.Create(&models.LoginCredentialLog{
		Principal: "admin",
		UserID:    1,
		Success:   false,
		Reason:    "password mismatch",
		CreatedAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed credential log: %v", err)
	}

	_, err := newAuthService().Login(request.LoginRequest{Username: "ADMIN", Password: "secret"}, config.AuthConfig{
		TokenTTLHours:        2,
		MaxFailedAttempts:    1,
		CredentialLockMinute: 15,
	}, "127.0.0.1", "go-test")
	if !hasCode(err, errorsx.CodeAuthCredentialLocked) {
		t.Fatalf("expected normalized principal to be locked, got %v", err)
	}

	var lockedLog models.LoginCredentialLog
	if err := db.Order("id DESC").Take(&lockedLog).Error; err != nil {
		t.Fatalf("query latest credential log: %v", err)
	}
	if lockedLog.Principal != "admin" || lockedLog.Reason != "credential locked" {
		t.Fatalf("unexpected locked log: %+v", lockedLog)
	}
}

func TestAuthServiceCredentialLockoutDisabledWhenMaxAttemptsNonPositive(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	user := createAuthTestUser(t, db, "admin", "secret")
	now := time.Now()
	for i := 0; i < 3; i++ {
		if err := db.Create(&models.LoginCredentialLog{
			Principal: "admin",
			UserID:    user.ID,
			Success:   false,
			Reason:    "credential locked",
			CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed credential log: %v", err)
		}
	}

	ret, err := newAuthService().Login(request.LoginRequest{Username: "admin", Password: "secret"}, config.AuthConfig{
		TokenTTLHours:        2,
		MaxFailedAttempts:    0,
		CredentialLockMinute: 15,
	}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("expected lockout to be disabled, got %v", err)
	}
	if ret == nil || ret.AccessToken == "" {
		t.Fatalf("expected login response with access token, got %+v", ret)
	}
}

func TestValidateSessionTokenStates(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	svc := newAuthService()
	now := time.Now()

	if _, err := svc.validateSessionToken("  "); !hasCode(err, errorsx.CodeAuthUnauthorized) {
		t.Fatalf("expected unauthorized for empty token, got %v", err)
	}
	if _, err := svc.validateSessionToken("missing"); !hasCode(err, errorsx.CodeAuthInvalidToken) {
		t.Fatalf("expected invalid token for missing session, got %v", err)
	}

	revokedAt := now
	if err := db.Create(&models.LoginSession{
		UserID:     1,
		Token:      "ak_revoked",
		ClientType: "admin_web",
		ExpiredAt:  now.Add(time.Hour),
		RevokedAt:  &revokedAt,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("seed revoked session: %v", err)
	}
	if _, err := svc.validateSessionToken("ak_revoked"); !hasCode(err, errorsx.CodeAuthInvalidToken) {
		t.Fatalf("expected invalid token for revoked session, got %v", err)
	}

	if err := db.Create(&models.LoginSession{
		UserID:     1,
		Token:      "ak_expired",
		ClientType: "admin_web",
		ExpiredAt:  now.Add(-time.Hour),
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("seed expired session: %v", err)
	}
	if _, err := svc.validateSessionToken("ak_expired"); !hasCode(err, errorsx.CodeAuthInvalidToken) {
		t.Fatalf("expected invalid token for expired session, got %v", err)
	}

	if err := db.Create(&models.LoginSession{
		UserID:     1,
		Token:      "ak_valid",
		ClientType: "admin_web",
		ExpiredAt:  now.Add(time.Hour),
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("seed valid session: %v", err)
	}
	session, err := svc.validateSessionToken("ak_valid")
	if err != nil {
		t.Fatalf("expected valid session token, got %v", err)
	}
	if session.Token != "ak_valid" {
		t.Fatalf("expected valid session token ak_valid, got %q", session.Token)
	}
}

func TestAuthServiceLogoutRevokesCurrentTokenOnly(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	now := time.Now()
	sessions := []models.LoginSession{
		{
			UserID:     1,
			Token:      "ak_current",
			ClientType: "admin_web",
			ExpiredAt:  now.Add(time.Hour),
			AuditFields: models.AuditFields{
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		{
			UserID:     1,
			Token:      "ak_other",
			ClientType: "admin_web",
			ExpiredAt:  now.Add(time.Hour),
			AuditFields: models.AuditFields{
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if err := db.Create(&sessions).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	if err := newAuthService().Logout("Bearer ak_current"); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	var current models.LoginSession
	if err := db.Take(&current, "token = ?", "ak_current").Error; err != nil {
		t.Fatalf("query current session: %v", err)
	}
	if current.RevokedAt == nil {
		t.Fatal("expected current session to be revoked")
	}
	var other models.LoginSession
	if err := db.Take(&other, "token = ?", "ak_other").Error; err != nil {
		t.Fatalf("query other session: %v", err)
	}
	if other.RevokedAt != nil {
		t.Fatal("expected other session to remain active")
	}
}

func setupAuthServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.Role{},
		&models.Permission{},
		&models.UserRole{},
		&models.RolePermission{},
		&models.UserPermission{},
		&models.LoginSession{},
		&models.LoginCredentialLog{},
		&models.PartnerCompany{},
		&models.PartnerAccount{},
		&models.PlatformStaffProfile{},
		&models.PlatformTenantGrant{},
		&models.TemporaryAccessGrant{},
		&models.TenantMember{},
		&models.CustomerUser{},
		&models.AuthRole{},
		&models.AuthRoleBinding{},
		&models.AuthRolePermission{},
	); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createAuthTestUser(t *testing.T, db *gorm.DB, username, password string) *models.User {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now()
	user := &models.User{
		Username: username,
		Nickname: username,
		Password: string(passwordHash),
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create auth test user: %v", err)
	}
	return user
}

func findCredentialLogs(t *testing.T, db *gorm.DB) []models.LoginCredentialLog {
	t.Helper()
	var logs []models.LoginCredentialLog
	if err := db.Order("id ASC").Find(&logs).Error; err != nil {
		t.Fatalf("query credential logs: %v", err)
	}
	return logs
}

func hasCode(err error, code int) bool {
	if err == nil {
		return false
	}
	var codeErr *web.CodeError
	if errors.As(err, &codeErr) {
		return codeErr.Code == code
	}
	return false
}
