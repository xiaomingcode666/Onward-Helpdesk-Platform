package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var PlatformIAMService = &platformIAMService{}

type platformIAMService struct{}

var provisionNewTenantDefaultAIKey = func(tenantID int64, operator *dto.AuthPrincipal) error {
	return TenantDefaultAIAgentService.EnsureDefaultCredential(tenantID, operator)
}

type PlatformTenantListAggregate struct {
	Items          []models.Tenant
	Paging         *sqls.Paging
	Summary        *repositories.PlatformTenantSummaryRow
	Subscriptions  map[int64]*models.TenantSubscription
	Plans          map[int64]*models.TenantPlan
	DeviceCounts   map[int64]int64
	MemberCounts   map[int64]int64
	MonthlyUsage   map[int64]repositories.PlatformAIUsageRow
	Brandings      map[int64]*models.TenantBranding
	ServerConsoles map[int64]*models.TenantIntegrationConfig
	Administrators map[int64]*models.TenantMember
	AdminUsers     map[int64]*models.User
}

type PlatformStaffListAggregate struct {
	Items []models.PlatformStaffProfile
	Users map[int64]*models.User
}

func (s *platformIAMService) ListTenants(search, status, lifecycle, billing string, page, limit int) (*PlatformTenantListAggregate, error) {
	items, paging, err := repositories.PlatformIAMRepository.FindTenantPage(sqls.DB(), repositories.PlatformTenantFilter{Search: search, Status: status, Lifecycle: lifecycle, Billing: billing}, page, limit)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	summary, err := repositories.PlatformConsoleRepository.GetTenantSummary(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	tenantIDs := make([]int64, 0, len(items))
	for _, item := range items {
		tenantIDs = append(tenantIDs, item.ID)
	}
	subscriptions, err := repositories.PlatformIAMRepository.FindActiveSubscriptions(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	planIDs := make([]int64, 0, len(subscriptions))
	for _, item := range subscriptions {
		planIDs = append(planIDs, item.PlanID)
	}
	plans, err := repositories.PlatformIAMRepository.FindPlansByIDs(sqls.DB(), uniqueInt64s(planIDs))
	if err != nil {
		return nil, err
	}
	deviceCounts, err := repositories.PlatformIAMRepository.CountDevicesByTenantIDs(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	memberCounts, err := repositories.PlatformIAMRepository.CountMembersByTenantIDs(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	administrators, err := repositories.EnterpriseIAMRepository.FindTenantAdministrators(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	adminUserIDs := make([]int64, 0, len(administrators))
	for _, administrator := range administrators {
		adminUserIDs = append(adminUserIDs, administrator.UserID)
	}
	adminUsers := make(map[int64]*models.User, len(adminUserIDs))
	for _, user := range repositories.UserRepository.FindByIds(sqls.DB(), uniqueInt64s(adminUserIDs)) {
		item := user
		adminUsers[item.ID] = &item
	}
	brandings, serverConsoles, err := TenantPortalSettingsService.FindByTenantIDsDB(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthlyUsage, err := repositories.PlatformConsoleRepository.GetAIUsageByTenantIDs(sqls.DB(), tenantIDs, startOfMonth, now.Add(time.Second))
	if err != nil {
		return nil, err
	}
	return &PlatformTenantListAggregate{
		Items: items, Paging: paging, Summary: summary,
		Subscriptions: subscriptions, Plans: plans,
		DeviceCounts: deviceCounts, MemberCounts: memberCounts,
		MonthlyUsage: monthlyUsage, Brandings: brandings, ServerConsoles: serverConsoles,
		Administrators: administrators, AdminUsers: adminUsers,
	}, nil
}

func (s *platformIAMService) CreateTenant(req request.PlatformTenantCreateRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("tenant name is required")
	}
	serviceScene, err := normalizeTenantServiceScene(req.ServiceScene)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var tenant *models.Tenant
	var defaultAgent *models.AIAgent
	aiEnabled := tenantAIEnabledOrDefault(req.AIEnabled)
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		status := enums.StatusOk
		if enums.IsValidStatus(req.Status) {
			status = enums.Status(req.Status)
		}
		trialEndsAt, err := parseOptionalTime(req.TrialEndsAt)
		if err != nil {
			return err
		}
		defaultLocale := defaultString(req.DefaultLocale, "en-US")
		customerDefaultLocale := defaultString(req.CustomerDefaultLocale, defaultLocale)
		supportedLocales := normalizeTenantPreferences(append([]string{customerDefaultLocale}, req.SupportedLocales...), defaultLocale)
		timezone := defaultString(req.Timezone, "UTC")
		supportedTimezones := normalizeTenantPreferences(req.SupportedTimezones, timezone)
		item := &models.Tenant{
			Name:                   name,
			ServiceScene:           serviceScene,
			Industry:               strings.TrimSpace(req.Industry),
			CountryRegion:          strings.TrimSpace(req.CountryRegion),
			DefaultLocale:          defaultLocale,
			SupportedLocalesJSON:   utils.MarshalStringListJSON(supportedLocales),
			Timezone:               timezone,
			SupportedTimezonesJSON: utils.MarshalStringListJSON(supportedTimezones),
			DataRegion:             strings.TrimSpace(req.DataRegion),
			Status:                 status,
			AIEnabled:              boolRef(aiEnabled),
			TrialEndsAt:            trialEndsAt,
			PurgeAfterDays:         30,
			AuditFields:            utils.BuildAuditFields(operator),
		}
		if err = repositories.PlatformIAMRepository.CreateTenant(ctx.Tx, item); err != nil {
			return err
		}
		tenant = item
		if _, err = TenantPortalSettingsService.SaveBrandingDB(ctx.Tx, item.ID, req.BrandName, req.LogoAssetID, req.LogoURL, req.CustomDomain, req.CustomerTheme, customerDefaultLocale, operator); err != nil {
			return err
		}
		if _, err = TenantPortalSettingsService.SaveOnePanelDB(ctx.Tx, item.ID, req.ServerConsoleName, req.ServerConsoleURL, req.ServerConsoleMode, req.ServerConsoleEnabled, operator); err != nil {
			return err
		}
		if err = EnsureTenantDefaultIAMRolesDB(ctx.Tx, item.ID, operator); err != nil {
			return err
		}
		if _, err = ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(ctx.Tx, item.ID, operator); err != nil {
			return err
		}
		if aiEnabled {
			if defaultAgent, err = TenantDefaultAIAgentService.EnsureDB(ctx.Tx, item.ID, operator); err != nil {
				return err
			}
		}
		if req.PlanID > 0 || strings.TrimSpace(req.PlanCode) != "" {
			plan := repositories.PlatformIAMRepository.GetPlan(ctx.Tx, req.PlanID)
			if plan == nil && strings.TrimSpace(req.PlanCode) != "" {
				plan = repositories.PlatformIAMRepository.GetPlanByCode(ctx.Tx, req.PlanCode)
			}
			if plan == nil {
				return errors.New("tenant plan not found")
			}
			if _, err = s.assignSubscriptionTx(ctx, item.ID, plan, "", "", "", operator); err != nil {
				return err
			}
		}
		adminUserID := req.AdminUserID
		if adminUserID <= 0 && strings.TrimSpace(req.AdminUsername) != "" {
			if strings.TrimSpace(req.AdminPassword) == "" {
				return errors.New("tenant administrator initial password is required")
			}
			adminUser, _, createErr := UserService.CreateUserDB(ctx.Tx, request.CreateUserRequest{
				Username: req.AdminUsername,
				Nickname: defaultString(req.AdminNickname, req.AdminUsername),
				Password: req.AdminPassword,
				Email:    utils.NormalizeNullableString(&req.AdminEmail),
				Mobile:   utils.NormalizeNullableString(&req.AdminMobile),
				Remark:   "Initial administrator for tenant " + name,
			}, operator)
			if createErr != nil {
				return createErr
			}
			adminUserID = adminUser.ID
		}
		if adminUserID > 0 {
			if err = s.initTenantAdminTx(ctx, item.ID, adminUserID, operator); err != nil {
				return err
			}
		}
		return s.recordAuthAuditTx(ctx, operator, item.ID, models.DomainTypePlatform, "tenant", fmt.Sprint(item.ID), models.AuditActionTenantCreated, nil, item, models.RiskLevelMedium, "")
	})
	if err != nil {
		return nil, err
	}
	tenant.CreatedAt = nowOrValue(tenant.CreatedAt, now)
	tenant.UpdatedAt = nowOrValue(tenant.UpdatedAt, now)
	if defaultAgent != nil {
		if err := provisionNewTenantDefaultAIKey(tenant.ID, operator); err != nil {
			slog.Warn("provision tenant default AI key after tenant creation failed", "tenant_id", tenant.ID, "error", err)
		}
	}
	return tenant, nil
}

func (s *platformIAMService) UpdateTenant(req request.PlatformTenantUpdateRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	if req.ID <= 0 {
		return nil, errors.New("tenant id is required")
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.ID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	columns := map[string]any{
		"updated_at":       time.Now(),
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errors.New("tenant name is required")
		}
		columns["name"] = name
	}
	serviceSceneChanged := false
	if req.ServiceScene != nil {
		serviceScene, err := normalizeTenantServiceScene(*req.ServiceScene)
		if err != nil {
			return nil, err
		}
		columns["service_scene"] = serviceScene
		serviceSceneChanged = serviceScene != tenant.EffectiveServiceScene()
	}
	if req.Industry != nil {
		columns["industry"] = strings.TrimSpace(*req.Industry)
	}
	if req.CountryRegion != nil {
		columns["country_region"] = strings.TrimSpace(*req.CountryRegion)
	}
	if req.DefaultLocale != nil {
		defaultLocale := strings.TrimSpace(*req.DefaultLocale)
		if defaultLocale == "" {
			return nil, errors.New("tenant default locale is required")
		}
		columns["default_locale"] = defaultLocale
		if req.SupportedLocales == nil {
			columns["supported_locales_json"] = utils.MarshalStringListJSON(normalizeTenantPreferences(utils.ParseStringListJSON(tenant.SupportedLocalesJSON), defaultLocale))
		}
	}
	if req.CustomerDefaultLocale != nil && strings.TrimSpace(*req.CustomerDefaultLocale) == "" {
		return nil, errors.New("customer default locale is required")
	}
	customerDefaultLocale := TenantPortalSettingsService.CustomerDefaultLocaleDB(sqls.DB(), tenant)
	if req.CustomerDefaultLocale != nil {
		customerDefaultLocale = strings.TrimSpace(*req.CustomerDefaultLocale)
	}
	if req.SupportedLocales != nil || req.CustomerDefaultLocale != nil {
		defaultLocale := strings.TrimSpace(tenant.DefaultLocale)
		if value, ok := columns["default_locale"].(string); ok {
			defaultLocale = value
		}
		supportedLocales := req.SupportedLocales
		if supportedLocales == nil {
			supportedLocales = utils.ParseStringListJSON(tenant.SupportedLocalesJSON)
		}
		columns["supported_locales_json"] = utils.MarshalStringListJSON(normalizeTenantPreferences(append([]string{customerDefaultLocale}, supportedLocales...), defaultLocale))
	}
	if req.Timezone != nil {
		timezone := strings.TrimSpace(*req.Timezone)
		if timezone == "" {
			return nil, errors.New("tenant timezone is required")
		}
		columns["timezone"] = timezone
		if req.SupportedTimezones == nil {
			columns["supported_timezones_json"] = utils.MarshalStringListJSON(normalizeTenantPreferences(utils.ParseStringListJSON(tenant.SupportedTimezonesJSON), timezone))
		}
	}
	if req.SupportedTimezones != nil {
		timezone := strings.TrimSpace(tenant.Timezone)
		if value, ok := columns["timezone"].(string); ok {
			timezone = value
		}
		columns["supported_timezones_json"] = utils.MarshalStringListJSON(normalizeTenantPreferences(req.SupportedTimezones, timezone))
	}
	if req.DataRegion != nil {
		columns["data_region"] = strings.TrimSpace(*req.DataRegion)
	}
	if req.Status != nil {
		if !enums.IsValidStatus(*req.Status) {
			return nil, errors.New("invalid tenant status")
		}
		columns["status"] = enums.Status(*req.Status)
	}
	if req.AIEnabled != nil {
		columns["ai_enabled"] = *req.AIEnabled
	}
	if req.TrialEndsAt != nil {
		t, err := parseOptionalTime(*req.TrialEndsAt)
		if err != nil {
			return nil, err
		}
		columns["trial_ends_at"] = t
	}
	var updated *models.Tenant
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.PlatformIAMRepository.UpdateTenant(ctx.Tx, req.ID, columns); err != nil {
			return err
		}
		if req.BrandName != nil || req.LogoAssetID != nil || req.LogoURL != nil || req.CustomDomain != nil || req.CustomerTheme != nil || req.CustomerDefaultLocale != nil {
			branding, _ := TenantPortalSettingsService.GetDB(ctx.Tx, req.ID)
			brandName, logoURL, customDomain, customerTheme := "", "", "", models.TenantCustomerThemeDefault
			customerDefaultLocale := tenant.DefaultLocale
			logoAssetID := int64(0)
			if branding != nil {
				brandName, logoURL, customDomain = branding.BrandName, branding.LogoURL, branding.CustomDomain
				logoAssetID = branding.LogoAssetID
				customerTheme = branding.EffectiveCustomerTheme()
				customerDefaultLocale = defaultString(branding.DefaultLocale, tenant.DefaultLocale)
			}
			if req.BrandName != nil {
				brandName = *req.BrandName
			}
			if req.LogoURL != nil {
				logoURL = *req.LogoURL
			}
			if req.LogoAssetID != nil {
				logoAssetID = *req.LogoAssetID
			}
			if req.CustomDomain != nil {
				customDomain = *req.CustomDomain
			}
			if req.CustomerTheme != nil {
				customerTheme = *req.CustomerTheme
			}
			if req.CustomerDefaultLocale != nil {
				customerDefaultLocale = *req.CustomerDefaultLocale
			}
			if _, err := TenantPortalSettingsService.SaveBrandingDB(ctx.Tx, req.ID, brandName, logoAssetID, logoURL, customDomain, customerTheme, customerDefaultLocale, operator); err != nil {
				return err
			}
		}
		if req.ServerConsoleName != nil || req.ServerConsoleURL != nil || req.ServerConsoleMode != nil || req.ServerConsoleEnabled != nil {
			_, serverConsole := TenantPortalSettingsService.GetDB(ctx.Tx, req.ID)
			name, baseURL, mode, enabled := "1Panel", "", TenantExternalPortalModeExternal, false
			if serverConsole != nil {
				metadata := TenantExternalPortalMetadata(serverConsole)
				name, baseURL, mode, enabled = metadata.DisplayName, serverConsole.BaseURL, metadata.EmbedMode, serverConsole.Enabled
			}
			if req.ServerConsoleName != nil {
				name = *req.ServerConsoleName
			}
			if req.ServerConsoleURL != nil {
				baseURL = *req.ServerConsoleURL
			}
			if req.ServerConsoleMode != nil {
				mode = *req.ServerConsoleMode
			}
			if req.ServerConsoleEnabled != nil {
				enabled = *req.ServerConsoleEnabled
			}
			if _, err := TenantPortalSettingsService.SaveOnePanelDB(ctx.Tx, req.ID, name, baseURL, mode, enabled, operator); err != nil {
				return err
			}
		}
		updated = repositories.PlatformIAMRepository.GetTenant(ctx.Tx, req.ID)
		if updated != nil && serviceSceneChanged {
			if err := EnsureTenantDefaultIAMRolesDB(ctx.Tx, req.ID, operator); err != nil {
				return err
			}
			if _, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(ctx.Tx, req.ID, operator); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if updated != nil && updated.IsAIEnabled() && ((req.AIEnabled != nil && *req.AIEnabled && !tenant.IsAIEnabled()) || serviceSceneChanged) {
		defaultAgent, err := TenantDefaultAIAgentService.EnsureDB(sqls.DB(), req.ID, operator)
		if err != nil {
			return nil, err
		}
		if defaultAgent != nil {
			if err := provisionNewTenantDefaultAIKey(req.ID, operator); err != nil {
				slog.Warn("provision tenant default AI key after enabling tenant AI capability failed", "tenant_id", req.ID, "error", err)
			}
		}
	}
	_ = s.RecordAuthAudit(operator, req.ID, models.DomainTypePlatform, "tenant", fmt.Sprint(req.ID), "tenant.updated", tenant, columns, models.RiskLevelMedium, "")
	return updated, nil
}

func normalizeTenantServiceScene(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return models.TenantServiceSceneEquipmentAfterSales, nil
	}
	if normalized := models.NormalizeTenantServiceScene(value); normalized != "" {
		return normalized, nil
	}
	return "", errors.New("invalid tenant service scene")
}

func (s *platformIAMService) FreezeTenant(req request.PlatformTenantStateRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	return s.changeTenantState(req.TenantID, enums.StatusDisabled, req.Reason, "tenant.frozen", operator)
}

func (s *platformIAMService) UnfreezeTenant(req request.PlatformTenantStateRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	return s.changeTenantState(req.TenantID, enums.StatusOk, "", "tenant.unfrozen", operator)
}

func (s *platformIAMService) EnterTenant(req request.PlatformTenantEnterRequest, operator *dto.AuthPrincipal, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 {
		return nil, errors.New("platform operator is required")
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return nil, errors.New("tenant not found")
	}
	var login *response.LoginResponse
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		staff := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(ctx.Tx, operator.UserID)
		if staff == nil {
			staff = &models.PlatformStaffProfile{
				UserID:         operator.UserID,
				TeamCode:       "platform-operations",
				JobTitle:       "Platform administrator",
				SupportLevel:   "administrator",
				EmploymentType: "employee",
				Status:         enums.StatusOk,
				AuditFields:    utils.BuildAuditFields(operator),
			}
			if err := repositories.PlatformIAMRepository.CreatePlatformStaff(ctx.Tx, staff); err != nil {
				return err
			}
		}
		if staff.Status != enums.StatusOk {
			return errors.New("platform staff is disabled")
		}
		member := repositories.EnterpriseIAMRepository.FindTenantAdministrator(ctx.Tx, req.TenantID)
		if member == nil {
			return errors.New("tenant administrator not found")
		}
		now := time.Now()
		grant := &models.PlatformTenantGrant{
			PlatformStaffID: staff.ID,
			TenantID:        req.TenantID,
			GrantScopeJSON:  `{"mode":"enterprise_console","role":"tenant_owner"}`,
			Reason:          defaultString(req.Reason, "平台管理员进入企业端"),
			ApprovedBy:      operator.UserID,
			ApprovedAt:      &now,
			ExpiredAt:       now.Add(2 * time.Hour),
			Status:          models.AccessGrantStatusActive,
			AuditFields:     utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreatePlatformTenantGrant(ctx.Tx, grant); err != nil {
			return err
		}
		var err error
		login, err = AuthService.IssuePlatformTenantSessionDB(ctx.Tx, operator, member, grant, clientIP, userAgent, authCfg)
		if err != nil {
			return err
		}
		return s.recordAuthAuditTx(ctx, operator, req.TenantID, models.DomainTypePlatform, "tenant", fmt.Sprint(req.TenantID), "tenant.support_session_started", nil, map[string]any{
			"tenantName": tenant.Name,
			"expiresAt":  grant.ExpiredAt,
		}, models.RiskLevelCritical, fmt.Sprint(grant.ID))
	})
	if err != nil {
		return nil, err
	}
	return login, nil
}

func (s *platformIAMService) ResetTenantAdministratorPassword(req request.PlatformTenantAdminPasswordRequest, operator *dto.AuthPrincipal) (*response.PlatformTenantAdminResponse, error) {
	member := repositories.EnterpriseIAMRepository.FindTenantAdministrator(sqls.DB(), req.TenantID)
	if member == nil {
		return nil, errors.New("tenant administrator not found")
	}
	user := repositories.UserRepository.Get(sqls.DB(), member.UserID)
	if user == nil {
		return nil, errors.New("tenant administrator account not found")
	}
	if err := UserService.SetPassword(user.ID, req.Password, operator); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant_administrator", fmt.Sprint(user.ID), "tenant_administrator.password_reset", nil, map[string]any{
		"username": user.Username,
	}, models.RiskLevelCritical, "")
	return &response.PlatformTenantAdminResponse{
		TenantID:    req.TenantID,
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: member.DisplayName,
	}, nil
}

