package models

import (
	"encoding/json"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

const (
	TenantCustomerThemeDefault         = "default"
	TenantCustomerThemeODTIntelligence = "odt-intelligence"
	TenantCustomerThemeClinicalCalm    = "clinical-calm"
	TenantCustomerThemeSignalCoral     = "signal-coral"
)

const (
	DomainTypePlatform       = "platform"
	DomainTypeEnterprise     = "enterprise"
	DomainTypeCustomer       = "customer"
	DomainTypePartner        = "partner"
	DomainTypeServiceAccount = "service_account"

	SubjectTypePlatformStaff   = "platform_staff"
	SubjectTypeTenantMember    = "enterprise_member"
	SubjectTypeCustomerUser    = "customer_user"
	SubjectTypePendingCustomer = "pending_customer"
	SubjectTypeTempVisitor     = "temp_visitor"
	SubjectTypePartnerAccount  = "partner_account"
	SubjectTypeServiceAccount  = "service_account"

	AccessGrantStatusActive  = "active"
	AccessGrantStatusRevoked = "revoked"
	AccessGrantStatusExpired = "expired"

	SupportModePlatformTenant = "platform_tenant"
	SupportModeCustomerPortal = "customer_portal"
	SupportModeEmployeePortal = "employee_portal"

	CustomerRegistrationGrantPending  = "pending"
	CustomerRegistrationGrantConsumed = "consumed"
	CustomerRegistrationGrantRevoked  = "revoked"

	AuthVerificationPurposeEnterpriseRegister = "enterprise_register"
	AuthVerificationPurposeCustomerRegister   = "customer_register"
	AuthVerificationPurposePasswordReset      = "password_reset"
	AuthVerificationStatusPending             = "pending"
	AuthVerificationStatusConsumed            = "consumed"
	AuthVerificationStatusExpired             = "expired"
)

// TenantBranding stores tenant-owned brand and portal presentation settings.
type TenantBranding struct {
	ID                   int64        `gorm:"primaryKey;autoIncrement"`
	TenantID             int64        `gorm:"type:bigint;not null;uniqueIndex"`
	LogoAssetID          int64        `gorm:"type:bigint;not null;default:0"`
	LogoURL              string       `gorm:"column:logo_url;type:varchar(1024);not null;default:''"`
	BrandName            string       `gorm:"type:varchar(128);not null;default:''"`
	PrimaryColor         string       `gorm:"type:varchar(32);not null;default:''"`
	CustomDomain         string       `gorm:"type:varchar(255);not null;default:'';index"`
	DefaultLocale        string       `gorm:"type:varchar(16);not null;default:'en-US'"`
	PrivacyPolicyVersion string       `gorm:"type:varchar(64);not null;default:''"`
	PrivacyCopy          string       `gorm:"type:text"`
	BrandConfigJSON      string       `gorm:"column:brand_config_json;type:text;not null;default:'{}'"`
	Status               enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TenantBranding) TableName() string { return "tenant_branding" }

type TenantBrandConfig struct {
	CustomerTheme string `json:"customerTheme,omitempty"`
}

func NormalizeTenantCustomerTheme(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case TenantCustomerThemeODTIntelligence,
		TenantCustomerThemeClinicalCalm,
		TenantCustomerThemeSignalCoral:
		return normalized
	default:
		return TenantCustomerThemeDefault
	}
}

func (b *TenantBranding) EffectiveCustomerTheme() string {
	if b == nil {
		return TenantCustomerThemeDefault
	}
	config := TenantBrandConfig{}
	_ = json.Unmarshal([]byte(b.BrandConfigJSON), &config)
	return NormalizeTenantCustomerTheme(config.CustomerTheme)
}

// TenantPlan is the platform plan template used by tenant subscriptions.
type TenantPlan struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`
	Code              string       `gorm:"type:varchar(64);not null;uniqueIndex"`
	Name              string       `gorm:"type:varchar(128);not null;default:'';index"`
	PlanType          string       `gorm:"type:varchar(32);not null;default:'';index"`
	FeatureJSON       string       `gorm:"column:feature_json;type:text;not null;default:'{}'"`
	QuotaTemplateJSON string       `gorm:"column:quota_template_json;type:text;not null;default:'{}'"`
	OverageStrategy   string       `gorm:"type:varchar(32);not null;default:'block';index"`
	Status            enums.Status `gorm:"type:int;not null;default:0;index"`
	SortNo            int          `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TenantPlan) TableName() string { return "tenant_plans" }

