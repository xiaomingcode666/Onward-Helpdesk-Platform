package builders

import (
	"reflect"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildCustomerEntrySession(item *models.CustomerEntrySession, tenant *models.Tenant, product *models.Product, model *models.ProductModel, device *models.Device, consent *models.CustomerPrivacyConsent, serviceCode, visitorToken string) *response.CustomerEntrySessionResponse {
	if item == nil {
		return nil
	}
	return &response.CustomerEntrySessionResponse{
		EntrySessionID: item.ID,
		Tenant:         buildEntrySummary(tenant, "", tenantName(tenant), ""),
		Product:        buildEntrySummary(product, productCode(product), productName(product), ""),
		Model:          buildEntrySummary(model, modelCode(model), modelName(model), modelCode(model)),
		Device:         buildEntrySummary(device, deviceName(device), deviceName(device), ""),
		ServiceCode:    serviceCode,
		ExpiresAt:      utils.FormatTimePtr(item.ExpiresAt),
		Chat:           response.CustomerEntryChatResponse{EntrySessionID: item.ID, VisitorID: item.VisitorID, VisitorToken: visitorToken, Locale: item.Locale},
		PrivacyConsent: BuildCustomerPrivacyConsent(consent),
		Session: map[string]any{
			"entrySessionId": item.ID,
			"customerUserId": item.CustomerUserID,
			"tenantId":       item.TenantID,
			"productId":      item.ProductID,
			"productModelId": item.ProductModelID,
			"deviceId":       item.DeviceID,
			"serviceCodeId":  item.ServiceCodeID,
		},
	}
}

func BuildCustomerPrivacyConsent(item *models.CustomerPrivacyConsent) response.CustomerPrivacyConsentResponse {
	ret := response.CustomerPrivacyConsentResponse{
		Required:      true,
		PolicyVersion: constants.CustomerPrivacyPolicyVersion,
	}
	if item == nil || !item.RequiredAccepted || item.PolicyVersion != constants.CustomerPrivacyPolicyVersion {
		return ret
	}
	ret.Accepted = true
	ret.Analytics = item.AnalyticsAccepted
	ret.Marketing = item.MarketingAccepted
	ret.ConsentedAt = utils.FormatTime(item.ConsentedAt)
	return ret
}

func buildEntrySummary(item any, code, name, modelCode string) *response.CustomerEntrySummaryResponse {
	if item == nil || (reflect.ValueOf(item).Kind() == reflect.Ptr && reflect.ValueOf(item).IsNil()) {
		return nil
	}
	var id int64
	switch v := item.(type) {
	case *models.Tenant:
		id = v.ID
	case *models.Product:
		id = v.ID
	case *models.ProductModel:
		id = v.ID
	case *models.Device:
		id = v.ID
	}
	return &response.CustomerEntrySummaryResponse{ID: id, Code: code, Name: name, ModelCode: modelCode}
}
func tenantName(v *models.Tenant) string {
	if v == nil {
		return ""
	}
	return v.Name
}
func productName(v *models.Product) string {
	if v == nil {
		return ""
	}
	return v.Name
}
func productCode(v *models.Product) string {
	if v == nil {
		return ""
	}
	return v.Code
}
func modelName(v *models.ProductModel) string {
	if v == nil {
		return ""
	}
	return v.Name
}
func modelCode(v *models.ProductModel) string {
	if v == nil {
		return ""
	}
	return v.ModelCode
}
func deviceName(v *models.Device) string {
	if v == nil {
		return ""
	}
	return v.DeviceNo
}

func BuildCustomerDeviceBinding(item *models.CustomerDeviceBinding, entrySessionID int64) *response.CustomerDeviceBindingResponse {
	if item == nil {
		return nil
	}
	return &response.CustomerDeviceBindingResponse{ID: item.ID, EntrySessionID: entrySessionID, TenantID: item.TenantID, DeviceID: item.DeviceID, CustomerUserID: item.CustomerUserID, CustomerOrgID: item.CustomerOrgID, BindingRole: item.BindingRole, Status: int(item.Status)}
}

func BuildCustomerPortalDeviceBinding(item *models.CustomerDeviceBinding, device *models.Device, product *models.Product) *response.CustomerPortalDeviceBindingResponse {
	if item == nil {
		return nil
	}
	ret := &response.CustomerPortalDeviceBindingResponse{
		DeviceID:    item.DeviceID,
		BindingRole: item.BindingRole,
		Status:      int(item.Status),
	}
	if device != nil {
		ret.DeviceNo = device.DeviceNo
	}
	if product != nil {
		ret.ProductName = product.Name
	}
	return ret
}