func (s *platformIAMService) changeTenantState(tenantID int64, status enums.Status, reason, action string, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	columns := map[string]any{
		"status":           status,
		"frozen_reason":    strings.TrimSpace(reason),
		"updated_at":       time.Now(),
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	if err := repositories.PlatformIAMRepository.UpdateTenant(sqls.DB(), tenantID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, tenantID, models.DomainTypePlatform, "tenant", fmt.Sprint(tenantID), action, tenant, columns, models.RiskLevelHigh, "")
	return repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID), nil
}

func (s *platformIAMService) DecommissionTenant(req request.PlatformTenantDecommissionRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	purgeAfterDays := req.PurgeAfterDays
	if purgeAfterDays <= 0 {
		purgeAfterDays = 30
	}
	now := time.Now()
	columns := map[string]any{
		"status":              enums.StatusDisabled,
		"frozen_reason":       strings.TrimSpace(req.Reason),
		"decommissioned_at":   now,
		"decommission_reason": strings.TrimSpace(req.Reason),
		"purge_after_days":    purgeAfterDays,
		"updated_at":          now,
		"update_user_id":      auditUserID(operator),
		"update_user_name":    auditUserName(operator),
	}
	if err := repositories.PlatformIAMRepository.UpdateTenant(sqls.DB(), req.TenantID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant", fmt.Sprint(req.TenantID), "tenant.decommissioned", tenant, columns, models.RiskLevelCritical, "")
	return repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID), nil
}