// TenantPlanQuota is a normalized quota row attached to a plan template.
type TenantPlanQuota struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	PlanID          int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_plan_quota"`
	QuotaKey        string       `gorm:"type:varchar(64);not null;default:'';index;uniqueIndex:uk_tenant_plan_quota"`
	QuotaValue      int64        `gorm:"type:bigint;not null;default:0"`
	Unit            string       `gorm:"type:varchar(32);not null;default:''"`
	Period          string       `gorm:"type:varchar(16);not null;default:'monthly';index;uniqueIndex:uk_tenant_plan_quota"`
	OverageStrategy string       `gorm:"type:varchar(32);not null;default:'block'"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TenantPlanQuota) TableName() string { return "tenant_plan_quotas" }

// TenantSubscription records the active billing/feature snapshot for a tenant.
type TenantSubscription struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_subscription_period"`
	PlanID           int64        `gorm:"type:bigint;not null;index"`
	PlanSnapshotJSON string       `gorm:"column:plan_snapshot_json;type:text;not null;default:'{}'"`
	StartsAt         time.Time    `gorm:"type:timestamp;not null;index;uniqueIndex:uk_tenant_subscription_period"`
	EndsAt           *time.Time   `gorm:"type:timestamp;index"`
	BillingCycle     string       `gorm:"type:varchar(32);not null;default:'monthly';index"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index;uniqueIndex:uk_tenant_subscription_period"`
	AuditFields
}

func (TenantSubscription) TableName() string { return "tenant_subscriptions" }

// TenantQuotaOverride stores enterprise-contract or temporary quota changes.
type TenantQuotaOverride struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_quota_override"`
	ProductID       int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_tenant_quota_override"`
	QuotaKey        string       `gorm:"type:varchar(64);not null;default:'';index;uniqueIndex:uk_tenant_quota_override"`
	QuotaValue      int64        `gorm:"type:bigint;not null;default:0"`
	Period          string       `gorm:"type:varchar(16);not null;default:'monthly';index;uniqueIndex:uk_tenant_quota_override"`
	OverageStrategy string       `gorm:"type:varchar(32);not null;default:''"`
	Reason          string       `gorm:"type:varchar(500);not null;default:''"`
	EffectiveFrom   *time.Time   `gorm:"type:timestamp;index"`
	EffectiveTo     *time.Time   `gorm:"type:timestamp;index"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TenantQuotaOverride) TableName() string { return "tenant_quota_overrides" }

// AuthVerificationCode stores short-lived email verification codes for public auth flows.
type AuthVerificationCode struct {
	ID         int64      `gorm:"primaryKey;autoIncrement"`
	Purpose    string     `gorm:"type:varchar(64);not null;default:'';index"`
	Email      string     `gorm:"type:varchar(255);not null;default:'';index"`
	CodeHash   string     `gorm:"type:varchar(128);not null;default:'';index"`
	ExpiresAt  time.Time  `gorm:"type:timestamp;not null;index"`
	ConsumedAt *time.Time `gorm:"type:timestamp;index"`
	Status     string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	AuditFields
}

func (AuthVerificationCode) TableName() string { return "auth_verification_codes" }

// Sub2APITenantAccount binds a tenant to its Sub2API account.
type Sub2APITenantAccount struct {
	ID                       int64        `gorm:"primaryKey;autoIncrement"`
	TenantID                 int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_sub2api_tenant_account"`
	Sub2APIAccountID         string       `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_sub2api_tenant_account"`
	AccountName              string       `gorm:"type:varchar(128);not null;default:'';index"`
	DashboardURL             string       `gorm:"type:varchar(512);not null;default:''"`
	LoginEmail               string       `gorm:"column:login_email;type:varchar(128);not null;default:'';index"`
	LoginPasswordCiphertext  string       `gorm:"column:login_password_ciphertext;type:text;not null;default:''"`
	LoginPasswordFingerprint string       `gorm:"column:login_password_fingerprint;type:varchar(64);not null;default:''"`
	AccessTokenExpiresAt     *time.Time   `gorm:"column:access_token_expires_at;type:timestamp;index"`
	AccountStatus            string       `gorm:"type:varchar(32);not null;default:'unknown';index"`
	ProvisionStatus          string       `gorm:"column:provision_status;type:varchar(32);not null;default:'pending';index"`
	Concurrency              int64        `gorm:"type:bigint;not null;default:0"`
	Balance                  float64      `gorm:"type:decimal(18,6);not null;default:0"`
	RPMLimit                 int64        `gorm:"column:rpm_limit;type:bigint;not null;default:0"`
	DefaultKeyID             string       `gorm:"column:default_key_id;type:varchar(128);not null;default:'';index"`
	DefaultKeyName           string       `gorm:"column:default_key_name;type:varchar(128);not null;default:''"`
	DefaultKeyGroupID        int64        `gorm:"column:default_key_group_id;type:bigint;not null;default:2"`
	DefaultKeyQuota          float64      `gorm:"column:default_key_quota;type:decimal(18,6);not null;default:0"`
	DefaultKeyQuotaUsed      float64      `gorm:"column:default_key_quota_used;type:decimal(18,6);not null;default:0"`
	DefaultKeyCiphertext     string       `gorm:"column:default_key_ciphertext;type:text;not null;default:''"`
	DefaultKeyFingerprint    string       `gorm:"column:default_key_fingerprint;type:varchar(64);not null;default:''"`
	DefaultKeyStatus         string       `gorm:"column:default_key_status;type:varchar(32);not null;default:'unknown';index"`
	DefaultKeyExpiresAt      *time.Time   `gorm:"column:default_key_expires_at;type:timestamp;index"`
	DefaultLLMModel          string       `gorm:"column:default_llm_model;type:varchar(128);not null;default:''"`
	QuotaSnapshotJSON        string       `gorm:"column:quota_snapshot_json;type:text;not null;default:'{}'"`
	LastSyncedAt             *time.Time   `gorm:"type:timestamp;index"`
	LastTestedAt             *time.Time   `gorm:"type:timestamp;index"`
	LastProvisionedAt        *time.Time   `gorm:"column:last_provisioned_at;type:timestamp;index"`
	ProvisionMessage         string       `gorm:"column:provision_message;type:text"`
	Status                   enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (Sub2APITenantAccount) TableName() string { return "sub2api_tenant_accounts" }

