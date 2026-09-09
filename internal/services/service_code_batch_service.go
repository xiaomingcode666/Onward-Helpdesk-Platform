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

var ServiceCodeBatchService = newServiceCodeBatchService()

func newServiceCodeBatchService() *serviceCodeBatchService {
	return &serviceCodeBatchService{}
}

type serviceCodeBatchService struct {
}

func (s *serviceCodeBatchService) Get(id int64) *models.ServiceCodeBatch {
	if id <= 0 {
		return nil
	}
	return repositories.ServiceCodeBatchRepository.Get(sqls.DB(), id)
}

func (s *serviceCodeBatchService) Find(cnd *sqls.Cnd) []models.ServiceCodeBatch {
	return repositories.ServiceCodeBatchRepository.Find(sqls.DB(), cnd)
}

func (s *serviceCodeBatchService) FindPageByCnd(cnd *sqls.Cnd) (list []models.ServiceCodeBatch, paging *sqls.Paging) {
	return repositories.ServiceCodeBatchRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *serviceCodeBatchService) CreateBatch(req request.CreateServiceCodeBatchRequest, operator *dto.AuthPrincipal) (*models.ServiceCodeBatch, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID); err != nil {
		return nil, err
	}

	batchNo := normalizeCode(req.BatchNo)
	if batchNo == "" {
		return nil, errorsx.InvalidParam("batchNo is required")
	}
	mode, err := normalizeServiceCodeMode(req.Mode)
	if err != nil {
		return nil, err
	}
	if req.Quantity <= 0 {
		return nil, errorsx.InvalidParam("quantity must be greater than 0")
	}

	existing := repositories.ServiceCodeBatchRepository.GetByTenantBatchNo(sqls.DB(), req.TenantID, batchNo)
	if existing != nil {
		return nil, errorsx.InvalidParam("batchNo already exists")
	}

	item := &models.ServiceCodeBatch{
		TenantID:        req.TenantID,
		BatchNo:         batchNo,
		Mode:            mode,
		ProductID:       req.ProductID,
		ProductModelID:  req.ProductModelID,
		Quantity:        req.Quantity,
		LabelTemplateID: req.LabelTemplateID,
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err := repositories.ServiceCodeBatchRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *serviceCodeBatchService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("service code batch not found")
	}
	return repositories.ServiceCodeBatchRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *serviceCodeBatchService) DeleteBatch(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("service code batch not found")
	}
	return repositories.ServiceCodeBatchRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *serviceCodeBatchService) validateReferences(tenantID int64, productID int64, productModelID int64) error {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return err
	}
	if productID <= 0 {
		return errorsx.InvalidParam("productId is required")
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product not found")
	}
	if product.TenantID != tenantID {
		return errorsx.InvalidParam("product does not belong to tenant")
	}
	if productModelID <= 0 {
		return nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), productModelID)
	if model == nil || model.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product model not found")
	}
	if model.TenantID != tenantID || model.ProductID != productID {
		return errorsx.InvalidParam("product model does not belong to product")
	}
	return nil
}

func normalizeServiceCodeMode(value string) (enums.ServiceCodeMode, error) {
	mode := enums.ServiceCodeMode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case enums.ServiceCodeModeTraceable, enums.ServiceCodeModeGeneral:
		return mode, nil
	default:
		return "", errorsx.InvalidParam("invalid service code mode")
	}
}
