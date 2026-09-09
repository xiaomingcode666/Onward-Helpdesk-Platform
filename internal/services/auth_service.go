package services

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/repositories"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	authPrincipalContextKey = "authPrincipal"
	defaultAuthTokenTTL     = 72 * time.Hour
)

var AuthService = newAuthService()

func newAuthService() *authService {
	return &authService{}
}

type authService struct {
}

func (s *authService) GetAuthPrincipal(ctx *gin.Context) *dto.AuthPrincipal {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Get(authPrincipalContextKey)
	if principal, ok := v.(*dto.AuthPrincipal); ok {
		return principal
	}
	return nil
}

func (s *authService) VerifyPassword(userID int64, password string) error {
	if userID <= 0 || strings.TrimSpace(password) == "" {
		return errorsx.InvalidParam("current password is required")
	}
	user := UserService.Get(userID)
	if user == nil || user.Status != enums.StatusOk || strs.IsBlank(user.Password) ||
		bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return errorsx.InvalidAccount("current password is incorrect")
	}
	return nil
}

func (s *authService) setAuthPrincipal(ctx *gin.Context, user *models.User, roles, permissions []string, preferredDomain string) *dto.AuthPrincipal {
	principal := &dto.AuthPrincipal{
		UserID:      user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Status:      user.Status,
		Roles:       roles,
		Permissions: permissions,
		Locale:      userLocale(user),
		Timezone:    userTimezone(user),
	}
	s.applyIdentityContext(sqls.DB(), principal, preferredDomain)
	s.mergeDomainAuthScope(sqls.DB(), principal)
	ctx.Set(authPrincipalContextKey, principal)
	return principal
}

func (s *authService) RequirePermission(ctx *gin.Context, permission constants.Permission) (principal *dto.AuthPrincipal, err error) {
	if principal = s.GetAuthPrincipal(ctx); principal == nil {
		if principal, err = s.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	if principal == nil {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}

	if !s.HasPermission(ctx, permission.Code) {
		return principal, errorsx.ForbiddenI18n("error.e0225")
	}
	return principal, nil
}

func (s *authService) RequireAnyPermission(ctx *gin.Context, permissions ...constants.Permission) (principal *dto.AuthPrincipal, err error) {
	if principal = s.GetAuthPrincipal(ctx); principal == nil {
		if principal, err = s.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	if principal == nil {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}

	for _, permission := range permissions {
		if s.HasPermission(ctx, permission.Code) {
			return principal, nil
		}
	}
	return principal, errorsx.ForbiddenI18n("error.e0225")
}

func (s *authService) Login(req request.LoginRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	principal := normalizeLoginPrincipal(username)
	password := req.Password
	if username == "" || strings.TrimSpace(password) == "" {
		return nil, errorsx.InvalidParamI18n("error.e0258")
	}

	if s.isCredentialLocked(principal, authCfg) {
		_ = s.createLoginCredentialLog(principal, 0, false, clientIP, userAgent, "credential locked")
		return nil, errorsx.CredentialLockedI18n("error.e0270")
	}

	user := UserService.GetByUsername(username)
	if user == nil {
		user = UserService.GetByEmail(username)
	}
	if user == nil {
		user = repositories.CustomerPortalRepository.FindUserByCustomerEmail(sqls.DB(), username)
	}
	if user == nil || user.Status != enums.StatusOk {
		_ = s.createLoginCredentialLog(principal, 0, false, clientIP, userAgent, "user not found")
		return nil, errorsx.InvalidAccountI18n("error.e0260")
	}
	if strs.IsBlank(user.Password) || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		_ = s.createLoginCredentialLog(principal, user.ID, false, clientIP, userAgent, "password mismatch")
		return nil, errorsx.InvalidAccountI18n("error.e0260")
	}
	if strings.TrimSpace(req.DomainType) == models.DomainTypePartner &&
		repositories.TicketSupplierCollaborationRepository.FindActivePartnerAccountByUser(sqls.DB(), user.ID) == nil {
		_ = s.createLoginCredentialLog(principal, user.ID, false, clientIP, userAgent, "inactive partner account")
		return nil, errorsx.InvalidAccountI18n("error.e0260")
	}

	var ret *response.LoginResponse
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var dbErr error
		ret, dbErr = s.issueTokens(ctx, user, clientIP, userAgent, authCfg, req.DomainType)
		if dbErr != nil {
			return dbErr
		}
		if strings.TrimSpace(req.DomainType) == models.DomainTypePlatform && ret.DomainType != models.DomainTypePlatform {
			return errorsx.InvalidAccountI18n("error.e0260")
		}
		s.enrichLoginResponse(ctx.Tx, ret, user.ID, strings.TrimSpace(req.DomainType), req.PortalChoiceConfirmed)
		if dbErr = repositories.UserRepository.Updates(ctx.Tx, user.ID, map[string]any{
			"last_login_at":    time.Now(),
			"last_login_ip":    clientIP,
			"update_user_id":   user.ID,
			"update_user_name": user.Username,
			"updated_at":       time.Now(),
		}); dbErr != nil {
			return dbErr
		}
		return nil
	}); err != nil {
		return nil, err
	}

	_ = s.createLoginCredentialLog(principal, user.ID, true, clientIP, userAgent, "")
	return ret, nil
}

func (s *authService) Logout(accessToken string) error {
	accessToken = s.extractBearerToken(accessToken)
	now := time.Now()
	var loggedOutUserID int64
	if accessToken != "" {
		if session := LoginSessionService.FindOne(sqls.NewCnd().Eq("token", accessToken)); session != nil && session.RevokedAt == nil {
			if err := LoginSessionService.Updates(session.ID, map[string]any{
				"revoked_at": now,
				"updated_at": now,
			}); err != nil {
				return err
			}
			loggedOutUserID = session.UserID
		}
	}
	if loggedOutUserID > 0 {
		s.markEngineerOfflineIfIdle(loggedOutUserID, now)
	}
	return nil
}

// markEngineerOfflineIfIdle 登出时若该用户没有其他活跃登录会话且是工程师，
// 则把其工作状态置为离线，避免已下线的工程师继续被自动派单。
func (s *authService) markEngineerOfflineIfIdle(userID int64, now time.Time) {
	activeSessions := LoginSessionService.Count(sqls.NewCnd().
		Eq("user_id", userID).
		Where("revoked_at IS NULL").
		Gt("expired_at", now))
	if activeSessions > 0 {
		return
	}
	if err := AgentWorkStatusService.MarkOffline(userID, now); err != nil {
		slog.Warn("mark engineer offline on logout failed", "user_id", userID, "error", err)
	}
}