// Sub2APIRechargeRecord stores platform-side balance recharge audit records.
type Sub2APIRechargeRecord struct {
	ID               int64     `gorm:"primaryKey;autoIncrement"`
	TenantID         int64     `gorm:"type:bigint;not null;index"`
	Sub2APIAccountID string    `gorm:"type:varchar(128);not null;default:'';index"`
	RemoteUserID     int64     `gorm:"type:bigint;not null;default:0;index"`
	Operation        string    `gorm:"type:varchar(32);not null;default:'add';index"`
	Amount           float64   `gorm:"type:decimal(18,6);not null;default:0"`
	BalanceBefore    float64   `gorm:"type:decimal(18,6);not null;default:0"`
	BalanceAfter     float64   `gorm:"type:decimal(18,6);not null;default:0"`
	Notes            string    `gorm:"type:varchar(500);not null;default:''"`
	Status           string    `gorm:"type:varchar(32);not null;default:'success';index"`
	Message          string    `gorm:"type:text"`
	OccurredAt       time.Time `gorm:"type:timestamp;not null;index"`
	AuditFields
}

func (Sub2APIRechargeRecord) TableName() string { return "sub2api_recharge_records" }

// MeteringUsageEvent stores raw usage events before daily/monthly aggregation.
type MeteringUsageEvent struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TenantID       int64     `gorm:"type:bigint;not null;index"`
	ProductID      int64     `gorm:"type:bigint;not null;default:0;index"`
	EventType      string    `gorm:"type:varchar(32);not null;default:'';index"`
	QuotaCode      string    `gorm:"type:varchar(64);not null;default:'';index"`
	Quantity       int64     `gorm:"type:bigint;not null;default:0"`
	Unit           string    `gorm:"type:varchar(32);not null;default:''"`
	Source         string    `gorm:"type:varchar(32);not null;default:'';index"`
	IdempotencyKey string    `gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	MetadataJSON   string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	OccurredAt     time.Time `gorm:"type:timestamp;not null;index"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

func (MeteringUsageEvent) TableName() string { return "metering_usage_events" }

// MeteringUsageDaily is the daily aggregate used by platform and tenant usage views.
type MeteringUsageDaily struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	TenantID     int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_metering_usage_daily"`
	ProductID    int64     `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_metering_usage_daily"`
	BucketDate   time.Time `gorm:"type:date;not null;index;uniqueIndex:uk_metering_usage_daily"`
	QuotaCode    string    `gorm:"type:varchar(64);not null;default:'';index;uniqueIndex:uk_metering_usage_daily"`
	Quantity     int64     `gorm:"type:bigint;not null;default:0"`
	Unit         string    `gorm:"type:varchar(32);not null;default:''"`
	MetadataJSON string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt    time.Time `gorm:"type:timestamp;not null;index"`
}

func (MeteringUsageDaily) TableName() string { return "metering_usage_daily" }

