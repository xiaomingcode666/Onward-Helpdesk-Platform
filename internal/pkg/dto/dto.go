package dto

import "remotehelpdesk/internal/pkg/enums"

// AuthPrincipal 是认证后的身份主体，包含用户基本信息、租户上下文和权限数据。
// 所有 handler / service 在需要身份信息和权限判断时应使用此对象。
type AuthPrincipal struct {
	UserID      int64
	Username    string
	Nickname    string
	Avatar      string
	Status      enums.Status
	Roles       []string
	Permissions []string

	// TenantID 当前请求的租户 ID（int64，与 models.Tenant.ID 一致）。
	// 平台级操作时为 0。
	TenantID int64 `json:"tenantId"`
	// TargetTenantID 平台人员进入某租户排障时的显式目标租户。
	TargetTenantID int64 `json:"targetTenantId,omitempty"`
	// TenantName 当前租户名称（冗余，用于展示）。
	TenantName string `json:"tenantName"`
	// DomainType 用户所属身份域：platform / enterprise / customer / partner / service_account。
	DomainType string `json:"domainType"`
	// Domain 保留旧字段兼容，语义等同 DomainType。
	Domain string `json:"domain"`
	// SubjectType 主体类型：platform_staff / enterprise_member / customer_user / temp_visitor / partner_account / service_account。
	SubjectType string `json:"subjectType"`
	// SubjectID 当前域主体 ID，用于角色绑定、数据范围、审计和缓存 key。
	SubjectID        int64  `json:"subjectId"`
	MemberID         int64  `json:"memberId,omitempty"`
	PlatformStaffID  int64  `json:"platformStaffId,omitempty"`
	CustomerUserID   int64  `json:"customerUserId,omitempty"`
	PartnerAccountID int64  `json:"partnerAccountId,omitempty"`
	ServiceAccountID int64  `json:"serviceAccountId,omitempty"`
	CustomerOrgID    int64  `json:"customerOrgId,omitempty"`
	SessionID        int64  `json:"sessionId,omitempty"`
	AuthMethod       string `json:"authMethod,omitempty"`
	// DataScopes 数据范围策略列表。
	DataScopes []DataScope `json:"dataScopes,omitempty"`
	// FeatureFlags 当前租户套餐、试点功能、禁用功能快照。
	FeatureFlags   map[string]bool `json:"featureFlags,omitempty"`
	SupportGrantID int64           `json:"supportGrantId,omitempty"`
	SupportMode    string          `json:"supportMode,omitempty"`
	ImpersonatedBy string          `json:"impersonatedBy,omitempty"`
	// Locale 用户语言偏好，例如 "zh-CN" / "en-US" / "es-ES"。
	Locale string `json:"locale"`
	// Timezone 用户时区，例如 "America/New_York" / "Asia/Shanghai"。
	Timezone string `json:"timezone"`
}

func (p *AuthPrincipal) EffectiveDomainType() string {
	if p == nil {
		return ""
	}
	if p.DomainType != "" {
		return p.DomainType
	}
	return p.Domain
}

// EffectiveTenantID returns the tenant being operated on. Platform support
// sessions use TargetTenantID while enterprise sessions carry TenantID directly.
func (p *AuthPrincipal) EffectiveTenantID() int64 {
	if p == nil {
		return 0
	}
	if p.TenantID > 0 {
		return p.TenantID
	}
	return p.TargetTenantID
}

func (p *AuthPrincipal) IsPlatform() bool {
	return p != nil && p.EffectiveDomainType() == "platform"
}

func (p *AuthPrincipal) IsEnterprise() bool {
	return p != nil && p.EffectiveDomainType() == "enterprise"
}

func (p *AuthPrincipal) IsCustomer() bool {
	return p != nil && p.EffectiveDomainType() == "customer"
}

func (p *AuthPrincipal) IsPartner() bool {
	return p != nil && p.EffectiveDomainType() == "partner"
}

func (p *AuthPrincipal) IsServiceAccount() bool {
	return p != nil && p.EffectiveDomainType() == "service_account"
}

func (p *AuthPrincipal) HasRole(roleCode string) bool {
	if p == nil {
		return false
	}
	for _, role := range p.Roles {
		if role == roleCode {
			return true
		}
	}
	return false
}

func (p *AuthPrincipal) HasPermission(permissionCode string) bool {
	if p == nil {
		return false
	}
	for _, permission := range p.Permissions {
		if permission == permissionCode {
			return true
		}
	}
	return false
}

// DataScope 表示一条数据范围策略，用于在 tenant_id 基础上进一步限制可操作的数据范围。
type DataScope struct {
	SubjectType string   `json:"subjectType"`
	SubjectID   int64    `json:"subjectId"`
	ScopeType   string   `json:"scopeType"`
	ScopeValues []string `json:"scopeValues,omitempty"`
	Include     bool     `json:"include"`
}

type WxWorkKFChannelConfig struct {
	OpenKfID string `json:"openKfId"`
}

type WebChannelConfig struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	ThemeColor      string `json:"themeColor"`
	Position        string `json:"position"`
	Width           string `json:"width"`
	UserTokenSecret string `json:"userTokenSecret,omitempty"`
}

type WechatMPChannelConfig struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	ThemeColor      string `json:"themeColor"`
	UserTokenSecret string `json:"userTokenSecret,omitempty"`
}
