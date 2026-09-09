package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/cache"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var PlatformAIModelService = &platformAIModelService{}

const (
	platformSub2APIHostConfigKey           = "platform.sub2api.host"
	platformSub2APIAdminKeyConfigKey       = "platform.sub2api.admin_api_key"
	platformSub2APILLMModelConfigKey       = "platform.sub2api.default_llm_model"
	platformSub2APITranslationKeyConfigKey = "platform.sub2api.translation_api_key"
	platformSub2APIDefaultKeyGroupID       = 2
	platformSub2APIAccountPassword         = "abcd@1234"
	platformSub2APITokenCachePrefix        = "sub2api:tenant:%d:access-token"
	platformSub2APIProvisionClaimTTL       = 10 * time.Minute
)

type platformAIModelService struct{}

type PlatformAIModelWorkspaceAggregate struct {
	GeneratedAt time.Time
	Provider    PlatformAIProviderState
	Models      []models.AIConfig
	Tenants     []PlatformAITenantWorkspaceItem
	Summary     PlatformAIModelSummary
}

type PlatformAIProviderState struct {
	Host                   string
	DefaultLLMModel        string
	AdminAPIKeyConfigured  bool
	AdminAPIKeyFingerprint string
	UpdatedAt              *time.Time
}

type PlatformAITenantWorkspaceItem struct {
	Tenant         models.Tenant
	Account        *models.Sub2APITenantAccount
	DefaultKey     string
	HasAccessToken bool
	NeedsAttention bool
}

type PlatformAIModelSummary struct {
	TotalTenants      int64
	ConfiguredTenants int64
	ActiveTenants     int64
	AttentionTenants  int64
	ActiveKeys        int64
	ModelCount        int64
}

type PlatformAIUsageAggregate struct {
	GeneratedAt time.Time
	Tenant      PlatformAITenantWorkspaceItem
	Stats       *providers.Sub2APIUsageStatsResponse
	Models      *providers.Sub2APIUsageDashboardModelsResponse
	RawUsage    []byte
}

type PlatformAITenantBillingAggregate struct {
	GeneratedAt time.Time
	Tenant      PlatformAITenantWorkspaceItem
	RemoteUser  *providers.Sub2APIAdminUser
	Records     []models.Sub2APIRechargeRecord
}

type PlatformSub2APIUserListAggregate struct {
	GeneratedAt time.Time
	Users       []providers.Sub2APIAdminUser
	Total       int64
	Page        int
	PageSize    int
	Pages       int
}

type PlatformSub2APIUserKeyListAggregate struct {
	GeneratedAt time.Time
	UserID      int64
	Keys        []providers.Sub2APIKey
	Total       int64
	Page        int
	PageSize    int
	Pages       int
}

type platformSecretValue struct {
	Ciphertext  string `json:"ciphertext"`
	Fingerprint string `json:"fingerprint"`
	UpdatedAt   string `json:"updated_at"`
}

type sub2APIUserResolution struct {
	UserID  int64
	Login   *providers.Sub2APILoginResponse
	Created bool
}

func (s *platformAIModelService) GetWorkspace() (*PlatformAIModelWorkspaceAggregate, error) {
	tenants, _, err := repositories.PlatformIAMRepository.FindTenantPage(sqls.DB(), repositories.PlatformTenantFilter{Lifecycle: "all"}, 1, 200)
	if err != nil {
		return nil, err
	}
	accounts, err := repositories.PlatformIAMRepository.FindSub2APIAccounts(sqls.DB(), 0)
	if err != nil {
		return nil, err
	}
	accountByTenantID := make(map[int64]*models.Sub2APITenantAccount, len(accounts))
	for i := range accounts {
		item := accounts[i]
		accountByTenantID[item.TenantID] = &item
	}
	modelsList := AIConfigService.Find(sqls.NewCnd().Desc("sort_no").Desc("id"))
	items := make([]PlatformAITenantWorkspaceItem, 0, len(tenants))
	var configuredTenants, activeTenants, attentionTenants, activeKeys int64
	for i := range tenants {
		tenant := tenants[i]
		if !tenant.IsAIEnabled() {
			continue
		}
		account := accountByTenantID[tenant.ID]
		hasAccessToken := account != nil && account.AccessTokenExpiresAt != nil && account.AccessTokenExpiresAt.After(time.Now())
		needsAttention := sub2APIAccountNeedsAttention(account)
		if account != nil {
			configuredTenants++
			if strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") {
				activeTenants++
			}
			if strings.TrimSpace(account.DefaultKeyCiphertext) != "" {
				activeKeys++
			}
			if needsAttention {
				attentionTenants++
			}
		} else {
			attentionTenants++
		}
		items = append(items, PlatformAITenantWorkspaceItem{
			Tenant:         tenant,
			Account:        account,
			DefaultKey:     platformTenantDefaultKey(account),
			HasAccessToken: hasAccessToken,
			NeedsAttention: needsAttention,
		})
	}
	host, adminKey, adminFingerprint, updatedAt, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	return &PlatformAIModelWorkspaceAggregate{
		GeneratedAt: time.Now(),
		Provider: PlatformAIProviderState{
			Host:                   host,
			DefaultLLMModel:        s.resolveDefaultLLMModel(),
			AdminAPIKeyConfigured:  adminKey != "",
			AdminAPIKeyFingerprint: adminFingerprint,
			UpdatedAt:              updatedAt,
		},
		Models:  modelsList,
		Tenants: items,
		Summary: PlatformAIModelSummary{
			TotalTenants:      int64(len(items)),
			ConfiguredTenants: configuredTenants,
			ActiveTenants:     activeTenants,
			AttentionTenants:  attentionTenants,
			ActiveKeys:        activeKeys,
			ModelCount:        int64(len(modelsList)),
		},
	}, nil
}

func (s *platformAIModelService) ListSub2APIUsers(ctx context.Context, query request.PlatformSub2APIUsersQueryRequest) (*PlatformSub2APIUserListAggregate, error) {
	query = normalizePlatformSub2APIUsersQuery(query)
	accounts, err := repositories.PlatformIAMRepository.FindSub2APIAccounts(sqls.DB(), 0)
	if err != nil {
		return nil, err
	}
	allowedUserIDs := platformSub2APIUserIDs(accounts)
	if len(allowedUserIDs) == 0 {
		return &PlatformSub2APIUserListAggregate{
			GeneratedAt: time.Now(),
			Users:       make([]providers.Sub2APIAdminUser, 0),
			Page:        query.Page,
			PageSize:    query.PageSize,
		}, nil
	}

	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	if strings.TrimSpace(adminAPIKey) == "" {
		return nil, errors.New("sub2api admin api key is required")
	}
	provider := s.providerFromConfig(host, adminAPIKey)
	users := make([]providers.Sub2APIAdminUser, 0, len(allowedUserIDs))
	for remotePage := 1; ; remotePage++ {
		resp, listErr := provider.AdminListUsers(ctx, adminAPIKey, providers.Sub2APIAdminListUsersQuery{
			Page:                 remotePage,
			PageSize:             100,
			Status:               strings.TrimSpace(query.Status),
			Role:                 strings.TrimSpace(query.Role),
			Search:               strings.TrimSpace(query.Search),
			IncludeSubscriptions: true,
			SortBy:               strings.TrimSpace(query.SortBy),
			SortOrder:            strings.TrimSpace(query.SortOrder),
			Timezone:             defaultTimezone(query.Timezone),
		})
		if listErr != nil {
			return nil, listErr
		}
		users = append(users, filterPlatformSub2APIUsers(resp.Data.Items, allowedUserIDs)...)
		if len(users) >= len(allowedUserIDs) || remotePage >= resp.Data.Pages || len(resp.Data.Items) == 0 {
			break
		}
	}

	pageUsers, total, pages := paginatePlatformSub2APIUsers(users, query.Page, query.PageSize)
	return &PlatformSub2APIUserListAggregate{
		GeneratedAt: time.Now(),
		Users:       pageUsers,
		Total:       total,
		Page:        query.Page,
		PageSize:    query.PageSize,
		Pages:       pages,
	}, nil
}

