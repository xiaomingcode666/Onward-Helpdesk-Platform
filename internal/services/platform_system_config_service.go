package services

import (
	"encoding/json"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	platformSystemPolicyConfigKey       = "platform.system_policy.v1"
	platformSystemPolicyDefaultUploadMB = 20
	platformSystemPolicyMaxUploadMB     = 1024
	platformSystemPolicyMaxTenantDocs   = 10_000_000
)

var PlatformSystemConfigService = newPlatformSystemConfigService()

func newPlatformSystemConfigService() *platformSystemConfigService {
	return &platformSystemConfigService{}
}

type platformSystemConfigService struct{}

type platformSystemPolicyStored struct {
	FreeTenantDocumentLimit int64 `json:"freeTenantDocumentLimit"`
	PaidTenantDocumentLimit int64 `json:"paidTenantDocumentLimit"`
	KnowledgeDocumentMaxMB  int64 `json:"knowledgeDocumentMaxMB"`
}

func (s *platformSystemConfigService) GetSettings(operator *dto.AuthPrincipal) *response.PlatformSystemPolicyResponse {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSystemPolicyConfigKey)
	stored, ok := parsePlatformSystemPolicy(item)
	if !ok {
		stored = platformSystemPolicyStored{KnowledgeDocumentMaxMB: platformSystemPolicyDefaultUploadMB}
	}
	ret := &response.PlatformSystemPolicyResponse{
		CanUpdate:  canUpdatePlatformSystemPolicy(operator),
		Configured: item != nil && item.Status != enums.StatusDeleted,
		TenantQuota: response.PlatformTenantQuotaPolicyResponse{
			FreeTenantDocumentLimit: stored.FreeTenantDocumentLimit,
			PaidTenantDocumentLimit: stored.PaidTenantDocumentLimit,
		},
		UploadLimit: response.PlatformUploadLimitPolicyResponse{
			KnowledgeDocumentMaxMB: stored.KnowledgeDocumentMaxMB,
		},
	}
	if item != nil && !item.UpdatedAt.IsZero() {
		ret.UpdatedAt = item.UpdatedAt.Format(time.DateTime)
	}
	return ret
}

func (s *platformSystemConfigService) UpdateSettings(req request.PlatformSystemPolicyUpdateRequest, operator *dto.AuthPrincipal) (*response.PlatformSystemPolicyResponse, error) {
	if !canUpdatePlatformSystemPolicy(operator) {
		return nil, errorsx.Forbidden("仅平台管理员可以修改系统级默认策略")
	}
	if err := validatePlatformSystemPolicy(req); err != nil {
		return nil, err
	}
	stored := platformSystemPolicyStored{
		FreeTenantDocumentLimit: req.FreeTenantDocumentLimit,
		PaidTenantDocumentLimit: req.PaidTenantDocumentLimit,
		KnowledgeDocumentMaxMB:  req.KnowledgeDocumentMaxMB,
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	before := s.GetSettings(operator)
	now := time.Now()
	auditFields := utils.BuildAuditFields(operator)
	auditFields.UpdatedAt = now
	item := &models.SystemConfig{
		ConfigKey:   platformSystemPolicyConfigKey,
		ConfigValue: string(payload),
		GroupCode:   "platform_system",
		Title:       "平台系统默认策略",
		Description: "平台级租户知识库文档配额与单文件上传限制。",
		Status:      enums.StatusOk,
		AuditFields: auditFields,
	}
	if err := repositories.SystemConfigRepository.SaveByKey(sqls.DB(), item); err != nil {
		return nil, err
	}
	after := s.GetSettings(operator)
	_ = PlatformIAMService.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "platform_system_policy", "platform", "platform_system_policy.updated", before, after, models.RiskLevelHigh, "")
	return after, nil
}

func (s *platformSystemConfigService) DefaultTenantDocumentLimit(paid bool) (int64, bool) {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSystemPolicyConfigKey)
	stored, ok := parsePlatformSystemPolicy(item)
	if !ok {
		return 0, false
	}
	value := stored.FreeTenantDocumentLimit
	if paid {
		value = stored.PaidTenantDocumentLimit
	}
	return value, value > 0
}

func (s *platformSystemConfigService) DefaultKnowledgeDocumentMaxMB() (int64, bool) {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSystemPolicyConfigKey)
	stored, ok := parsePlatformSystemPolicy(item)
	if !ok || stored.KnowledgeDocumentMaxMB <= 0 {
		return 0, false
	}
	return stored.KnowledgeDocumentMaxMB, true
}

func parsePlatformSystemPolicy(item *models.SystemConfig) (platformSystemPolicyStored, bool) {
	if item == nil || item.Status == enums.StatusDeleted || strings.TrimSpace(item.ConfigValue) == "" {
		return platformSystemPolicyStored{}, false
	}
	var stored platformSystemPolicyStored
	if err := json.Unmarshal([]byte(item.ConfigValue), &stored); err != nil {
		return platformSystemPolicyStored{}, false
	}
	return stored, true
}

func validatePlatformSystemPolicy(req request.PlatformSystemPolicyUpdateRequest) error {
	if req.FreeTenantDocumentLimit < 0 || req.PaidTenantDocumentLimit < 0 {
		return errorsx.InvalidParam("租户文档配额不能为负数")
	}
	if req.FreeTenantDocumentLimit > platformSystemPolicyMaxTenantDocs || req.PaidTenantDocumentLimit > platformSystemPolicyMaxTenantDocs {
		return errorsx.InvalidParam("租户文档配额不能超过 10000000")
	}
	if req.KnowledgeDocumentMaxMB < 0 || req.KnowledgeDocumentMaxMB > platformSystemPolicyMaxUploadMB {
		return errorsx.InvalidParam("单文件上传限制需在 0 到 1024 MB 之间")
	}
	return nil
}

func canUpdatePlatformSystemPolicy(operator *dto.AuthPrincipal) bool {
	return operator != nil && operator.IsPlatform() && operator.EffectiveTenantID() == 0
}
