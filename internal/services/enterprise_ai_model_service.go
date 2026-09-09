package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EnterpriseAIModelService = &enterpriseAIModelService{}

type enterpriseAIModelService struct{}

type EnterpriseAIModelWorkspaceAggregate struct {
	GeneratedAt        time.Time
	Account            *models.Sub2APITenantAccount
	User               *providers.Sub2APIUser
	Stats              *providers.Sub2APIUsageDashboardStatsResponse
	Trend              *providers.Sub2APIUsageDashboardTrendResponse
	Keys               *providers.Sub2APIListKeysResponse
	KeyProductBindings map[int64]EnterpriseAIKeyProductBinding
	Subscriptions      *providers.Sub2APIListSubscriptionsResponse
}

type EnterpriseAIKeyProductBinding struct {
	ProductID   int64
	ProductName string
	ProductCode string
}

func (s *enterpriseAIModelService) GetWorkspace(ctx context.Context, tenantID int64, query request.EnterpriseAIModelWorkspaceQueryRequest, operator *dto.AuthPrincipal) (*EnterpriseAIModelWorkspaceAggregate, error) {
	query = normalizeEnterpriseAIModelWorkspaceQuery(query)
	provider, account, token, err := s.resolveTenantProvider(ctx, tenantID, operator)
	if err != nil {
		return nil, err
	}
	timezone := defaultTimezone(query.Timezone)
	meResp, err := provider.AuthMe(ctx, token, timezone)
	if err != nil {
		return nil, err
	}
	user := meResp.Data
	if err := PlatformAIModelService.updateSub2APIAccountFromAdminUser(account, sub2APIUserToAdminUser(user), operator); err != nil {
		return nil, err
	}
	statsResp, err := provider.UsageDashboardStats(ctx, token, timezone)
	if err != nil {
		return nil, err
	}
	subscriptionsResp, err := provider.ListSubscriptions(ctx, token, timezone)
	if err != nil {
		return nil, err
	}
	trendResp, err := provider.UsageDashboardTrend(ctx, token, providers.Sub2APIUsageQuery{
		StartDate:   query.StartDate,
		EndDate:     query.EndDate,
		Granularity: query.Granularity,
		Timezone:    timezone,
	})
	if err != nil {
		return nil, err
	}
	keysResp, err := provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
		Page:      query.Page,
		PageSize:  query.PageSize,
		SortBy:    query.SortBy,
		SortOrder: query.SortOrder,
		Timezone:  timezone,
	})
	if err != nil {
		return nil, err
	}
	if err := s.syncDefaultKeySnapshot(account, keysResp.Data.Items, operator); err != nil {
		return nil, err
	}
	if err := s.syncProductKeySnapshots(tenantID, nil, keysResp.Data.Items); err != nil {
		return nil, err
	}
	keyProductBindings := s.listKeyProductBindings(tenantID, keysResp.Data.Items)
	account = repositories.PlatformIAMRepository.GetSub2APIAccount(sqls.DB(), account.ID)
	return &EnterpriseAIModelWorkspaceAggregate{
		GeneratedAt:        time.Now(),
		Account:            account,
		User:               &user,
		Stats:              statsResp,
		Trend:              trendResp,
		Keys:               keysResp,
		KeyProductBindings: keyProductBindings,
		Subscriptions:      subscriptionsResp,
	}, nil
}

func (s *enterpriseAIModelService) refreshAccountBalance(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal) error {
	provider, account, token, err := s.resolveTenantProvider(ctx, tenantID, operator)
	if err != nil {
		return err
	}
	meResp, err := provider.AuthMe(ctx, token, defaultTimezone(""))
	if err != nil {
		return err
	}
	return PlatformAIModelService.updateSub2APIAccountFromAdminUser(account, sub2APIUserToAdminUser(meResp.Data), operator)
}

