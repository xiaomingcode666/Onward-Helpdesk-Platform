package services

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductServiceProfileService = newProductServiceProfileService()

func newProductServiceProfileService() *productServiceProfileService {
	return &productServiceProfileService{}
}

type productServiceProfileService struct {
}

type productServiceProfileJSONConfig struct {
	SupportLocalesJSON string
	SupportRegionsJSON string
	WarrantyPolicyJSON string
	ServicePolicyJSON  string
}

func (s *productServiceProfileService) Get(id int64) *models.ProductServiceProfile {
	if id <= 0 {
		return nil
	}
	return repositories.ProductServiceProfileRepository.Get(sqls.DB(), id)
}

func (s *productServiceProfileService) GetByProductID(productID int64) *models.ProductServiceProfile {
	if productID <= 0 {
		return nil
	}
	return repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
}

func (s *productServiceProfileService) FindPageByCnd(cnd *sqls.Cnd) (list []models.ProductServiceProfile, paging *sqls.Paging) {
	return repositories.ProductServiceProfileRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *productServiceProfileService) CreateProductServiceProfile(req request.CreateProductServiceProfileRequest, operator *dto.AuthPrincipal) (*models.ProductServiceProfile, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, err := s.requireProduct(req.TenantID, req.ProductID)
	if err != nil {
		return nil, err
	}
	config, err := normalizeProductServiceProfileJSON(req)
	if err != nil {
		return nil, err
	}
	var item *models.ProductServiceProfile
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := s.lockDefaultFlowTemplate(ctx.Tx, product, req.DefaultFlowTemplateID); err != nil {
			return err
		}
		existing := repositories.ProductServiceProfileRepository.GetByProductID(ctx.Tx, product.ID)
		if existing != nil && existing.Status != enums.StatusDeleted {
			return errorsx.InvalidParam("product service profile already exists")
		}
		if existing != nil {
			now := time.Now()
			if err := repositories.ProductServiceProfileRepository.Updates(ctx.Tx, existing.ID, s.buildUpdateColumns(req, product, config, operator, now, true)); err != nil {
				return err
			}
			item = repositories.ProductServiceProfileRepository.Get(ctx.Tx, existing.ID)
			return nil
		}

		item = &models.ProductServiceProfile{
			TenantID:               product.TenantID,
			ProductID:              product.ID,
			SupportLocalesJSON:     config.SupportLocalesJSON,
			SupportRegionsJSON:     config.SupportRegionsJSON,
			WarrantyPolicyJSON:     config.WarrantyPolicyJSON,
			SafetyLevel:            strings.TrimSpace(req.SafetyLevel),
			DefaultFlowTemplateID:  req.DefaultFlowTemplateID,
			DefaultKnowledgeBaseID: req.DefaultKnowledgeBaseID,
			MeetingEnabled:         req.MeetingEnabled,
			ServicePolicyJSON:      config.ServicePolicyJSON,
			Status:                 enums.StatusOk,
			AuditFields:            utils.BuildAuditFields(operator),
		}
		return repositories.ProductServiceProfileRepository.Create(ctx.Tx, item)
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *productServiceProfileService) UpdateProductServiceProfile(req request.UpdateProductServiceProfileRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(req.ID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product service profile not found")
	}
	product, err := s.requireProduct(req.TenantID, req.ProductID)
	if err != nil {
		return err
	}
	config, err := normalizeProductServiceProfileJSON(req.CreateProductServiceProfileRequest)
	if err != nil {
		return err
	}
	existing := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if existing != nil && existing.ID != req.ID {
		return errorsx.InvalidParam("product service profile already exists")
	}

	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := s.lockDefaultFlowTemplate(ctx.Tx, product, req.DefaultFlowTemplateID); err != nil {
			return err
		}
		current := repositories.ProductServiceProfileRepository.Get(ctx.Tx, req.ID)
		if current == nil || current.Status == enums.StatusDeleted || current.TenantID != product.TenantID || current.ProductID != product.ID {
			return errorsx.InvalidParam("product service profile not found")
		}
		return repositories.ProductServiceProfileRepository.Updates(ctx.Tx, req.ID, s.buildUpdateColumns(req.CreateProductServiceProfileRequest, product, config, operator, time.Now(), false))
	})
}