func (s *authService) Authenticate(ctx *gin.Context) (*dto.AuthPrincipal, error) {
	if principal := s.GetAuthPrincipal(ctx); principal != nil {
		return principal, nil
	}

	token := s.extractBearerToken(ctx.GetHeader("Authorization"))
	if token == "" {
		token = webSocketProtocolCredential(ctx, webSocketAccessTokenProtocolPrefix)
	}
	if token == "" {
		token = strings.TrimSpace(ctx.Query("accessToken"))
	}
	if token == "" {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}

	session, err := s.validateSessionToken(token)
	if err != nil {
		return nil, err
	}
	if session.DomainType == models.DomainTypePartner {
		account := repositories.TicketSupplierCollaborationRepository.FindActivePartnerAccountByUser(sqls.DB(), session.UserID)
		if account == nil || session.SubjectType != models.SubjectTypePartnerAccount || session.SubjectID != account.ID {
			return nil, errorsx.InvalidTokenI18n("error.e0267")
		}
	}

	user := UserService.Get(session.UserID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.UnauthorizedI18n("error.e0256")
	}

	var principal *dto.AuthPrincipal
	if session.SupportGrantID > 0 {
		switch session.DomainType {
		case models.DomainTypeEnterprise:
			if session.SupportMode == models.SupportModeEmployeePortal ||
				(session.SupportMode == "" && s.isEnterpriseMemberSupportSession(sqls.DB(), user, session)) {
				principal, err = s.buildEnterpriseMemberSupportPrincipal(sqls.DB(), user, session)
			} else {
				principal, err = s.buildPlatformTenantPrincipal(sqls.DB(), user, session)
			}
		case models.DomainTypeCustomer:
			principal, err = s.buildEnterpriseCustomerSupportPrincipal(sqls.DB(), user, session)
		default:
			err = errorsx.InvalidTokenI18n("error.e0267")
		}
		if err != nil {
			return nil, err
		}
		ctx.Set(authPrincipalContextKey, principal)
	} else {
		roles, permissions, err := s.loadUserAuthScope(sqls.DB(), user.ID)
		if err != nil {
			return nil, err
		}
		principal = s.setAuthPrincipal(ctx, user, roles, permissions, session.DomainType)
	}

	now := time.Now()
	_ = LoginSessionService.Updates(session.ID, map[string]any{
		"last_seen_at": now,
		"updated_at":   now,
	})

	return principal, nil
}

func (s *authService) HasPermission(ctx *gin.Context, permissionCode string) bool {
	principal := s.GetAuthPrincipal(ctx)
	if principal == nil {
		return false
	}
	return slices.Contains(principal.Permissions, permissionCode)
}

func (s *authService) CurrentProfile(ctx *gin.Context) (*response.LoginResponse, error) {
	principal := s.GetAuthPrincipal(ctx)
	if principal == nil {
		var err error
		principal, err = s.Authenticate(ctx)
		if err != nil {
			return nil, err
		}
	}

	user := UserService.Get(principal.UserID)
	return buildLoginResponse("", "", user, principal), nil
}

func (s *authService) GetUserRoles(userID int64) ([]models.Role, error) {
	return s.loadUserRoles(sqls.DB(), userID)
}

func (s *authService) GetUserPermissions(userID int64) ([]string, error) {
	return s.loadUserPermissionCodes(sqls.DB(), userID)
}

// GetTenantMemberPermissions resolves the effective RBAC permission set for an
// enterprise member. Domain-aware IAM bindings are authoritative; legacy roles
// are used only for members that have not been migrated. Explicit subject
// overrides are applied last.
func (s *authService) GetTenantMemberPermissions(db *gorm.DB, tenantID, memberID, userID int64) ([]string, error) {
	permissionSet := make(map[string]bool)
	if db == nil || tenantID <= 0 || memberID <= 0 || userID <= 0 {
		return []string{}, nil
	}
	bindings := make([]models.AuthRoleBinding, 0)
	if db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		var err error
		bindings, err = repositories.PlatformIAMRepository.FindRoleBindings(
			db,
			tenantID,
			models.DomainTypeEnterprise,
			models.SubjectTypeTenantMember,
			memberID,
		)
		if err != nil {
			return nil, err
		}
	}
	if len(bindings) == 0 && db.Migrator().HasTable(&models.Role{}) &&
		db.Migrator().HasTable(&models.Permission{}) &&
		db.Migrator().HasTable(&models.UserRole{}) &&
		db.Migrator().HasTable(&models.RolePermission{}) {
		legacy, err := s.loadTenantLegacyPermissionCodes(db, tenantID, userID)
		if err != nil {
			return nil, err
		}
		for _, code := range legacy {
			permissionSet[code] = true
		}
	}
	if len(bindings) > 0 && db.Migrator().HasTable(&models.AuthRolePermission{}) {
		roleIDs := make([]int64, 0, len(bindings))
		for _, binding := range bindings {
			roleIDs = append(roleIDs, binding.RoleID)
		}
		rows, err := repositories.PlatformIAMRepository.FindAuthRolePermissionRows(db, tenantID, roleIDs)
		if err != nil {
			return nil, err
		}
		denied := make(map[string]bool)
		for _, row := range rows {
			if row.Effect == "deny" {
				denied[row.PermissionCode] = true
				delete(permissionSet, row.PermissionCode)
				continue
			}
			if !denied[row.PermissionCode] {
				permissionSet[row.PermissionCode] = true
			}
		}
	}
	overrides, err := repositories.PlatformIAMRepository.FindSubjectPermissionOverrides(
		db,
		tenantID,
		models.DomainTypeEnterprise,
		models.SubjectTypeTenantMember,
		memberID,
	)
	if err != nil {
		return nil, err
	}
	for _, override := range overrides {
		if override.Effect == "deny" {
			delete(permissionSet, override.PermissionCode)
			continue
		}
		permissionSet[override.PermissionCode] = true
	}
	permissions := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		permissions = append(permissions, code)
	}
	sort.Strings(permissions)
	return permissions, nil
}

func (s *authService) loadTenantLegacyPermissionCodes(db *gorm.DB, tenantID, userID int64) ([]string, error) {
	permissionRows := make([]struct {
		Code string
	}, 0)
	err := db.Table("t_permission AS p").
		Select("DISTINCT p.code").
		Joins("JOIN t_role_permission AS rp ON rp.permission_id = p.id").
		Joins("JOIN t_user_role AS ur ON ur.role_id = rp.role_id").
		Joins("JOIN t_role AS r ON r.id = ur.role_id").
		Where("ur.user_id = ?", userID).
		Where("(ur.tenant_id = ? OR (ur.tenant_id = 0 AND r.tenant_id = ?))", tenantID, tenantID).
		Where("ur.domain_type IN ? AND r.domain_type IN ? AND rp.domain_type IN ?", []string{"", models.DomainTypeEnterprise}, []string{"", models.DomainTypeEnterprise}, []string{"", models.DomainTypeEnterprise}).
		Where("rp.tenant_id IN ?", []int64{0, tenantID}).
		Where("r.status = ? AND p.status = ?", enums.StatusOk, enums.StatusOk).
		Order("p.code ASC").
		Scan(&permissionRows).Error
	if err != nil {
		return nil, err
	}
	permissionSet := make(map[string]bool, len(permissionRows))
	for _, row := range permissionRows {
		permissionSet[row.Code] = true
	}
	if db.Migrator().HasTable(&models.UserPermission{}) {
		overrideRows := make([]struct {
			Code   string
			Effect int
		}, 0)
		err = db.Table("t_user_permission AS up").
			Select("p.code, up.effect").
			Joins("JOIN t_permission AS p ON p.id = up.permission_id").
			Where("up.user_id = ? AND up.tenant_id = ?", userID, tenantID).
			Where("up.domain_type IN ?", []string{"", models.DomainTypeEnterprise}).
			Where("up.expired_at IS NULL OR up.expired_at > ?", time.Now()).
			Scan(&overrideRows).Error
		if err != nil {
			return nil, err
		}
		for _, override := range overrideRows {
			if override.Effect < 0 {
				delete(permissionSet, override.Code)
				continue
			}
			permissionSet[override.Code] = true
		}
	}
	permissions := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		permissions = append(permissions, code)
	}
	sort.Strings(permissions)
	return permissions, nil
}

