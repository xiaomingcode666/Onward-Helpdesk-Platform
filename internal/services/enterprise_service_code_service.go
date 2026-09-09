package services

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EnterpriseServiceCodeService = newEnterpriseServiceCodeService()

func newEnterpriseServiceCodeService() *enterpriseServiceCodeService {
	return &enterpriseServiceCodeService{}
}

type enterpriseServiceCodeService struct{}

type EnterpriseServiceCodeQuery struct {
	Search    string
	Status    string
	ProductID int64
	Page      int
	PageSize  int
}

func (s *enterpriseServiceCodeService) ListCodes(tenantID int64, query EnterpriseServiceCodeQuery) (*dto.EnterpriseListResponse[dto.EnterpriseServiceCodeListItemDTO], error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	query.Page, query.PageSize = normalizeEnterpriseServiceCodePage(query.Page, query.PageSize)
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID)
	if query.ProductID > 0 {
		cnd.Eq("product_id", query.ProductID)
	}
	if query.Status != "" && query.Status != "all" {
		cnd.Eq("status", query.Status)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + search + "%"
		productIDs := s.matchingProductIDs(tenantID, search)
		batchIDs := s.matchingBatchIDs(tenantID, search)
		switch {
		case len(productIDs) > 0 && len(batchIDs) > 0:
			cnd.Where("service_code LIKE ? OR product_id in ? OR batch_id in ?", pattern, productIDs, batchIDs)
		case len(productIDs) > 0:
			cnd.Where("service_code LIKE ? OR product_id in ?", pattern, productIDs)
		case len(batchIDs) > 0:
			cnd.Where("service_code LIKE ? OR batch_id in ?", pattern, batchIDs)
		default:
			cnd.Where("service_code LIKE ?", pattern)
		}
	}
	cnd.Desc("created_at").Desc("id")
	cnd.Page(query.Page, query.PageSize)
	codes, paging := repositories.ServiceCodeRepository.FindPageByCnd(sqls.DB(), cnd)
	items := make([]dto.EnterpriseServiceCodeListItemDTO, 0, len(codes))
	for _, item := range codes {
		row := s.buildCodeItem(item)
		items = append(items, row)
	}
	return enterprisePage(items, paging, query.Page, query.PageSize), nil
}

func (s *enterpriseServiceCodeService) ListBatches(tenantID int64, query EnterpriseServiceCodeQuery) (*dto.EnterpriseListResponse[dto.EnterpriseServiceCodeBatchListItemDTO], error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	query.Page, query.PageSize = normalizeEnterpriseServiceCodePage(query.Page, query.PageSize)
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted)
	if query.ProductID > 0 {
		cnd.Eq("product_id", query.ProductID)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + search + "%"
		productIDs := s.matchingProductIDs(tenantID, search)
		if len(productIDs) > 0 {
			cnd.Where("batch_no LIKE ? OR product_id in ?", pattern, productIDs)
		} else {
			cnd.Where("batch_no LIKE ?", pattern)
		}
	}
	cnd.Desc("created_at").Desc("id")
	cnd.Page(query.Page, query.PageSize)
	batches, paging := repositories.ServiceCodeBatchRepository.FindPageByCnd(sqls.DB(), cnd)
	items := make([]dto.EnterpriseServiceCodeBatchListItemDTO, 0, len(batches))
	for _, item := range batches {
		row := s.buildBatchItem(item)
		items = append(items, row)
	}
	return enterprisePage(items, paging, query.Page, query.PageSize), nil
}

func normalizeEnterpriseServiceCodePage(page, pageSize int) (int, int) {
	if pageSize <= 0 {
		pageSize = 50
	}
	return normalizeEnterprisePage(page, pageSize)
}

func (s *enterpriseServiceCodeService) matchingProductIDs(tenantID int64, search string) []int64 {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil
	}
	pattern := "%" + strings.ToLower(search) + "%"
	products := repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Where("LOWER(name) LIKE ? OR LOWER(code) LIKE ?", pattern, pattern))
	ids := make([]int64, 0, len(products))
	for _, product := range products {
		ids = append(ids, product.ID)
	}
	return ids
}

func (s *enterpriseServiceCodeService) matchingBatchIDs(tenantID int64, search string) []int64 {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil
	}
	pattern := "%" + search + "%"
	batches := repositories.ServiceCodeBatchRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Where("batch_no LIKE ?", pattern))
	ids := make([]int64, 0, len(batches))
	for _, batch := range batches {
		ids = append(ids, batch.ID)
	}
	return ids
}

func (s *enterpriseServiceCodeService) buildCodeItem(item models.ServiceCode) dto.EnterpriseServiceCodeListItemDTO {
	row := dto.EnterpriseServiceCodeListItemDTO{
		ID:          item.ID,
		ServiceCode: servicecode.Normalize(item.ServiceCode),
		EntryURL:    servicecode.BuildEntryURL(item.ServiceCode),
		QRURL:       servicecode.BuildQRURL(item.ServiceCode),
		QRImageURL:  servicecode.BuildQRImageURL(item.ServiceCode),
		Mode:        string(item.Mode),
		Status:      string(item.Status),
		CreatedAt:   formatEnterpriseTime(item.CreatedAt),
	}
	if item.BatchID > 0 {
		if batch := repositories.ServiceCodeBatchRepository.Get(sqls.DB(), item.BatchID); batch != nil {
			row.BatchNo = batch.BatchNo
		}
	}
	if item.ProductID > 0 {
		if product := repositories.ProductRepository.Get(sqls.DB(), item.ProductID); product != nil {
			row.ProductName = product.Name
		}
	}
	if item.DeviceID > 0 {
		if device := repositories.DeviceRepository.Get(sqls.DB(), item.DeviceID); device != nil {
			value := device.DeviceNo
			row.BoundDevice = &value
		}
	}
	if activatedAt := formatEnterpriseTimePtr(item.ActivatedAt); activatedAt != "" {
		row.ActivatedAt = &activatedAt
	}
	return row
}

func (s *enterpriseServiceCodeService) buildBatchItem(item models.ServiceCodeBatch) dto.EnterpriseServiceCodeBatchListItemDTO {
	row := dto.EnterpriseServiceCodeBatchListItemDTO{
		ID:        item.ID,
		BatchNo:   item.BatchNo,
		Mode:      string(item.Mode),
		Quantity:  item.Quantity,
		Generated: item.GeneratedCount,
		Exported:  item.ExportedCount,
		Status:    mapServiceCodeBatchStatus(item),
		CreatedAt: formatEnterpriseTime(item.CreatedAt),
	}
	if item.ProductID > 0 {
		if product := repositories.ProductRepository.Get(sqls.DB(), item.ProductID); product != nil {
			row.ProductName = product.Name
		}
	}
	return row
}

func mapServiceCodeBatchStatus(item models.ServiceCodeBatch) string {
	if item.Status == enums.StatusDeleted {
		return "cancelled"
	}
	if item.Status != enums.StatusOk {
		return "draft"
	}
	if item.Quantity > 0 && item.GeneratedCount >= item.Quantity {
		return "completed"
	}
	return "active"
}
