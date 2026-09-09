package services

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductModuleService = newProductModuleService()

func newProductModuleService() *productModuleService {
	return &productModuleService{}
}

type productModuleService struct{}

type CreateProductModuleRequest struct {
	TenantID          int64  `json:"tenantId"`
	ProductID         int64  `json:"productId"`
	ModuleCode        string `json:"moduleCode"`
	Name              string `json:"name"`
	DefaultSupplierID int64  `json:"defaultSupplierId"`
	IsSafetyCritical  bool   `json:"isSafetyCritical"`
}

type UpdateProductModuleRequest struct {
	ID                int64  `json:"id"`
	ModuleCode        string `json:"moduleCode"`
	Name              string `json:"name"`
	DefaultSupplierID int64  `json:"defaultSupplierId"`
	IsSafetyCritical  bool   `json:"isSafetyCritical"`
}

func (s *productModuleService) Get(id int64) *models.ProductModule {
	if id <= 0 {
		return nil
	}
	return repositories.ProductModuleRepository.Get(sqls.DB(), id)
}

func (s *productModuleService) FindByProductID(productID int64) []models.ProductModule {
	return repositories.ProductModuleRepository.FindByProductID(sqls.DB(), productID)
}

func (s *productModuleService) FindPageByCnd(cnd *sqls.Cnd) ([]models.ProductModule, *sqls.Paging) {
	return repositories.ProductModuleRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *productModuleService) CreateModule(req CreateProductModuleRequest, operator *dto.AuthPrincipal) (*models.ProductModule, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	code := strings.TrimSpace(req.ModuleCode)
	name := strings.TrimSpace(req.Name)
	if code == "" {
		return nil, errorsx.InvalidParam("module code is required")
	}
	if name == "" {
		return nil, errorsx.InvalidParam("module name is required")
	}
	if req.TenantID <= 0 || req.ProductID <= 0 {
		return nil, errorsx.InvalidParam("tenantId and productId are required")
	}
	if operator.TenantID > 0 && operator.TenantID != req.TenantID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	product := repositories.ProductRepository.Get(sqls.DB(), req.ProductID)
	if product == nil || product.TenantID != req.TenantID || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product does not belong to the current tenant")
	}
	if err := validateProductModuleSupplier(sqls.DB(), req.TenantID, req.DefaultSupplierID); err != nil {
		return nil, err
	}

	existing := repositories.ProductModuleRepository.GetByTenantProductCode(sqls.DB(), req.TenantID, req.ProductID, code)
	if existing != nil {
		return nil, errorsx.InvalidParam("module code already exists for this product")
	}

	now := time.Now()
	module := &models.ProductModule{
		TenantID:          req.TenantID,
		ProductID:         req.ProductID,
		ModuleCode:        code,
		Name:              name,
		DefaultSupplierID: req.DefaultSupplierID,
		IsSafetyCritical:  req.IsSafetyCritical,
		Status:            enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}

	if err := repositories.ProductModuleRepository.Create(sqls.DB(), module); err != nil {
		return nil, err
	}
	return module, nil
}

func (s *productModuleService) UpdateModule(req UpdateProductModuleRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	existing := s.Get(req.ID)
	if existing == nil {
		return errorsx.InvalidParamI18n("error.e0178")
	}
	if operator.TenantID > 0 && operator.TenantID != existing.TenantID {
		return errorsx.ForbiddenI18n("error.e0225")
	}

	code := strings.TrimSpace(req.ModuleCode)
	name := strings.TrimSpace(req.Name)
	if code == "" {
		return errorsx.InvalidParam("module code is required")
	}
	if name == "" {
		return errorsx.InvalidParam("module name is required")
	}
	if err := validateProductModuleSupplier(sqls.DB(), existing.TenantID, req.DefaultSupplierID); err != nil {
		return err
	}

	// Check uniqueness if code changed
	if code != existing.ModuleCode {
		dup := repositories.ProductModuleRepository.GetByTenantProductCode(sqls.DB(), existing.TenantID, existing.ProductID, code)
		if dup != nil && dup.ID != existing.ID {
			return errorsx.InvalidParam("module code already exists for this product")
		}
	}

	now := time.Now()
	return repositories.ProductModuleRepository.Updates(sqls.DB(), existing.ID, map[string]any{
		"module_code":         code,
		"name":                name,
		"default_supplier_id": req.DefaultSupplierID,
		"is_safety_critical":  req.IsSafetyCritical,
		"updated_at":          now,
		"update_user_id":      operator.UserID,
		"update_user_name":    operator.Username,
	})
}

func validateProductModuleSupplier(db *gorm.DB, tenantID, supplierID int64) error {
	if supplierID <= 0 {
		return nil
	}
	supplier := repositories.EnterpriseIAMRepository.GetPartnerCompany(db, tenantID, supplierID)
	if supplier == nil || supplier.Status != enums.StatusOk {
		return errorsx.InvalidParam("default supplier must be an active supplier in the current tenant")
	}
	return nil
}