func (s *authService) issueTokens(ctx *sqls.TxContext, user *models.User, clientIP, userAgent string, authCfg config.AuthConfig, preferredDomain string) (*response.LoginResponse, error) {
	roles, permissions, err := s.loadUserAuthScope(ctx.Tx, user.ID)
	if err != nil {
		return nil, err
	}

	tokenTTL := s.resolveTokenTTL(authCfg)
	accessToken, err := randomToken(constants.AuthTokenPrefix)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	principal := &dto.AuthPrincipal{UserID: user.ID, Roles: roles, Permissions: permissions}
	s.applyIdentityContext(ctx.Tx, principal, preferredDomain)
	s.mergeDomainAuthScope(ctx.Tx, principal)

	if err := repositories.LoginSessionRepository.Create(ctx.Tx, &models.LoginSession{
		UserID:      user.ID,
		Token:       accessToken,
		ClientType:  constants.ClientTypeAdminWeb,
		DomainType:  principal.DomainType,
		TenantID:    principal.TenantID,
		SubjectType: principal.SubjectType,
		SubjectID:   principal.SubjectID,
		ClientIP:    clientIP,
		UserAgent:   userAgent,
		ExpiredAt:   now.Add(tokenTTL),
		LastSeenAt:  &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   user.ID,
			CreateUserName: user.Username,
			UpdatedAt:      now,
			UpdateUserID:   user.ID,
			UpdateUserName: user.Username,
		},
	}); err != nil {
		return nil, err
	}

	return buildLoginResponse(accessToken, now.Add(tokenTTL).Format(time.DateTime), user, principal), nil
}

func (s *authService) IssuePortalInvitationSession(domainType string, userID, tenantID, subjectID int64, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	domainType = strings.TrimSpace(domainType)
	if userID <= 0 || tenantID <= 0 || subjectID <= 0 {
		return nil, errorsx.InvalidParam("portal invitation session context is invalid")
	}
	var ret *response.LoginResponse
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		user := repositories.UserRepository.Get(ctx.Tx, userID)
		if user == nil || user.Status != enums.StatusOk {
			return errorsx.InvalidAccountI18n("error.e0260")
		}
		issued, err := s.issuePortalInvitationTokensDB(ctx.Tx, user, domainType, tenantID, subjectID, clientIP, userAgent, authCfg)
		if err != nil {
			return err
		}
		s.enrichLoginResponse(ctx.Tx, issued, user.ID, domainType, true)
		if err := repositories.UserRepository.Updates(ctx.Tx, user.ID, map[string]any{
			"last_login_at":    time.Now(),
			"last_login_ip":    clientIP,
			"update_user_id":   user.ID,
			"update_user_name": user.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return err
		}
		ret = issued
		return nil
	})
	return ret, err
}

func (s *authService) issuePortalInvitationTokensDB(db *gorm.DB, user *models.User, domainType string, tenantID, subjectID int64, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	roles, permissions, err := s.loadUserAuthScope(db, user.ID)
	if err != nil {
		return nil, err
	}
	principal := &dto.AuthPrincipal{
		UserID:      user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Status:      user.Status,
		Roles:       roles,
		Permissions: permissions,
		Locale:      userLocale(user),
		Timezone:    userTimezone(user),
	}
	switch domainType {
	case models.DomainTypeCustomer:
		customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(db, tenantID, subjectID)
		if customerUser == nil || customerUser.UserID != user.ID || customerUser.Status != enums.StatusOk {
			return nil, errorsx.InvalidAccountI18n("error.e0260")
		}
		principal.TenantID = tenantID
		principal.TargetTenantID = tenantID
		principal.DomainType = models.DomainTypeCustomer
		principal.Domain = models.DomainTypeCustomer
		principal.SubjectType = models.SubjectTypeCustomerUser
		principal.SubjectID = customerUser.ID
		principal.CustomerUserID = customerUser.ID
		principal.CustomerOrgID = customerUser.CustomerOrgID
	case models.DomainTypePartner:
		account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(db, tenantID, 0, subjectID)
		if account == nil || account.UserID != user.ID || account.Status != enums.StatusOk {
			return nil, errorsx.InvalidAccountI18n("error.e0260")
		}
		s.applyPartnerIdentityContext(db, principal, account)
	default:
		return nil, errorsx.InvalidParam("unsupported invitation portal")
	}
	if principal.TargetTenantID == 0 {
		principal.TargetTenantID = principal.TenantID
	}
	s.mergeDomainAuthScope(db, principal)

	tokenTTL := s.resolveTokenTTL(authCfg)
	accessToken, err := randomToken(constants.AuthTokenPrefix)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := repositories.LoginSessionRepository.Create(db, &models.LoginSession{
		UserID:      user.ID,
		Token:       accessToken,
		ClientType:  constants.ClientTypeAdminWeb,
		DomainType:  principal.DomainType,
		TenantID:    principal.TenantID,
		SubjectType: principal.SubjectType,
		SubjectID:   principal.SubjectID,
		ClientIP:    clientIP,
		UserAgent:   userAgent,
		ExpiredAt:   now.Add(tokenTTL),
		LastSeenAt:  &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   user.ID,
			CreateUserName: user.Username,
			UpdatedAt:      now,
			UpdateUserID:   user.ID,
			UpdateUserName: user.Username,
		},
	}); err != nil {
		return nil, err
	}
	return buildLoginResponse(accessToken, now.Add(tokenTTL).Format(time.DateTime), user, principal), nil
}

func (s *authService) enrichLoginResponse(db *gorm.DB, ret *response.LoginResponse, userID int64, requestedDomain string, portalChoiceConfirmed bool) {
	if ret == nil {
		return
	}
	ret.Membership = TenantCommercialService.DescribeMembership(db, ret.TenantID)
	options := s.buildLoginPortalOptions(db, userID)
	if len(options) > 0 {
		ret.AvailablePortals = options
	}
	hasEnterprise := false
	hasPartner := false
	for _, option := range options {
		if option.DomainType == models.DomainTypeEnterprise {
			hasEnterprise = true
		}
		if option.DomainType == models.DomainTypePartner {
			hasPartner = true
		}
	}
	ret.RequiresPortalChoice = !portalChoiceConfirmed &&
		requestedDomain == models.DomainTypeEnterprise &&
		hasEnterprise &&
		hasPartner
}

