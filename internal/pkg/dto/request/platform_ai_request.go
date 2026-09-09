package request

type PlatformAIProviderSaveRequest struct {
	Host              string `json:"host" binding:"required"`
	AdminAPIKey       string `json:"adminApiKey"`
	DefaultLLMModel   string `json:"defaultLlmModel"`
	TranslationAPIKey string `json:"translationApiKey"`
}

type PlatformAITenantProvisionRequest struct {
	TenantID            int64   `json:"tenantId" binding:"required"`
	Concurrency         int64   `json:"concurrency"`
	Balance             float64 `json:"balance"`
	RPMLimit            int64   `json:"rpmLimit"`
	DefaultKeyExpiresAt string  `json:"defaultKeyExpiresAt"`
}

type PlatformAIUsageQueryRequest struct {
	TenantID    int64  `json:"tenantId" binding:"required"`
	StartDate   string `json:"startDate" binding:"required"`
	EndDate     string `json:"endDate" binding:"required"`
	ModelSource string `json:"modelSource"`
	Timezone    string `json:"timezone"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
	SortBy      string `json:"sortBy"`
	SortOrder   string `json:"sortOrder"`
}

type PlatformSub2APIUsersQueryRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Status    string `json:"status"`
	Role      string `json:"role"`
	Search    string `json:"search"`
	Timezone  string `json:"timezone"`
	SortBy    string `json:"sortBy"`
	SortOrder string `json:"sortOrder"`
}

type PlatformAITenantRechargeRequest struct {
	TenantID  int64   `json:"tenantId" binding:"required"`
	Amount    float64 `json:"amount" binding:"required"`
	Operation string  `json:"operation"`
	Notes     string  `json:"notes"`
	Timezone  string  `json:"timezone"`
}