func (s *enterpriseAIModelService) refreshProductKeyUsage(ctx context.Context, tenantID int64, productIDs []int64, operator *dto.AuthPrincipal) error {
	productIDs = uniqueWorkbenchInt64s(productIDs)
	if tenantID <= 0 || len(productIDs) == 0 {
		return nil
	}
	credentials := repositories.ProductAIUsageCredentialRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		In("product_id", productIDs).
		NotEq("status", enums.StatusDeleted).
		NotEq("sub2api_key_id", ""))
	if len(credentials) == 0 || productKeySnapshotsAreFresh(credentials, time.Now()) {
		return nil
	}

	provider, _, token, err := s.resolveTenantProvider(ctx, tenantID, operator)
	if err != nil {
		return err
	}
	keys, err := listAllSub2APIKeys(ctx, provider, token)
	if err != nil {
		return err
	}
	return s.syncProductKeySnapshots(tenantID, productIDs, keys)
}

func (s *enterpriseAIModelService) syncProductKeySnapshots(tenantID int64, productIDs []int64, keys []providers.Sub2APIKey) error {
	if tenantID <= 0 || len(keys) == 0 {
		return nil
	}
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted).NotEq("sub2api_key_id", "")
	productIDs = uniqueWorkbenchInt64s(productIDs)
	if len(productIDs) > 0 {
		cnd.In("product_id", productIDs)
	}
	credentials := repositories.ProductAIUsageCredentialRepository.Find(sqls.DB(), cnd)
	keysByID := make(map[int64]providers.Sub2APIKey, len(keys))
	for _, key := range keys {
		keysByID[key.ID] = key
	}
	now := time.Now()
	for _, credential := range credentials {
		keyID, err := strconv.ParseInt(strings.TrimSpace(credential.Sub2APIKeyID), 10, 64)
		if err != nil || keyID <= 0 {
			continue
		}
		key, ok := keysByID[keyID]
		if !ok {
			continue
		}
		currency := firstNonEmptyString(credential.Currency, "USD")
		if err := repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, map[string]any{
			"key_name":          key.Name,
			"quota_limit":       key.Quota,
			"quota_used":        key.QuotaUsed,
			"quota_policy_json": buildProductQuotaPolicyJSON(credential.Sub2APIKeyID, key.Quota, key.QuotaUsed, currency),
			"last_synced_at":    now,
			"updated_at":        now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *enterpriseAIModelService) listKeyProductBindings(tenantID int64, keys []providers.Sub2APIKey) map[int64]EnterpriseAIKeyProductBinding {
	bindings := make(map[int64]EnterpriseAIKeyProductBinding)
	if tenantID <= 0 {
		return bindings
	}
	keyProductIDs := make(map[int64]int64)
	credentials := repositories.ProductAIUsageCredentialRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		NotEq("sub2api_key_id", ""))
	productIDs := make([]int64, 0, len(credentials))
	for _, credential := range credentials {
		if keyID := parseInt64(credential.Sub2APIKeyID); keyID > 0 && credential.ProductID > 0 {
			keyProductIDs[keyID] = credential.ProductID
			productIDs = append(productIDs, credential.ProductID)
		}
	}
	for _, key := range keys {
		if key.ID <= 0 {
			continue
		}
		if _, ok := keyProductIDs[key.ID]; ok {
			continue
		}
		productID := parseProductIDFromSub2APIKeyName(key.Name)
		if productID <= 0 {
			continue
		}
		keyProductIDs[key.ID] = productID
		productIDs = append(productIDs, productID)
	}
	productIDs = uniqueWorkbenchInt64s(productIDs)
	if len(productIDs) == 0 {
		return bindings
	}
	products := repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		In("id", productIDs).
		NotEq("status", enums.StatusDeleted))
	productsByID := make(map[int64]models.Product, len(products))
	for _, product := range products {
		productsByID[product.ID] = product
	}
	for keyID, productID := range keyProductIDs {
		if keyID <= 0 || productID <= 0 {
			continue
		}
		product, ok := productsByID[productID]
		if !ok {
			continue
		}
		bindings[keyID] = EnterpriseAIKeyProductBinding{
			ProductID:   product.ID,
			ProductName: firstNonEmptyString(product.Name, product.Code),
			ProductCode: product.Code,
		}
	}
	return bindings
}

func parseProductIDFromSub2APIKeyName(value string) int64 {
	const prefix = "RHD-PRODUCT-"
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToUpper(value), prefix) {
		return 0
	}
	return parseInt64(value[len(prefix):])
}

