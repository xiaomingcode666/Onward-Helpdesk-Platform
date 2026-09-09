package services

import (
	"encoding/json"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	QuotaKnowledgeDocumentsPerTenant       = "knowledge_documents_per_tenant"
	QuotaKnowledgeDocumentsPerProduct      = "knowledge_documents_per_product"
	QuotaKnowledgeDocumentsPerUser         = "knowledge_documents_per_user"
	QuotaKnowledgeDocumentsPerNonAdminUser = "knowledge_documents_per_non_admin_user"
	QuotaKnowledgeDocumentSizeBytes        = "knowledge_document_file_size_bytes"
	QuotaKnowledgeDocumentSizeMB           = "knowledge_document_file_size_mb"

	defaultKnowledgeDocumentsPerProduct      = 100
	defaultNonAdminKnowledgeDocumentsPerUser = 10
)

var TenantCommercialService = &tenantCommercialService{}

type tenantCommercialService struct{}

type tenantCommercialState struct {
	Paid         bool
	Plan         *models.TenantPlan
	Subscription *models.TenantSubscription
}

func (s *tenantCommercialService) DescribeMembership(db *gorm.DB, tenantID int64) *response.TenantMembershipResponse {
	if tenantID <= 0 {
		return nil
	}
	state := s.currentState(db, tenantID)
	ret := &response.TenantMembershipResponse{Paid: state.Paid}
	if state.Plan != nil {
		ret.PlanID = state.Plan.ID
		ret.PlanCode = strings.TrimSpace(state.Plan.Code)
		ret.PlanName = strings.TrimSpace(state.Plan.Name)
		ret.PlanType = strings.TrimSpace(state.Plan.PlanType)
	}
	return ret
}

func (s *tenantCommercialService) IsPaidTenant(tenantID int64) bool {
	return s.currentState(sqls.DB(), tenantID).Paid
}

func (s *tenantCommercialService) RequireTenantAIWorkspaceForProductCreate(tenantID int64) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return nil
	}
	db := sqls.DB()
	if db != nil && db.Migrator().HasTable(&models.Sub2APITenantAccount{}) && tenantAIWorkspaceReady(
		repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(db, tenantID),
		time.Now(),
	) {
		return nil
	}
	return errorsx.Forbidden("租户 AI 服务未开通或默认租户密钥不可用")
}

func (s *tenantCommercialService) RequireKnowledgeDocumentUpload(tenantID int64, productID int64, operator *dto.AuthPrincipal, fileSize int64) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	if limit, ok := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentSizeBytes}, "single"); ok && limit > 0 && fileSize > limit {
		return errorsx.InvalidParam("上传文件超过当前租户允许的单文件大小")
	}
	if limitMB, ok := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentSizeMB}, "single"); ok && limitMB > 0 && fileSize > limitMB*1024*1024 {
		return errorsx.InvalidParam("上传文件超过当前租户允许的单文件大小")
	}
	if limit, ok := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentsPerTenant}, "total"); ok && limit > 0 {
		current := repositories.KnowledgeDocumentRepository.CountActiveByTenantID(sqls.DB(), tenantID)
		if current >= limit {
			return errorsx.Forbidden("当前租户知识库文档数量已达到平台配置上限")
		}
	}
	if productID > 0 {
		if limit, ok := s.resolveKnowledgeProductDocumentQuota(sqls.DB(), tenantID, productID); ok && limit > 0 {
			current := repositories.KnowledgeDocumentRepository.CountActiveByTenantProductID(sqls.DB(), tenantID, productID)
			if current >= limit {
				return errorsx.Forbidden("当前产品知识库文档数量已达到平台配置上限")
			}
		}
	}
	if operator != nil && operator.UserID > 0 {
		if limit, ok := s.resolveKnowledgeUserDocumentQuota(sqls.DB(), tenantID, operator); ok && limit > 0 {
			current := repositories.KnowledgeDocumentRepository.CountActiveByTenantUserID(sqls.DB(), tenantID, operator.UserID)
			if current >= limit {
				return errorsx.Forbidden("当前账号知识库文档数量已达到平台配置上限")
			}
		}
	}
	return nil
}

