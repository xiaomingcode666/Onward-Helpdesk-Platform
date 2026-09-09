package response

import "encoding/json"

type PlatformAIProviderResponse struct {
	Host                   string `json:"host"`
	DefaultLLMModel        string `json:"defaultLlmModel"`
	AdminAPIKeyConfigured  bool   `json:"adminApiKeyConfigured"`
	AdminAPIKeyFingerprint string `json:"adminApiKeyFingerprint"`
	UpdatedAt              string `json:"updatedAt"`
}

type PlatformAIModelSummaryResponse struct {
	TotalTenants      int64 `json:"totalTenants"`
	ConfiguredTenants int64 `json:"configuredTenants"`
	ActiveTenants     int64 `json:"activeTenants"`
	AttentionTenants  int64 `json:"attentionTenants"`
	ActiveKeys        int64 `json:"activeKeys"`
	ModelCount        int64 `json:"modelCount"`
}

type PlatformAIConfigResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	BaseURL          string `json:"baseUrl"`
	ModelType        string `json:"modelType"`
	ModelName        string `json:"modelName"`
	Dimension        int    `json:"dimension"`
	MaxContextTokens int    `json:"maxContextTokens"`
	MaxOutputTokens  int    `json:"maxOutputTokens"`
	TimeoutMS        int    `json:"timeoutMs"`
	MaxRetryCount    int    `json:"maxRetryCount"`
	RPMLimit         int    `json:"rpmLimit"`
	TPMLimit         int    `json:"tpmLimit"`
	Status           int    `json:"status"`
	SortNo           int    `json:"sortNo"`
	Remark           string `json:"remark"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type PlatformAITenantAccountResponse struct {
	ID                   int64   `json:"id"`
	TenantID             int64   `json:"tenantId"`
	Sub2APIAccountID     string  `json:"sub2apiAccountId"`
	AccountName          string  `json:"accountName"`
	DashboardURL         string  `json:"dashboardUrl"`
	LoginEmail           string  `json:"loginEmail"`
	AccountStatus        string  `json:"accountStatus"`
	ProvisionStatus      string  `json:"provisionStatus"`
	Concurrency          int64   `json:"concurrency"`
	Balance              float64 `json:"balance"`
	RPMLimit             int64   `json:"rpmLimit"`
	DefaultKeyID         string  `json:"defaultKeyId"`
	DefaultKeyName       string  `json:"defaultKeyName"`
	DefaultKeyGroupID    int64   `json:"defaultKeyGroupId"`
	DefaultKeyQuota      float64 `json:"defaultKeyQuota"`
	DefaultKeyQuotaUsed  float64 `json:"defaultKeyQuotaUsed"`
	DefaultKeyStatus     string  `json:"defaultKeyStatus"`
	DefaultKeyExpiresAt  string  `json:"defaultKeyExpiresAt"`
	DefaultKey           string  `json:"defaultKey,omitempty"`
	DefaultLLMModel      string  `json:"defaultLlmModel"`
	AccessTokenExpiresAt string  `json:"accessTokenExpiresAt"`
	LastSyncedAt         string  `json:"lastSyncedAt"`
	LastProvisionedAt    string  `json:"lastProvisionedAt"`
	ProvisionMessage     string  `json:"provisionMessage"`
	Status               int     `json:"status"`
}

type PlatformAISub2APIUserResponse struct {
	ID                 int64   `json:"id"`
	Email              string  `json:"email"`
	Username           string  `json:"username"`
	Role               string  `json:"role"`
	Balance            float64 `json:"balance"`
	FrozenBalance      float64 `json:"frozenBalance"`
	Concurrency        int64   `json:"concurrency"`
	CurrentConcurrency int64   `json:"currentConcurrency"`
	Status             string  `json:"status"`
	TotalRecharged     float64 `json:"totalRecharged"`
	RPMLimit           int64   `json:"rpmLimit"`
	LastActiveAt       string  `json:"lastActiveAt"`
	LastUsedAt         string  `json:"lastUsedAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type PlatformSub2APIUserListItemResponse struct {
	ID                 int64   `json:"id"`
	Email              string  `json:"email"`
	Username           string  `json:"username"`
	Role               string  `json:"role"`
	Balance            float64 `json:"balance"`
	FrozenBalance      float64 `json:"frozenBalance"`
	Concurrency        int64   `json:"concurrency"`
	CurrentConcurrency int64   `json:"currentConcurrency"`
	Status             string  `json:"status"`
	LastActiveAt       string  `json:"lastActiveAt"`
	LastUsedAt         string  `json:"lastUsedAt"`
	CreatedAt          string  `json:"createdAt"`
}

