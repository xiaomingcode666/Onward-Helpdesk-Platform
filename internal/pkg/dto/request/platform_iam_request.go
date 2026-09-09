package request

type PlatformTenantCreateRequest struct {
	Name                  string   `json:"name" binding:"required"`
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
	AIEnabled             *bool    `json:"aiEnabled"`
	TrialEndsAt           string   `json:"trialEndsAt"`
	PlanID                int64    `json:"planId"`
	PlanCode              string   `json:"planCode"`
	AdminUserID           int64    `json:"adminUserId"`
	AdminUsername         string   `json:"adminUsername"`
	AdminNickname         string   `json:"adminNickname"`
	AdminEmail            string   `json:"adminEmail"`
	AdminMobile           string   `json:"adminMobile"`
	AdminPassword         string   `json:"adminPassword"`
}

type PlatformTenantUpdateRequest struct {
	ID                    int64    `json:"id" binding:"required"`
	Name                  *string  `json:"name"`
	ServiceScene          *string  `json:"serviceScene"`
	BrandName             *string  `json:"brandName"`
	LogoAssetID           *int64   `json:"logoAssetId"`
	LogoURL               *string  `json:"logoUrl"`
	CustomDomain          *string  `json:"customDomain"`
	CustomerTheme         *string  `json:"customerTheme"`
	ServerConsoleName     *string  `json:"serverConsoleName"`
	ServerConsoleURL      *string  `json:"serverConsoleUrl"`
	ServerConsoleMode     *string  `json:"serverConsoleMode"`
	ServerConsoleEnabled  *bool    `json:"serverConsoleEnabled"`
	Industry              *string  `json:"industry"`
	CountryRegion         *string  `json:"countryRegion"`
	DefaultLocale         *string  `json:"defaultLocale"`
	CustomerDefaultLocale *string  `json:"customerDefaultLocale"`
	SupportedLocales      []string `json:"supportedLocales"`
	Timezone              *string  `json:"timezone"`
	SupportedTimezones    []string `json:"supportedTimezones"`
	DataRegion            *string  `json:"dataRegion"`
	Status                *int     `json:"status"`
	AIEnabled             *bool    `json:"aiEnabled"`
	TrialEndsAt           *string  `json:"trialEndsAt"`
}

type PlatformTenantStateRequest struct {
	TenantID int64  `json:"tenantId" binding:"required"`
	Reason   string `json:"reason"`
}

type PlatformTenantEnterRequest struct {
	TenantID int64  `json:"tenantId" binding:"required"`
	Reason   string `json:"reason"`
}

