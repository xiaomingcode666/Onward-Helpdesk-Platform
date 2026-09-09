package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"time"
)

func maskFingerprint(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

func BuildTenantIntegrationConfig(item *models.TenantIntegrationConfig) *response.TenantIntegrationConfigResponse {
	if item == nil {
		return nil
	}
	return &response.TenantIntegrationConfigResponse{ID: item.ID, TenantID: item.TenantID, Provider: item.Provider, BaseURL: item.BaseURL, AppID: item.AppID, Enabled: item.Enabled, Status: int(item.Status), MetadataJSON: item.MetadataJSON, SecretRef: item.AppSecretRef, MaskedAppSecret: maskFingerprint(item.AppSecretFingerprint), CreatedAt: item.CreatedAt.Format(time.DateTime), UpdatedAt: item.UpdatedAt.Format(time.DateTime)}
}
func BuildTenantIntegrationConfigList(items []models.TenantIntegrationConfig) []*response.TenantIntegrationConfigResponse {
	ret := make([]*response.TenantIntegrationConfigResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildTenantIntegrationConfig(&items[i]))
	}
	return ret
}
func BuildProductAIUsageCredential(item *models.ProductAIUsageCredential) *response.ProductAIUsageCredentialResponse {
	if item == nil {
		return nil
	}
	last := ""
	if item.LastSyncedAt != nil {
		last = item.LastSyncedAt.Format(time.DateTime)
	}
	return &response.ProductAIUsageCredentialResponse{ID: item.ID, TenantID: item.TenantID, ProductID: item.ProductID, Sub2APIAccount: item.Sub2APIAccount, QuotaPolicyJSON: item.QuotaPolicyJSON, Status: int(item.Status), LastSyncedAt: last, APIKeyRef: item.APIKeyRef, MaskedAPIKey: maskFingerprint(item.APIKeyFingerprint), CreatedAt: item.CreatedAt.Format(time.DateTime), UpdatedAt: item.UpdatedAt.Format(time.DateTime)}
}
func BuildProductAIUsageCredentialList(items []models.ProductAIUsageCredential) []*response.ProductAIUsageCredentialResponse {
	ret := make([]*response.ProductAIUsageCredentialResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildProductAIUsageCredential(&items[i]))
	}
	return ret
}