type PlatformSub2APIUserListResponse struct {
	GeneratedAt string                                `json:"generatedAt"`
	Items       []PlatformSub2APIUserListItemResponse `json:"items"`
	Total       int64                                 `json:"total"`
	Page        int                                   `json:"page"`
	PageSize    int                                   `json:"pageSize"`
	Pages       int                                   `json:"pages"`
}

type PlatformSub2APIUserKeyResponse struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"userId"`
	KeyPreview         string  `json:"keyPreview"`
	Name               string  `json:"name"`
	GroupID            int64   `json:"groupId"`
	GroupName          string  `json:"groupName"`
	Status             string  `json:"status"`
	LastUsedAt         string  `json:"lastUsedAt"`
	LastUsedIP         string  `json:"lastUsedIp"`
	Quota              float64 `json:"quota"`
	QuotaUsed          float64 `json:"quotaUsed"`
	ExpiresAt          string  `json:"expiresAt"`
	CreatedAt          string  `json:"createdAt"`
	CurrentConcurrency int64   `json:"currentConcurrency"`
}

type PlatformSub2APIUserKeyListResponse struct {
	GeneratedAt string                           `json:"generatedAt"`
	UserID      int64                            `json:"userId"`
	Items       []PlatformSub2APIUserKeyResponse `json:"items"`
	Total       int64                            `json:"total"`
	Page        int                              `json:"page"`
	PageSize    int                              `json:"pageSize"`
	Pages       int                              `json:"pages"`
}

type PlatformAIRechargeRecordResponse struct {
	ID               int64   `json:"id"`
	TenantID         int64   `json:"tenantId"`
	Sub2APIAccountID string  `json:"sub2apiAccountId"`
	RemoteUserID     int64   `json:"remoteUserId"`
	Operation        string  `json:"operation"`
	Amount           float64 `json:"amount"`
	BalanceBefore    float64 `json:"balanceBefore"`
	BalanceAfter     float64 `json:"balanceAfter"`
	Notes            string  `json:"notes"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	OccurredAt       string  `json:"occurredAt"`
	CreatedAt        string  `json:"createdAt"`
	OperatorID       int64   `json:"operatorId"`
	OperatorName     string  `json:"operatorName"`
}

type PlatformAITenantWorkspaceResponse struct {
	Tenant         PlatformTenantResponse           `json:"tenant"`
	Account        *PlatformAITenantAccountResponse `json:"account,omitempty"`
	HasAccessToken bool                             `json:"hasAccessToken"`
	NeedsAttention bool                             `json:"needsAttention"`
}

type PlatformAIModelWorkspaceResponse struct {
	GeneratedAt string                              `json:"generatedAt"`
	Provider    PlatformAIProviderResponse          `json:"provider"`
	Models      []PlatformAIConfigResponse          `json:"models"`
	Tenants     []PlatformAITenantWorkspaceResponse `json:"tenants"`
	Summary     PlatformAIModelSummaryResponse      `json:"summary"`
}

type PlatformAITenantBillingResponse struct {
	GeneratedAt     string                             `json:"generatedAt"`
	Tenant          PlatformAITenantWorkspaceResponse  `json:"tenant"`
	RemoteUser      *PlatformAISub2APIUserResponse     `json:"remoteUser,omitempty"`
	RechargeRecords []PlatformAIRechargeRecordResponse `json:"rechargeRecords"`
}

type PlatformAIUsageDashboardModelResponse struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actualCost"`
}

type PlatformAIUsageEndpointResponse struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"totalTokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actualCost"`
}

type PlatformAIUsageStatsResponse struct {
	TotalRequests            int64                             `json:"totalRequests"`
	TotalInputTokens         int64                             `json:"totalInputTokens"`
	TotalOutputTokens        int64                             `json:"totalOutputTokens"`
	TotalCacheTokens         int64                             `json:"totalCacheTokens"`
	TotalCacheCreationTokens int64                             `json:"totalCacheCreationTokens"`
	TotalCacheReadTokens     int64                             `json:"totalCacheReadTokens"`
	TotalTokens              int64                             `json:"totalTokens"`
	TotalCost                float64                           `json:"totalCost"`
	TotalActualCost          float64                           `json:"totalActualCost"`
	AverageDurationMS        float64                           `json:"averageDurationMs"`
	Currency                 string                            `json:"currency"`
	Endpoints                []PlatformAIUsageEndpointResponse `json:"endpoints"`
}

type PlatformAIUsageDetailResponse struct {
	GeneratedAt string                                  `json:"generatedAt"`
	Tenant      PlatformAITenantWorkspaceResponse       `json:"tenant"`
	Stats       PlatformAIUsageStatsResponse            `json:"stats"`
	Models      []PlatformAIUsageDashboardModelResponse `json:"models"`
	Usage       json.RawMessage                         `json:"usage"`
}