func productKeySnapshotsAreFresh(credentials []models.ProductAIUsageCredential, now time.Time) bool {
	for _, credential := range credentials {
		if credential.LastSyncedAt == nil || now.Sub(*credential.LastSyncedAt) >= time.Minute {
			return false
		}
	}
	return true
}

func listAllSub2APIKeys(ctx context.Context, provider providers.Sub2APIProvider, token string) ([]providers.Sub2APIKey, error) {
	const pageSize = 100
	response, err := provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
		Page: 1, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		return nil, err
	}
	items := append([]providers.Sub2APIKey(nil), response.Data.Items...)
	pages := response.Data.Pages
	if pages > 100 {
		pages = 100
	}
	for page := 2; page <= pages; page++ {
		response, err = provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
			Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc", Timezone: "Asia/Shanghai",
		})
		if err != nil {
			return nil, err
		}
		items = append(items, response.Data.Items...)
	}
	return items, nil
}

func (s *enterpriseAIModelService) UpdateKeyQuota(ctx context.Context, tenantID, keyID int64, quota float64, query request.EnterpriseAIModelWorkspaceQueryRequest, operator *dto.AuthPrincipal) (*EnterpriseAIModelWorkspaceAggregate, error) {
	if keyID <= 0 {
		return nil, errors.New("keyId is required")
	}
	if quota < 0 {
		return nil, errors.New("quota must be greater than or equal to 0")
	}
	query = normalizeEnterpriseAIModelWorkspaceQuery(query)
	provider, _, token, err := s.resolveTenantProvider(ctx, tenantID, operator)
	if err != nil {
		return nil, err
	}
	keysResp, err := provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
		Page:      1,
		PageSize:  100,
		SortBy:    "created_at",
		SortOrder: "desc",
		Timezone:  defaultTimezone(query.Timezone),
	})
	if err != nil {
		return nil, err
	}
	current := findSub2APIKey(keysResp.Data.Items, keyID)
	if current == nil {
		return nil, fmt.Errorf("sub2api key %d not found", keyID)
	}
	ipWhitelist := stringSliceFromAny(current.IPWhitelist)
	ipBlacklist := stringSliceFromAny(current.IPBlacklist)
	expiresAt := ""
	if current.ExpiresAt != nil {
		expiresAt = *current.ExpiresAt
	}
	_, err = provider.UpdateKey(ctx, token, keyID, providers.Sub2APIUpdateKeyRequest{
		Name:        current.Name,
		GroupID:     current.GroupID,
		IPWhitelist: &ipWhitelist,
		IPBlacklist: &ipBlacklist,
		Quota:       &quota,
		ExpiresAt:   expiresAt,
		RateLimit5h: current.RateLimit5h,
		RateLimit1d: current.RateLimit1d,
		RateLimit7d: current.RateLimit7d,
		Status:      current.Status,
	})
	if err != nil {
		return nil, err
	}
	return s.GetWorkspace(ctx, tenantID, query, operator)
}

func (s *enterpriseAIModelService) ResetKeyQuota(ctx context.Context, tenantID, keyID int64, query request.EnterpriseAIModelWorkspaceQueryRequest, operator *dto.AuthPrincipal) (*EnterpriseAIModelWorkspaceAggregate, error) {
	if keyID <= 0 {
		return nil, errors.New("keyId is required")
	}
	query = normalizeEnterpriseAIModelWorkspaceQuery(query)
	provider, _, token, err := s.resolveTenantProvider(ctx, tenantID, operator)
	if err != nil {
		return nil, err
	}
	if _, err := provider.UpdateKey(ctx, token, keyID, providers.Sub2APIUpdateKeyRequest{ResetQuota: true}); err != nil {
		return nil, err
	}
	return s.GetWorkspace(ctx, tenantID, query, operator)
}