func (s *platformIAMService) DeleteTenant(req request.PlatformTenantDeleteRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	if req.TenantID <= 0 {
		return nil, errors.New("tenant id is required")
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return nil, errors.New("tenant not found")
	}
	if tenant.DecommissionedAt == nil {
		return nil, errors.New("only decommissioned tenants can be deleted")
	}

	now := time.Now()
	columns := map[string]any{
		"status":           enums.StatusDeleted,
		"updated_at":       now,
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	if err := repositories.PlatformIAMRepository.UpdateTenant(sqls.DB(), req.TenantID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant", fmt.Sprint(req.TenantID), "tenant.deleted", tenant, map[string]any{
		"reason": req.Reason,
		"status": enums.StatusDeleted,
	}, models.RiskLevelCritical, "")
	return repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID), nil
}

func (s *platformIAMService) UndoDecommissionTenant(req request.PlatformTenantStateRequest, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	columns := map[string]any{
		"status":              enums.StatusOk,
		"frozen_reason":       "",
		"decommissioned_at":   nil,
		"decommission_reason": "",
		"updated_at":          time.Now(),
		"update_user_id":      auditUserID(operator),
		"update_user_name":    auditUserName(operator),
	}
	if err := repositories.PlatformIAMRepository.UpdateTenant(sqls.DB(), req.TenantID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant", fmt.Sprint(req.TenantID), "tenant.decommission_undone", tenant, columns, models.RiskLevelHigh, "")
	return repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID), nil
}