func (s *productServiceProfileService) DeleteProductServiceProfile(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product service profile not found")
	}
	return repositories.ProductServiceProfileRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *productServiceProfileService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product service profile not found")
	}
	return repositories.ProductServiceProfileRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *productServiceProfileService) requireProduct(tenantID int64, productID int64) (*models.Product, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, err
	}
	if productID <= 0 {
		return nil, errorsx.InvalidParam("productId is required")
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	if product.TenantID != tenantID {
		return nil, errorsx.InvalidParam("product does not belong to tenant")
	}
	return product, nil
}

func (s *productServiceProfileService) lockDefaultFlowTemplate(db *gorm.DB, product *models.Product, workflowID int64) error {
	if workflowID <= 0 {
		return nil
	}
	workflow := repositories.AIWorkflowRepository.GetForUpdate(db, workflowID)
	if workflow == nil || workflow.Status == enums.StatusDeleted || workflow.AgentID != 0 || workflow.CurrentStableVersionID <= 0 {
		return errorsx.InvalidParam("default workflow template is not an enabled reusable template")
	}
	switch workflow.Scope {
	case models.AIWorkflowScopePlatform:
		if workflow.TenantID != 0 || !workflow.Locked {
			return errorsx.InvalidParam("default workflow template is not an enabled reusable template")
		}
	case models.AIWorkflowScopeTenant:
		if workflow.TenantID != product.TenantID || workflow.Locked {
			return errorsx.Forbidden("default workflow template belongs to another tenant")
		}
	default:
		return errorsx.InvalidParam("default workflow template scope is invalid")
	}
	version := repositories.AIWorkflowVersionRepository.Get(db, workflow.CurrentStableVersionID)
	if version == nil || version.Status != enums.StatusOk || version.WorkflowID != workflow.ID || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return errorsx.InvalidParam("default workflow template has no enabled stable version")
	}
	return nil
}

func (s *productServiceProfileService) buildUpdateColumns(req request.CreateProductServiceProfileRequest, product *models.Product, config productServiceProfileJSONConfig, operator *dto.AuthPrincipal, now time.Time, includeCreateAudit bool) map[string]any {
	columns := map[string]any{
		"tenant_id":                 product.TenantID,
		"product_id":                product.ID,
		"support_locales_json":      config.SupportLocalesJSON,
		"support_regions_json":      config.SupportRegionsJSON,
		"warranty_policy_json":      config.WarrantyPolicyJSON,
		"safety_level":              strings.TrimSpace(req.SafetyLevel),
		"default_flow_template_id":  req.DefaultFlowTemplateID,
		"default_knowledge_base_id": req.DefaultKnowledgeBaseID,
		"meeting_enabled":           req.MeetingEnabled,
		"service_policy_json":       config.ServicePolicyJSON,
		"update_user_id":            operator.UserID,
		"update_user_name":          operator.Username,
		"updated_at":                now,
	}
	if includeCreateAudit {
		columns["status"] = enums.StatusOk
		columns["create_user_id"] = operator.UserID
		columns["create_user_name"] = operator.Username
		columns["created_at"] = now
	}
	return columns
}

func normalizeProductServiceProfileJSON(req request.CreateProductServiceProfileRequest) (productServiceProfileJSONConfig, error) {
	supportLocalesJSON, err := normalizeJSON(req.SupportLocalesJSON, "[]")
	if err != nil {
		return productServiceProfileJSONConfig{}, err
	}
	supportRegionsJSON, err := normalizeJSON(req.SupportRegionsJSON, "[]")
	if err != nil {
		return productServiceProfileJSONConfig{}, err
	}
	warrantyPolicyJSON, err := normalizeJSON(req.WarrantyPolicyJSON, "{}")
	if err != nil {
		return productServiceProfileJSONConfig{}, err
	}
	servicePolicyJSON, err := normalizeJSON(req.ServicePolicyJSON, "{}")
	if err != nil {
		return productServiceProfileJSONConfig{}, err
	}
	return productServiceProfileJSONConfig{
		SupportLocalesJSON: supportLocalesJSON,
		SupportRegionsJSON: supportRegionsJSON,
		WarrantyPolicyJSON: warrantyPolicyJSON,
		ServicePolicyJSON:  servicePolicyJSON,
	}, nil
}