func (s *enterpriseAIModelService) resolveTenantProvider(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal) (providers.Sub2APIProvider, *models.Sub2APITenantAccount, string, error) {
	if tenantID <= 0 {
		return nil, nil, "", errors.New("tenant is required")
	}
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID)
	if account == nil {
		return nil, nil, "", errors.New("tenant sub2api account is not configured")
	}
	if strings.TrimSpace(account.LoginEmail) == "" || strings.TrimSpace(account.LoginPasswordCiphertext) == "" {
		return nil, nil, "", errors.New("tenant sub2api login credential is not configured")
	}
	host, adminAPIKey, _, _, err := PlatformAIModelService.resolveProviderConfig()
	if err != nil {
		return nil, nil, "", err
	}
	if strings.TrimSpace(host) == "" {
		return nil, nil, "", errors.New("sub2api host is required")
	}
	provider := PlatformAIModelService.providerFromConfig(host, adminAPIKey)
	token, _, err := PlatformAIModelService.ensureAccessToken(ctx, provider, account, operator)
	if err != nil {
		return nil, nil, "", err
	}
	return provider, account, token, nil
}

func (s *enterpriseAIModelService) syncDefaultKeySnapshot(account *models.Sub2APITenantAccount, keys []providers.Sub2APIKey, operator *dto.AuthPrincipal) error {
	if account == nil {
		return nil
	}
	current := findAccountDefaultSub2APIKey(account, keys)
	if current == nil {
		return nil
	}
	audit := utils.BuildAuditFields(operator)
	now := time.Now()
	return repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"default_key_id":         fmt.Sprintf("%d", current.ID),
		"default_key_name":       current.Name,
		"default_key_group_id":   current.GroupID,
		"default_key_quota":      current.Quota,
		"default_key_quota_used": current.QuotaUsed,
		"default_key_status":     current.Status,
		"default_key_expires_at": parseSub2APIOptionalTimePtrFromPointer(current.ExpiresAt),
		"last_synced_at":         now,
		"updated_at":             now,
		"update_user_id":         audit.UpdateUserID,
		"update_user_name":       audit.UpdateUserName,
	})
}

func normalizeEnterpriseAIModelWorkspaceQuery(query request.EnterpriseAIModelWorkspaceQueryRequest) request.EnterpriseAIModelWorkspaceQueryRequest {
	query.Timezone = defaultTimezone(query.Timezone)
	if query.Granularity = strings.TrimSpace(query.Granularity); query.Granularity == "" {
		query.Granularity = "day"
	}
	loc, err := time.LoadLocation(query.Timezone)
	if err != nil {
		loc = time.FixedZone("UTC+8", 8*3600)
	}
	now := time.Now().In(loc)
	if query.EndDate = strings.TrimSpace(query.EndDate); query.EndDate == "" {
		query.EndDate = now.Format("2006-01-02")
	}
	if query.StartDate = strings.TrimSpace(query.StartDate); query.StartDate == "" {
		query.StartDate = now.AddDate(0, 0, -6).Format("2006-01-02")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.SortBy = strings.TrimSpace(query.SortBy); query.SortBy == "" {
		query.SortBy = "created_at"
	}
	if query.SortOrder = strings.ToLower(strings.TrimSpace(query.SortOrder)); query.SortOrder == "" {
		query.SortOrder = "desc"
	}
	if query.SortOrder != "asc" && query.SortOrder != "desc" {
		query.SortOrder = "desc"
	}
	return query
}

func findSub2APIKey(items []providers.Sub2APIKey, keyID int64) *providers.Sub2APIKey {
	for i := range items {
		if items[i].ID == keyID {
			return &items[i]
		}
	}
	return nil
}

func findAccountDefaultSub2APIKey(account *models.Sub2APITenantAccount, keys []providers.Sub2APIKey) *providers.Sub2APIKey {
	if account == nil {
		return nil
	}
	expectedName := defaultSub2APIKeyName(account.AccountName)
	defaultID := parseInt64(account.DefaultKeyID)
	if defaultID > 0 {
		if item := findSub2APIKey(keys, defaultID); item != nil && strings.EqualFold(strings.TrimSpace(item.Name), expectedName) {
			return item
		}
	}
	return nil
}

func stringSliceFromAny(value any) []string {
	if value == nil {
		return []string{}
	}
	switch typed := value.(type) {
	case []string:
		return compactStringSlice(typed)
	case []any:
		ret := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				ret = append(ret, text)
			}
		}
		return compactStringSlice(ret)
	default:
		return []string{}
	}
}

func compactStringSlice(items []string) []string {
	ret := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(item); value != "" {
			ret = append(ret, value)
		}
	}
	return ret
}