func (s *authService) buildLoginPortalOptions(db *gorm.DB, userID int64) []response.LoginPortalOptionResponse {
	options := make([]response.LoginPortalOptionResponse, 0, 3)
	if userID <= 0 || db == nil {
		return options
	}
	if s.userHasPlatformAccess(db, userID) {
		options = append(options, response.LoginPortalOptionResponse{
			DomainType:  models.DomainTypePlatform,
			Label:       "平台端",
			SubjectType: models.SubjectTypePlatformStaff,
		})
	}
	if member := repositories.EnterpriseIAMRepository.FindActiveTenantMemberByUserID(db, userID); member != nil {
		options = append(options, response.LoginPortalOptionResponse{
			DomainType:  models.DomainTypeEnterprise,
			Label:       "企业端",
			TenantID:    member.TenantID,
			SubjectType: models.SubjectTypeTenantMember,
			SubjectID:   member.ID,
		})
	}
	if account := repositories.TicketSupplierCollaborationRepository.FindActivePartnerAccountByUser(db, userID); account != nil {
		options = append(options, response.LoginPortalOptionResponse{
			DomainType:       models.DomainTypePartner,
			Label:            "供应商端",
			TenantID:         account.TenantID,
			SubjectType:      models.SubjectTypePartnerAccount,
			SubjectID:        account.ID,
			PartnerAccountID: account.ID,
			Default:          true,
		})
	}
	if len(options) == 0 {
		options = append(options, response.LoginPortalOptionResponse{
			DomainType:  models.DomainTypeEnterprise,
			Label:       "企业端",
			SubjectType: models.SubjectTypeTenantMember,
			SubjectID:   userID,
		})
	}
	return options
}

func (s *authService) userHasPlatformAccess(db *gorm.DB, userID int64) bool {
	if db == nil || userID <= 0 {
		return false
	}
	if db.Migrator().HasTable(&models.PlatformStaffProfile{}) {
		if profile := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, userID); profile != nil && profile.Status == enums.StatusOk {
			return true
		}
	}
	roles, _, err := s.loadUserAuthScope(db, userID)
	if err != nil {
		return false
	}
	for _, role := range roles {
		if role == PlatformRoleAdmin || role == "platform_staff" || role == constants.RoleCodeSuperAdmin || role == constants.RoleCodeAdmin {
			return true
		}
	}
	return false
}

func (s *authService) IssuePlatformTenantSessionDB(db *gorm.DB, operator *dto.AuthPrincipal, member *models.TenantMember, grant *models.PlatformTenantGrant, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 || member == nil || grant == nil || member.TenantID != grant.TenantID {
		return nil, errorsx.InvalidParam("invalid platform tenant session")
	}
	user := repositories.UserRepository.Get(db, operator.UserID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.UnauthorizedI18n("error.e0256")
	}
	token, err := randomToken(constants.AuthTokenPrefix)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(s.resolveTokenTTL(authCfg))
	if supportLimit := now.Add(2 * time.Hour); supportLimit.Before(expiresAt) {
		expiresAt = supportLimit
	}
	if grant.ExpiredAt.Before(expiresAt) {
		expiresAt = grant.ExpiredAt
	}
	session := &models.LoginSession{
		UserID:         user.ID,
		Token:          token,
		ClientType:     constants.ClientTypeAdminWeb,
		DomainType:     models.DomainTypeEnterprise,
		TenantID:       member.TenantID,
		SubjectType:    models.SubjectTypeTenantMember,
		SubjectID:      member.ID,
		SupportGrantID: grant.ID,
		SupportMode:    models.SupportModePlatformTenant,
		ImpersonatedBy: operator.Username,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		ExpiredAt:      expiresAt,
		LastSeenAt:     &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}
	if err := repositories.LoginSessionRepository.Create(db, session); err != nil {
		return nil, err
	}
	principal, err := s.buildPlatformTenantPrincipal(db, user, session)
	if err != nil {
		return nil, err
	}
	return buildLoginResponse(token, expiresAt.Format(time.DateTime), user, principal), nil
}