func platformSub2APIUserIDs(accounts []models.Sub2APITenantAccount) map[int64]struct{} {
	userIDs := make(map[int64]struct{}, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if account.Status == enums.StatusDeleted {
			continue
		}
		if userID := parseInt64(account.Sub2APIAccountID); userID > 0 {
			userIDs[userID] = struct{}{}
		}
	}
	return userIDs
}

func filterPlatformSub2APIUsers(users []providers.Sub2APIAdminUser, allowedUserIDs map[int64]struct{}) []providers.Sub2APIAdminUser {
	filtered := make([]providers.Sub2APIAdminUser, 0, len(users))
	for i := range users {
		if _, ok := allowedUserIDs[users[i].ID]; ok {
			filtered = append(filtered, users[i])
		}
	}
	return filtered
}

func paginatePlatformSub2APIUsers(users []providers.Sub2APIAdminUser, page, pageSize int) ([]providers.Sub2APIAdminUser, int64, int) {
	total := len(users)
	pages := 0
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start >= total {
		return make([]providers.Sub2APIAdminUser, 0), int64(total), pages
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return users[start:end], int64(total), pages
}

func normalizePlatformSub2APIUsersQuery(query request.PlatformSub2APIUsersQueryRequest) request.PlatformSub2APIUsersQueryRequest {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 100
	}
	if strings.TrimSpace(query.SortBy) == "" {
		query.SortBy = "created_at"
	}
	if strings.TrimSpace(query.SortOrder) == "" {
		query.SortOrder = "desc"
	}
	return query
}

func (s *platformAIModelService) ListSub2APIUserKeys(ctx context.Context, userID int64, timezone string) (*PlatformSub2APIUserKeyListAggregate, error) {
	if userID <= 0 {
		return nil, errors.New("sub2api user id is required")
	}
	accounts, err := repositories.PlatformIAMRepository.FindSub2APIAccounts(sqls.DB(), 0)
	if err != nil {
		return nil, err
	}
	if _, ok := platformSub2APIUserIDs(accounts)[userID]; !ok {
		return nil, errorsx.Forbidden("sub2api account is not bound to this platform")
	}
	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	if strings.TrimSpace(adminAPIKey) == "" {
		return nil, errors.New("sub2api admin api key is required")
	}
	provider := s.providerFromConfig(host, adminAPIKey)
	resp, err := provider.AdminListUserKeys(ctx, adminAPIKey, userID, defaultTimezone(timezone))
	if err != nil {
		return nil, err
	}
	return &PlatformSub2APIUserKeyListAggregate{
		GeneratedAt: time.Now(),
		UserID:      userID,
		Keys:        resp.Data.Items,
		Total:       resp.Data.Total,
		Page:        resp.Data.Page,
		PageSize:    resp.Data.PageSize,
		Pages:       resp.Data.Pages,
	}, nil
}