func (s *platformIAMService) ListPlans(status string) ([]models.TenantPlan, error) {
	return repositories.PlatformIAMRepository.FindPlans(sqls.DB(), status)
}

func (s *platformIAMService) SavePlan(req request.PlatformPlanSaveRequest, operator *dto.AuthPrincipal) (*models.TenantPlan, error) {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return nil, errors.New("plan code and name are required")
	}
	now := time.Now()
	status := enums.StatusOk
	if req.Status != nil {
		if !enums.IsValidStatus(*req.Status) {
			return nil, errors.New("invalid plan status")
		}
		status = enums.Status(*req.Status)
	}
	if req.ID > 0 {
		if repositories.PlatformIAMRepository.GetPlan(sqls.DB(), req.ID) == nil {
			return nil, errors.New("tenant plan not found")
		}
		columns := map[string]any{
			"code":                code,
			"name":                name,
			"plan_type":           strings.TrimSpace(req.PlanType),
			"feature_json":        defaultJSON(req.FeatureJSON, "{}"),
			"quota_template_json": defaultJSON(req.QuotaTemplateJSON, "{}"),
			"overage_strategy":    defaultString(req.OverageStrategy, "block"),
			"status":              status,
			"sort_no":             req.SortNo,
			"updated_at":          now,
			"update_user_id":      auditUserID(operator),
			"update_user_name":    auditUserName(operator),
		}
		if err := repositories.PlatformIAMRepository.UpdatePlan(sqls.DB(), req.ID, columns); err != nil {
			return nil, err
		}
		_ = s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "tenant_plan", fmt.Sprint(req.ID), "plan.updated", nil, columns, models.RiskLevelMedium, "")
		return repositories.PlatformIAMRepository.GetPlan(sqls.DB(), req.ID), nil
	}
	if repositories.PlatformIAMRepository.GetPlanByCode(sqls.DB(), code) != nil {
		return nil, errors.New("tenant plan code already exists")
	}
	item := &models.TenantPlan{
		Code:              code,
		Name:              name,
		PlanType:          strings.TrimSpace(req.PlanType),
		FeatureJSON:       defaultJSON(req.FeatureJSON, "{}"),
		QuotaTemplateJSON: defaultJSON(req.QuotaTemplateJSON, "{}"),
		OverageStrategy:   defaultString(req.OverageStrategy, "block"),
		Status:            status,
		SortNo:            req.SortNo,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	if err := repositories.PlatformIAMRepository.CreatePlan(sqls.DB(), item); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "tenant_plan", fmt.Sprint(item.ID), "plan.created", nil, item, models.RiskLevelMedium, "")
	return item, nil
}

func (s *platformIAMService) AssignSubscription(req request.PlatformSubscriptionAssignRequest, operator *dto.AuthPrincipal) (*models.TenantSubscription, error) {
	plan := repositories.PlatformIAMRepository.GetPlan(sqls.DB(), req.PlanID)
	if plan == nil {
		return nil, errors.New("tenant plan not found")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID) == nil {
		return nil, errors.New("tenant not found")
	}
	var ret *models.TenantSubscription
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		subscription, err := s.assignSubscriptionTx(ctx, req.TenantID, plan, req.StartsAt, req.EndsAt, req.BillingCycle, operator)
		if err != nil {
			return err
		}
		ret = subscription
		return s.recordAuthAuditTx(ctx, operator, req.TenantID, models.DomainTypePlatform, "tenant_subscription", fmt.Sprint(subscription.ID), "subscription.assigned", nil, subscription, models.RiskLevelMedium, "")
	})
	return ret, err
}

func (s *platformIAMService) assignSubscriptionTx(ctx *sqls.TxContext, tenantID int64, plan *models.TenantPlan, startsAtRaw, endsAtRaw, billingCycle string, operator *dto.AuthPrincipal) (*models.TenantSubscription, error) {
	startsAt, err := parseOptionalTime(startsAtRaw)
	if err != nil {
		return nil, err
	}
	if startsAt == nil {
		now := time.Now()
		startsAt = &now
	}
	endsAt, err := parseOptionalTime(endsAtRaw)
	if err != nil {
		return nil, err
	}
	snapshot, _ := json.Marshal(plan)
	if err = repositories.PlatformIAMRepository.DisableActiveSubscriptions(ctx.Tx, tenantID, time.Now()); err != nil {
		return nil, err
	}
	item := &models.TenantSubscription{
		TenantID:         tenantID,
		PlanID:           plan.ID,
		PlanSnapshotJSON: string(snapshot),
		StartsAt:         *startsAt,
		EndsAt:           endsAt,
		BillingCycle:     defaultString(billingCycle, "monthly"),
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	}
	return item, repositories.PlatformIAMRepository.CreateSubscription(ctx.Tx, item)
}

