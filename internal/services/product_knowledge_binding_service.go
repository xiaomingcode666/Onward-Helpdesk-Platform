package services

import (
	"errors"
	"sort"
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

var ProductKnowledgeBindingService = newProductKnowledgeBindingService()

func newProductKnowledgeBindingService() *productKnowledgeBindingService {
	return &productKnowledgeBindingService{}
}

type productKnowledgeBindingService struct{}

// EnsureDefaultBindingTx keeps the product service profile and the runtime
// knowledge resolver on the same source of truth. An empty locale/region makes
// this the product-wide fallback; model-specific bindings can still override it.
func (s *productKnowledgeBindingService) EnsureDefaultBindingTx(
	db *gorm.DB,
	tenantID, productID, knowledgeBaseID int64,
	operator *dto.AuthPrincipal,
) (*models.ProductKnowledgeBinding, error) {
	if db == nil {
		db = sqls.DB()
	}
	product := repositories.ProductRepository.GetByTenant(db, productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("knowledge base not found for this tenant")
	}

	var existing models.ProductKnowledgeBinding
	err := db.Where(
		"tenant_id = ? AND product_id = ? AND product_model_id = 0 AND knowledge_base_id = ? AND scope_type = ? AND locale = ? AND region_code = ? AND status <> ?",
		tenantID, productID, knowledgeBaseID, "product", "", "", enums.StatusDeleted,
	).Order("id ASC").First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	item := &models.ProductKnowledgeBinding{
		TenantID:        tenantID,
		ProductID:       productID,
		KnowledgeBaseID: knowledgeBaseID,
		ScopeType:       "product",
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(db, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *productKnowledgeBindingService) Get(id int64) *models.ProductKnowledgeBinding {
	if id <= 0 {
		return nil
	}
	return repositories.ProductKnowledgeBindingRepository.Get(sqls.DB(), id)
}

func (s *productKnowledgeBindingService) FindPageByCnd(cnd *sqls.Cnd) ([]models.ProductKnowledgeBinding, *sqls.Paging) {
	return repositories.ProductKnowledgeBindingRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *productKnowledgeBindingService) Create(req request.CreateProductKnowledgeBindingRequest, operator *dto.AuthPrincipal) (*models.ProductKnowledgeBinding, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if operator.TenantID > 0 && operator.TenantID != req.TenantID {
		return nil, errorsx.InvalidParam("tenant does not match current operator")
	}
	product, model, err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID, req.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	scope, locale, region, err := normalizeBinding(req.ScopeType, req.ProductModelID, req.Locale, req.RegionCode)
	if err != nil {
		return nil, err
	}
	item := &models.ProductKnowledgeBinding{TenantID: product.TenantID, ProductID: product.ID, ProductModelID: 0, KnowledgeBaseID: req.KnowledgeBaseID, ScopeType: scope, Locale: locale, RegionCode: region, SortNo: req.SortNo, Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator)}
	if model != nil {
		item.ProductModelID = model.ID
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *productKnowledgeBindingService) Update(req request.UpdateProductKnowledgeBindingRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(req.ID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	if item.TenantID != req.TenantID {
		return errorsx.InvalidParam("product knowledge binding does not belong to tenant")
	}
	product, model, err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID, req.KnowledgeBaseID)
	if err != nil {
		return err
	}
	scope, locale, region, err := normalizeBinding(req.ScopeType, req.ProductModelID, req.Locale, req.RegionCode)
	if err != nil {
		return err
	}
	modelID := int64(0)
	if model != nil {
		modelID = model.ID
	}
	return repositories.ProductKnowledgeBindingRepository.Updates(sqls.DB(), req.ID, map[string]any{"tenant_id": product.TenantID, "product_id": product.ID, "product_model_id": modelID, "knowledge_base_id": req.KnowledgeBaseID, "scope_type": scope, "locale": locale, "region_code": region, "sort_no": req.SortNo, "update_user_id": operator.UserID, "update_user_name": operator.Username, "updated_at": time.Now()})
}

func (s *productKnowledgeBindingService) Delete(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	return repositories.ProductKnowledgeBindingRepository.Updates(sqls.DB(), id, map[string]any{"status": enums.StatusDeleted, "update_user_id": operator.UserID, "update_user_name": operator.Username, "updated_at": time.Now()})
}

func (s *productKnowledgeBindingService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge binding not found")
	}
	return repositories.ProductKnowledgeBindingRepository.Updates(sqls.DB(), id, map[string]any{"status": status, "update_user_id": operator.UserID, "update_user_name": operator.Username, "updated_at": time.Now()})
}

func (s *productKnowledgeBindingService) Resolve(req request.ResolveProductKnowledgeBindingRequest) ([]models.KnowledgeBase, error) {
	product, model, err := s.validateProductModel(req.TenantID, req.ProductID, req.ProductModelID)
	if err != nil {
		return nil, err
	}
	bindings := repositories.ProductKnowledgeBindingRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", req.TenantID).Eq("product_id", product.ID).Eq("status", enums.StatusOk))
	type candidate struct {
		binding               models.ProductKnowledgeBinding
		kb                    models.KnowledgeBase
		scope, locale, region int
	}
	candidates := make([]candidate, 0, len(bindings))
	for _, binding := range bindings {
		if binding.ProductModelID != 0 && (model == nil || binding.ProductModelID != model.ID) {
			continue
		}
		if binding.ProductModelID == 0 && binding.ScopeType == "model" {
			continue
		}
		locale := matchRank(binding.Locale, req.Locale)
		region := matchRank(binding.RegionCode, req.RegionCode)
		if locale < 0 || region < 0 {
			continue
		}
		kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), binding.KnowledgeBaseID)
		if kb == nil || kb.TenantID != req.TenantID || kb.Status != enums.StatusOk {
			continue
		}
		candidates = append(candidates, candidate{binding: binding, kb: *kb, scope: boolRank(binding.ProductModelID != 0), locale: locale, region: region})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.scope != b.scope {
			return a.scope > b.scope
		}
		if a.locale != b.locale {
			return a.locale > b.locale
		}
		if a.region != b.region {
			return a.region > b.region
		}
		if a.binding.SortNo != b.binding.SortNo {
			return a.binding.SortNo < b.binding.SortNo
		}
		return a.binding.ID < b.binding.ID
	})
	result := make([]models.KnowledgeBase, 0, len(candidates))
	seen := make(map[int64]bool)
	for _, item := range candidates {
		if !seen[item.kb.ID] {
			seen[item.kb.ID] = true
			result = append(result, item.kb)
		}
	}
	return result, nil
}