func (s *platformAIModelService) UpdateProvider(req request.PlatformAIProviderSaveRequest, operator *dto.AuthPrincipal) error {
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return errors.New("sub2api host is required")
	}
	if err := repositories.SystemConfigRepository.SaveByKey(sqls.DB(), &models.SystemConfig{
		ConfigKey:   platformSub2APIHostConfigKey,
		ConfigValue: host,
		GroupCode:   "platform_ai",
		Title:       "Sub2API Host",
		Description: "Platform-side Sub2API host",
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}); err != nil {
		return err
	}
	if strings.TrimSpace(req.AdminAPIKey) != "" {
		if err := s.saveAdminAPIKey(strings.TrimSpace(req.AdminAPIKey), operator); err != nil {
			return err
		}
	}
	if strings.TrimSpace(req.TranslationAPIKey) != "" {
		if err := s.saveTranslationAPIKey(strings.TrimSpace(req.TranslationAPIKey), operator); err != nil {
			return err
		}
	}
	if modelName := strings.TrimSpace(req.DefaultLLMModel); modelName != "" {
		if err := repositories.SystemConfigRepository.SaveByKey(sqls.DB(), &models.SystemConfig{
			ConfigKey:   platformSub2APILLMModelConfigKey,
			ConfigValue: modelName,
			GroupCode:   "platform_ai",
			Title:       "Sub2API Default LLM Model",
			Description: "Default chat model used by the unified AI capability router",
			Status:      enums.StatusOk,
			AuditFields: utils.BuildAuditFields(operator),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *platformAIModelService) resolveDefaultLLMModel() string {
	if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APILLMModelConfigKey); item != nil {
		if value := strings.TrimSpace(item.ConfigValue); value != "" {
			return value
		}
	}
	return strings.TrimSpace(config.CurrentOrDefault().Sub2API.DefaultModel)
}

func (s *platformAIModelService) ProvisionTenant(req request.PlatformAITenantProvisionRequest, operator *dto.AuthPrincipal) (ret *models.Sub2APITenantAccount, retErr error) {
	if operator == nil {
		return nil, errors.New("operator is required")
	}
	defaultKeyExpiresAt, defaultKeyExpiresAtValue, err := normalizeSub2APIKeyExpiresAt(req.DefaultKeyExpiresAt)
	if err != nil {
		return nil, err
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	if strings.TrimSpace(adminAPIKey) == "" {
		return nil, errors.New("sub2api admin api key is required")
	}
	loginEmail := tenantEmail(tenant.ID)
	loginPassword := platformSub2APIAccountPassword
	loginPasswordCiphertext, err := secretstore.Encrypt(loginPassword)
	if err != nil {
		return nil, err
	}
	loginPasswordFingerprint := secretstore.Fingerprint(loginPassword)
	now := time.Now()
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenant.ID)
	provisionClaimed := false
	defer func() {
		if retErr != nil && provisionClaimed {
			_ = s.markProvisionFailed(account, retErr, operator)
		}
	}()
	if account == nil {
		account = &models.Sub2APITenantAccount{
			TenantID:                 tenant.ID,
			AccountName:              tenant.Name,
			LoginEmail:               loginEmail,
			LoginPasswordCiphertext:  loginPasswordCiphertext,
			LoginPasswordFingerprint: loginPasswordFingerprint,
			Concurrency:              max64(req.Concurrency, 0),
			Balance:                  req.Balance,
			RPMLimit:                 req.RPMLimit,
			ProvisionStatus:          "provisioning_user",
			AccountStatus:            "unknown",
			DefaultKeyStatus:         "unknown",
			DefaultLLMModel:          s.resolveDefaultLLMModel(),
			DashboardURL:             host,
			QuotaSnapshotJSON:        "{}",
			AuditFields:              utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreateSub2APIAccount(sqls.DB(), account); err != nil {
			if current := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenant.ID); current != nil {
				return current, errorsx.BusinessError(41, "该租户的模型服务配置正在更新，请稍后刷新")
			}
			return nil, err
		}
		provisionClaimed = true
	} else {
		account.AccountName = tenant.Name
		account.LoginEmail = loginEmail
		account.LoginPasswordCiphertext = loginPasswordCiphertext
		account.LoginPasswordFingerprint = loginPasswordFingerprint
		account.Concurrency = max64(req.Concurrency, account.Concurrency)
		if req.Balance > 0 {
			account.Balance = req.Balance
		}
		if req.RPMLimit >= 0 {
			account.RPMLimit = req.RPMLimit
		}
		account.DashboardURL = host
		account.ProvisionStatus = "provisioning_user"
		account.ProvisionMessage = ""
		account.UpdatedAt = now
		claimed, err := repositories.PlatformIAMRepository.ClaimSub2APIAccountProvision(sqls.DB(), account.ID, now.Add(-platformSub2APIProvisionClaimTTL), map[string]any{
			"account_name":               account.AccountName,
			"login_email":                account.LoginEmail,
			"login_password_ciphertext":  account.LoginPasswordCiphertext,
			"login_password_fingerprint": account.LoginPasswordFingerprint,
			"concurrency":                account.Concurrency,
			"balance":                    account.Balance,
			"rpm_limit":                  account.RPMLimit,
			"provision_status":           account.ProvisionStatus,
			"provision_message":          account.ProvisionMessage,
			"dashboard_url":              account.DashboardURL,
			"updated_at":                 now,
			"update_user_id":             operator.UserID,
			"update_user_name":           operator.Username,
		})
		if err != nil {
			return nil, err
		}
		if !claimed {
			return account, errorsx.BusinessError(41, "该租户的模型服务配置正在更新，请稍后刷新")
		}
		provisionClaimed = true
		account = repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), account.ID)
	}
	sub2API := s.providerFromConfig(host, adminAPIKey)
	userResolution, err := s.ensureSub2APIUser(context.Background(), sub2API, adminAPIKey, account, tenant, operator)
	if err != nil {
		_ = s.markProvisionFailed(account, err, operator)
		return account, err
	}
	userID := userResolution.UserID
	loginResp := userResolution.Login
	if userResolution.Created {
		clearSub2APIDefaultKey(account)
	}
	account.Sub2APIAccountID = fmt.Sprintf("%d", userID)
	account.AccountStatus = loginResp.Data.User.Status
	account.ProvisionStatus = "provisioning_key"
	account.AccessTokenExpiresAt = tokenExpiryFromLogin(loginResp.Data.ExpiresIn)
	account.LastSyncedAt = &now
	accountUpdates := map[string]any{
		"sub2_api_account_id":     account.Sub2APIAccountID,
		"account_status":          account.AccountStatus,
		"provision_status":        account.ProvisionStatus,
		"access_token_expires_at": account.AccessTokenExpiresAt,
		"last_synced_at":          now,
		"updated_at":              now,
		"update_user_id":          operator.UserID,
		"update_user_name":        operator.Username,
	}
	if userResolution.Created {
		accountUpdates["default_key_id"] = account.DefaultKeyID
		accountUpdates["default_key_name"] = account.DefaultKeyName
		accountUpdates["default_key_group_id"] = account.DefaultKeyGroupID
		accountUpdates["default_key_quota"] = account.DefaultKeyQuota
		accountUpdates["default_key_quota_used"] = account.DefaultKeyQuotaUsed
		accountUpdates["default_key_ciphertext"] = account.DefaultKeyCiphertext
		accountUpdates["default_key_fingerprint"] = account.DefaultKeyFingerprint
		accountUpdates["default_key_status"] = account.DefaultKeyStatus
		accountUpdates["default_key_expires_at"] = account.DefaultKeyExpiresAt
		accountUpdates["quota_snapshot_json"] = account.QuotaSnapshotJSON
	}
	if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, accountUpdates); err != nil {
		return nil, err
	}
	token := loginResp.Data.AccessToken
	if token == "" {
		token, loginResp, err = s.loginAndCacheToken(context.Background(), sub2API, account, operator)
		if err != nil {
			return account, err
		}
	}
	desiredQuota := normalizedQuota(account.Balance)
	defaultKeyName := defaultSub2APIKeyName(tenant.Name)
	key, err := s.ensureRemoteTenantDefaultKey(
		context.Background(),
		sub2API,
		token,
		account,
		defaultKeyName,
		desiredQuota,
		defaultKeyExpiresAt,
		defaultKeyExpiresAtValue,
	)
	if err != nil {
		return account, err
	}
	keyCiphertext, keyFingerprint, err := resolveTenantDefaultKeySecret(account, key, defaultKeyName)
	if err != nil {
		return account, err
	}
	account.DefaultKeyID = fmt.Sprintf("%d", key.ID)
	account.DefaultKeyName = key.Name
	account.DefaultKeyGroupID = key.GroupID
	account.DefaultKeyQuota = key.Quota
	account.DefaultKeyQuotaUsed = key.QuotaUsed
	account.DefaultKeyCiphertext = keyCiphertext
	account.DefaultKeyFingerprint = keyFingerprint
	account.DefaultKeyStatus = key.Status
	account.DefaultKeyExpiresAt = firstTimePtr(parseSub2APIOptionalTimePtrFromPointer(key.ExpiresAt), defaultKeyExpiresAt)
	account.ProvisionStatus = "active"
	account.ProvisionMessage = ""
	account.LastProvisionedAt = &now
	account.AccessTokenExpiresAt = tokenExpiryFromLogin(loginResp.Data.ExpiresIn)
	snapshot, _ := json.Marshal(map[string]any{
		"user_id":          userID,
		"login_email":      loginEmail,
		"key_id":           account.DefaultKeyID,
		"default_key_name": account.DefaultKeyName,
		"group_id":         account.DefaultKeyGroupID,
		"quota":            account.DefaultKeyQuota,
		"expires_at":       formatOptionalTime(account.DefaultKeyExpiresAt),
		"rpm_limit":        account.RPMLimit,
		"concurrency":      account.Concurrency,
		"balance":          account.Balance,
	})
	account.QuotaSnapshotJSON = string(snapshot)
	if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"sub2_api_account_id":     account.Sub2APIAccountID,
		"account_status":          account.AccountStatus,
		"provision_status":        account.ProvisionStatus,
		"provision_message":       account.ProvisionMessage,
		"last_provisioned_at":     account.LastProvisionedAt,
		"access_token_expires_at": account.AccessTokenExpiresAt,
		"default_key_id":          account.DefaultKeyID,
		"default_key_name":        account.DefaultKeyName,
		"default_key_group_id":    account.DefaultKeyGroupID,
		"default_key_quota":       account.DefaultKeyQuota,
		"default_key_quota_used":  account.DefaultKeyQuotaUsed,
		"default_key_ciphertext":  account.DefaultKeyCiphertext,
		"default_key_fingerprint": account.DefaultKeyFingerprint,
		"default_key_status":      account.DefaultKeyStatus,
		"default_key_expires_at":  account.DefaultKeyExpiresAt,
		"quota_snapshot_json":     account.QuotaSnapshotJSON,
		"last_synced_at":          now,
		"updated_at":              now,
		"update_user_id":          operator.UserID,
		"update_user_name":        operator.Username,
	}); err != nil {
		return nil, err
	}
	return repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), account.ID), nil
}

