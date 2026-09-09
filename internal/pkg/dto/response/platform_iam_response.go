package response

type PlatformTenantResponse struct {
	ID                    int64    `json:"id"`
	Code                  string   `json:"code"`
	Name                  string   `json:"name"`
	ServiceScene          string   `json:"serviceScene"`
	BrandName             string   `json:"brandName"`
	LogoAssetID           int64    `json:"logoAssetId"`
	LogoURL               string   `json:"logoUrl"`
	CustomDomain          string   `json:"customDomain"`
	CustomerTheme         string   `json:"customerTheme"`
	ServerConsoleName     string   `json:"serverConsoleName"`
	ServerConsoleURL      string   `json:"serverConsoleUrl"`
	ServerConsoleMode     string   `json:"serverConsoleMode"`
	ServerConsoleEnabled  bool     `json:"serverConsoleEnabled"`
	Industry              string   `json:"industry"`
	CountryRegion         string   `json:"countryRegion"`
	DefaultLocale         string   `json:"defaultLocale"`
	CustomerDefaultLocale string   `json:"customerDefaultLocale"`
	SupportedLocales      []string `json:"supportedLocales"`
	Timezone              string   `json:"timezone"`
	SupportedTimezones    []string `json:"supportedTimezones"`
	DataRegion            string   `json:"dataRegion"`
	Status                int      `json:"status"`
	AIEnabled             bool     `json:"aiEnabled"`
	TrialEndsAt           string   `json:"trialEndsAt"`
	FrozenReason          string   `json:"frozenReason"`
	DecommissionedAt      string   `json:"decommissionedAt"`
	DecommissionReason    string   `json:"decommissionReason"`
	PurgeAfterDays        int      `json:"purgeAfterDays"`
	PurgedAt              string   `json:"purgedAt"`
	DataExportCompletedAt string   `json:"dataExportCompletedAt"`
	PlanCode              string   `json:"planCode"`
	PlanName              string   `json:"planName"`
	SubscriptionStatus    int      `json:"subscriptionStatus"`
	AdminUserID           int64    `json:"adminUserId"`
	AdminUsername         string   `json:"adminUsername"`
	AdminDisplayName      string   `json:"adminDisplayName"`
	AdminEmail            string   `json:"adminEmail"`
	DeviceCount           int64    `json:"deviceCount"`
	MemberCount           int64    `json:"memberCount"`
	MonthlyAIRequests     int64    `json:"monthlyAiRequests"`
	MonthlyAITokens       int64    `json:"monthlyAiTokens"`
	MonthlyAICost         float64  `json:"monthlyAiCost"`
	LastMeteredAt         string   `json:"lastMeteredAt"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
}

type PlatformTenantLogoUploadResponse struct {
	AssetID    int64  `json:"assetId"`
	URL        string `json:"url"`
	PreviewURL string `json:"previewUrl"`
}

type PlatformTenantListResponse struct {
	Tenants    []*PlatformTenantResponse     `json:"tenants"`
	TotalCount int64                         `json:"totalCount"`
	Summary    PlatformTenantSummaryResponse `json:"summary"`
}

type PlatformTenantAdminResponse struct {
	TenantID    int64  `json:"tenantId"`
	UserID      int64  `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type PlatformPlanResponse struct {
	ID                int64  `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	PlanType          string `json:"planType"`
	FeatureJSON       string `json:"featureJson"`
	QuotaTemplateJSON string `json:"quotaTemplateJson"`
	OverageStrategy   string `json:"overageStrategy"`
	Status            int    `json:"status"`
	SortNo            int    `json:"sortNo"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type PlatformSubscriptionResponse struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenantId"`
	PlanID           int64  `json:"planId"`
	PlanSnapshotJSON string `json:"planSnapshotJson"`
	StartsAt         string `json:"startsAt"`
	EndsAt           string `json:"endsAt"`
	BillingCycle     string `json:"billingCycle"`
	Status           int    `json:"status"`
}

type PlatformQuotaOverrideResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	QuotaKey        string `json:"quotaKey"`
	QuotaValue      int64  `json:"quotaValue"`
	Period          string `json:"period"`
	OverageStrategy string `json:"overageStrategy"`
	Reason          string `json:"reason"`
	EffectiveFrom   string `json:"effectiveFrom"`
	EffectiveTo     string `json:"effectiveTo"`
	Status          int    `json:"status"`
}