func (s *platformIAMService) OverrideQuota(req request.PlatformQuotaOverrideRequest, operator *dto.AuthPrincipal) (*models.TenantQuotaOverride, error) {
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID) == nil {
		return nil, errors.New("tenant not found")
	}
	quotaKey := strings.TrimSpace(req.QuotaKey)
	if quotaKey == "" {
		return nil, errors.New("quota key is required")
	}
	period := defaultString(req.Period, "monthly")
	effectiveFrom, err := parseOptionalTime(req.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	exists := repositories.PlatformIAMRepository.FindQuotaOverride(sqls.DB(), req.TenantID, req.ProductID, quotaKey, period)
	columns := map[string]any{
		"quota_value":      req.QuotaValue,
		"overage_strategy": strings.TrimSpace(req.OverageStrategy),
		"reason":           strings.TrimSpace(req.Reason),
		"effective_from":   effectiveFrom,
		"effective_to":     effectiveTo,
		"status":           enums.StatusOk,
		"updated_at":       now,
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	if exists != nil {
		if err = repositories.PlatformIAMRepository.UpdateQuotaOverride(sqls.DB(), exists.ID, columns); err != nil {
			return nil, err
		}
		_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant_quota_override", fmt.Sprint(exists.ID), "quota.override_updated", exists, columns, models.RiskLevelHigh, "")
		return repositories.PlatformIAMRepository.FindQuotaOverride(sqls.DB(), req.TenantID, req.ProductID, quotaKey, period), nil
	}
	item := &models.TenantQuotaOverride{
		TenantID:        req.TenantID,
		ProductID:       req.ProductID,
		QuotaKey:        quotaKey,
		QuotaValue:      req.QuotaValue,
		Period:          period,
		OverageStrategy: strings.TrimSpace(req.OverageStrategy),
		Reason:          strings.TrimSpace(req.Reason),
		EffectiveFrom:   effectiveFrom,
		EffectiveTo:     effectiveTo,
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err = repositories.PlatformIAMRepository.CreateQuotaOverride(sqls.DB(), item); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "tenant_quota_override", fmt.Sprint(item.ID), "quota.override_created", nil, item, models.RiskLevelHigh, "")
	return item, nil
}

func (s *platformIAMService) ListSub2APIAccounts(tenantID int64) ([]models.Sub2APITenantAccount, error) {
	return repositories.PlatformIAMRepository.FindSub2APIAccounts(sqls.DB(), tenantID)
}

func (s *platformIAMService) BindSub2APIAccount(req request.PlatformSub2APIAccountBindRequest, operator *dto.AuthPrincipal) (*models.Sub2APITenantAccount, error) {
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID) == nil {
		return nil, errors.New("tenant not found")
	}
	accountID := strings.TrimSpace(req.Sub2APIAccountID)
	if accountID == "" {
		return nil, errors.New("sub2api account id is required")
	}
	now := time.Now()
	exists := repositories.PlatformIAMRepository.FindSub2APIAccount(sqls.DB(), req.TenantID, accountID)
	columns := map[string]any{
		"account_name":        strings.TrimSpace(req.AccountName),
		"dashboard_url":       strings.TrimSpace(req.DashboardURL),
		"account_status":      defaultString(req.AccountStatus, "bound"),
		"quota_snapshot_json": defaultJSON(req.QuotaSnapshotJSON, "{}"),
		"last_synced_at":      now,
		"status":              enums.StatusOk,
		"updated_at":          now,
		"update_user_id":      auditUserID(operator),
		"update_user_name":    auditUserName(operator),
	}
	if exists != nil {
		if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), exists.ID, columns); err != nil {
			return nil, err
		}
		_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "sub2api_account", fmt.Sprint(exists.ID), "sub2api.account_updated", exists, columns, models.RiskLevelHigh, "")
		return repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), exists.ID), nil
	}
	item := &models.Sub2APITenantAccount{
		TenantID:          req.TenantID,
		Sub2APIAccountID:  accountID,
		AccountName:       strings.TrimSpace(req.AccountName),
		DashboardURL:      strings.TrimSpace(req.DashboardURL),
		AccountStatus:     defaultString(req.AccountStatus, "bound"),
		QuotaSnapshotJSON: defaultJSON(req.QuotaSnapshotJSON, "{}"),
		LastSyncedAt:      &now,
		Status:            enums.StatusOk,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	if err := repositories.PlatformIAMRepository.CreateSub2APIAccount(sqls.DB(), item); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "sub2api_account", fmt.Sprint(item.ID), "sub2api.account_bound", nil, item, models.RiskLevelHigh, "")
	return item, nil
}

func (s *platformIAMService) TouchSub2APIAccount(req request.PlatformSub2APIAccountActionRequest, action string, operator *dto.AuthPrincipal) (*models.Sub2APITenantAccount, error) {
	account := repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), req.AccountID)
	if account == nil {
		return nil, errors.New("sub2api account not found")
	}
	now := time.Now()
	columns := map[string]any{"updated_at": now, "update_user_id": auditUserID(operator), "update_user_name": auditUserName(operator)}
	if action == "test" {
		columns["last_tested_at"] = now
		columns["account_status"] = "healthy"
	} else {
		columns["last_synced_at"] = now
		columns["account_status"] = "synced"
	}
	if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, account.TenantID, models.DomainTypePlatform, "sub2api_account", fmt.Sprint(account.ID), "sub2api.account_"+action, account, columns, models.RiskLevelMedium, "")
	return repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), account.ID), nil
}

func (s *platformIAMService) ListPlatformStaff(status string) (*PlatformStaffListAggregate, error) {
	items, err := repositories.PlatformIAMRepository.FindPlatformStaff(sqls.DB(), status)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	users := make(map[int64]*models.User, len(userIDs))
	for _, user := range repositories.UserRepository.FindByIds(sqls.DB(), uniqueInt64s(userIDs)) {
		item := user
		users[item.ID] = &item
	}
	return &PlatformStaffListAggregate{Items: items, Users: users}, nil
}

func (s *platformIAMService) InvitePlatformStaff(req request.PlatformStaffInviteRequest, operator *dto.AuthPrincipal) (*models.PlatformStaffProfile, error) {
	var item *models.PlatformStaffProfile
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := EnsurePlatformDefaultIAMRolesDB(ctx.Tx, operator); err != nil {
			return err
		}
		userID := req.UserID
		if userID <= 0 {
			if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
				return errors.New("username and initial password are required")
			}
			user, _, err := UserService.CreateUserDB(ctx.Tx, request.CreateUserRequest{
				Username: req.Username,
				Nickname: defaultString(req.Nickname, req.Username),
				Password: req.Password,
				Email:    utils.NormalizeNullableString(&req.Email),
				Mobile:   utils.NormalizeNullableString(&req.Mobile),
				Remark:   "Platform staff account",
			}, operator)
			if err != nil {
				return err
			}
			userID = user.ID
		}
		if repositories.UserRepository.Get(ctx.Tx, userID) == nil {
			return errors.New("user not found")
		}
		item = repositories.PlatformIAMRepository.FindPlatformStaffByUserID(ctx.Tx, userID)
		if item == nil {
			item = &models.PlatformStaffProfile{
				UserID: userID, TeamCode: strings.TrimSpace(req.TeamCode), JobTitle: strings.TrimSpace(req.JobTitle),
				SupportLevel: strings.TrimSpace(req.SupportLevel), EmploymentType: defaultString(req.EmploymentType, "employee"),
				Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.PlatformIAMRepository.CreatePlatformStaff(ctx.Tx, item); err != nil {
				return err
			}
		}
		return replaceIAMRoleBindingsDB(ctx.Tx, 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, item.ID, req.RoleCodes, PlatformRoleOperations, operator)
	})
	if err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "platform_staff", fmt.Sprint(item.ID), "platform_staff.invited", nil, item, models.RiskLevelMedium, "")
	return item, nil
}