// AuthSession stores identity-domain aware sessions. LoginSession remains for legacy admin tokens.
type AuthSession struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	UserID           int64      `gorm:"type:bigint;not null;default:0;index"`
	DomainType       string     `gorm:"type:varchar(32);not null;default:'';index"`
	TenantID         int64      `gorm:"type:bigint;not null;default:0;index"`
	SubjectType      string     `gorm:"type:varchar(32);not null;default:'';index"`
	SubjectID        int64      `gorm:"type:bigint;not null;default:0;index"`
	RefreshTokenHash string     `gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	ClientIP         string     `gorm:"type:varchar(64);not null;default:''"`
	UserAgent        string     `gorm:"type:varchar(255);not null;default:''"`
	AuthMethod       string     `gorm:"type:varchar(32);not null;default:'password';index"`
	ExpiresAt        time.Time  `gorm:"type:timestamp;not null;index"`
	RevokedAt        *time.Time `gorm:"type:timestamp;index"`
	LastSeenAt       *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

func (AuthSession) TableName() string { return "auth_sessions" }

// PlatformStaffProfile is the business identity for platform employees.
type PlatformStaffProfile struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	UserID         int64        `gorm:"type:bigint;not null;index;uniqueIndex"`
	TeamCode       string       `gorm:"type:varchar(64);not null;default:'';index"`
	JobTitle       string       `gorm:"type:varchar(128);not null;default:''"`
	SupportLevel   string       `gorm:"type:varchar(32);not null;default:'';index"`
	EmploymentType string       `gorm:"type:varchar(32);not null;default:'employee';index"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (PlatformStaffProfile) TableName() string { return "platform_staff_profiles" }

// PlatformTenantGrant allows platform staff to enter a target tenant within a bounded scope.
type PlatformTenantGrant struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	PlatformStaffID int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_platform_tenant_grant"`
	TenantID        int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_platform_tenant_grant"`
	GrantScopeJSON  string     `gorm:"column:grant_scope_json;type:text;not null;default:'{}'"`
	Reason          string     `gorm:"type:varchar(500);not null;default:''"`
	ApprovedBy      int64      `gorm:"type:bigint;not null;default:0;index"`
	ApprovedAt      *time.Time `gorm:"type:timestamp;index"`
	ExpiredAt       time.Time  `gorm:"type:timestamp;not null;index;uniqueIndex:uk_platform_tenant_grant"`
	RevokedAt       *time.Time `gorm:"type:timestamp;index"`
	Status          string     `gorm:"type:varchar(32);not null;default:'active';index;uniqueIndex:uk_platform_tenant_grant"`
	AuditFields
}

func (PlatformTenantGrant) TableName() string { return "platform_tenant_grants" }

// TenantMember is an enterprise employee identity under a tenant.
type TenantMember struct {
	ID           int64        `gorm:"primaryKey;autoIncrement"`
	TenantID     int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_member_user"`
	UserID       int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_member_user"`
	DepartmentID int64        `gorm:"type:bigint;not null;default:0;index"`
	MemberNo     string       `gorm:"type:varchar(64);not null;default:'';index"`
	DisplayName  string       `gorm:"type:varchar(128);not null;default:'';index"`
	JobTitle     string       `gorm:"type:varchar(128);not null;default:''"`
	MemberType   string       `gorm:"type:varchar(32);not null;default:'employee';index"`
	Status       enums.Status `gorm:"type:int;not null;default:0;index"`
	InvitedAt    *time.Time   `gorm:"type:timestamp"`
	JoinedAt     *time.Time   `gorm:"type:timestamp"`
	DisabledAt   *time.Time   `gorm:"type:timestamp"`
	AuditFields
}

func (TenantMember) TableName() string { return "tenant_members" }

// Department stores the enterprise organization tree.
type Department struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_department_tenant_code"`
	ParentID        int64        `gorm:"type:bigint;not null;default:0;index"`
	DepartmentCode  string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_department_tenant_code"`
	Name            string       `gorm:"type:varchar(128);not null;default:'';index"`
	Path            string       `gorm:"type:varchar(512);not null;default:'';index"`
	Depth           int          `gorm:"type:int;not null;default:0;index"`
	ManagerMemberID int64        `gorm:"type:bigint;not null;default:0;index"`
	RegionCode      string       `gorm:"type:varchar(64);not null;default:'';index"`
	SortNo          int          `gorm:"type:int;not null;default:0;index"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (Department) TableName() string { return "departments" }

// EngineerProfile stores enterprise IAM engineer metadata. Product dispatch
// eligibility is owned by AgentProfile and AgentTeamMember.
type EngineerProfile struct {
	ID                 int64        `gorm:"primaryKey;autoIncrement"`
	TenantID           int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_engineer_profile_member"`
	MemberID           int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_engineer_profile_member"`
	SkillTagsJSON      string       `gorm:"column:skill_tags_json;type:text;not null;default:'[]'"`
	LanguagesJSON      string       `gorm:"column:languages_json;type:text;not null;default:'[]'"`
	ServiceRegionsJSON string       `gorm:"column:service_regions_json;type:text;not null;default:'[]'"`
	Timezone           string       `gorm:"type:varchar(64);not null;default:'UTC'"`
	MaxTicketLoad      int          `gorm:"type:int;not null;default:0"`
	DispatchEnabled    bool         `gorm:"not null;default:false;index"`
	Status             enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (EngineerProfile) TableName() string { return "engineer_profiles" }