func (s *authService) buildPlatformTenantPrincipal(db *gorm.DB, user *models.User, session *models.LoginSession) (*dto.AuthPrincipal, error) {
	if user == nil || session == nil || session.TenantID <= 0 || session.SubjectID <= 0 || session.SupportGrantID <= 0 {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	grant := repositories.PlatformIAMRepository.GetActivePlatformTenantGrant(db, session.SupportGrantID, session.TenantID)
	staff := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, user.ID)
	member := repositories.EnterpriseIAMRepository.GetTenantMember(db, session.TenantID, session.SubjectID)
	if grant == nil || staff == nil || staff.Status != enums.StatusOk || grant.PlatformStaffID != staff.ID || member == nil {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	principal := &dto.AuthPrincipal{
		UserID:          user.ID,
		Username:        user.Username,
		Nickname:        user.Nickname,
		Avatar:          user.Avatar,
		Status:          user.Status,
		Roles:           make([]string, 0),
		Permissions:     make([]string, 0),
		TenantID:        session.TenantID,
		TargetTenantID:  session.TenantID,
		DomainType:      models.DomainTypeEnterprise,
		Domain:          models.DomainTypeEnterprise,
		SubjectType:     models.SubjectTypeTenantMember,
		SubjectID:       member.ID,
		MemberID:        member.ID,
		PlatformStaffID: staff.ID,
		SupportGrantID:  grant.ID,
		SupportMode:     models.SupportModePlatformTenant,
		ImpersonatedBy:  session.ImpersonatedBy,
		Locale:          userLocale(user),
		Timezone:        userTimezone(user),
	}
	s.mergeDomainAuthScope(db, principal)
	s.mergeTenantOwnerScope(db, principal)
	return principal, nil
}

func (s *authService) IssueEnterpriseCustomerSessionDB(db *gorm.DB, operator *dto.AuthPrincipal, customerUser *models.CustomerUser, grant *models.TemporaryAccessGrant, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 || customerUser == nil || grant == nil || customerUser.TenantID != grant.TenantID {
		return nil, errorsx.InvalidParam("invalid customer support session")
	}
	if grant.ResourceType != models.SubjectTypeCustomerUser || strings.TrimSpace(grant.ResourceID) != strconv.FormatInt(customerUser.ID, 10) {
		return nil, errorsx.InvalidParam("invalid customer support grant")
	}
	user := repositories.UserRepository.Get(db, customerUser.UserID)
	if user == nil || user.Status != enums.StatusOk || customerUser.Status != enums.StatusOk {
		return nil, errorsx.UnauthorizedI18n("error.e0256")
	}
	token, err := randomToken(constants.AuthTokenPrefix)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(s.resolveTokenTTL(authCfg))
	if supportLimit := now.Add(2 * time.Hour); supportLimit.Before(expiresAt) {
		expiresAt = supportLimit
	}
	if grant.ExpiredAt.Before(expiresAt) {
		expiresAt = grant.ExpiredAt
	}
	session := &models.LoginSession{
		UserID:         user.ID,
		Token:          token,
		ClientType:     constants.ClientTypeAdminWeb,
		DomainType:     models.DomainTypeCustomer,
		TenantID:       customerUser.TenantID,
		SubjectType:    models.SubjectTypeCustomerUser,
		SubjectID:      customerUser.ID,
		SupportGrantID: grant.ID,
		SupportMode:    models.SupportModeCustomerPortal,
		ImpersonatedBy: operator.Username,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		ExpiredAt:      expiresAt,
		LastSeenAt:     &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}
	if err := repositories.LoginSessionRepository.Create(db, session); err != nil {
		return nil, err
	}
	principal, err := s.buildEnterpriseCustomerSupportPrincipal(db, user, session)
	if err != nil {
		return nil, err
	}
	return buildLoginResponse(token, expiresAt.Format(time.DateTime), user, principal), nil
}

func (s *authService) buildEnterpriseCustomerSupportPrincipal(db *gorm.DB, user *models.User, session *models.LoginSession) (*dto.AuthPrincipal, error) {
	if user == nil || session == nil || session.TenantID <= 0 || session.SubjectID <= 0 || session.SupportGrantID <= 0 {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	grant := repositories.PlatformIAMRepository.GetActiveTemporaryAccessGrant(db, session.SupportGrantID, session.TenantID, models.SubjectTypeCustomerUser, strconv.FormatInt(session.SubjectID, 10))
	customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(db, session.TenantID, session.SubjectID)
	if grant == nil || customerUser == nil || customerUser.UserID != user.ID || customerUser.Status != enums.StatusOk {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	roles, permissions, err := s.loadUserAuthScope(db, user.ID)
	if err != nil {
		return nil, err
	}
	principal := &dto.AuthPrincipal{
		UserID:         user.ID,
		Username:       user.Username,
		Nickname:       user.Nickname,
		Avatar:         user.Avatar,
		Status:         user.Status,
		Roles:          roles,
		Permissions:    permissions,
		TenantID:       session.TenantID,
		TargetTenantID: session.TenantID,
		DomainType:     models.DomainTypeCustomer,
		Domain:         models.DomainTypeCustomer,
		SubjectType:    models.SubjectTypeCustomerUser,
		SubjectID:      customerUser.ID,
		CustomerUserID: customerUser.ID,
		CustomerOrgID:  customerUser.CustomerOrgID,
		SupportGrantID: grant.ID,
		SupportMode:    models.SupportModeCustomerPortal,
		ImpersonatedBy: session.ImpersonatedBy,
		Locale:         userLocale(user),
		Timezone:       userTimezone(user),
	}
	s.mergeDomainAuthScope(db, principal)
	return principal, nil
}

func (s *authService) IssueEnterpriseMemberSessionDB(db *gorm.DB, operator *dto.AuthPrincipal, member *models.TenantMember, grant *models.TemporaryAccessGrant, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 || member == nil || grant == nil || member.TenantID != grant.TenantID {
		return nil, errorsx.InvalidParam("invalid employee support session")
	}
	if grant.ResourceType != models.SubjectTypeTenantMember || strings.TrimSpace(grant.ResourceID) != strconv.FormatInt(member.ID, 10) {
		return nil, errorsx.InvalidParam("invalid employee support grant")
	}
	user := repositories.UserRepository.Get(db, member.UserID)
	if user == nil || user.Status != enums.StatusOk || member.Status != enums.StatusOk {
		return nil, errorsx.UnauthorizedI18n("error.e0256")
	}
	token, err := randomToken(constants.AuthTokenPrefix)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(s.resolveTokenTTL(authCfg))
	if supportLimit := now.Add(2 * time.Hour); supportLimit.Before(expiresAt) {
		expiresAt = supportLimit
	}
	if grant.ExpiredAt.Before(expiresAt) {
		expiresAt = grant.ExpiredAt
	}
	session := &models.LoginSession{
		UserID:         user.ID,
		Token:          token,
		ClientType:     constants.ClientTypeAdminWeb,
		DomainType:     models.DomainTypeEnterprise,
		TenantID:       member.TenantID,
		SubjectType:    models.SubjectTypeTenantMember,
		SubjectID:      member.ID,
		SupportGrantID: grant.ID,
		SupportMode:    models.SupportModeEmployeePortal,
		ImpersonatedBy: operator.Username,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		ExpiredAt:      expiresAt,
		LastSeenAt:     &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}
	if err := repositories.LoginSessionRepository.Create(db, session); err != nil {
		return nil, err
	}
	principal, err := s.buildEnterpriseMemberSupportPrincipal(db, user, session)
	if err != nil {
		return nil, err
	}
	return buildLoginResponse(token, expiresAt.Format(time.DateTime), user, principal), nil
}

func (s *authService) buildEnterpriseMemberSupportPrincipal(db *gorm.DB, user *models.User, session *models.LoginSession) (*dto.AuthPrincipal, error) {
	if user == nil || session == nil || session.TenantID <= 0 || session.SubjectID <= 0 || session.SupportGrantID <= 0 {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	grant := repositories.PlatformIAMRepository.GetActiveTemporaryAccessGrant(db, session.SupportGrantID, session.TenantID, models.SubjectTypeTenantMember, strconv.FormatInt(session.SubjectID, 10))
	member := repositories.EnterpriseIAMRepository.GetTenantMember(db, session.TenantID, session.SubjectID)
	if grant == nil || member == nil || member.UserID != user.ID || member.Status != enums.StatusOk {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	roles, permissions, err := s.loadTenantMemberAuthScope(db, session.TenantID, member.ID, user.ID)
	if err != nil {
		return nil, err
	}
	principal := &dto.AuthPrincipal{
		UserID:         user.ID,
		Username:       user.Username,
		Nickname:       user.Nickname,
		Avatar:         user.Avatar,
		Status:         user.Status,
		Roles:          roles,
		Permissions:    permissions,
		TenantID:       session.TenantID,
		TargetTenantID: session.TenantID,
		DomainType:     models.DomainTypeEnterprise,
		Domain:         models.DomainTypeEnterprise,
		SubjectType:    models.SubjectTypeTenantMember,
		SubjectID:      member.ID,
		MemberID:       member.ID,
		SupportGrantID: grant.ID,
		SupportMode:    models.SupportModeEmployeePortal,
		ImpersonatedBy: session.ImpersonatedBy,
		Locale:         userLocale(user),
		Timezone:       userTimezone(user),
	}
	return principal, nil
}

func (s *authService) loadTenantMemberAuthScope(db *gorm.DB, tenantID, memberID, userID int64) ([]string, []string, error) {
	roleSet := make(map[string]bool)
	if db == nil || tenantID <= 0 || memberID <= 0 || userID <= 0 {
		return []string{}, []string{}, nil
	}
	bindings := make([]models.AuthRoleBinding, 0)
	if db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		var err error
		bindings, err = repositories.PlatformIAMRepository.FindRoleBindings(db, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberID)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(bindings) == 0 && db.Migrator().HasTable(&models.Role{}) && db.Migrator().HasTable(&models.UserRole{}) {
		legacyRoles := make([]struct{ Code string }, 0)
		err := db.Table("t_role AS r").
			Select("DISTINCT r.code").
			Joins("JOIN t_user_role AS ur ON ur.role_id = r.id").
			Where("ur.user_id = ?", userID).
			Where("(ur.tenant_id = ? OR (ur.tenant_id = 0 AND r.tenant_id = ?))", tenantID, tenantID).
			Where("ur.domain_type IN ? AND r.domain_type IN ?", []string{"", models.DomainTypeEnterprise}, []string{"", models.DomainTypeEnterprise}).
			Where("r.status = ?", enums.StatusOk).
			Order("r.code ASC").
			Scan(&legacyRoles).Error
		if err != nil {
			return nil, nil, err
		}
		for _, role := range legacyRoles {
			roleSet[role.Code] = true
		}
	}
	if len(bindings) > 0 {
		roleIDs := make([]int64, 0, len(bindings))
		for _, binding := range bindings {
			roleIDs = append(roleIDs, binding.RoleID)
		}
		roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, roleIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, binding := range bindings {
			if role := roles[binding.RoleID]; role != nil && role.Status == enums.StatusOk {
				roleSet[role.Code] = true
			}
		}
	}
	permissions, err := s.GetTenantMemberPermissions(db, tenantID, memberID, userID)
	if err != nil {
		return nil, nil, err
	}
	roles := make([]string, 0, len(roleSet))
	for code := range roleSet {
		roles = append(roles, code)
	}
	sort.Strings(roles)
	return roles, permissions, nil
}

func (s *authService) isEnterpriseMemberSupportSession(db *gorm.DB, user *models.User, session *models.LoginSession) bool {
	if user == nil || session == nil {
		return false
	}
	member := repositories.EnterpriseIAMRepository.GetTenantMember(db, session.TenantID, session.SubjectID)
	if member == nil || member.UserID != user.ID {
		return false
	}
	return repositories.PlatformIAMRepository.GetActiveTemporaryAccessGrant(
		db,
		session.SupportGrantID,
		session.TenantID,
		models.SubjectTypeTenantMember,
		strconv.FormatInt(session.SubjectID, 10),
	) != nil
}

func (s *authService) mergeTenantOwnerScope(db *gorm.DB, principal *dto.AuthPrincipal) {
	if principal == nil || principal.TenantID <= 0 {
		return
	}
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, principal.TenantID, models.DomainTypeEnterprise, EnterpriseRoleOwner)
	if role == nil || role.Status != enums.StatusOk {
		return
	}
	if !slices.Contains(principal.Roles, role.Code) {
		principal.Roles = append(principal.Roles, role.Code)
	}
	rows, err := repositories.PlatformIAMRepository.FindAuthRolePermissionRows(db, principal.TenantID, []int64{role.ID})
	if err != nil {
		return
	}
	permissionSet := make(map[string]bool, len(principal.Permissions)+len(rows))
	for _, code := range principal.Permissions {
		permissionSet[code] = true
	}
	for _, row := range rows {
		if row.Effect == "deny" {
			delete(permissionSet, row.PermissionCode)
			continue
		}
		permissionSet[row.PermissionCode] = true
	}
	principal.Permissions = principal.Permissions[:0]
	for code := range permissionSet {
		principal.Permissions = append(principal.Permissions, code)
	}
	sort.Strings(principal.Roles)
	sort.Strings(principal.Permissions)
}

func (s *authService) applyIdentityContext(db *gorm.DB, principal *dto.AuthPrincipal, preferredDomain string) {
	if principal == nil {
		return
	}
	preferredDomain = strings.TrimSpace(preferredDomain)
	if preferredDomain == models.DomainTypePlatform && s.applyPlatformIdentityContext(db, principal) {
		return
	}
	if preferredDomain == models.DomainTypeEnterprise && s.applyEnterpriseIdentityContext(db, principal) {
		return
	}
	if preferredDomain == models.DomainTypePartner {
		if account := repositories.TicketSupplierCollaborationRepository.FindActivePartnerAccountByUser(db, principal.UserID); account != nil {
			s.applyPartnerIdentityContext(db, principal, account)
		}
		return
	}
	if preferredDomain == models.DomainTypeCustomer && s.applyCustomerIdentityContext(db, principal) {
		return
	}
	if preferredDomain == models.DomainTypeCustomer {
		// A signed-in account may enter the customer domain before its first
		// device binding. Customer APIs still require a formal customer_user;
		// only the binding endpoint accepts this pending identity.
		principal.DomainType = models.DomainTypeCustomer
		principal.Domain = models.DomainTypeCustomer
		principal.SubjectType = models.SubjectTypePendingCustomer
		principal.SubjectID = principal.UserID
		return
	}
	for _, role := range principal.Roles {
		if role == "platform_admin" || role == "platform_staff" || role == "super_admin" || role == constants.RoleCodeAdmin {
			s.applyPlatformIdentityContext(db, principal)
			return
		}
	}
	if s.applyPlatformIdentityContext(db, principal) {
		return
	}
	if account := repositories.TicketSupplierCollaborationRepository.FindActivePartnerAccountByUser(db, principal.UserID); account != nil {
		s.applyPartnerIdentityContext(db, principal, account)
		return
	}
	if s.applyEnterpriseIdentityContext(db, principal) {
		return
	}
	if s.applyCustomerIdentityContext(db, principal) {
		return
	}
	principal.DomainType = models.DomainTypeEnterprise
	principal.Domain = models.DomainTypeEnterprise
	principal.SubjectType = models.SubjectTypeTenantMember
	principal.SubjectID = principal.UserID
}

func (s *authService) applyPlatformIdentityContext(db *gorm.DB, principal *dto.AuthPrincipal) bool {
	if principal == nil {
		return false
	}
	var profile *models.PlatformStaffProfile
	if db.Migrator().HasTable(&models.PlatformStaffProfile{}) {
		profile = repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, principal.UserID)
	}
	if profile != nil && profile.Status != enums.StatusOk {
		profile = nil
	}
	legacyPlatformRole := false
	for _, role := range principal.Roles {
		if role == PlatformRoleAdmin || role == "platform_staff" || role == constants.RoleCodeSuperAdmin || role == constants.RoleCodeAdmin {
			legacyPlatformRole = true
			break
		}
	}
	if profile == nil && !legacyPlatformRole {
		return false
	}
	principal.TenantID = 0
	principal.DomainType = models.DomainTypePlatform
	principal.Domain = models.DomainTypePlatform
	principal.SubjectType = models.SubjectTypePlatformStaff
	principal.SubjectID = principal.UserID
	if profile != nil {
		principal.SubjectID = profile.ID
	}
	return true
}

func (s *authService) applyEnterpriseIdentityContext(db *gorm.DB, principal *dto.AuthPrincipal) bool {
	if principal == nil || !db.Migrator().HasTable(&models.TenantMember{}) {
		return false
	}
	member := repositories.EnterpriseIAMRepository.FindActiveTenantMemberByUserID(db, principal.UserID)
	if member == nil {
		return false
	}
	principal.TenantID = member.TenantID
	principal.DomainType = models.DomainTypeEnterprise
	principal.Domain = models.DomainTypeEnterprise
	principal.SubjectType = models.SubjectTypeTenantMember
	principal.SubjectID = member.ID
	return true
}

func (s *authService) applyCustomerIdentityContext(db *gorm.DB, principal *dto.AuthPrincipal) bool {
	if principal == nil || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return false
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByUserID(db, principal.UserID)
	if customerUser == nil {
		return false
	}
	principal.TenantID = customerUser.TenantID
	principal.DomainType = models.DomainTypeCustomer
	principal.Domain = models.DomainTypeCustomer
	principal.SubjectType = models.SubjectTypeCustomerUser
	principal.SubjectID = customerUser.ID
	return true
}

func (s *authService) mergeDomainAuthScope(db *gorm.DB, principal *dto.AuthPrincipal) {
	if db == nil || principal == nil || principal.SubjectID <= 0 {
		return
	}
	if principal.DomainType == models.DomainTypeEnterprise && principal.SubjectType == models.SubjectTypeTenantMember {
		roles, permissions, err := s.loadTenantMemberAuthScope(db, principal.TenantID, principal.SubjectID, principal.UserID)
		if err != nil {
			slog.Error("failed to resolve authoritative enterprise member permissions", "tenantId", principal.TenantID, "memberId", principal.SubjectID, "userId", principal.UserID, "error", err)
			principal.Roles = []string{}
			principal.Permissions = []string{}
			return
		}
		principal.Roles = roles
		principal.Permissions = permissions
		return
	}
	if !db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		return
	}
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, principal.TenantID, principal.DomainType, principal.SubjectType, principal.SubjectID)
	if err != nil || len(bindings) == 0 {
		return
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
	}
	roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, roleIDs)
	if err != nil {
		return
	}
	for _, binding := range bindings {
		role := roles[binding.RoleID]
		if role != nil && role.Status == enums.StatusOk && !slices.Contains(principal.Roles, role.Code) {
			principal.Roles = append(principal.Roles, role.Code)
		}
	}
	permissionRows, err := repositories.PlatformIAMRepository.FindAuthRolePermissionRows(db, principal.TenantID, roleIDs)
	if err != nil {
		return
	}
	permissionSet := make(map[string]bool, len(principal.Permissions)+len(permissionRows))
	for _, code := range principal.Permissions {
		permissionSet[code] = true
	}
	for _, row := range permissionRows {
		if row.Effect == "deny" {
			delete(permissionSet, row.PermissionCode)
			continue
		}
		permissionSet[row.PermissionCode] = true
	}
	principal.Permissions = principal.Permissions[:0]
	for code := range permissionSet {
		principal.Permissions = append(principal.Permissions, code)
	}
	sort.Strings(principal.Roles)
	sort.Strings(principal.Permissions)
}