func (s *platformIAMService) UpdatePlatformStaff(req request.PlatformStaffUpdateRequest, operator *dto.AuthPrincipal) (*models.PlatformStaffProfile, error) {
	if req.PlatformStaffID <= 0 {
		return nil, errors.New("platform staff id is required")
	}
	staff := repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID)
	if staff == nil || staff.Status == enums.StatusDeleted {
		return nil, errors.New("platform staff not found")
	}
	if req.Status != nil {
		if !enums.IsValidStatus(*req.Status) {
			return nil, errors.New("invalid platform staff status")
		}
		if staff.UserID == auditUserID(operator) && enums.Status(*req.Status) != enums.StatusOk {
			return nil, errors.New("cannot disable current platform staff account")
		}
	}
	now := time.Now()
	revokeSessions := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		user := repositories.UserRepository.Get(ctx.Tx, staff.UserID)
		if user == nil {
			return errors.New("platform staff user not found")
		}
		userColumns := map[string]any{}
		if req.Nickname != nil {
			nickname := strings.TrimSpace(*req.Nickname)
			if nickname == "" {
				return errors.New("platform staff display name is required")
			}
			userColumns["nickname"] = nickname
		}
		if req.Email != nil {
			email := utils.NormalizeNullableString(req.Email)
			if email != nil {
				if existed := repositories.UserRepository.GetByEmail(ctx.Tx, *email); existed != nil && existed.ID != user.ID {
					return errors.New("email is already used")
				}
			}
			userColumns["email"] = email
		}
		if req.Mobile != nil {
			mobile := utils.NormalizeNullableString(req.Mobile)
			if mobile != nil {
				if existed := repositories.UserRepository.GetByMobile(ctx.Tx, *mobile); existed != nil && existed.ID != user.ID {
					return errors.New("mobile is already used")
				}
			}
			userColumns["mobile"] = mobile
		}
		if len(userColumns) > 0 {
			userColumns["updated_at"] = now
			userColumns["update_user_id"] = auditUserID(operator)
			userColumns["update_user_name"] = auditUserName(operator)
			if err := repositories.UserRepository.Updates(ctx.Tx, user.ID, userColumns); err != nil {
				return err
			}
		}

		columns := map[string]any{
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if req.TeamCode != nil {
			columns["team_code"] = strings.TrimSpace(*req.TeamCode)
		}
		if req.JobTitle != nil {
			columns["job_title"] = strings.TrimSpace(*req.JobTitle)
		}
		if req.SupportLevel != nil {
			columns["support_level"] = strings.TrimSpace(*req.SupportLevel)
		}
		if req.EmploymentType != nil {
			columns["employment_type"] = defaultString(*req.EmploymentType, "employee")
		}
		if req.Status != nil {
			status := enums.Status(*req.Status)
			columns["status"] = status
			revokeSessions = status != enums.StatusOk
		}
		return repositories.PlatformIAMRepository.UpdatePlatformStaff(ctx.Tx, staff.ID, columns)
	})
	if err != nil {
		return nil, err
	}
	if revokeSessions {
		_ = LoginSessionService.RevokeByUser(staff.UserID, auditUserID(operator), auditUserName(operator))
	}
	_ = s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "platform_staff", fmt.Sprint(req.PlatformStaffID), "platform_staff.updated", staff, req, models.RiskLevelMedium, "")
	return repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID), nil
}

func (s *platformIAMService) GrantPlatformTenant(req request.PlatformStaffGrantTenantRequest, operator *dto.AuthPrincipal) (*models.PlatformTenantGrant, error) {
	if repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID) == nil {
		return nil, errors.New("platform staff not found")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID) == nil {
		return nil, errors.New("tenant not found")
	}
	expiredAt, err := parseRequiredTime(req.ExpiredAt)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	approvedBy := req.ApprovedBy
	if approvedBy <= 0 {
		approvedBy = auditUserID(operator)
	}
	item := &models.PlatformTenantGrant{
		PlatformStaffID: req.PlatformStaffID,
		TenantID:        req.TenantID,
		GrantScopeJSON:  defaultJSON(req.GrantScopeJSON, "{}"),
		Reason:          strings.TrimSpace(req.Reason),
		ApprovedBy:      approvedBy,
		ApprovedAt:      &now,
		ExpiredAt:       expiredAt,
		Status:          models.AccessGrantStatusActive,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err = repositories.PlatformIAMRepository.CreatePlatformTenantGrant(sqls.DB(), item); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, models.DomainTypePlatform, "platform_tenant_grant", fmt.Sprint(item.ID), "platform_staff.tenant_granted", nil, item, models.RiskLevelCritical, fmt.Sprint(item.ID))
	return item, nil
}

func (s *platformIAMService) DisablePlatformStaff(req request.PlatformStaffDisableRequest, operator *dto.AuthPrincipal) (*models.PlatformStaffProfile, error) {
	if repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID) == nil {
		return nil, errors.New("platform staff not found")
	}
	columns := map[string]any{
		"status":           enums.StatusDisabled,
		"updated_at":       time.Now(),
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	if err := repositories.PlatformIAMRepository.UpdatePlatformStaff(sqls.DB(), req.PlatformStaffID, columns); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "platform_staff", fmt.Sprint(req.PlatformStaffID), "platform_staff.disabled", nil, map[string]any{"reason": req.Reason}, models.RiskLevelHigh, "")
	return repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID), nil
}

func (s *platformIAMService) DeletePlatformStaff(req request.PlatformStaffDeleteRequest, operator *dto.AuthPrincipal) error {
	if req.PlatformStaffID <= 0 {
		return errors.New("platform staff id is required")
	}
	staff := repositories.PlatformIAMRepository.GetPlatformStaff(sqls.DB(), req.PlatformStaffID)
	if staff == nil || staff.Status == enums.StatusDeleted {
		return errors.New("platform staff not found")
	}
	if staff.UserID == auditUserID(operator) {
		return errors.New("cannot delete current platform staff account")
	}
	now := time.Now()
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		auditColumns := map[string]any{
			"status":           enums.StatusDeleted,
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if err := repositories.PlatformIAMRepository.UpdatePlatformStaff(ctx.Tx, staff.ID, auditColumns); err != nil {
			return err
		}
		if err := repositories.PlatformIAMRepository.UpdateRoleBindingsBySubject(ctx.Tx, 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, staff.ID, auditColumns); err != nil {
			return err
		}
		return repositories.UserRepository.Updates(ctx.Tx, staff.UserID, map[string]any{
			"status":           enums.StatusDisabled,
			"deleted_at":       now,
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		})
	})
	if err != nil {
		return err
	}
	_ = LoginSessionService.RevokeByUser(staff.UserID, auditUserID(operator), auditUserName(operator))
	return s.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "platform_staff", fmt.Sprint(req.PlatformStaffID), "platform_staff.deleted", staff, map[string]any{"reason": req.Reason}, models.RiskLevelHigh, "")
}

func (s *platformIAMService) ListAuthRoles(tenantID int64, domainType string) ([]models.AuthRole, map[int64][]string, error) {
	items, err := repositories.PlatformIAMRepository.FindAuthRoles(sqls.DB(), tenantID, domainType)
	if err != nil {
		return nil, nil, err
	}
	roleIDs := make([]int64, 0, len(items))
	for _, item := range items {
		roleIDs = append(roleIDs, item.ID)
	}
	permissions, err := repositories.PlatformIAMRepository.FindPermissionsByRoleIDs(sqls.DB(), roleIDs)
	return items, permissions, err
}