func (s *platformAIModelService) GetTenantUsage(tenantID int64, query request.PlatformAIUsageQueryRequest) (*PlatformAIUsageAggregate, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID)
	if account == nil || strings.TrimSpace(account.DefaultKeyCiphertext) == "" {
		return nil, errors.New("tenant default key is not configured")
	}
	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	provider := s.providerFromConfig(host, adminAPIKey)
	token, _, err := s.ensureAccessToken(context.Background(), provider, account, nil)
	if err != nil {
		return nil, err
	}
	modelsResp, err := provider.UsageDashboardModels(context.Background(), token, providers.Sub2APIUsageQuery{
		StartDate:   query.StartDate,
		EndDate:     query.EndDate,
		ModelSource: query.ModelSource,
		Timezone:    query.Timezone,
	})
	if err != nil {
		return nil, err
	}
	statsResp, err := provider.UsageStats(context.Background(), token, providers.Sub2APIUsageQuery{
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
		Timezone:  query.Timezone,
	})
	if err != nil {
		return nil, err
	}
	rawResp, err := provider.UsageList(context.Background(), token, providers.Sub2APIUsageQuery{
		StartDate:   query.StartDate,
		EndDate:     query.EndDate,
		ModelSource: query.ModelSource,
		Timezone:    query.Timezone,
		Page:        query.Page,
		PageSize:    query.PageSize,
		SortBy:      query.SortBy,
		SortOrder:   query.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(rawResp.Data)
	return &PlatformAIUsageAggregate{
		GeneratedAt: time.Now(),
		Tenant: PlatformAITenantWorkspaceItem{
			Tenant:     *tenant,
			Account:    account,
			DefaultKey: platformTenantDefaultKey(account),
		},
		Stats:    statsResp,
		Models:   modelsResp,
		RawUsage: raw,
	}, nil
}

func (s *platformAIModelService) GetTenantBilling(tenantID int64, timezone string, operator *dto.AuthPrincipal) (*PlatformAITenantBillingAggregate, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID)
	if account == nil {
		return nil, errors.New("tenant sub2api account is not configured")
	}
	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	if strings.TrimSpace(adminAPIKey) == "" {
		return nil, errors.New("sub2api admin api key is required")
	}
	provider := s.providerFromConfig(host, adminAPIKey)
	remoteUser, err := s.syncSub2APIAccountBalance(context.Background(), provider, adminAPIKey, account, timezone, operator)
	if err != nil {
		return nil, err
	}
	account = repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), account.ID)
	records, err := repositories.PlatformIAMRepository.FindSub2APIRechargeRecords(sqls.DB(), tenantID, 20)
	if err != nil {
		return nil, err
	}
	return &PlatformAITenantBillingAggregate{
		GeneratedAt: time.Now(),
		Tenant: PlatformAITenantWorkspaceItem{
			Tenant:         *tenant,
			Account:        account,
			DefaultKey:     platformTenantDefaultKey(account),
			HasAccessToken: account != nil && account.AccessTokenExpiresAt != nil && account.AccessTokenExpiresAt.After(time.Now()),
			NeedsAttention: sub2APIAccountNeedsAttention(account),
		},
		RemoteUser: remoteUser,
		Records:    records,
	}, nil
}

func (s *platformAIModelService) RechargeTenant(req request.PlatformAITenantRechargeRequest, operator *dto.AuthPrincipal) (*PlatformAITenantBillingAggregate, error) {
	if req.TenantID <= 0 {
		return nil, errors.New("tenantId is required")
	}
	if req.Amount <= 0 {
		return nil, errors.New("recharge amount must be greater than 0")
	}
	operation := strings.ToLower(strings.TrimSpace(req.Operation))
	if operation == "" {
		operation = "add"
	}
	if operation != "add" {
		return nil, errors.New("only add balance operation is supported")
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), req.TenantID)
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), req.TenantID)
	if account == nil {
		return nil, errors.New("tenant sub2api account is not configured")
	}
	host, adminAPIKey, _, _, err := s.resolveProviderConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("sub2api host is required")
	}
	if strings.TrimSpace(adminAPIKey) == "" {
		return nil, errors.New("sub2api admin api key is required")
	}
	provider := s.providerFromConfig(host, adminAPIKey)
	beforeUser, err := s.syncSub2APIAccountBalance(context.Background(), provider, adminAPIKey, account, req.Timezone, operator)
	if err != nil {
		return nil, err
	}
	remoteUserID := beforeUser.ID
	if remoteUserID <= 0 {
		remoteUserID = parseInt64(account.Sub2APIAccountID)
	}
	if remoteUserID <= 0 {
		return nil, errors.New("sub2api account id is required before recharge")
	}
	balanceBefore := beforeUser.Balance
	resp, err := provider.AdminUpdateUserBalance(context.Background(), adminAPIKey, remoteUserID, providers.Sub2APIAdminUpdateBalanceRequest{
		Balance:   req.Amount,
		Operation: operation,
		Notes:     strings.TrimSpace(req.Notes),
	})
	if err != nil {
		_ = s.createSub2APIRechargeRecord(account, remoteUserID, operation, req.Amount, balanceBefore, balanceBefore, strings.TrimSpace(req.Notes), "failed", err.Error(), operator)
		return nil, err
	}
	afterUser := &resp.Data
	if err := s.updateSub2APIAccountFromAdminUser(account, afterUser, operator); err != nil {
		return nil, err
	}
	if err := s.createSub2APIRechargeRecord(account, afterUser.ID, operation, req.Amount, balanceBefore, afterUser.Balance, strings.TrimSpace(req.Notes), "success", resp.Message, operator); err != nil {
		return nil, err
	}
	return s.GetTenantBilling(req.TenantID, req.Timezone, operator)
}

