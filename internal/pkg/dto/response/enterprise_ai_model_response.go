package response

type EnterpriseAIModelAccountResponse struct {
	TenantID             int64   `json:"tenantId"`
	Sub2APIAccountID     string  `json:"sub2apiAccountId"`
	AccountName          string  `json:"accountName"`
	LoginEmail           string  `json:"loginEmail"`
	AccountStatus        string  `json:"accountStatus"`
	ProvisionStatus      string  `json:"provisionStatus"`
	Balance              float64 `json:"balance"`
	Concurrency          int64   `json:"concurrency"`
	RPMLimit             int64   `json:"rpmLimit"`
	DefaultKeyID         string  `json:"defaultKeyId"`
	DefaultKeyName       string  `json:"defaultKeyName"`
	DefaultKeyQuota      float64 `json:"defaultKeyQuota"`
	DefaultKeyQuotaUsed  float64 `json:"defaultKeyQuotaUsed"`
	DefaultKeyStatus     string  `json:"defaultKeyStatus"`
	DefaultLLMModel      string  `json:"defaultLlmModel"`
	AccessTokenExpiresAt string  `json:"accessTokenExpiresAt"`
	LastSyncedAt         string  `json:"lastSyncedAt"`
}

type EnterpriseAISub2APIUserResponse struct {
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
	UpdatedAt          string  `json:"updatedAt"`
}

type EnterpriseAIKeyGroupResponse struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Platform       string  `json:"platform"`
	RateMultiplier float64 `json:"rateMultiplier"`
	Status         string  `json:"status"`
	RPMLimit       int64   `json:"rpmLimit"`
}

type EnterpriseAIKeyResponse struct {
	ID                 int64                         `json:"id"`
	UserID             int64                         `json:"userId"`
	KeyPreview         string                        `json:"keyPreview"`
	Name               string                        `json:"name"`
	ProductID          int64                         `json:"productId,omitempty"`
	ProductName        string                        `json:"productName,omitempty"`
	ProductCode        string                        `json:"productCode,omitempty"`
	GroupID            int64                         `json:"groupId"`
	Group              *EnterpriseAIKeyGroupResponse `json:"group,omitempty"`
	Status             string                        `json:"status"`
	Quota              float64                       `json:"quota"`
	QuotaUsed          float64                       `json:"quotaUsed"`
	QuotaRemaining     float64                       `json:"quotaRemaining"`
	QuotaUsagePercent  float64                       `json:"quotaUsagePercent"`
	ExpiresAt          string                        `json:"expiresAt"`
	LastUsedAt         string                        `json:"lastUsedAt"`
	LastUsedIP         string                        `json:"lastUsedIp"`
	CurrentConcurrency int64                         `json:"currentConcurrency"`
	RateLimit5h        int64                         `json:"rateLimit5h"`
	RateLimit1d        int64                         `json:"rateLimit1d"`
	RateLimit7d        int64                         `json:"rateLimit7d"`
	Usage5h            int64                         `json:"usage5h"`
	Usage1d            int64                         `json:"usage1d"`
	Usage7d            int64                         `json:"usage7d"`
	CreatedAt          string                        `json:"createdAt"`
	UpdatedAt          string                        `json:"updatedAt"`
}

type EnterpriseAIKeyPagingResponse struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Pages    int   `json:"pages"`
}

type EnterpriseAIUsageTrendPointResponse struct {
	Date                string  `json:"date"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actualCost"`
}

type EnterpriseAIUsagePlatformStatsResponse struct {
	Platform        string  `json:"platform"`
	TotalRequests   int64   `json:"totalRequests"`
	TotalTokens     int64   `json:"totalTokens"`
	TotalActualCost float64 `json:"totalActualCost"`
	TodayRequests   int64   `json:"todayRequests"`
	TodayTokens     int64   `json:"todayTokens"`
	TodayActualCost float64 `json:"todayActualCost"`
}