func (s *tenantCommercialService) DescribeKnowledgeDocumentUploadQuota(tenantID int64, productID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeUploadQuotaDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if productID > 0 && repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID) == nil {
		return nil, errorsx.InvalidParam("product not found")
	}
	userID := int64(0)
	if operator != nil {
		userID = operator.UserID
	}
	documentLimit, hasDocumentLimit := s.resolveKnowledgeUserDocumentQuota(sqls.DB(), tenantID, operator)
	tenantDocumentLimit, hasTenantDocumentLimit := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentsPerTenant}, "total")
	productDocumentLimit, hasProductDocumentLimit := s.resolveKnowledgeProductDocumentQuota(sqls.DB(), tenantID, productID)
	singleFileLimit, _ := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentSizeBytes}, "single")
	if singleFileLimit <= 0 {
		if limitMB, ok := s.resolveQuota(sqls.DB(), tenantID, 0, []string{QuotaKnowledgeDocumentSizeMB}, "single"); ok && limitMB > 0 {
			singleFileLimit = limitMB * 1024 * 1024
		}
	}
	documentUsed, totalFileSize, err := repositories.KnowledgeDocumentRepository.ActiveUsageByTenantUserID(sqls.DB(), tenantID, userID)
	if err != nil {
		return nil, err
	}
	tenantDocumentUsed, _, err := repositories.KnowledgeDocumentRepository.ActiveUsageByTenantID(sqls.DB(), tenantID)
	if err != nil {
		return nil, err
	}
	productDocumentUsed := int64(0)
	if productID > 0 {
		productDocumentUsed = repositories.KnowledgeDocumentRepository.CountActiveByTenantProductID(sqls.DB(), tenantID, productID)
	}
	ret := &dto.EnterpriseKnowledgeUploadQuotaDTO{
		DocumentLimit:            documentLimit,
		DocumentUsed:             documentUsed,
		DocumentRemaining:        -1,
		TenantDocumentLimit:      tenantDocumentLimit,
		TenantDocumentUsed:       tenantDocumentUsed,
		TenantDocumentRemaining:  -1,
		ProductDocumentLimit:     productDocumentLimit,
		ProductDocumentUsed:      productDocumentUsed,
		ProductDocumentRemaining: -1,
		TotalFileSize:            totalFileSize,
		SingleFileSizeLimit:      singleFileLimit,
		UnlimitedDocuments:       !hasDocumentLimit && !hasTenantDocumentLimit && !hasProductDocumentLimit,
	}
	if hasDocumentLimit && documentLimit > 0 {
		ret.DocumentRemaining = remainingQuota(documentLimit, documentUsed)
	}
	if hasTenantDocumentLimit && tenantDocumentLimit > 0 {
		ret.TenantDocumentRemaining = remainingQuota(tenantDocumentLimit, tenantDocumentUsed)
		if ret.DocumentRemaining < 0 || ret.TenantDocumentRemaining < ret.DocumentRemaining {
			ret.DocumentRemaining = ret.TenantDocumentRemaining
		}
	}
	if hasProductDocumentLimit && productDocumentLimit > 0 {
		ret.ProductDocumentRemaining = remainingQuota(productDocumentLimit, productDocumentUsed)
		if ret.DocumentRemaining < 0 || ret.ProductDocumentRemaining < ret.DocumentRemaining {
			ret.DocumentRemaining = ret.ProductDocumentRemaining
		}
	}
	return ret, nil
}

func (s *tenantCommercialService) currentState(db *gorm.DB, tenantID int64) tenantCommercialState {
	if db == nil || tenantID <= 0 {
		return tenantCommercialState{}
	}
	state := tenantCommercialState{
		Paid: repositories.PlatformIAMRepository.HasPositiveSub2APIBalance(db, tenantID),
	}
	now := time.Now()
	subscription := repositories.PlatformIAMRepository.GetCurrentSubscription(db, tenantID, now)
	if subscription == nil || subscription.Status != enums.StatusOk {
		return state
	}
	state.Subscription = subscription
	plan := repositories.PlatformIAMRepository.GetPlan(db, subscription.PlanID)
	if plan == nil || plan.Status != enums.StatusOk {
		return state
	}
	state.Plan = plan
	return state
}

func remainingQuota(limit, used int64) int64 {
	if limit <= 0 {
		return -1
	}
	if used >= limit {
		return 0
	}
	return limit - used
}