func (s *platformIAMService) SaveAuthRole(req request.PlatformAuthRoleSaveRequest, operator *dto.AuthPrincipal) (*models.AuthRole, error) {
	domainType := strings.TrimSpace(req.DomainType)
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if domainType == "" || code == "" || name == "" {
		return nil, errors.New("domain type, code and name are required")
	}
	status := enums.StatusOk
	if req.Status != nil {
		if !enums.IsValidStatus(*req.Status) {
			return nil, errors.New("invalid role status")
		}
		status = enums.Status(*req.Status)
	}
	if req.ID > 0 {
		existing := repositories.PlatformIAMRepository.GetAuthRole(sqls.DB(), req.ID)
		if existing == nil || existing.Status == enums.StatusDeleted {
			return nil, errors.New("auth role not found")
		}
		if existing.TenantID != req.TenantID || existing.DomainType != domainType {
			return nil, errors.New("auth role tenant or domain cannot be changed")
		}
		if existing.Code != code {
			return nil, errors.New("auth role code cannot be changed")
		}
		columns := map[string]any{
			"tenant_id":        req.TenantID,
			"domain_type":      domainType,
			"code":             code,
			"name":             name,
			"description":      strings.TrimSpace(req.Description),
			"is_builtin":       existing.IsBuiltin,
			"status":           status,
			"sort_no":          req.SortNo,
			"updated_at":       time.Now(),
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if err := repositories.PlatformIAMRepository.UpdateAuthRole(sqls.DB(), req.ID, columns); err != nil {
			return nil, err
		}
		_ = s.RecordAuthAudit(operator, req.TenantID, domainType, "auth_role", fmt.Sprint(req.ID), "auth_role.updated", nil, columns, models.RiskLevelMedium, "")
		return repositories.PlatformIAMRepository.GetAuthRole(sqls.DB(), req.ID), nil
	}
	if repositories.PlatformIAMRepository.FindAuthRoleByCode(sqls.DB(), req.TenantID, domainType, code) != nil {
		return nil, errors.New("auth role code already exists in tenant domain")
	}
	item := &models.AuthRole{
		TenantID:    req.TenantID,
		DomainType:  domainType,
		Code:        code,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		IsBuiltin:   req.IsBuiltin,
		Status:      status,
		SortNo:      req.SortNo,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.PlatformIAMRepository.CreateAuthRole(sqls.DB(), item); err != nil {
		return nil, err
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, domainType, "auth_role", fmt.Sprint(item.ID), "auth_role.created", nil, item, models.RiskLevelMedium, "")
	return item, nil
}

func (s *platformIAMService) SaveAuthPolicy(req request.PlatformAuthPolicySaveRequest, operator *dto.AuthPrincipal) (*models.AuthRole, []string, error) {
	role := repositories.PlatformIAMRepository.GetAuthRole(sqls.DB(), req.RoleID)
	if role == nil {
		return nil, nil, errors.New("auth role not found")
	}
	if role.TenantID != req.TenantID {
		return nil, nil, errors.New("auth role does not belong to requested tenant")
	}
	effect := defaultString(req.Effect, "allow")
	if effect != "allow" && effect != "deny" {
		return nil, nil, errors.New("effect must be allow or deny")
	}
	now := time.Now()
	permissions := make([]models.AuthRolePermission, 0, len(req.PermissionCodes))
	for _, code := range req.PermissionCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		permissions = append(permissions, models.AuthRolePermission{
			TenantID:       req.TenantID,
			RoleID:         role.ID,
			PermissionCode: code,
			Effect:         effect,
			Status:         enums.StatusOk,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   auditUserID(operator),
				CreateUserName: auditUserName(operator),
				UpdatedAt:      now,
				UpdateUserID:   auditUserID(operator),
				UpdateUserName: auditUserName(operator),
			},
		})
	}
	if err := repositories.PlatformIAMRepository.ReplaceAuthRolePermissions(sqls.DB(), req.TenantID, role.ID, permissions); err != nil {
		return nil, nil, err
	}
	permissionCodes := make([]string, 0, len(permissions))
	for _, item := range permissions {
		permissionCodes = append(permissionCodes, item.PermissionCode)
	}
	_ = s.RecordAuthAudit(operator, req.TenantID, role.DomainType, "auth_role_permission", fmt.Sprint(role.ID), "auth_policy.saved", nil, permissionCodes, models.RiskLevelMedium, "")
	return role, permissionCodes, nil
}

func (s *platformIAMService) DeleteAuthRole(req request.PlatformAuthRoleDeleteRequest, operator *dto.AuthPrincipal) error {
	if req.RoleID <= 0 {
		return errors.New("auth role id is required")
	}
	role := repositories.PlatformIAMRepository.GetAuthRole(sqls.DB(), req.RoleID)
	if role == nil || role.Status == enums.StatusDeleted {
		return errors.New("auth role not found")
	}
	if role.IsBuiltin {
		return errors.New("built-in auth role cannot be deleted")
	}
	now := time.Now()
	columns := map[string]any{
		"status":           enums.StatusDeleted,
		"updated_at":       now,
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.PlatformIAMRepository.UpdateAuthRole(ctx.Tx, role.ID, columns); err != nil {
			return err
		}
		if err := repositories.PlatformIAMRepository.UpdateRoleBindingsByRole(ctx.Tx, role.TenantID, role.DomainType, role.ID, columns); err != nil {
			return err
		}
		return repositories.PlatformIAMRepository.UpdateRolePermissionsByRole(ctx.Tx, role.TenantID, role.ID, columns)
	})
	if err != nil {
		return err
	}
	return s.RecordAuthAudit(operator, role.TenantID, role.DomainType, "auth_role", fmt.Sprint(role.ID), "auth_role.deleted", role, map[string]any{"reason": req.Reason}, models.RiskLevelMedium, "")
}

func (s *platformIAMService) CheckAuthPermission(req request.PlatformAuthPermissionCheckRequest) (*response.PlatformAuthPermissionCheckResponse, error) {
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(sqls.DB(), req.TenantID, req.DomainType, req.SubjectType, req.SubjectID)
	if err != nil {
		return nil, err
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, item := range bindings {
		roleIDs = append(roleIDs, item.RoleID)
	}
	permissionRows, err := repositories.PlatformIAMRepository.FindRolePermissions(sqls.DB(), req.TenantID, roleIDs, req.PermissionCode)
	if err != nil {
		return nil, err
	}
	roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(sqls.DB(), roleIDs)
	if err != nil {
		return nil, err
	}
	matchedRoles := make([]string, 0, len(permissionRows))
	allowed := false
	effect := "none"
	for _, row := range permissionRows {
		if role := roles[row.RoleID]; role != nil {
			matchedRoles = append(matchedRoles, role.Code)
		}
		if row.Effect == "deny" {
			return &response.PlatformAuthPermissionCheckResponse{Allowed: false, MatchedRoles: matchedRoles, PermissionCode: req.PermissionCode, Effect: "deny"}, nil
		}
		if row.Effect == "allow" {
			allowed = true
			effect = "allow"
		}
	}
	return &response.PlatformAuthPermissionCheckResponse{Allowed: allowed, MatchedRoles: matchedRoles, PermissionCode: req.PermissionCode, Effect: effect}, nil
}

func (s *platformIAMService) ListAuthAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, *sqls.Paging, error) {
	return repositories.PlatformIAMRepository.FindAuthAuditLogs(sqls.DB(), auditRepositoryFilter(tenantID, query), query.Page, query.Limit)
}