type PlatformSub2APIAccountResponse struct {
	ID                int64  `json:"id"`
	TenantID          int64  `json:"tenantId"`
	Sub2APIAccountID  string `json:"sub2apiAccountId"`
	AccountName       string `json:"accountName"`
	DashboardURL      string `json:"dashboardUrl"`
	AccountStatus     string `json:"accountStatus"`
	QuotaSnapshotJSON string `json:"quotaSnapshotJson"`
	LastSyncedAt      string `json:"lastSyncedAt"`
	LastTestedAt      string `json:"lastTestedAt"`
	Status            int    `json:"status"`
}

type PlatformStaffResponse struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"userId"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Email          string `json:"email"`
	Mobile         string `json:"mobile"`
	TeamCode       string `json:"teamCode"`
	JobTitle       string `json:"jobTitle"`
	SupportLevel   string `json:"supportLevel"`
	EmploymentType string `json:"employmentType"`
	Status         int    `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type PlatformTenantGrantResponse struct {
	ID              int64  `json:"id"`
	PlatformStaffID int64  `json:"platformStaffId"`
	TenantID        int64  `json:"tenantId"`
	GrantScopeJSON  string `json:"grantScopeJson"`
	Reason          string `json:"reason"`
	ApprovedBy      int64  `json:"approvedBy"`
	ApprovedAt      string `json:"approvedAt"`
	ExpiredAt       string `json:"expiredAt"`
	RevokedAt       string `json:"revokedAt"`
	Status          string `json:"status"`
}

type PlatformAuthRoleResponse struct {
	ID          int64    `json:"id"`
	TenantID    int64    `json:"tenantId"`
	DomainType  string   `json:"domainType"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsBuiltin   bool     `json:"isBuiltin"`
	Status      int      `json:"status"`
	SortNo      int      `json:"sortNo"`
	Permissions []string `json:"permissions,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type PlatformAuthPermissionCheckResponse struct {
	Allowed        bool     `json:"allowed"`
	MatchedRoles   []string `json:"matchedRoles"`
	PermissionCode string   `json:"permissionCode"`
	Effect         string   `json:"effect"`
}

type PlatformAuditLogResponse struct {
	ID               int64             `json:"id"`
	TenantID         int64             `json:"tenantId"`
	DomainType       string            `json:"domainType"`
	ActorUserID      int64             `json:"actorUserId"`
	ActorSubjectType string            `json:"actorSubjectType"`
	ActorSubjectID   int64             `json:"actorSubjectId"`
	TargetType       string            `json:"targetType"`
	TargetID         string            `json:"targetId"`
	Action           string            `json:"action"`
	Summary          string            `json:"summary"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	RequestID        string            `json:"requestId"`
	SupportGrantID   int64             `json:"supportGrantId"`
	IPAddress        string            `json:"ipAddress"`
	UserAgent        string            `json:"userAgent"`
	RiskLevel        string            `json:"riskLevel"`
	Status           string            `json:"status"`
	OccurredAt       string            `json:"occurredAt"`
}

type PlatformAuditRetentionResponse struct {
	RetentionDays        int    `json:"retentionDays"`
	DefaultRetentionDays int    `json:"defaultRetentionDays"`
	MaxRetentionDays     int    `json:"maxRetentionDays"`
	MinRetentionDays     int    `json:"minRetentionDays"`
	CutoffAt             string `json:"cutoffAt"`
	Configured           bool   `json:"configured"`
}

type PlatformTenantDataExportResponse struct {
	ExportJobID   string `json:"exportJobId"`
	EstimatedSize string `json:"estimatedSize"`
	AssetURL      string `json:"assetUrl"`
}

type PlatformTenantRightToErasureResponse struct {
	ErasureRequestID      string `json:"erasureRequestId"`
	EstimatedCompletionAt string `json:"estimatedCompletionAt"`
}