// CustomerOrg is the customer-side organization boundary for device ownership.
type CustomerOrg struct {
	ID                 int64        `gorm:"primaryKey;autoIncrement"`
	TenantID           int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_org_no"`
	CustomerNo         string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_customer_org_no"`
	Name               string       `gorm:"type:varchar(200);not null;default:'';index"`
	CountryRegion      string       `gorm:"type:varchar(64);not null;default:'';index"`
	Industry           string       `gorm:"type:varchar(100);not null;default:'';index"`
	Level              string       `gorm:"type:varchar(32);not null;default:'';index"`
	DefaultLocale      string       `gorm:"type:varchar(16);not null;default:'en-US'"`
	Timezone           string       `gorm:"type:varchar(64);not null;default:'UTC'"`
	ExternalCustomerID string       `gorm:"type:varchar(128);not null;default:'';index"`
	Status             enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (CustomerOrg) TableName() string { return "customer_orgs" }

// CustomerUser is a formal customer contact identity.
type CustomerUser struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	TenantID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_user_identity"`
	CustomerOrgID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_user_identity"`
	UserID        int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_customer_user_identity"`
	DisplayName   string       `gorm:"type:varchar(128);not null;default:'';index"`
	Email         string       `gorm:"type:varchar(255);not null;default:'';index"`
	Phone         string       `gorm:"type:varchar(64);not null;default:'';index"`
	Locale        string       `gorm:"type:varchar(16);not null;default:'en-US'"`
	Timezone      string       `gorm:"type:varchar(64);not null;default:'UTC'"`
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`
	LastSeenAt    *time.Time   `gorm:"type:timestamp;index"`
	AuditFields
}

func (CustomerUser) TableName() string { return "customer_users" }

// CustomerRegistrationGrant is a one-time enterprise invitation for creating a
// customer portal identity. Only the credential hash is persisted.
type CustomerRegistrationGrant struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	TenantID         int64      `gorm:"type:bigint;not null;index"`
	DomainType       string     `gorm:"type:varchar(32);not null;default:'customer';index"`
	CustomerOrgID    int64      `gorm:"type:bigint;not null;default:0;index"`
	PartnerCompanyID int64      `gorm:"type:bigint;not null;default:0;index"`
	Email            string     `gorm:"type:varchar(255);not null;index"`
	DisplayName      string     `gorm:"type:varchar(128);not null;default:''"`
	TokenHash        string     `gorm:"type:varchar(64);not null;uniqueIndex"`
	RoleCodesJSON    string     `gorm:"column:role_codes_json;type:text;not null;default:'[]'"`
	Status           string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	ExpiresAt        time.Time  `gorm:"type:timestamp;not null;index"`
	ConsumedAt       *time.Time `gorm:"type:timestamp;index"`
	ConsumedUserID   int64      `gorm:"type:bigint;not null;default:0;index"`
	AuditFields
}

func (CustomerRegistrationGrant) TableName() string { return "customer_registration_grants" }

type CustomerUserIdentity struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	TenantID       int64        `gorm:"type:bigint;not null;index"`
	CustomerUserID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_user_identity_provider"`
	IdentityType   string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_customer_user_identity_provider"`
	IdentifierHash string       `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_customer_user_identity_provider"`
	Provider       string       `gorm:"type:varchar(64);not null;default:'';index"`
	VerifiedAt     *time.Time   `gorm:"type:timestamp"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (CustomerUserIdentity) TableName() string { return "customer_user_identities" }

type CustomerUserRole struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	TenantID       int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_user_role"`
	CustomerUserID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_user_role"`
	RoleCode       string       `gorm:"type:varchar(64);not null;default:'';index;uniqueIndex:uk_customer_user_role"`
	ScopeRuleJSON  string       `gorm:"column:scope_rule_json;type:text;not null;default:'{}'"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (CustomerUserRole) TableName() string { return "customer_user_roles" }

type PartnerCompany struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	TenantID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_partner_company_no"`
	PartnerNo     string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_partner_company_no"`
	Name          string       `gorm:"type:varchar(200);not null;default:'';index"`
	PartnerType   string       `gorm:"type:varchar(32);not null;default:'';index"`
	CountryRegion string       `gorm:"type:varchar(64);not null;default:'';index"`
	ContactName   string       `gorm:"type:varchar(128);not null;default:''"`
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (PartnerCompany) TableName() string { return "partner_companies" }

type PartnerContract struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_partner_contract_no"`
	PartnerCompanyID int64        `gorm:"type:bigint;not null;index"`
	ContractNo       string       `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_partner_contract_no"`
	EffectiveFrom    time.Time    `gorm:"type:timestamp;not null;index"`
	EffectiveTo      *time.Time   `gorm:"type:timestamp;index"`
	ServiceLevel     string       `gorm:"type:varchar(64);not null;default:'';index"`
	ScopeRuleJSON    string       `gorm:"column:scope_rule_json;type:text;not null;default:'{}'"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (PartnerContract) TableName() string { return "partner_contracts" }

type PartnerAccount struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_partner_account_user"`
	PartnerCompanyID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_partner_account_user"`
	UserID           int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_partner_account_user"`
	DisplayName      string       `gorm:"type:varchar(128);not null;default:'';index"`
	Email            string       `gorm:"type:varchar(255);not null;default:'';index"`
	Phone            string       `gorm:"type:varchar(64);not null;default:'';index"`
	LanguagesJSON    string       `gorm:"column:languages_json;type:text;not null;default:'[]'"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	LastActiveAt     *time.Time   `gorm:"type:timestamp;index"`
	AuditFields
}

func (PartnerAccount) TableName() string { return "partner_accounts" }

type PartnerAuthorizationScope struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`
	TenantID          int64        `gorm:"type:bigint;not null;index"`
	PartnerContractID int64        `gorm:"type:bigint;not null;default:0;index"`
	PartnerAccountID  int64        `gorm:"type:bigint;not null;default:0;index"`
	ResourceType      string       `gorm:"type:varchar(64);not null;default:'';index"`
	ResourceID        string       `gorm:"type:varchar(128);not null;default:'';index"`
	ScopeRuleJSON     string       `gorm:"column:scope_rule_json;type:text;not null;default:'{}'"`
	ExpiredAt         *time.Time   `gorm:"type:timestamp;index"`
	Status            enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (PartnerAuthorizationScope) TableName() string { return "partner_authorization_scopes" }

type AuthRole struct {
	ID          int64        `gorm:"primaryKey;autoIncrement"`
	TenantID    int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_role_code"`
	DomainType  string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_auth_role_code"`
	Code        string       `gorm:"type:varchar(100);not null;default:'';uniqueIndex:uk_auth_role_code"`
	Name        string       `gorm:"type:varchar(128);not null;default:'';index"`
	Description string       `gorm:"type:text"`
	IsBuiltin   bool         `gorm:"not null;default:false;index"`
	Status      enums.Status `gorm:"type:int;not null;default:0;index"`
	SortNo      int          `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (AuthRole) TableName() string { return "auth_roles" }

type AuthRoleBinding struct {
	ID          int64        `gorm:"primaryKey;autoIncrement"`
	TenantID    int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_role_binding"`
	DomainType  string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_auth_role_binding"`
	RoleID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_auth_role_binding"`
	SubjectType string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_auth_role_binding"`
	SubjectID   int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_role_binding"`
	Status      enums.Status `gorm:"type:int;not null;default:0;index"`
	EffectiveAt *time.Time   `gorm:"type:timestamp;index"`
	ExpiredAt   *time.Time   `gorm:"type:timestamp;index"`
	AuditFields
}

func (AuthRoleBinding) TableName() string { return "auth_role_bindings" }

type AuthPermissionCatalog struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	DomainType     string       `gorm:"type:varchar(32);not null;default:'';index"`
	PermissionCode string       `gorm:"type:varchar(150);not null;uniqueIndex"`
	Name           string       `gorm:"type:varchar(128);not null;default:''"`
	PermissionType string       `gorm:"type:varchar(32);not null;default:'';index"`
	ResourcePath   string       `gorm:"type:varchar(255);not null;default:'';index"`
	Action         string       `gorm:"type:varchar(32);not null;default:'';index"`
	Description    string       `gorm:"type:text"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	SortNo         int          `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (AuthPermissionCatalog) TableName() string { return "auth_permission_catalog" }

type AuthRolePermission struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	TenantID       int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_role_permission"`
	RoleID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_auth_role_permission"`
	PermissionCode string       `gorm:"type:varchar(150);not null;default:'';index;uniqueIndex:uk_auth_role_permission"`
	Effect         string       `gorm:"type:varchar(16);not null;default:'allow';index"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (AuthRolePermission) TableName() string { return "auth_role_permissions" }

type AuthSubjectPermissionOverride struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	TenantID       int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_subject_permission_override"`
	DomainType     string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_auth_subject_permission_override"`
	SubjectType    string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_auth_subject_permission_override"`
	SubjectID      int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_auth_subject_permission_override"`
	PermissionCode string       `gorm:"type:varchar(150);not null;default:'';index;uniqueIndex:uk_auth_subject_permission_override"`
	Effect         string       `gorm:"type:varchar(16);not null;default:'allow';index"`
	ExpiredAt      *time.Time   `gorm:"type:timestamp;index"`
	ApprovalID     int64        `gorm:"type:bigint;not null;default:0;index"`
	Remark         string       `gorm:"type:text"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (AuthSubjectPermissionOverride) TableName() string {
	return "auth_subject_permission_overrides"
}

type DataScopePolicy struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	TenantID      int64        `gorm:"type:bigint;not null;default:0;index"`
	DomainType    string       `gorm:"type:varchar(32);not null;default:'';index"`
	SubjectType   string       `gorm:"type:varchar(32);not null;default:'';index"`
	SubjectID     int64        `gorm:"type:bigint;not null;default:0;index"`
	ScopeType     string       `gorm:"type:varchar(64);not null;default:'';index"`
	ScopeRuleJSON string       `gorm:"column:scope_rule_json;type:text;not null;default:'{}'"`
	Include       bool         `gorm:"not null;default:true;index"`
	EffectiveAt   *time.Time   `gorm:"type:timestamp;index"`
	ExpiredAt     *time.Time   `gorm:"type:timestamp;index"`
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (DataScopePolicy) TableName() string { return "data_scope_policies" }

type FieldMaskingPolicy struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;default:0;index"`
	DomainType       string       `gorm:"type:varchar(32);not null;default:'';index"`
	FieldPath        string       `gorm:"type:varchar(255);not null;default:'';index"`
	MaskRule         string       `gorm:"type:varchar(64);not null;default:'';index"`
	VisibleRolesJSON string       `gorm:"column:visible_roles_json;type:text;not null;default:'[]'"`
	ExportRule       string       `gorm:"type:varchar(64);not null;default:'';index"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (FieldMaskingPolicy) TableName() string { return "field_masking_policies" }

type TemporaryAccessGrant struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	TenantID      int64      `gorm:"type:bigint;not null;default:0;index"`
	GranteeType   string     `gorm:"type:varchar(32);not null;default:'';index"`
	GranteeID     int64      `gorm:"type:bigint;not null;default:0;index"`
	ResourceType  string     `gorm:"type:varchar(64);not null;default:'';index"`
	ResourceID    string     `gorm:"type:varchar(128);not null;default:'';index"`
	ScopeRuleJSON string     `gorm:"column:scope_rule_json;type:text;not null;default:'{}'"`
	Reason        string     `gorm:"type:varchar(500);not null;default:''"`
	ApprovedBy    int64      `gorm:"type:bigint;not null;default:0;index"`
	ApprovedAt    *time.Time `gorm:"type:timestamp;index"`
	ExpiredAt     time.Time  `gorm:"type:timestamp;not null;index"`
	RevokedAt     *time.Time `gorm:"type:timestamp;index"`
	Status        string     `gorm:"type:varchar(32);not null;default:'active';index"`
	AuditFields
}

func (TemporaryAccessGrant) TableName() string { return "temporary_access_grants" }

type AuthAuditLog struct {
	ID               int64     `gorm:"primaryKey;autoIncrement"`
	TenantID         int64     `gorm:"type:bigint;not null;default:0;index"`
	DomainType       string    `gorm:"type:varchar(32);not null;default:'';index"`
	ActorUserID      int64     `gorm:"type:bigint;not null;default:0;index"`
	ActorSubjectType string    `gorm:"type:varchar(32);not null;default:'';index"`
	ActorSubjectID   int64     `gorm:"type:bigint;not null;default:0;index"`
	TargetType       string    `gorm:"type:varchar(64);not null;default:'';index"`
	TargetID         string    `gorm:"type:varchar(128);not null;default:'';index"`
	Action           string    `gorm:"type:varchar(64);not null;default:'';index"`
	BeforeStateJSON  string    `gorm:"column:before_state_json;type:text"`
	AfterStateJSON   string    `gorm:"column:after_state_json;type:text"`
	RequestID        string    `gorm:"type:varchar(128);not null;default:'';index"`
	SupportGrantID   int64     `gorm:"type:bigint;not null;default:0;index"`
	RiskLevel        string    `gorm:"type:varchar(16);not null;default:'medium';index"`
	Status           string    `gorm:"type:varchar(20);not null;default:'success';index"`
	IPAddress        string    `gorm:"type:varchar(64);not null;default:''"`
	UserAgent        string    `gorm:"type:varchar(255);not null;default:''"`
	OccurredAt       time.Time `gorm:"type:timestamp;not null;index"`
}

func (AuthAuditLog) TableName() string { return "auth_audit_logs" }

type UserMFASetting struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	UserID          int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_mfa_type"`
	MFAType         string     `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_user_mfa_type"`
	SecretEncrypted string     `gorm:"type:text"`
	PhoneNumber     string     `gorm:"type:varchar(32);not null;default:''"`
	EmailAddress    string     `gorm:"type:varchar(255);not null;default:''"`
	CredentialID    string     `gorm:"type:text"`
	PublicKey       string     `gorm:"type:text"`
	IsPrimary       bool       `gorm:"not null;default:false;index"`
	Enabled         bool       `gorm:"not null;default:false;index"`
	LastUsedAt      *time.Time `gorm:"type:timestamp;index"`
	CreatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	DeletedAt       *time.Time `gorm:"type:timestamp;index"`
}