func (s *authService) applyPartnerIdentityContext(db *gorm.DB, principal *dto.AuthPrincipal, account *models.PartnerAccount) {
	principal.TenantID = account.TenantID
	principal.DomainType = models.DomainTypePartner
	principal.Domain = models.DomainTypePartner
	principal.SubjectType = models.SubjectTypePartnerAccount
	principal.SubjectID = account.ID
	principal.PartnerAccountID = account.ID
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, account.TenantID, models.DomainTypePartner, models.SubjectTypePartnerAccount, account.ID)
	if err != nil || len(bindings) == 0 {
		s.mergePartnerFallbackRoleScope(db, principal, account)
		return
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
	}
	roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, roleIDs)
	if err != nil {
		return
	}
	for _, binding := range bindings {
		if role := roles[binding.RoleID]; role != nil && !slices.Contains(principal.Roles, role.Code) {
			principal.Roles = append(principal.Roles, role.Code)
		}
	}
}

func (s *authService) mergePartnerFallbackRoleScope(db *gorm.DB, principal *dto.AuthPrincipal, account *models.PartnerAccount) {
	if principal == nil || account == nil {
		return
	}
	roles, err := TicketSupplierCollaborationService.partnerRoleCodes(db, account)
	if err != nil || len(roles) == 0 {
		return
	}
	roleSet := make(map[string]bool, len(principal.Roles)+len(roles))
	for _, role := range principal.Roles {
		roleSet[role] = true
	}
	permissionSet := make(map[string]bool, len(principal.Permissions))
	for _, permission := range principal.Permissions {
		permissionSet[permission] = true
	}
	for _, role := range roles {
		if role == "" {
			continue
		}
		roleSet[role] = true
		for _, permission := range tenantDefaultRolePermissions(role) {
			permissionSet[permission] = true
		}
	}
	principal.Roles = principal.Roles[:0]
	for role := range roleSet {
		principal.Roles = append(principal.Roles, role)
	}
	principal.Permissions = principal.Permissions[:0]
	for permission := range permissionSet {
		principal.Permissions = append(principal.Permissions, permission)
	}
	sort.Strings(principal.Roles)
	sort.Strings(principal.Permissions)
}