type PlatformTenantAdminPasswordRequest struct {
	TenantID int64  `json:"tenantId" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type PlatformTenantDecommissionRequest struct {
	TenantID       int64  `json:"tenantId" binding:"required"`
	Reason         string `json:"reason" binding:"required"`
	PurgeAfterDays int    `json:"purgeAfterDays"`
}

type PlatformTenantDeleteRequest struct {
	TenantID int64  `json:"tenantId" binding:"required"`
	Reason   string `json:"reason"`
}

type PlatformTenantDataExportRequest struct {
	TenantID    int64    `json:"tenantId" binding:"required"`
	ExportScope []string `json:"exportScope"`
}

type PlatformTenantRightToErasureRequest struct {
	TenantID       int64  `json:"tenantId" binding:"required"`
	RequestChannel string `json:"requestChannel"`
	RequestedBy    string `json:"requestedBy"`
}

type PlatformAuditRetentionUpdateRequest struct {
	RetentionDays int `json:"retentionDays" binding:"required"`
}

type PlatformPlanSaveRequest struct {
	ID                int64  `json:"id"`
	Code              string `json:"code" binding:"required"`
	Name              string `json:"name" binding:"required"`
	PlanType          string `json:"planType"`
	FeatureJSON       string `json:"featureJson"`
	QuotaTemplateJSON string `json:"quotaTemplateJson"`
	OverageStrategy   string `json:"overageStrategy"`
	Status            *int   `json:"status"`
	SortNo            int    `json:"sortNo"`
}

type PlatformSubscriptionAssignRequest struct {
	TenantID     int64  `json:"tenantId" binding:"required"`
	PlanID       int64  `json:"planId" binding:"required"`
	StartsAt     string `json:"startsAt"`
	EndsAt       string `json:"endsAt"`
	BillingCycle string `json:"billingCycle"`
}

type PlatformQuotaOverrideRequest struct {
	TenantID        int64  `json:"tenantId" binding:"required"`
	ProductID       int64  `json:"productId"`
	QuotaKey        string `json:"quotaKey" binding:"required"`
	QuotaValue      int64  `json:"quotaValue"`
	Period          string `json:"period"`
	OverageStrategy string `json:"overageStrategy"`
	Reason          string `json:"reason"`
	EffectiveFrom   string `json:"effectiveFrom"`
	EffectiveTo     string `json:"effectiveTo"`
}

type PlatformSub2APIAccountBindRequest struct {
	TenantID          int64  `json:"tenantId" binding:"required"`
	Sub2APIAccountID  string `json:"sub2apiAccountId" binding:"required"`
	AccountName       string `json:"accountName"`
	DashboardURL      string `json:"dashboardUrl"`
	AccountStatus     string `json:"accountStatus"`
	QuotaSnapshotJSON string `json:"quotaSnapshotJson"`
}

type PlatformSub2APIAccountActionRequest struct {
	TenantID  int64 `json:"tenantId"`
	AccountID int64 `json:"accountId"`
}

type PlatformStaffInviteRequest struct {
	UserID         int64    `json:"userId"`
	Username       string   `json:"username"`
	Nickname       string   `json:"nickname"`
	Email          string   `json:"email"`
	Mobile         string   `json:"mobile"`
	Password       string   `json:"password"`
	RoleCodes      []string `json:"roleCodes"`
	TeamCode       string   `json:"teamCode"`
	JobTitle       string   `json:"jobTitle"`
	SupportLevel   string   `json:"supportLevel"`
	EmploymentType string   `json:"employmentType"`
}

type PlatformStaffUpdateRequest struct {
	PlatformStaffID int64   `json:"platformStaffId" binding:"required"`
	Nickname        *string `json:"nickname"`
	Email           *string `json:"email"`
	Mobile          *string `json:"mobile"`
	TeamCode        *string `json:"teamCode"`
	JobTitle        *string `json:"jobTitle"`
	SupportLevel    *string `json:"supportLevel"`
	EmploymentType  *string `json:"employmentType"`
	Status          *int    `json:"status"`
}

type PlatformStaffDeleteRequest struct {
	PlatformStaffID int64  `json:"platformStaffId" binding:"required"`
	Reason          string `json:"reason"`
}

type PlatformStaffGrantTenantRequest struct {
	PlatformStaffID int64  `json:"platformStaffId" binding:"required"`
	TenantID        int64  `json:"tenantId" binding:"required"`
	GrantScopeJSON  string `json:"grantScopeJson"`
	Reason          string `json:"reason" binding:"required"`
	ApprovedBy      int64  `json:"approvedBy"`
	ExpiredAt       string `json:"expiredAt" binding:"required"`
}

type PlatformStaffDisableRequest struct {
	PlatformStaffID int64  `json:"platformStaffId" binding:"required"`
	Reason          string `json:"reason"`
}

type PlatformAuthRoleSaveRequest struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenantId"`
	DomainType  string `json:"domainType" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"isBuiltin"`
	Status      *int   `json:"status"`
	SortNo      int    `json:"sortNo"`
}

type PlatformAuthPolicySaveRequest struct {
	TenantID        int64    `json:"tenantId"`
	RoleID          int64    `json:"roleId" binding:"required"`
	PermissionCodes []string `json:"permissionCodes"`
	Effect          string   `json:"effect"`
}

type PlatformAuthRoleDeleteRequest struct {
	RoleID int64  `json:"roleId" binding:"required"`
	Reason string `json:"reason"`
}

type PlatformAuthPermissionCheckRequest struct {
	TenantID       int64  `json:"tenantId"`
	DomainType     string `json:"domainType" binding:"required"`
	SubjectType    string `json:"subjectType" binding:"required"`
	SubjectID      int64  `json:"subjectId" binding:"required"`
	PermissionCode string `json:"permissionCode" binding:"required"`
}