func (s *tenantCommercialService) resolveQuota(db *gorm.DB, tenantID, productID int64, keys []string, period string) (int64, bool) {
	now := time.Now()
	periods := uniqueQuotaPeriods(period)
	for _, key := range keys {
		for _, candidatePeriod := range periods {
			if override := repositories.PlatformIAMRepository.FindActiveQuotaOverride(db, tenantID, productID, key, candidatePeriod, now); override != nil {
				return override.QuotaValue, true
			}
			if productID > 0 {
				if override := repositories.PlatformIAMRepository.FindActiveQuotaOverride(db, tenantID, 0, key, candidatePeriod, now); override != nil {
					return override.QuotaValue, true
				}
			}
		}
	}
	state := s.currentState(db, tenantID)
	if state.Plan != nil {
		for _, key := range keys {
			for _, candidatePeriod := range periods {
				if quota := repositories.PlatformIAMRepository.FindPlanQuota(db, state.Plan.ID, key, candidatePeriod); quota != nil {
					return quota.QuotaValue, true
				}
			}
		}
		if value, ok := resolvePlanQuotaTemplateValue(state.Plan, keys, periods); ok {
			return value, true
		}
	}
	for _, key := range keys {
		switch key {
		case QuotaKnowledgeDocumentsPerTenant:
			if value, ok := PlatformSystemConfigService.DefaultTenantDocumentLimit(state.Paid); ok {
				return value, true
			}
		case QuotaKnowledgeDocumentSizeMB:
			if value, ok := PlatformSystemConfigService.DefaultKnowledgeDocumentMaxMB(); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func (s *tenantCommercialService) resolveKnowledgeProductDocumentQuota(db *gorm.DB, tenantID, productID int64) (int64, bool) {
	if productID <= 0 {
		return 0, false
	}
	if limit, ok := s.resolveQuota(db, tenantID, productID, []string{QuotaKnowledgeDocumentsPerProduct}, "total"); ok {
		return limit, true
	}
	return defaultKnowledgeDocumentsPerProduct, true
}

func tenantAIWorkspaceReady(account *models.Sub2APITenantAccount, now time.Time) bool {
	if account == nil || account.Status != enums.StatusOk || !tenantAIAccountStatusReady(account.AccountStatus) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") ||
		strings.TrimSpace(account.LoginEmail) == "" ||
		strings.TrimSpace(account.LoginPasswordCiphertext) == "" ||
		account.AccessTokenExpiresAt == nil ||
		!account.AccessTokenExpiresAt.After(now) ||
		account.Balance <= 0 ||
		strings.TrimSpace(account.DefaultKeyID) == "" ||
		strings.TrimSpace(account.DefaultKeyCiphertext) == "" ||
		!strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") {
		return false
	}
	return account.DefaultKeyExpiresAt == nil || account.DefaultKeyExpiresAt.After(now)
}

func tenantAIAccountStatusReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "enabled", "启用":
		return true
	default:
		return false
	}
}

func (s *tenantCommercialService) resolveKnowledgeUserDocumentQuota(db *gorm.DB, tenantID int64, operator *dto.AuthPrincipal) (int64, bool) {
	if operator == nil || operator.UserID <= 0 {
		return 0, false
	}
	if isTenantKnowledgeAdmin(operator) {
		return s.resolveQuota(db, tenantID, 0, []string{QuotaKnowledgeDocumentsPerUser}, "total")
	}
	if limit, ok := s.resolveQuota(db, tenantID, 0, []string{QuotaKnowledgeDocumentsPerNonAdminUser, QuotaKnowledgeDocumentsPerUser}, "total"); ok {
		return limit, true
	}
	return defaultNonAdminKnowledgeDocumentsPerUser, true
}

func isTenantKnowledgeAdmin(operator *dto.AuthPrincipal) bool {
	return operator != nil && (operator.IsPlatform() || operator.HasRole(EnterpriseRoleOwner) || operator.HasRole(EnterpriseRoleAdmin))
}

func resolvePlanQuotaTemplateValue(plan *models.TenantPlan, keys []string, periods []string) (int64, bool) {
	if plan == nil || strings.TrimSpace(plan.QuotaTemplateJSON) == "" {
		return 0, false
	}
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(plan.QuotaTemplateJSON), &raw); err != nil {
		return 0, false
	}
	for _, key := range keys {
		canonicalKey := canonicalQuotaKey(key)
		if canonicalKey == "" {
			continue
		}
		for rawKey, value := range raw {
			if canonicalQuotaKey(rawKey) != canonicalKey {
				continue
			}
			quotaValue, _, quotaPeriod := decodeQuotaValue(value)
			if quotaTemplatePeriodMatches(quotaPeriod, periods) {
				return quotaValue, true
			}
		}
	}
	return 0, false
}

func quotaTemplatePeriodMatches(period string, candidates []string) bool {
	period = strings.TrimSpace(period)
	if period == "" {
		return true
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == period {
			return true
		}
	}
	return false
}

func uniqueQuotaPeriods(period string) []string {
	ret := make([]string, 0, 3)
	seen := make(map[string]bool)
	for _, item := range []string{strings.TrimSpace(period), "monthly", ""} {
		if seen[item] {
			continue
		}
		seen[item] = true
		ret = append(ret, item)
	}
	return ret
}