func (s *platformAIModelService) providerFromConfig(host, adminKey string) providers.Sub2APIProvider {
	cfg := config.CurrentOrDefault().Sub2API
	cfg.BaseURL = host
	cfg.DefaultKey = adminKey
	return providers.NewSub2APIProvider(&cfg)
}

func (s *platformAIModelService) resolveProviderConfig() (string, string, string, *time.Time, error) {
	providerConfig := config.CurrentOrDefault().Sub2API
	host := strings.TrimSpace(providerConfig.BaseURL)
	adminKey := firstNonBlank(providerConfig.AdminAPIKey, providerConfig.DefaultKey)
	fingerprint := ""
	if adminKey != "" {
		fingerprint = secretstore.Fingerprint(adminKey)
	}
	var updatedAt *time.Time
	if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APIHostConfigKey); item != nil && strings.TrimSpace(item.ConfigValue) != "" {
		host = strings.TrimSpace(item.ConfigValue)
		if item.UpdatedAt.IsZero() {
			updatedAt = nil
		} else {
			value := item.UpdatedAt
			updatedAt = &value
		}
	}
	if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APIAdminKeyConfigKey); item != nil && strings.TrimSpace(item.ConfigValue) != "" {
		payload := platformSecretValue{}
		if err := json.Unmarshal([]byte(item.ConfigValue), &payload); err == nil {
			fingerprint = strings.TrimSpace(payload.Fingerprint)
			decrypted, err := secretstore.Decrypt(payload.Ciphertext)
			if err != nil {
				return "", "", "", nil, err
			}
			adminKey = strings.TrimSpace(decrypted)
		} else {
			decrypted, err := secretstore.Decrypt(item.ConfigValue)
			if err != nil {
				return "", "", "", nil, err
			}
			adminKey = strings.TrimSpace(decrypted)
		}
		if item.UpdatedAt.IsZero() {
			updatedAt = nil
		} else {
			value := item.UpdatedAt
			updatedAt = &value
		}
		if fingerprint == "" {
			fingerprint = strings.TrimSpace(item.Description)
		}
	}
	if host == "" {
		return "", adminKey, fingerprint, updatedAt, nil
	}
	return host, adminKey, fingerprint, updatedAt, nil
}

func (s *platformAIModelService) saveAdminAPIKey(adminAPIKey string, operator *dto.AuthPrincipal) error {
	ciphertext, err := secretstore.Encrypt(adminAPIKey)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(platformSecretValue{
		Ciphertext:  ciphertext,
		Fingerprint: secretstore.Fingerprint(adminAPIKey),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	})
	return repositories.SystemConfigRepository.SaveByKey(sqls.DB(), &models.SystemConfig{
		ConfigKey:   platformSub2APIAdminKeyConfigKey,
		ConfigValue: string(payload),
		GroupCode:   "platform_ai",
		Title:       "Sub2API Admin API Key",
		Description: secretstore.Fingerprint(adminAPIKey),
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	})
}

func (s *platformAIModelService) saveTranslationAPIKey(apiKey string, operator *dto.AuthPrincipal) error {
	ciphertext, err := secretstore.Encrypt(apiKey)
	if err != nil {
		return err
	}
	fingerprint := secretstore.Fingerprint(apiKey)
	payload, _ := json.Marshal(platformSecretValue{
		Ciphertext:  ciphertext,
		Fingerprint: fingerprint,
		UpdatedAt:   time.Now().Format(time.RFC3339),
	})
	return repositories.SystemConfigRepository.SaveByKey(sqls.DB(), &models.SystemConfig{
		ConfigKey:   platformSub2APITranslationKeyConfigKey,
		ConfigValue: string(payload),
		GroupCode:   "platform_ai",
		Title:       "Sub2API Translation API Key",
		Description: fingerprint,
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	})
}

func (s *platformAIModelService) syncSub2APIAccountBalance(ctx context.Context, provider providers.Sub2APIProvider, adminAPIKey string, account *models.Sub2APITenantAccount, timezone string, operator *dto.AuthPrincipal) (*providers.Sub2APIAdminUser, error) {
	if account == nil {
		return nil, errors.New("tenant sub2api account is not configured")
	}
	query := providers.Sub2APIAdminListUsersQuery{
		Page:                 1,
		PageSize:             20,
		Search:               account.LoginEmail,
		IncludeSubscriptions: true,
		SortBy:               "created_at",
		SortOrder:            "desc",
		Timezone:             defaultTimezone(timezone),
	}
	resp, err := provider.AdminListUsers(ctx, adminAPIKey, query)
	if err == nil {
		if user := pickSub2APIAdminUser(resp.Data.Items, account); user != nil {
			if err := s.updateSub2APIAccountFromAdminUser(account, user, operator); err != nil {
				return nil, err
			}
			return user, nil
		}
	}
	if err != nil {
		slog.Warn("sub2api admin user sync failed, fallback to auth/me", "tenantId", account.TenantID, "error", err)
	}
	token, _, tokenErr := s.ensureAccessToken(ctx, provider, account, operator)
	if tokenErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, tokenErr
	}
	meResp, meErr := provider.AuthMe(ctx, token, defaultTimezone(timezone))
	if meErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, meErr
	}
	user := sub2APIUserToAdminUser(meResp.Data)
	if err := s.updateSub2APIAccountFromAdminUser(account, user, operator); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *platformAIModelService) updateSub2APIAccountFromAdminUser(account *models.Sub2APITenantAccount, user *providers.Sub2APIAdminUser, operator *dto.AuthPrincipal) error {
	if account == nil || user == nil || user.ID <= 0 {
		return nil
	}
	audit := utils.BuildAuditFields(operator)
	now := time.Now()
	account.Sub2APIAccountID = fmt.Sprintf("%d", user.ID)
	account.AccountStatus = strings.TrimSpace(user.Status)
	account.Balance = user.Balance
	account.Concurrency = user.Concurrency
	account.RPMLimit = user.RPMLimit
	account.LastSyncedAt = &now
	return repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"sub2_api_account_id": account.Sub2APIAccountID,
		"account_status":      account.AccountStatus,
		"balance":             account.Balance,
		"concurrency":         account.Concurrency,
		"rpm_limit":           account.RPMLimit,
		"last_synced_at":      now,
		"updated_at":          now,
		"update_user_id":      audit.UpdateUserID,
		"update_user_name":    audit.UpdateUserName,
	})
}

func (s *platformAIModelService) createSub2APIRechargeRecord(account *models.Sub2APITenantAccount, remoteUserID int64, operation string, amount, balanceBefore, balanceAfter float64, notes, status, message string, operator *dto.AuthPrincipal) error {
	if account == nil {
		return nil
	}
	audit := utils.BuildAuditFields(operator)
	now := time.Now()
	return repositories.PlatformIAMRepository.CreateSub2APIRechargeRecord(sqls.DB(), &models.Sub2APIRechargeRecord{
		TenantID:         account.TenantID,
		Sub2APIAccountID: account.Sub2APIAccountID,
		RemoteUserID:     remoteUserID,
		Operation:        operation,
		Amount:           amount,
		BalanceBefore:    balanceBefore,
		BalanceAfter:     balanceAfter,
		Notes:            notes,
		Status:           status,
		Message:          message,
		OccurredAt:       now,
		AuditFields:      audit,
	})
}