func (s *productKnowledgeBindingService) validateReferences(tenantID, productID, modelID, knowledgeBaseID int64) (*models.Product, *models.ProductModel, error) {
	product, model, err := s.validateProductModel(tenantID, productID, modelID)
	if err != nil {
		return nil, nil, err
	}
	if knowledgeBaseID <= 0 {
		return nil, nil, errorsx.InvalidParam("knowledgeBaseId is required")
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if kb == nil || kb.TenantID != tenantID || kb.Status != enums.StatusOk {
		return nil, nil, errorsx.InvalidParam("knowledge base not found or disabled")
	}
	return product, model, nil
}

func (s *productKnowledgeBindingService) validateProductModel(tenantID, productID, modelID int64) (*models.Product, *models.ProductModel, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, nil, err
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status != enums.StatusOk || product.TenantID != tenantID {
		return nil, nil, errorsx.InvalidParam("product not found or disabled")
	}
	if modelID <= 0 {
		return product, nil, nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
	if model == nil || model.Status != enums.StatusOk || model.TenantID != tenantID || model.ProductID != product.ID {
		return nil, nil, errorsx.InvalidParam("product model not found or does not belong to product")
	}
	return product, model, nil
}

func normalizeBinding(scope string, modelID int64, locale, region string) (string, string, string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		if modelID > 0 {
			scope = "model"
		} else {
			scope = "product"
		}
	}
	if scope != "product" && scope != "model" {
		return "", "", "", errorsx.InvalidParam("scopeType must be product or model")
	}
	if scope == "model" && modelID <= 0 {
		return "", "", "", errorsx.InvalidParam("model scope requires productModelId")
	}
	if scope == "product" && modelID > 0 {
		return "", "", "", errorsx.InvalidParam("product scope cannot set productModelId")
	}
	return scope, strings.TrimSpace(locale), strings.TrimSpace(region), nil
}

func matchRank(value, requested string) int {
	value, requested = strings.TrimSpace(value), strings.TrimSpace(requested)
	if value == "" {
		return 0
	}
	if value == requested && requested != "" {
		return 1
	}
	return -1
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}
