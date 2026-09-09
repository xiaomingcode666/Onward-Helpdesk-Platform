package builders

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildResolvedServiceCode(item *models.ServiceCode) *response.ResolvedServiceCodeResponse {
	if item == nil {
		return nil
	}
	return &response.ResolvedServiceCodeResponse{
		ID:          item.ID,
		ServiceCode: servicecode.Normalize(item.ServiceCode),
		EntryURL:    servicecode.BuildEntryURL(item.ServiceCode),
		QRURL:       servicecode.BuildQRURL(item.ServiceCode),
		QRImageURL:  servicecode.BuildQRImageURL(item.ServiceCode),
		Mode:        string(item.Mode),
		Status:      string(item.Status),
	}
}

func BuildResolvedTenant(item *models.Tenant) *response.ResolvedTenantResponse {
	if item == nil {
		return nil
	}
	return &response.ResolvedTenantResponse{
		ID:            item.ID,
		Name:          strings.TrimSpace(item.Name),
		DefaultLocale: strings.TrimSpace(item.DefaultLocale),
		Timezone:      strings.TrimSpace(item.Timezone),
	}
}

func BuildResolvedProduct(item *models.Product) *response.ResolvedProductResponse {
	if item == nil {
		return nil
	}
	return &response.ResolvedProductResponse{
		ID:       item.ID,
		Code:     strings.TrimSpace(item.Code),
		Name:     strings.TrimSpace(item.Name),
		Category: strings.TrimSpace(item.Category),
	}
}

func BuildResolvedProductModel(item *models.ProductModel) *response.ResolvedProductModelResponse {
	if item == nil {
		return nil
	}
	return &response.ResolvedProductModelResponse{
		ID:        item.ID,
		ModelCode: strings.TrimSpace(item.ModelCode),
		Name:      strings.TrimSpace(item.Name),
	}
}

func BuildResolvedDevice(item *models.Device) *response.ResolvedDeviceResponse {
	if item == nil {
		return nil
	}
	return &response.ResolvedDeviceResponse{
		ID:         item.ID,
		DeviceNo:   strings.TrimSpace(item.DeviceNo),
		SerialNo:   strings.TrimSpace(item.SerialNo),
		RegionCode: strings.TrimSpace(item.RegionCode),
	}
}

func BuildProduct(item *models.Product) *response.ProductResponse {
	if item == nil {
		return nil
	}
	return &response.ProductResponse{
		ID:            item.ID,
		TenantID:      item.TenantID,
		ProductLineID: item.ProductLineID,
		Code:          strings.TrimSpace(item.Code),
		Name:          strings.TrimSpace(item.Name),
		Category:      strings.TrimSpace(item.Category),
		OwnerMemberID: item.OwnerMemberID,
		DefaultLocale: strings.TrimSpace(item.DefaultLocale),
		Status:        int(item.Status),
		CreatedAt:     utils.FormatTime(item.CreatedAt),
		UpdatedAt:     utils.FormatTime(item.UpdatedAt),
	}
}

func BuildProductList(list []models.Product) []response.ProductResponse {
	results := make([]response.ProductResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildProduct(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}

func BuildProductModel(item *models.ProductModel) *response.ProductModelResponse {
	if item == nil {
		return nil
	}
	return &response.ProductModelResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		ProductID:       item.ProductID,
		ModelCode:       strings.TrimSpace(item.ModelCode),
		Name:            strings.TrimSpace(item.Name),
		VersionPolicy:   strings.TrimSpace(item.VersionPolicy),
		RegionScopeJSON: strings.TrimSpace(item.RegionScopeJSON),
		Status:          int(item.Status),
		CreatedAt:       utils.FormatTime(item.CreatedAt),
		UpdatedAt:       utils.FormatTime(item.UpdatedAt),
	}
}

func BuildProductModelList(list []models.ProductModel) []response.ProductModelResponse {
	results := make([]response.ProductModelResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildProductModel(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}

func BuildDevice(item *models.Device) *response.DeviceResponse {
	if item == nil {
		return nil
	}
	return &response.DeviceResponse{
		ID:                  item.ID,
		TenantID:            item.TenantID,
		DeviceNo:            strings.TrimSpace(item.DeviceNo),
		ProductID:           item.ProductID,
		ProductModelID:      item.ProductModelID,
		SerialNo:            strings.TrimSpace(item.SerialNo),
		CustomerOrgID:       item.CustomerOrgID,
		ExternalDeviceID:    strings.TrimSpace(item.ExternalDeviceID),
		InstallLocationJSON: strings.TrimSpace(item.InstallLocationJSON),
		RegionCode:          strings.TrimSpace(item.RegionCode),
		Source:              strings.TrimSpace(item.Source),
		Status:              int(item.Status),
		MetadataJSON:        strings.TrimSpace(item.MetadataJSON),
		CreatedAt:           utils.FormatTime(item.CreatedAt),
		UpdatedAt:           utils.FormatTime(item.UpdatedAt),
	}
}

func BuildDeviceList(list []models.Device) []response.DeviceResponse {
	results := make([]response.DeviceResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildDevice(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}