func (s *platformAIModelService) ensureSub2APIUser(ctx context.Context, provider providers.Sub2APIProvider, adminAPIKey string, account *models.Sub2APITenantAccount, tenant *models.Tenant, operator *dto.AuthPrincipal) (*sub2APIUserResolution, error) {
	if account.Sub2APIAccountID != "" {
		_, loginResp, err := s.loginAndCacheToken(ctx, provider, account, operator)
		if err == nil {
			return &sub2APIUserResolution{UserID: loginResp.Data.User.ID, Login: loginResp}, nil
		}
		if !isSub2APIInvalidCredentialsError(err) {
			return nil, err
		}
		remoteUser, lookupErr := findSub2APIUserByEmail(ctx, provider, adminAPIKey, account.LoginEmail)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if remoteUser != nil {
			return nil, sub2APICredentialConflictError(account.LoginEmail)
		}
		// The local mapping points at a user that no longer exists upstream.
		// Creating the derived tenant email is safe only after the exact lookup.
	}
	resp, err := provider.AdminCreateUser(ctx, adminAPIKey, providers.Sub2APIAdminCreateUserRequest{
		Email:       account.LoginEmail,
		Password:    platformSub2APIAccountPassword,
		Username:    tenant.Name,
		Notes:       "",
		Role:        "user",
		Concurrency: max64(account.Concurrency, 0),
		RPMLimit:    account.RPMLimit,
		Balance:     account.Balance,
	})
	if err != nil {
		if isSub2APIConflictError(err) {
			_, loginResp, loginErr := s.loginAndCacheToken(ctx, provider, account, operator)
			if loginErr != nil {
				if isSub2APIInvalidCredentialsError(loginErr) {
					return nil, sub2APICredentialConflictError(account.LoginEmail)
				}
				return nil, loginErr
			}
			return &sub2APIUserResolution{UserID: loginResp.Data.User.ID, Login: loginResp}, nil
		}
		return nil, err
	}
	_, loginResp, loginErr := s.loginAndCacheToken(ctx, provider, account, operator)
	if loginErr != nil {
		return nil, loginErr
	}
	userID := resp.Data.ID
	if userID <= 0 {
		userID = loginResp.Data.User.ID
	}
	if loginResp.Data.User.ID <= 0 {
		loginResp.Data.User.ID = userID
	}
	return &sub2APIUserResolution{UserID: userID, Login: loginResp, Created: true}, nil
}

func findSub2APIUserByEmail(ctx context.Context, provider providers.Sub2APIProvider, adminAPIKey, email string) (*providers.Sub2APIAdminUser, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	for page := 1; page <= 100; page++ {
		resp, err := provider.AdminListUsers(ctx, adminAPIKey, providers.Sub2APIAdminListUsersQuery{
			Page:                 page,
			PageSize:             100,
			Search:               email,
			IncludeSubscriptions: false,
			SortBy:               "created_at",
			SortOrder:            "desc",
			Timezone:             "Asia/Shanghai",
		})
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("sub2api admin user lookup returned an empty response")
		}
		for i := range resp.Data.Items {
			if strings.EqualFold(strings.TrimSpace(resp.Data.Items[i].Email), email) {
				return &resp.Data.Items[i], nil
			}
		}
		if len(resp.Data.Items) == 0 || resp.Data.Pages <= page {
			break
		}
	}
	return nil, nil
}

func sub2APICredentialConflictError(email string) error {
	return errorsx.BusinessError(41, fmt.Sprintf("Sub2API 账号 %s 已存在，但平台保存的登录凭据与远端不一致；请先在 Sub2API 管理端重置该账号密码，再重新修复模型服务", strings.TrimSpace(email)))
}

func clearSub2APIDefaultKey(account *models.Sub2APITenantAccount) {
	account.DefaultKeyID = ""
	account.DefaultKeyName = ""
	account.DefaultKeyGroupID = platformSub2APIDefaultKeyGroupID
	account.DefaultKeyQuota = 0
	account.DefaultKeyQuotaUsed = 0
	account.DefaultKeyCiphertext = ""
	account.DefaultKeyFingerprint = ""
	account.DefaultKeyStatus = "unknown"
	account.DefaultKeyExpiresAt = nil
	account.QuotaSnapshotJSON = "{}"
}

// The platform model-management screen is explicitly allowed to copy tenant model credentials.
func platformTenantDefaultKey(account *models.Sub2APITenantAccount) string {
	if account == nil || strings.TrimSpace(account.DefaultKeyCiphertext) == "" {
		return ""
	}
	plain, err := secretstore.Decrypt(account.DefaultKeyCiphertext)
	if err != nil {
		slog.Warn("failed to decrypt tenant model credential for platform workspace", "tenant_id", account.TenantID, "error", err)
		return ""
	}
	return strings.TrimSpace(plain)
}

func (s *platformAIModelService) loginAndCacheToken(ctx context.Context, provider providers.Sub2APIProvider, account *models.Sub2APITenantAccount, operator *dto.AuthPrincipal) (string, *providers.Sub2APILoginResponse, error) {
	if !sub2APIAccountHasLoginCredentials(account) {
		return "", nil, errorsx.BusinessError(41, "租户 Sub2API 登录凭据未配置，请重新开通或修复该租户的模型服务")
	}
	password, err := secretstore.Decrypt(account.LoginPasswordCiphertext)
	if err != nil {
		return "", nil, err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil, errorsx.BusinessError(41, "租户 Sub2API 登录凭据未配置，请重新开通或修复该租户的模型服务")
	}
	loginResp, err := provider.Login(ctx, providers.Sub2APILoginRequest{
		Email:    strings.TrimSpace(account.LoginEmail),
		Password: password,
	})
	if err != nil {
		return "", nil, err
	}
	if loginResp.Data.AccessToken == "" {
		return "", nil, errors.New("sub2api login did not return access token")
	}
	expiresIn := loginResp.Data.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	ttl := time.Duration(expiresIn) * time.Second
	if ttl > time.Minute {
		ttl -= time.Minute
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	cacheKey := sub2APITokenCacheKey(account.TenantID)
	audit := utils.BuildAuditFields(operator)
	if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"access_token_expires_at": tokenExpiryFromLogin(expiresIn),
		"last_synced_at":          time.Now(),
		"updated_at":              time.Now(),
		"update_user_id":          audit.UpdateUserID,
		"update_user_name":        audit.UpdateUserName,
	}); err != nil {
		return "", nil, err
	}
	if err := cache.SetJSON(ctx, cacheKey, map[string]any{
		"access_token":  loginResp.Data.AccessToken,
		"refresh_token": loginResp.Data.RefreshToken,
		"token_type":    loginResp.Data.TokenType,
		"expires_in":    expiresIn,
		"stored_at":     time.Now().UTC().Format(time.RFC3339),
	}, ttl); err != nil {
		slog.Warn("sub2api access token cache failed", "tenantId", account.TenantID, "error", err)
	}
	return loginResp.Data.AccessToken, loginResp, nil
}

