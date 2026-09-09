package services

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	TenantFeatureAI               = "ai"
	TenantFeatureKnowledgeSupport = "knowledgeSupport"
	TenantFeatureProduct          = "product"
	TenantFeatureDevice           = "device"
	TenantFeatureServiceCode      = "serviceCode"
	TenantFeatureDeviceDiagnosis  = "deviceDiagnosis"
	TenantFeatureServerConsole    = "serverConsole"
)

var TenantCapabilityService = &tenantCapabilityService{}

type tenantCapabilityService struct{}

func (s *tenantCapabilityService) AIEnabled(tenantID int64) bool {
	if tenantID <= 0 {
		return true
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return false
	}
	return tenant.IsAIEnabled()
}

func (s *tenantCapabilityService) AIDisabled(tenantID int64) bool {
	return s.AIDisabledDB(sqls.DB(), tenantID)
}

func (s *tenantCapabilityService) AIDisabledDB(db *gorm.DB, tenantID int64) bool {
	if tenantID <= 0 {
		return false
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID)
	return tenant != nil && tenant.Status != enums.StatusDeleted && !tenant.IsAIEnabled()
}

func (s *tenantCapabilityService) FeatureFlags(tenantID int64) map[string]bool {
	if tenantID <= 0 {
		return nil
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return nil
	}
	knowledgeSupport := tenant.IsKnowledgeSupportScene()
	serverConsole := repositories.TenantIntegrationConfigRepository.GetByProvider(sqls.DB(), tenantID, models.TenantIntegrationProviderOnePanel)
	serverConsoleEnabled := serverConsole != nil && serverConsole.Status == enums.StatusOk && serverConsole.Enabled && TenantExternalPortalURL(serverConsole) != ""
	return map[string]bool{
		TenantFeatureAI:               tenant.IsAIEnabled(),
		TenantFeatureKnowledgeSupport: knowledgeSupport,
		TenantFeatureProduct:          !knowledgeSupport,
		TenantFeatureDevice:           !knowledgeSupport,
		TenantFeatureServiceCode:      !knowledgeSupport,
		TenantFeatureDeviceDiagnosis:  tenant.IsAIEnabled() && !knowledgeSupport,
		TenantFeatureServerConsole:    serverConsoleEnabled,
	}
}

func (s *tenantCapabilityService) KnowledgeSupport(tenantID int64) bool {
	return s.KnowledgeSupportDB(sqls.DB(), tenantID)
}

func (s *tenantCapabilityService) KnowledgeSupportDB(db *gorm.DB, tenantID int64) bool {
	if tenantID <= 0 {
		return false
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID)
	return tenant != nil && tenant.Status != enums.StatusDeleted && tenant.IsKnowledgeSupportScene()
}

func (s *tenantCapabilityService) CanUseWorkflowTemplate(tenantID int64, workflow *models.AIWorkflow) bool {
	if workflow == nil {
		return false
	}
	if workflow.TenantID != 0 || workflow.Scope != models.AIWorkflowScopePlatform {
		return s.AIEnabled(tenantID)
	}
	if s.AIEnabled(tenantID) {
		if s.KnowledgeSupport(tenantID) {
			return workflow.Code == PlatformKnowledgeSupportWorkflowCode
		}
		return workflow.Code != PlatformDispatchOnlyWorkflowCode && workflow.Code != PlatformKnowledgeSupportWorkflowCode
	}
	return workflow.TenantID == 0 &&
		workflow.Scope == models.AIWorkflowScopePlatform &&
		workflow.Code == PlatformDispatchOnlyWorkflowCode
}

func tenantAIEnabledOrDefault(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func boolRef(value bool) *bool {
	return &value
}

func tenantAIEnabled(tenant *models.Tenant) bool {
	if tenant == nil {
		return false
	}
	return tenant.IsAIEnabled()
}