func (s *productModuleService) DeleteModule(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	existing := s.Get(id)
	if existing == nil {
		return errorsx.InvalidParamI18n("error.e0178")
	}
	now := time.Now()
	return repositories.ProductModuleRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"updated_at":       now,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	})
}

func (s *productModuleService) CreateModelLink(moduleID, modelID int64, tenantID int64, operator *dto.AuthPrincipal) (*models.ProductModuleModelLink, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if moduleID <= 0 || modelID <= 0 {
		return nil, errorsx.InvalidParam("moduleId and modelId are required")
	}

	now := time.Now()
	link := &models.ProductModuleModelLink{
		TenantID:        tenantID,
		ProductModuleID: moduleID,
		ProductModelID:  modelID,
		Status:          enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}

	if err := repositories.ProductModuleModelLinkRepository.Create(sqls.DB(), link); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *productModuleService) DeleteModelLink(id int64) error {
	// 关联为纯关系行，采用物理删除以配合联合唯一索引（§5.1）。
	repositories.ProductModuleModelLinkRepository.Delete(sqls.DB(), id)
	return nil
}

// requireTenantModule 校验模块属于指定租户和产品且未删除。
func (s *productModuleService) requireTenantModule(tenantID, productID, moduleID int64) (*models.ProductModule, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, err
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.TenantID != tenantID || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	module := s.Get(moduleID)
	if module == nil || module.Status == enums.StatusDeleted || module.TenantID != tenantID || module.ProductID != productID {
		return nil, errorsx.InvalidParam("product module not found")
	}
	return module, nil
}

// EnableModule 启用模块（租户内校验）。
func (s *productModuleService) EnableModule(tenantID, productID, moduleID int64, operator *dto.AuthPrincipal) error {
	module, err := s.requireTenantModule(tenantID, productID, moduleID)
	if err != nil {
		return err
	}
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if module.Status == enums.StatusOk {
		return nil
	}
	return repositories.ProductModuleRepository.Updates(sqls.DB(), module.ID, map[string]any{
		"status":           enums.StatusOk,
		"updated_at":       time.Now(),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	})
}

// DisableModule 停用模块（租户内校验）。
func (s *productModuleService) DisableModule(tenantID, productID, moduleID int64, operator *dto.AuthPrincipal) error {
	module, err := s.requireTenantModule(tenantID, productID, moduleID)
	if err != nil {
		return err
	}
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if module.Status == enums.StatusDisabled {
		return nil
	}
	return repositories.ProductModuleRepository.Updates(sqls.DB(), module.ID, map[string]any{
		"status":           enums.StatusDisabled,
		"updated_at":       time.Now(),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	})
}

// ReplaceModelLinks 在事务中整体替换模块适用型号集合（PUT 语义，§10.3）。
// 校验：模块与全部型号同租户同产品、型号未删除、去重。
func (s *productModuleService) ReplaceModelLinks(tenantID, productID, moduleID int64, modelIDs []int64, operator *dto.AuthPrincipal) ([]int64, error) {
	module, err := s.requireTenantModule(tenantID, productID, moduleID)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}

	// 去重并校验每个型号归属。
	unique := make([]int64, 0, len(modelIDs))
	seen := make(map[int64]bool, len(modelIDs))
	for _, modelID := range modelIDs {
		if modelID <= 0 || seen[modelID] {
			continue
		}
		model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
		if model == nil || model.TenantID != tenantID || model.ProductID != productID || model.Status == enums.StatusDeleted {
			return nil, errorsx.InvalidParam("product model not found or does not belong to this product")
		}
		seen[modelID] = true
		unique = append(unique, modelID)
	}

	now := time.Now()
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := repositories.ProductModuleModelLinkRepository.DeleteByModuleID(tx, module.ID); err != nil {
			return err
		}
		for _, modelID := range unique {
			link := &models.ProductModuleModelLink{
				TenantID:        tenantID,
				ProductModuleID: module.ID,
				ProductModelID:  modelID,
				Status:          enums.StatusOk,
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserID:   operator.UserID,
					CreateUserName: operator.Username,
					UpdatedAt:      now,
					UpdateUserID:   operator.UserID,
					UpdateUserName: operator.Username,
				},
			}
			if err := repositories.ProductModuleModelLinkRepository.Create(tx, link); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return unique, nil
}

// ListModuleModelIDs 返回模块当前适用型号 ID（过滤软删除）。
func (s *productModuleService) ListModuleModelIDs(moduleID int64) []int64 {
	links := repositories.ProductModuleModelLinkRepository.FindByModuleID(sqls.DB(), moduleID)
	ids := make([]int64, 0, len(links))
	for _, link := range links {
		if link.Status == enums.StatusDeleted {
			continue
		}
		ids = append(ids, link.ProductModelID)
	}
	return ids
}

func (s *productModuleService) FindModelLinksByModule(moduleID int64) []models.ProductModuleModelLink {
	return repositories.ProductModuleModelLinkRepository.FindByModuleID(sqls.DB(), moduleID)
}

func (s *productModuleService) FindModelLinksByModel(modelID int64) []models.ProductModuleModelLink {
	return repositories.ProductModuleModelLinkRepository.FindByModelID(sqls.DB(), modelID)
}