func sub2APIAccountHasLoginCredentials(account *models.Sub2APITenantAccount) bool {
	return account != nil &&
		strings.TrimSpace(account.LoginEmail) != "" &&
		strings.TrimSpace(account.LoginPasswordCiphertext) != ""
}

func sub2APIAccountNeedsAttention(account *models.Sub2APITenantAccount) bool {
	return account == nil ||
		strings.TrimSpace(account.ProvisionStatus) != "active" ||
		strings.TrimSpace(account.DefaultKeyID) == "" ||
		strings.TrimSpace(account.DefaultKeyCiphertext) == "" ||
		!strings.EqualFold(strings.TrimSpace(account.DefaultKeyName), defaultSub2APIKeyName(account.AccountName)) ||
		account.DefaultKeyGroupID != platformSub2APIDefaultKeyGroupID ||
		!strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") ||
		!sub2APIAccountHasLoginCredentials(account)
}

func isSub2APIInvalidCredentialsError(err error) bool {
	var clientErr *providers.Sub2APIClientError
	if !errors.As(err, &clientErr) || clientErr.StatusCode != 401 {
		return false
	}
	reason := strings.ToUpper(strings.TrimSpace(clientErr.Reason))
	return reason == "" || reason == "INVALID_CREDENTIALS"
}

func (s *platformAIModelService) markProvisionFailed(account *models.Sub2APITenantAccount, err error, operator *dto.AuthPrincipal) error {
	if account == nil || err == nil {
		return nil
	}
	audit := utils.BuildAuditFields(operator)
	account.ProvisionStatus = "failed"
	account.ProvisionMessage = err.Error()
	return repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"provision_status":  account.ProvisionStatus,
		"provision_message": account.ProvisionMessage,
		"updated_at":        time.Now(),
		"update_user_id":    audit.UpdateUserID,
		"update_user_name":  audit.UpdateUserName,
	})
}

func (s *platformAIModelService) ensureAccessToken(ctx context.Context, provider providers.Sub2APIProvider, account *models.Sub2APITenantAccount, operator *dto.AuthPrincipal) (string, *providers.Sub2APILoginResponse, error) {
	var cached struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		StoredAt     string `json:"stored_at"`
	}
	if ok, err := cache.GetJSON(ctx, sub2APITokenCacheKey(account.TenantID), &cached); err == nil && ok && cached.AccessToken != "" {
		if account.AccessTokenExpiresAt == nil || account.AccessTokenExpiresAt.After(time.Now()) {
			return cached.AccessToken, &providers.Sub2APILoginResponse{
				Data: providers.Sub2APILoginData{
					AccessToken:  cached.AccessToken,
					RefreshToken: cached.RefreshToken,
					ExpiresIn:    cached.ExpiresIn,
					TokenType:    cached.TokenType,
				},
			}, nil
		}
	}
	return s.loginAndCacheToken(ctx, provider, account, operator)
}

func sub2APITokenCacheKey(tenantID int64) string {
	return fmt.Sprintf(platformSub2APITokenCachePrefix, tenantID)
}

func tokenExpiryFromLogin(expiresIn int64) *time.Time {
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	t := time.Now().Add(time.Duration(expiresIn) * time.Second)
	return &t
}

func tenantEmail(tenantID int64) string {
	return fmt.Sprintf("%s@remotedesk.com", strings.ToLower(tenantCode(tenantID)))
}

func tenantCode(tenantID int64) string {
	return fmt.Sprintf("T-%06d", tenantID)
}

func defaultTimezone(timezone string) string {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return "Asia/Shanghai"
	}
	return timezone
}

func defaultSub2APIKeyName(tenantName string) string {
	return strings.TrimSpace(tenantName) + "默认key"
}

func (s *platformAIModelService) ensureRemoteTenantDefaultKey(
	ctx context.Context,
	provider providers.Sub2APIProvider,
	token string,
	account *models.Sub2APITenantAccount,
	defaultKeyName string,
	desiredQuota float64,
	desiredExpiresAt *time.Time,
	desiredExpiresAtValue string,
) (*providers.Sub2APIKey, error) {
	keys, err := listAllTenantSub2APIKeys(ctx, provider, token)
	if err != nil {
		return nil, err
	}
	current, duplicateCount := selectTenantDefaultSub2APIKey(account, keys, defaultKeyName, desiredQuota, desiredExpiresAt)
	if duplicateCount > 1 {
		slog.Warn("duplicate tenant default sub2api keys detected", "tenantId", account.TenantID, "name", defaultKeyName, "count", duplicateCount)
	}
	if current == nil {
		created, createErr := provider.CreateKey(ctx, token, providers.Sub2APICreateKeyRequest{
			Name:      defaultKeyName,
			GroupID:   platformSub2APIDefaultKeyGroupID,
			Quota:     desiredQuota,
			ExpiresAt: desiredExpiresAtValue,
		})
		if createErr != nil {
			return nil, createErr
		}
		if created == nil || created.Data.ID <= 0 {
			return nil, errors.New("sub2api default key creation returned an empty key")
		}
		return &created.Data, nil
	}
	if remoteTenantDefaultKeyMatches(current, defaultKeyName, desiredQuota, desiredExpiresAt) {
		return current, nil
	}
	ipWhitelist := stringSliceFromAny(current.IPWhitelist)
	ipBlacklist := stringSliceFromAny(current.IPBlacklist)
	updated, err := provider.UpdateKey(ctx, token, current.ID, providers.Sub2APIUpdateKeyRequest{
		Name:        defaultKeyName,
		GroupID:     platformSub2APIDefaultKeyGroupID,
		IPWhitelist: &ipWhitelist,
		IPBlacklist: &ipBlacklist,
		Quota:       &desiredQuota,
		ExpiresAt:   desiredExpiresAtValue,
		RateLimit5h: current.RateLimit5h,
		RateLimit1d: current.RateLimit1d,
		RateLimit7d: current.RateLimit7d,
		Status:      "active",
	})
	if err != nil {
		return nil, err
	}
	if updated == nil || updated.Data.ID <= 0 {
		return nil, errors.New("sub2api default key update returned an empty key")
	}
	if strings.TrimSpace(updated.Data.Key) == "" {
		updated.Data.Key = current.Key
	}
	return &updated.Data, nil
}

