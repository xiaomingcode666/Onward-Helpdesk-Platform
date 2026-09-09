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
)

var ProductModelService = newProductModelService()

func newProductModelService() *productModelService {
	return &productModelService{}
}

type productModelService struct {
}

func (s *productModelService) Get(id int64) *models.ProductModel {
	if id <= 0 {
		return nil
	}
	return repositories.ProductModelRepository.Get(sqls.DB(), id)
}

func (s *productModelService) Find(cnd *sqls.Cnd) []models.ProductModel {
	return repositories.ProductModelRepository.Find(sqls.DB(), cnd)
}

func (s *productModelService) FindPageByCnd(cnd *sqls.Cnd) (list []models.ProductModel, paging *sqls.Paging) {
	return repositories.ProductModelRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *productModelService) CreateProductModel(req request.CreateProductModelRequest, operator *dto.AuthPrincipal) (*models.ProductModel, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, err := s.requireProduct(req.TenantID, req.ProductID)
	if err != nil {
		return nil, err
	}

	modelCode := normalizeCode(req.ModelCode)
	name := strings.TrimSpace(req.Name)
	if modelCode == "" {
		return nil, errorsx.InvalidParam("modelCode is required")
	}
	if name == "" {
		return nil, errorsx.InvalidParam("model name is required")
	}
	existing := repositories.ProductModelRepository.GetByProductModelCode(sqls.DB(), product.ID, modelCode)
	if existing != nil {
		return nil, errorsx.InvalidParam("model code already exists")
	}
	regionScopeJSON, err := normalizeJSON(req.RegionScopeJSON, "[]")
	if err != nil {
		return nil, err
	}

	item := &models.ProductModel{
		TenantID:        product.TenantID,
		ProductID:       product.ID,
		ModelCode:       modelCode,
		Name:            name,
		VersionPolicy:   strings.TrimSpace(req.VersionPolicy),
		RegionScopeJSON: regionScopeJSON,
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err := repositories.ProductModelRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *productModelService) UpdateProductModel(req request.UpdateProductModelRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, err := s.requireProduct(req.TenantID, req.ProductID)
	if err != nil {
		return err
	}
	item := s.Get(req.ID)
	if item == nil || item.Status == enums.StatusDeleted || item.TenantID != req.TenantID || item.ProductID != req.ProductID {
		return errorsx.InvalidParam("product model not found")
	}

	modelCode := normalizeCode(req.ModelCode)
	name := strings.TrimSpace(req.Name)
	if modelCode == "" {
		return errorsx.InvalidParam("modelCode is required")
	}
	if name == "" {
		return errorsx.InvalidParam("model name is required")
	}
	existing := repositories.ProductModelRepository.GetByProductModelCode(sqls.DB(), product.ID, modelCode)
	if existing != nil && existing.ID != req.ID {
		return errorsx.InvalidParam("model code already exists")
	}
	regionScopeJSON, err := normalizeJSON(req.RegionScopeJSON, "[]")
	if err != nil {
		return err
	}

	return repositories.ProductModelRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"tenant_id":         product.TenantID,
		"product_id":        product.ID,
		"model_code":        modelCode,
		"name":              name,
		"version_policy":    strings.TrimSpace(req.VersionPolicy),
		"region_scope_json": regionScopeJSON,
		"update_user_id":    operator.UserID,
		"update_user_name":  operator.Username,
		"updated_at":        time.Now(),
	})
}

func (s *productModelService) DeleteProductModel(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product model not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product model not found")
	}
	deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("product_model_id", id).
		Where("status <> ?", enums.StatusDeleted))
	if deviceCount > 0 {
		return errorsx.InvalidParam("product model has related devices")
	}
	return repositories.ProductModelRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *productModelService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product model not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product model not found")
	}
	return repositories.ProductModelRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

// EnableModel 启用型号（租户内校验）。
func (s *productModelService) EnableModel(tenantID, productID, modelID int64, operator *dto.AuthPrincipal) error {
	item, err := s.requireTenantModel(tenantID, productID, modelID)
	if err != nil {
		return err
	}
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if item.Status == enums.StatusOk {
		return nil
	}
	return repositories.ProductModelRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"status":           enums.StatusOk,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

// DisableModel 停用型号（租户内校验）。
// 停用前校验设备、工单和有效知识关联引用，避免下游数据失去型号维度（§10.2）。
func (s *productModelService) DisableModel(tenantID, productID, modelID int64, operator *dto.AuthPrincipal) error {
	item, err := s.requireTenantModel(tenantID, productID, modelID)
	if err != nil {
		return err
	}
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if item.Status == enums.StatusDisabled {
		return nil
	}
	deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_model_id", item.ID).
		Where("status <> ?", enums.StatusDeleted))
	if deviceCount > 0 {
		return errorsx.InvalidParam("product model has related devices; reassign or decommission them before disabling")
	}
	ticketCount := repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_model_id", item.ID).
		Where("status <> ?", enums.TicketStatusClosed).
		Where("status <> ?", enums.TicketStatusDone))
	if ticketCount > 0 {
		return errorsx.InvalidParam("product model has open tickets; close them before disabling")
	}
	linkCount := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_model_id", item.ID).
		Eq("publish_status", "published").
		Where("status <> ?", enums.StatusDeleted))
	if linkCount > 0 {
		return errorsx.InvalidParam("product model has published knowledge links; deprecate them before disabling")
	}
	return repositories.ProductModelRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"status":           enums.StatusDisabled,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

// requireTenantModel 校验型号属于指定租户和产品且未删除。
func (s *productModelService) requireTenantModel(tenantID, productID, modelID int64) (*models.ProductModel, error) {
	if _, err := s.requireProduct(tenantID, productID); err != nil {
		return nil, err
	}
	item := s.Get(modelID)
	if item == nil || item.Status == enums.StatusDeleted || item.TenantID != tenantID || item.ProductID != productID {
		return nil, errorsx.InvalidParam("product model not found")
	}
	return item, nil
}

func (s *productModelService) requireProduct(tenantID int64, productID int64) (*models.Product, error) {
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
