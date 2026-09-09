package request

type CreateTenantIntegrationConfigRequest struct {
	TenantID     int64  `json:"tenantId"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"baseUrl"`
	AppID        string `json:"appId"`
	AppKey       string `json:"appKey"`
	AppSecret    string `json:"appSecret"`
	Enabled      bool   `json:"enabled"`
	MetadataJSON string `json:"metadataJson"`
}

type UpdateTenantIntegrationConfigRequest struct {
	ID int64 `json:"id"`
	CreateTenantIntegrationConfigRequest
}

type CreateProductAIUsageCredentialRequest struct {
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	Sub2APIAccount  string `json:"sub2apiAccount"`
	APIKey          string `json:"apiKey"`
	QuotaPolicyJSON string `json:"quotaPolicyJson"`
}

type UpdateProductAIUsageCredentialRequest struct {
	ID int64 `json:"id"`
	CreateProductAIUsageCredentialRequest
}

type DeleteIntegrationRequest struct {
	ID int64 `json:"id"`
}
type UpdateIntegrationStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type MeetingPreviewCreateRequest struct {
	TenantID     int64  `json:"tenantId"`
	ProductID    int64  `json:"productId"`
	BusinessType string `json:"businessType"`
	BusinessID   int64  `json:"businessId"`
}