func (s *platformIAMService) ListAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, *sqls.Paging, error) {
	return listAuditLogs(tenantID, query)
}

func (s *platformIAMService) ExportAuthAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, error) {
	query.Page = 1
	query.Limit = 200
	items := make([]models.AuthAuditLog, 0, 200)
	for {
		pageItems, paging, err := s.ListAuthAuditLogs(tenantID, query)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(items) > 10000 {
			return nil, fmt.Errorf("审计记录超过 10000 条，请缩小筛选范围后导出")
		}
		if len(pageItems) == 0 || int64(query.Page*query.Limit) >= paging.Total {
			break
		}
		query.Page++
	}
	return items, nil
}

func (s *platformIAMService) ExportAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, error) {
	query.Page = 1
	query.Limit = 200
	items := make([]models.AuthAuditLog, 0, 200)
	for {
		pageItems, paging, err := s.ListAuditLogs(tenantID, query)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(items) > 10000 {
			return nil, fmt.Errorf("审计记录超过 10000 条，请缩小筛选范围后导出")
		}
		if len(pageItems) == 0 || int64(query.Page*query.Limit) >= paging.Total {
			break
		}
		query.Page++
	}
	return items, nil
}

func (s *platformIAMService) RecordAuthAudit(operator *dto.AuthPrincipal, tenantID int64, domainType, targetType, targetID, action string, before, after any, riskLevel, supportGrantID string) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return s.recordAuthAuditTx(ctx, operator, tenantID, domainType, targetType, targetID, action, before, after, riskLevel, supportGrantID)
	})
}

func (s *platformIAMService) recordAuthAuditTx(ctx *sqls.TxContext, operator *dto.AuthPrincipal, tenantID int64, domainType, targetType, targetID, action string, before, after any, riskLevel, supportGrantID string) error {
	beforeJSON := marshalPlatformAuditState(before)
	afterJSON := marshalPlatformAuditState(after)
	log := &models.AuthAuditLog{
		TenantID:         tenantID,
		DomainType:       defaultString(domainType, models.DomainTypePlatform),
		ActorUserID:      auditUserID(operator),
		ActorSubjectType: principalSubjectType(operator),
		ActorSubjectID:   principalSubjectID(operator),
		TargetType:       targetType,
		TargetID:         targetID,
		Action:           action,
		BeforeStateJSON:  beforeJSON,
		AfterStateJSON:   afterJSON,
		SupportGrantID:   parseInt64OrZero(supportGrantID),
		RiskLevel:        defaultString(riskLevel, models.RiskLevelMedium),
		Status:           models.AuditStatusSuccess,
		OccurredAt:       time.Now(),
	}
	return repositories.PlatformIAMRepository.CreateAuthAuditLog(ctx.Tx, log)
}

func (s *platformIAMService) initTenantAdminTx(ctx *sqls.TxContext, tenantID, userID int64, operator *dto.AuthPrincipal) error {
	now := time.Now()
	root := repositories.EnterpriseIAMRepository.FindRootDepartment(ctx.Tx, tenantID)
	if root == nil {
		if _, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(ctx.Tx, tenantID, operator); err != nil {
			return err
		}
		root = repositories.EnterpriseIAMRepository.FindRootDepartment(ctx.Tx, tenantID)
		if root == nil {
			return errors.New("tenant root department provisioning failed")
		}
	}
	user := repositories.UserRepository.Get(ctx.Tx, userID)
	if user == nil {
		return errors.New("admin user not found")
	}
	member := &models.TenantMember{
		TenantID:     tenantID,
		UserID:       userID,
		DepartmentID: root.ID,
		DisplayName:  user.Nickname,
		MemberType:   "owner",
		Status:       enums.StatusOk,
		JoinedAt:     &now,
		AuditFields:  utils.BuildAuditFields(operator),
	}
	if err := ctx.Tx.Create(member).Error; err != nil {
		return err
	}
	return replaceIAMRoleBindingsDB(ctx.Tx, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, member.ID, []string{EnterpriseRoleOwner}, EnterpriseRoleOwner, operator)
}

func (s *platformIAMService) BuildTenantDataExport(req request.PlatformTenantDataExportRequest) *response.PlatformTenantDataExportResponse {
	return &response.PlatformTenantDataExportResponse{
		ExportJobID:   fmt.Sprintf("tenant-export-%d-%s", req.TenantID, time.Now().Format("20060102150405")),
		EstimatedSize: "pending",
		AssetURL:      "",
	}
}

func (s *platformIAMService) BuildRightToErasure(req request.PlatformTenantRightToErasureRequest) *response.PlatformTenantRightToErasureResponse {
	return &response.PlatformTenantRightToErasureResponse{
		ErasureRequestID:      fmt.Sprintf("erasure-%d-%s", req.TenantID, time.Now().Format("20060102150405")),
		EstimatedCompletionAt: time.Now().Add(24 * time.Hour).Format(time.DateTime),
	}
}

func parseOptionalTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := parseRequiredTime(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func parseRequiredTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("time value is required")
	}
	for _, layout := range []string{time.RFC3339, time.DateTime, "2006-01-02"} {
		value, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time value: %s", raw)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func normalizeTenantPreferences(values []string, fallback string) []string {
	ret := utils.NormalizeStringList(values)
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return ret
	}
	next := []string{fallback}
	for _, value := range ret {
		if value != fallback {
			next = append(next, value)
		}
	}
	return next
}

func defaultJSON(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func auditUserID(operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return constants.SystemAuditUserID
	}
	return operator.UserID
}

func auditUserName(operator *dto.AuthPrincipal) string {
	if operator == nil || strings.TrimSpace(operator.Username) == "" {
		return constants.SystemAuditUserName
	}
	return operator.Username
}

func principalSubjectType(operator *dto.AuthPrincipal) string {
	if operator == nil || operator.SubjectType == "" {
		return models.SubjectTypePlatformStaff
	}
	return operator.SubjectType
}

func principalSubjectID(operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return 0
	}
	if operator.SubjectID > 0 {
		return operator.SubjectID
	}
	switch {
	case operator.PlatformStaffID > 0:
		return operator.PlatformStaffID
	case operator.MemberID > 0:
		return operator.MemberID
	case operator.CustomerUserID > 0:
		return operator.CustomerUserID
	case operator.PartnerAccountID > 0:
		return operator.PartnerAccountID
	case operator.ServiceAccountID > 0:
		return operator.ServiceAccountID
	default:
		return operator.UserID
	}
}

func uniqueInt64s(values []int64) []int64 {
	ret := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}

func nowOrValue(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func marshalPlatformAuditState(value any) string {
	if value == nil {
		return ""
	}
	buf, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(buf)
}

func parseInt64OrZero(value string) int64 {
	var ret int64
	_, _ = fmt.Sscan(strings.TrimSpace(value), &ret)
	return ret
}