func (s *authService) resolveTokenTTL(authCfg config.AuthConfig) time.Duration {
	tokenTTL := defaultAuthTokenTTL
	if authCfg.TokenTTLHours > 0 {
		tokenTTL = time.Duration(authCfg.TokenTTLHours) * time.Hour
	}
	return tokenTTL
}

func (s *authService) validateSessionToken(token string) (*models.LoginSession, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	session := LoginSessionService.FindOne(sqls.NewCnd().Eq("token", token))
	if session == nil {
		return nil, errorsx.InvalidTokenI18n("error.e0269")
	}
	if session.RevokedAt != nil {
		return nil, errorsx.InvalidTokenI18n("error.e0267")
	}
	if time.Now().After(session.ExpiredAt) {
		return nil, errorsx.InvalidTokenI18n("error.e0268")
	}
	return session, nil
}

func (s *authService) loadUserAuthScope(tx *gorm.DB, userID int64) ([]string, []string, error) {
	roleCodes, err := s.loadUserRoleCodes(tx, userID)
	if err != nil {
		return nil, nil, err
	}
	permissionCodes, err := s.loadUserPermissionCodes(tx, userID)
	if err != nil {
		return nil, nil, err
	}
	return roleCodes, permissionCodes, nil
}

func (s *authService) loadUserRoleCodes(tx *gorm.DB, userID int64) ([]string, error) {
	roles, err := s.loadUserRoles(tx, userID)
	if err != nil {
		return nil, err
	}
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleCodes = append(roleCodes, role.Code)
	}
	return roleCodes, nil
}

func (s *authService) loadUserRoles(tx *gorm.DB, userID int64) ([]models.Role, error) {
	roles := make([]models.Role, 0)
	if err := tx.
		Table("t_role AS r").
		Select("r.*").
		Joins("JOIN t_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.status = ?", userID, enums.StatusOk).
		Order("r.sort_no ASC, r.id ASC").
		Scan(&roles).Error; err != nil {
		return nil, err
	}

	return roles, nil
}