func listAllTenantSub2APIKeys(ctx context.Context, provider providers.Sub2APIProvider, token string) ([]providers.Sub2APIKey, error) {
	items := make([]providers.Sub2APIKey, 0)
	for page := 1; page <= 100; page++ {
		response, err := provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
			Page:      page,
			PageSize:  100,
			SortBy:    "created_at",
			SortOrder: "desc",
			Timezone:  defaultTimezone(""),
		})
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, errors.New("sub2api key list returned an empty response")
		}
		items = append(items, response.Data.Items...)
		if len(response.Data.Items) == 0 || response.Data.Pages <= page {
			break
		}
	}
	return items, nil
}

func selectTenantDefaultSub2APIKey(
	account *models.Sub2APITenantAccount,
	keys []providers.Sub2APIKey,
	defaultKeyName string,
	desiredQuota float64,
	desiredExpiresAt *time.Time,
) (*providers.Sub2APIKey, int) {
	defaultKeyName = strings.TrimSpace(defaultKeyName)
	localID := int64(0)
	if account != nil {
		localID = parseInt64(account.DefaultKeyID)
	}
	candidates := make([]*providers.Sub2APIKey, 0)
	for i := range keys {
		if !strings.EqualFold(strings.TrimSpace(keys[i].Name), defaultKeyName) {
			continue
		}
		candidates = append(candidates, &keys[i])
	}
	for _, candidate := range candidates {
		if candidate.ID == localID {
			return candidate, len(candidates)
		}
	}
	var selected *providers.Sub2APIKey
	for _, candidate := range candidates {
		if !remoteTenantDefaultKeyMatches(candidate, defaultKeyName, desiredQuota, desiredExpiresAt) {
			continue
		}
		if selected == nil || candidate.ID > selected.ID {
			selected = candidate
		}
	}
	if selected != nil {
		return selected, len(candidates)
	}
	for _, candidate := range candidates {
		if selected == nil || candidate.ID > selected.ID {
			selected = candidate
		}
	}
	return selected, len(candidates)
}

func remoteTenantDefaultKeyMatches(key *providers.Sub2APIKey, defaultKeyName string, desiredQuota float64, desiredExpiresAt *time.Time) bool {
	if key == nil || !strings.EqualFold(strings.TrimSpace(key.Name), strings.TrimSpace(defaultKeyName)) {
		return false
	}
	return key.GroupID == platformSub2APIDefaultKeyGroupID &&
		strings.EqualFold(strings.TrimSpace(key.Status), "active") &&
		sameFloat(key.Quota, desiredQuota) &&
		sameOptionalTime(parseSub2APIOptionalTimePtrFromPointer(key.ExpiresAt), desiredExpiresAt)
}

func resolveTenantDefaultKeySecret(account *models.Sub2APITenantAccount, key *providers.Sub2APIKey, defaultKeyName string) (string, string, error) {
	if key == nil || key.ID <= 0 {
		return "", "", errors.New("sub2api default key is required")
	}
	if account != nil &&
		parseInt64(account.DefaultKeyID) == key.ID &&
		strings.EqualFold(strings.TrimSpace(account.DefaultKeyName), strings.TrimSpace(defaultKeyName)) &&
		strings.TrimSpace(account.DefaultKeyCiphertext) != "" {
		return account.DefaultKeyCiphertext, account.DefaultKeyFingerprint, nil
	}
	plain := strings.TrimSpace(key.Key)
	if plain == "" || strings.Contains(plain, "*") || strings.Contains(plain, "...") {
		return "", "", errors.New("远端默认 Key 已存在，但无法恢复完整密钥；请在 Sub2API 管理端轮换该 Key 后重试")
	}
	ciphertext, err := secretstore.Encrypt(plain)
	if err != nil {
		return "", "", err
	}
	return ciphertext, secretstore.Fingerprint(plain), nil
}

func normalizeSub2APIKeyExpiresAt(value string) (*time.Time, string, error) {
	parsed, ok := parseSub2APIOptionalTime(value)
	if !ok {
		if strings.TrimSpace(value) == "" {
			return nil, "", nil
		}
		return nil, "", errors.New("defaultKeyExpiresAt format is invalid")
	}
	if !parsed.After(time.Now()) {
		return nil, "", errors.New("defaultKeyExpiresAt must be in the future")
	}
	return &parsed, parsed.Format(time.RFC3339), nil
}

func parseSub2APIOptionalTimePtrFromPointer(value *string) *time.Time {
	if value == nil {
		return nil
	}
	parsed, ok := parseSub2APIOptionalTime(*value)
	if !ok {
		return nil
	}
	return &parsed
}

func parseSub2APIOptionalTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	timezoneFormats := []string{time.RFC3339Nano, time.RFC3339}
	for _, layout := range timezoneFormats {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	localFormats := []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range localFormats {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func sameOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
}

func firstTimePtr(items ...*time.Time) *time.Time {
	for _, item := range items {
		if item != nil {
			return item
		}
	}
	return nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func normalizedQuota(balance float64) float64 {
	if balance <= 0 {
		return 0
	}
	return balance / 2
}

func max64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func pickSub2APIAdminUser(items []providers.Sub2APIAdminUser, account *models.Sub2APITenantAccount) *providers.Sub2APIAdminUser {
	if account == nil {
		return nil
	}
	expectedID := parseInt64(account.Sub2APIAccountID)
	expectedEmail := strings.ToLower(strings.TrimSpace(account.LoginEmail))
	for i := range items {
		item := &items[i]
		if expectedID > 0 && item.ID == expectedID {
			return item
		}
		if expectedEmail != "" && strings.EqualFold(strings.TrimSpace(item.Email), expectedEmail) {
			return item
		}
	}
	if len(items) == 1 {
		return &items[0]
	}
	return nil
}

func sub2APIUserToAdminUser(user providers.Sub2APIUser) *providers.Sub2APIAdminUser {
	return &providers.Sub2APIAdminUser{
		ID:                         user.ID,
		Email:                      user.Email,
		Username:                   user.Username,
		Role:                       user.Role,
		Balance:                    user.Balance,
		FrozenBalance:              user.FrozenBalance,
		Concurrency:                user.Concurrency,
		Status:                     user.Status,
		AllowedGroups:              user.AllowedGroups,
		LastActiveAt:               user.LastActiveAt,
		CreatedAt:                  user.CreatedAt,
		UpdatedAt:                  user.UpdatedAt,
		BalanceNotifyEnabled:       user.BalanceNotifyEnabled,
		BalanceNotifyThresholdType: user.BalanceNotifyThresholdType,
		BalanceNotifyThreshold:     user.BalanceNotifyThreshold,
		BalanceNotifyExtraEmails:   user.BalanceNotifyExtraEmails,
		TotalRecharged:             user.TotalRecharged,
		RPMLimit:                   user.RPMLimit,
		CurrentConcurrency:         user.CurrentConcurrency,
	}
}

func isSub2APIConflictError(err error) bool {
	if err == nil {
		return false
	}
	var clientErr *providers.Sub2APIClientError
	if errors.As(err, &clientErr) && clientErr.StatusCode == 409 {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "conflict") || strings.Contains(text, "duplicate") || strings.Contains(text, "already exists")
}
