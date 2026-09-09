package builders

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildServiceCodeBatch(item *models.ServiceCodeBatch) *response.ServiceCodeBatchResponse {
	if item == nil {
		return nil
	}
	return &response.ServiceCodeBatchResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		BatchNo:         strings.TrimSpace(item.BatchNo),
		Mode:            string(item.Mode),
		ProductID:       item.ProductID,
		ProductModelID:  item.ProductModelID,
		Quantity:        item.Quantity,
		LabelTemplateID: item.LabelTemplateID,
		Status:          int(item.Status),
		GeneratedCount:  item.GeneratedCount,
		ExportedCount:   item.ExportedCount,
		CreatedAt:       utils.FormatTime(item.CreatedAt),
		UpdatedAt:       utils.FormatTime(item.UpdatedAt),
	}
}

func BuildServiceCodeBatchList(list []models.ServiceCodeBatch) []response.ServiceCodeBatchResponse {
	results := make([]response.ServiceCodeBatchResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildServiceCodeBatch(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}

func BuildServiceCode(item *models.ServiceCode) *response.ServiceCodeResponse {
	if item == nil {
		return nil
	}
	return &response.ServiceCodeResponse{
		ID:             item.ID,
		TenantID:       item.TenantID,
		BatchID:        item.BatchID,
		ServiceCode:    servicecode.Normalize(item.ServiceCode),
		EntryURL:       servicecode.BuildEntryURL(item.ServiceCode),
		QRURL:          servicecode.BuildQRURL(item.ServiceCode),
		QRImageURL:     servicecode.BuildQRImageURL(item.ServiceCode),
		Mode:           string(item.Mode),
		DeviceID:       item.DeviceID,
		ProductID:      item.ProductID,
		ProductModelID: item.ProductModelID,
		Status:         string(item.Status),
		ActivatedAt:    utils.FormatTimePtr(item.ActivatedAt),
		RevokedAt:      utils.FormatTimePtr(item.RevokedAt),
		MetadataJSON:   strings.TrimSpace(item.MetadataJSON),
		CreatedAt:      utils.FormatTime(item.CreatedAt),
		UpdatedAt:      utils.FormatTime(item.UpdatedAt),
	}
}

func BuildServiceCodeList(list []models.ServiceCode) []response.ServiceCodeResponse {
	results := make([]response.ServiceCodeResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildServiceCode(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}