func (s *authService) loadUserPermissionCodes(tx *gorm.DB, userID int64) ([]string, error) {
	permissionRows := make([]struct {
		Code   string
		SortNo int
		ID     int64
	}, 0)
	db := tx.Table("t_permission AS p").
		Select("DISTINCT p.code, p.sort_no, p.id").
		Joins("JOIN t_role_permission AS rp ON rp.permission_id = p.id").
		Joins("JOIN t_user_role AS ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Where("p.status = ?", enums.StatusOk)
	if err := db.Order("p.sort_no ASC, p.id ASC").Scan(&permissionRows).Error; err != nil {
		return nil, err
	}

	permissionCodes := make([]string, 0, len(permissionRows))
	for _, permission := range permissionRows {
		permissionCodes = append(permissionCodes, permission.Code)
	}

	overrideRows := make([]struct {
		Code   string
		Effect int
	}, 0)
	if err := tx.
		Table("t_user_permission AS up").
		Select("p.code, up.effect").
		Joins("JOIN t_permission AS p ON p.id = up.permission_id").
		Where("up.user_id = ? AND (up.expired_at IS NULL OR up.expired_at > ?)", userID, time.Now()).
		Scan(&overrideRows).Error; err != nil {
		return nil, err
	}

	permissionSet := make(map[string]bool, len(permissionCodes))
	for _, code := range permissionCodes {
		permissionSet[code] = true
	}
	for _, override := range overrideRows {
		if override.Effect < 0 {
			delete(permissionSet, override.Code)
			continue
		}
		permissionSet[override.Code] = true
	}

	permissionCodes = permissionCodes[:0]
	for code := range permissionSet {
		permissionCodes = append(permissionCodes, code)
	}
	sort.Strings(permissionCodes)
	return permissionCodes, nil
}

func (s *authService) extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (s *authService) createLoginCredentialLog(principal string, userID int64, success bool, clientIP, userAgent, reason string) error {
	return LoginCredentialLogService.Create(&models.LoginCredentialLog{
		Principal: principal,
		UserID:    userID,
		Success:   success,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		Reason:    reason,
		CreatedAt: time.Now(),
	})
}

func (s *authService) isCredentialLocked(principal string, authCfg config.AuthConfig) bool {
	maxFailedAttempts := authCfg.MaxFailedAttempts
	if maxFailedAttempts <= 0 {
		return false
	}
	lockMinute := authCfg.CredentialLockMinute
	if lockMinute <= 0 {
		lockMinute = 15
	}
	since := time.Now().Add(-time.Duration(lockMinute) * time.Minute)
	return LoginCredentialLogService.Count(sqls.NewCnd().
		Eq("principal", normalizeLoginPrincipal(principal)).
		Eq("success", false).
		NotEq("reason", "credential locked").
		Where("created_at >= ?", since)) >= int64(maxFailedAttempts)
}

func normalizeLoginPrincipal(principal string) string {
	return strings.ToLower(strings.TrimSpace(principal))
}

func buildLoginResponse(accessToken, expiresAt string, user *models.User, principal *dto.AuthPrincipal) *response.LoginResponse {
	ret := &response.LoginResponse{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
		User:        buildAuthUserResponse(user, principal),
	}
	if principal != nil {
		ret.Permissions = principal.Permissions
		ret.Roles = principal.Roles
		ret.TenantID = principal.TenantID
		ret.DomainType = principal.DomainType
		ret.SubjectType = principal.SubjectType
		ret.SubjectID = principal.SubjectID
		ret.PartnerAccountID = principal.PartnerAccountID
		ret.SupportGrantID = principal.SupportGrantID
		ret.SupportMode = principal.SupportMode
		ret.ImpersonatedBy = principal.ImpersonatedBy
		if principal.FeatureFlags != nil {
			ret.FeatureFlags = principal.FeatureFlags
		} else {
			ret.FeatureFlags = TenantCapabilityService.FeatureFlags(principal.TenantID)
		}
		ret.Membership = TenantCommercialService.DescribeMembership(sqls.DB(), principal.TenantID)
		if principal.TenantID > 0 {
			if tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), principal.TenantID); tenant != nil {
				ret.TenantDefaultLocale = strings.TrimSpace(tenant.DefaultLocale)
				ret.CustomerDefaultLocale = ret.TenantDefaultLocale
			}
			branding, serverConsole := TenantPortalSettingsService.GetDB(sqls.DB(), principal.TenantID)
			if branding != nil && branding.Status == enums.StatusOk {
				if defaultLocale := strings.TrimSpace(branding.DefaultLocale); defaultLocale != "" {
					ret.CustomerDefaultLocale = defaultLocale
				}
				ret.TenantBranding = &response.TenantBrandingResponse{
					BrandName:     branding.BrandName,
					LogoURL:       branding.LogoURL,
					CustomDomain:  branding.CustomDomain,
					CustomerTheme: branding.EffectiveCustomerTheme(),
				}
			}
			if serverConsoleURL := TenantExternalPortalURL(serverConsole); principal.EffectiveDomainType() == models.DomainTypeEnterprise && serverConsole != nil && serverConsole.Status == enums.StatusOk && serverConsole.Enabled && serverConsoleURL != "" {
				metadata := TenantExternalPortalMetadata(serverConsole)
				ret.ExternalPortals = []response.TenantExternalPortalResponse{{
					Provider:  models.TenantIntegrationProviderOnePanel,
					Name:      metadata.DisplayName,
					URL:       serverConsoleURL,
					EmbedMode: metadata.EmbedMode,
				}}
			}
		}
	}
	ret.Locale = userLocale(user)
	ret.Timezone = userTimezone(user)
	return ret
}

func buildAuthUserResponse(user *models.User, principal *dto.AuthPrincipal) *response.AuthUserResponse {
	ret := &response.AuthUserResponse{
		Email:    userEmail(user),
		Mobile:   userMobile(user),
		Locale:   userLocale(user),
		Timezone: userTimezone(user),
	}
	if user != nil {
		ret.ID = user.ID
		ret.Username = user.Username
		ret.Nickname = user.Nickname
		ret.Avatar = user.Avatar
		ret.Status = user.Status
	}
	if principal != nil {
		if ret.ID == 0 {
			ret.ID = principal.UserID
			ret.Username = principal.Username
			ret.Nickname = principal.Nickname
			ret.Avatar = principal.Avatar
			ret.Status = principal.Status
		}
		ret.Roles = principal.Roles
	}
	return ret
}

func userEmail(user *models.User) string {
	if user == nil || user.Email == nil {
		return ""
	}
	return *user.Email
}

func userMobile(user *models.User) string {
	if user == nil || user.Mobile == nil {
		return ""
	}
	return *user.Mobile
}

func userLocale(user *models.User) string {
	if user == nil {
		return i18nx.DefaultLocale
	}
	return i18nx.NormalizeLocale(user.Locale)
}

func userTimezone(user *models.User) string {
	if user == nil || strings.TrimSpace(user.Timezone) == "" {
		return "Asia/Shanghai"
	}
	return strings.TrimSpace(user.Timezone)
}

func randomToken(prefix string) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