func (UserMFASetting) TableName() string { return "user_mfa_settings" }

type UserMFABackupCode struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	UserID    int64      `gorm:"type:bigint;not null;index"`
	CodeHash  string     `gorm:"type:varchar(128);not null;index"`
	Used      bool       `gorm:"not null;default:false;index"`
	UsedAt    *time.Time `gorm:"type:timestamp;index"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;index"`
}

func (UserMFABackupCode) TableName() string { return "user_mfa_backup_codes" }

type UserPasswordHistory struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	UserID       int64     `gorm:"type:bigint;not null;index"`
	PasswordHash string    `gorm:"type:varchar(256);not null"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index"`
}

func (UserPasswordHistory) TableName() string { return "user_password_history" }

type ServiceAccount struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	TenantID            int64      `gorm:"type:bigint;not null;index"`
	Name                string     `gorm:"type:varchar(128);not null;default:'';index"`
	Description         string     `gorm:"type:varchar(500);not null;default:''"`
	ClientID            string     `gorm:"type:varchar(64);not null;uniqueIndex"`
	ClientSecretHash    string     `gorm:"type:varchar(128);not null;default:''"`
	APIKeyHash          string     `gorm:"type:varchar(128);not null;default:'';index"`
	PermissionsJSON     string     `gorm:"column:permissions_json;type:text;not null;default:'[]'"`
	AllowedIPsJSON      string     `gorm:"column:allowed_ips_json;type:text;not null;default:'[]'"`
	AllowedReferersJSON string     `gorm:"column:allowed_referers_json;type:text;not null;default:'[]'"`
	RateLimitProfile    string     `gorm:"type:varchar(32);not null;default:'service';index"`
	ExpiresAt           *time.Time `gorm:"type:timestamp;index"`
	LastUsedAt          *time.Time `gorm:"type:timestamp;index"`
	LastRotatedAt       *time.Time `gorm:"type:timestamp;index"`
	Status              string     `gorm:"type:varchar(32);not null;default:'active';index"`
	CreatedByType       string     `gorm:"type:varchar(32);not null;default:''"`
	CreatedByID         int64      `gorm:"type:bigint;not null;default:0"`
	CreatedAt           time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt           time.Time  `gorm:"type:timestamp;not null;index"`
	DeletedAt           *time.Time `gorm:"type:timestamp;index"`
}

