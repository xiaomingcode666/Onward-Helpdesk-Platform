package response

import "remotehelpdesk/internal/pkg/enums"

type AuthUserResponse struct {
	ID       int64        `json:"id"`
	Username string       `json:"username"`
	Nickname string       `json:"nickname"`
	Avatar   string       `json:"avatar"`
	Email    string       `json:"email"`
	Mobile   string       `json:"mobile"`
	Locale   string       `json:"locale"`
	Timezone string       `json:"timezone"`
	Status   enums.Status `json:"status"`
	Roles    []string     `json:"roles"`
}

type LoginResponse struct {
	AccessToken           string                         `json:"accessToken"`
	ExpiresAt             string                         `json:"expiresAt"`
	User                  *AuthUserResponse              `json:"user"`
	Permissions           []string                       `json:"permissions"`
	Roles                 []string                       `json:"roles"`
	FeatureFlags          map[string]bool                `json:"featureFlags,omitempty"`
	TenantID              int64                          `json:"tenantId"`
	DomainType            string                         `json:"domainType"`
	SubjectType           string                         `json:"subjectType"`
	SubjectID             int64                          `json:"subjectId"`
	PartnerAccountID      int64                          `json:"partnerAccountId,omitempty"`
	SupportGrantID        int64                          `json:"supportGrantId,omitempty"`
	SupportMode           string                         `json:"supportMode,omitempty"`
	ImpersonatedBy        string                         `json:"impersonatedBy,omitempty"`
	Locale                string                         `json:"locale"`
	TenantDefaultLocale   string                         `json:"tenantDefaultLocale,omitempty"`
	CustomerDefaultLocale string                         `json:"customerDefaultLocale,omitempty"`
	Timezone              string                         `json:"timezone"`
	AvailablePortals      []LoginPortalOptionResponse    `json:"availablePortals,omitempty"`
	RequiresPortalChoice  bool                           `json:"requiresPortalChoice,omitempty"`
	Membership            *TenantMembershipResponse      `json:"membership,omitempty"`
	TenantBranding        *TenantBrandingResponse        `json:"tenantBranding,omitempty"`
	ExternalPortals       []TenantExternalPortalResponse `json:"externalPortals,omitempty"`
}

type TenantBrandingResponse struct {
	BrandName     string `json:"brandName"`
	LogoURL       string `json:"logoUrl"`
	CustomDomain  string `json:"customDomain,omitempty"`
	CustomerTheme string `json:"customerTheme"`
}

type TenantExternalPortalResponse struct {
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	EmbedMode string `json:"embedMode"`
}

type LoginPortalOptionResponse struct {
	DomainType       string `json:"domainType"`
	Label            string `json:"label"`
	TenantID         int64  `json:"tenantId"`
	SubjectType      string `json:"subjectType"`
	SubjectID        int64  `json:"subjectId"`
	PartnerAccountID int64  `json:"partnerAccountId,omitempty"`
	Default          bool   `json:"default"`
}

type TenantMembershipResponse struct {
	Paid     bool   `json:"paid"`
	PlanID   int64  `json:"planId,omitempty"`
	PlanCode string `json:"planCode,omitempty"`
	PlanName string `json:"planName,omitempty"`
	PlanType string `json:"planType,omitempty"`
}

type PublicConfigResponse struct {
	Language      string `json:"language"`
	WxWorkEnabled bool   `json:"wxworkEnabled"`
	OIDCEnabled   bool   `json:"oidcEnabled"`
}

type CustomerRegistrationContextResponse struct {
	Method      string                   `json:"method"`
	DomainType  string                   `json:"domainType,omitempty"`
	Email       string                   `json:"email,omitempty"`
	DisplayName string                   `json:"displayName,omitempty"`
	ExpiresAt   string                   `json:"expiresAt,omitempty"`
	Registered  bool                     `json:"registered"`
	Tenant      *ResolvedTenantResponse  `json:"tenant"`
	CustomerOrg *CustomerRegistrationOrg `json:"customerOrg,omitempty"`
	Partner     *CustomerRegistrationOrg `json:"partner,omitempty"`
	Product     *ResolvedProductResponse `json:"product,omitempty"`
	Device      *ResolvedDeviceResponse  `json:"device,omitempty"`
}

type CustomerRegistrationOrg struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
