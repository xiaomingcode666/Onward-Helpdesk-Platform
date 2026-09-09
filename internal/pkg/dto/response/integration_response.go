package response

type TenantIntegrationConfigResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	Provider        string `json:"provider"`
	BaseURL         string `json:"baseUrl"`
	AppID           string `json:"appId"`
	Enabled         bool   `json:"enabled"`
	Status          int    `json:"status"`
	MetadataJSON    string `json:"metadataJson"`
	SecretRef       string `json:"secretRef"`
	MaskedAppSecret string `json:"maskedAppSecret"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type ProductAIUsageCredentialResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	Sub2APIAccount  string `json:"sub2apiAccount"`
	QuotaPolicyJSON string `json:"quotaPolicyJson"`
	Status          int    `json:"status"`
	LastSyncedAt    string `json:"lastSyncedAt,omitempty"`
	APIKeyRef       string `json:"apiKeyRef"`
	MaskedAPIKey    string `json:"maskedApiKey"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type MeetingPreviewResponse struct {
	RoomName  string `json:"roomName"`
	JoinURL   string `json:"joinUrl"`
	ExpiresAt string `json:"expiresAt"`
}