type EnterpriseAIUsageStatsResponse struct {
	TotalAPIKeys             int64                                    `json:"totalApiKeys"`
	ActiveAPIKeys            int64                                    `json:"activeApiKeys"`
	TotalRequests            int64                                    `json:"totalRequests"`
	TotalInputTokens         int64                                    `json:"totalInputTokens"`
	TotalOutputTokens        int64                                    `json:"totalOutputTokens"`
	TotalCacheCreationTokens int64                                    `json:"totalCacheCreationTokens"`
	TotalCacheReadTokens     int64                                    `json:"totalCacheReadTokens"`
	TotalTokens              int64                                    `json:"totalTokens"`
	TotalCost                float64                                  `json:"totalCost"`
	TotalActualCost          float64                                  `json:"totalActualCost"`
	TodayRequests            int64                                    `json:"todayRequests"`
	TodayInputTokens         int64                                    `json:"todayInputTokens"`
	TodayOutputTokens        int64                                    `json:"todayOutputTokens"`
	TodayCacheCreationTokens int64                                    `json:"todayCacheCreationTokens"`
	TodayCacheReadTokens     int64                                    `json:"todayCacheReadTokens"`
	TodayTokens              int64                                    `json:"todayTokens"`
	TodayCost                float64                                  `json:"todayCost"`
	TodayActualCost          float64                                  `json:"todayActualCost"`
	AverageDurationMS        float64                                  `json:"averageDurationMs"`
	RPM                      int64                                    `json:"rpm"`
	TPM                      int64                                    `json:"tpm"`
	ByPlatform               []EnterpriseAIUsagePlatformStatsResponse `json:"byPlatform"`
}

type EnterpriseAISubscriptionGroupResponse struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Platform         string  `json:"platform"`
	RateMultiplier   float64 `json:"rateMultiplier"`
	IsExclusive      bool    `json:"isExclusive"`
	Status           string  `json:"status"`
	SubscriptionType string  `json:"subscriptionType"`
	DailyLimitUSD    float64 `json:"dailyLimitUsd"`
	WeeklyLimitUSD   float64 `json:"weeklyLimitUsd"`
	MonthlyLimitUSD  float64 `json:"monthlyLimitUsd"`
	RPMLimit         int64   `json:"rpmLimit"`
}

type EnterpriseAISubscriptionResponse struct {
	ID                 int64                                  `json:"id"`
	UserID             int64                                  `json:"userId"`
	GroupID            int64                                  `json:"groupId"`
	StartsAt           string                                 `json:"startsAt"`
	ExpiresAt          string                                 `json:"expiresAt"`
	Status             string                                 `json:"status"`
	DurationDays       int64                                  `json:"durationDays"`
	RemainingDays      int64                                  `json:"remainingDays"`
	ProgressPercent    float64                                `json:"progressPercent"`
	DailyWindowStart   string                                 `json:"dailyWindowStart"`
	WeeklyWindowStart  string                                 `json:"weeklyWindowStart"`
	MonthlyWindowStart string                                 `json:"monthlyWindowStart"`
	DailyUsageUSD      float64                                `json:"dailyUsageUsd"`
	WeeklyUsageUSD     float64                                `json:"weeklyUsageUsd"`
	MonthlyUsageUSD    float64                                `json:"monthlyUsageUsd"`
	Group              *EnterpriseAISubscriptionGroupResponse `json:"group,omitempty"`
	CreatedAt          string                                 `json:"createdAt"`
	UpdatedAt          string                                 `json:"updatedAt"`
}

type EnterpriseAIModelWorkspaceResponse struct {
	GeneratedAt        string                                `json:"generatedAt"`
	Account            EnterpriseAIModelAccountResponse      `json:"account"`
	User               EnterpriseAISub2APIUserResponse       `json:"user"`
	Stats              EnterpriseAIUsageStatsResponse        `json:"stats"`
	Trend              []EnterpriseAIUsageTrendPointResponse `json:"trend"`
	Keys               []EnterpriseAIKeyResponse             `json:"keys"`
	KeyPaging          EnterpriseAIKeyPagingResponse         `json:"keyPaging"`
	Subscriptions      []EnterpriseAISubscriptionResponse    `json:"subscriptions"`
	ActiveSubscription *EnterpriseAISubscriptionResponse     `json:"activeSubscription,omitempty"`
}