func (ServiceAccount) TableName() string { return "service_accounts" }

type RateLimitProfile struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	ProfileCode     string       `gorm:"type:varchar(64);not null;uniqueIndex"`
	Name            string       `gorm:"type:varchar(128);not null;default:''"`
	TenantLimit     int          `gorm:"type:int;not null;default:1000"`
	UserLimit       int          `gorm:"type:int;not null;default:100"`
	IPLimit         int          `gorm:"column:ip_limit;type:int;not null;default:60"`
	BurstMultiplier float64      `gorm:"type:decimal(3,1);not null;default:1.5"`
	WindowSeconds   int          `gorm:"type:int;not null;default:60"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (RateLimitProfile) TableName() string { return "rate_limit_profiles" }

type SSOConfig struct {
	ID                     int64        `gorm:"primaryKey;autoIncrement"`
	TenantID               int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_sso_config_provider"`
	ProviderType           string       `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_sso_config_provider"`
	ProviderName           string       `gorm:"type:varchar(128);not null;default:''"`
	IssuerURL              string       `gorm:"type:varchar(512);not null;default:''"`
	ClientID               string       `gorm:"type:varchar(256);not null;default:''"`
	ClientSecretEncrypted  string       `gorm:"type:text"`
	AuthorizationEndpoint  string       `gorm:"type:varchar(512);not null;default:''"`
	TokenEndpoint          string       `gorm:"type:varchar(512);not null;default:''"`
	UserinfoEndpoint       string       `gorm:"type:varchar(512);not null;default:''"`
	JWKSURI                string       `gorm:"column:jwks_uri;type:varchar(512);not null;default:''"`
	Scopes                 string       `gorm:"type:varchar(256);not null;default:'openid profile email'"`
	AttributeMappingJSON   string       `gorm:"column:attribute_mapping_json;type:text;not null;default:'{}'"`
	DomainRestrictionsJSON string       `gorm:"column:domain_restrictions_json;type:text;not null;default:'[]'"`
	Enabled                bool         `gorm:"not null;default:false;index"`
	AutoProvision          bool         `gorm:"not null;default:false;index"`
	DefaultRoleID          int64        `gorm:"type:bigint;not null;default:0"`
	MetadataXML            string       `gorm:"column:metadata_xml;type:text"`
	Status                 enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (SSOConfig) TableName() string { return "sso_configs" }
