package dto

type EnterpriseIAMMemberDTO struct {
	ID                 int64                          `json:"id"`
	TenantID           int64                          `json:"tenant_id"`
	UserID             int64                          `json:"user_id"`
	Username           string                         `json:"username"`
	Email              string                         `json:"email"`
	Mobile             string                         `json:"mobile"`
	DisplayName        string                         `json:"display_name"`
	MemberNo           string                         `json:"member_no"`
	DepartmentID       int64                          `json:"department_id"`
	DepartmentName     string                         `json:"department_name"`
	JobTitle           string                         `json:"job_title"`
	MemberType         string                         `json:"member_type"`
	Roles              []string                       `json:"roles"`
	SkillTagsJSON      string                         `json:"skill_tags_json"`
	LanguagesJSON      string                         `json:"languages_json"`
	ServiceRegionsJSON string                         `json:"service_regions_json"`
	DispatchEnabled    bool                           `json:"dispatch_enabled"`
	ProductGroups      []EnterpriseIAMProductGroupDTO `json:"product_groups"`
	Status             int                            `json:"status"`
	JoinedAt           string                         `json:"joined_at"`
	UpdatedAt          string                         `json:"updated_at"`
}

type EnterpriseIAMProductGroupDTO struct {
	TeamID      int64  `json:"team_id"`
	TeamName    string `json:"team_name"`
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
}

type EnterpriseIAMCustomerUserDTO struct {
	ID              int64    `json:"id"`
	TenantID        int64    `json:"tenant_id"`
	CustomerOrgID   int64    `json:"customer_org_id"`
	CustomerOrgName string   `json:"customer_org_name"`
	UserID          int64    `json:"user_id"`
	DisplayName     string   `json:"display_name"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	Locale          string   `json:"locale"`
	Timezone        string   `json:"timezone"`
	Roles           []string `json:"roles"`
	Status          int      `json:"status"`
	LastSeenAt      string   `json:"last_seen_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type EnterpriseIAMPartnerDTO struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenant_id"`
	PartnerNo     string `json:"partner_no"`
	Name          string `json:"name"`
	PartnerType   string `json:"partner_type"`
	CountryRegion string `json:"country_region"`
	ContactName   string `json:"contact_name"`
	AccountCount  int64  `json:"account_count"`
	ContractCount int64  `json:"contract_count"`
	Status        int    `json:"status"`
	UpdatedAt     string `json:"updated_at"`
}

type EnterpriseIAMDepartmentDTO struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenant_id"`
	ParentID        int64  `json:"parent_id"`
	DepartmentCode  string `json:"department_code"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	Depth           int    `json:"depth"`
	ManagerMemberID int64  `json:"manager_member_id"`
	ManagerName     string `json:"manager_name"`
	RegionCode      string `json:"region_code"`
	MemberCount     int64  `json:"member_count"`
	Status          int    `json:"status"`
	UpdatedAt       string `json:"updated_at"`
}

type EnterpriseIAMMemberInviteDTO struct {
	Member          *EnterpriseIAMMemberDTO `json:"member"`
	InitialPassword string                  `json:"initialPassword"`
}

type EnterpriseIAMCustomerAuthorizeDTO struct {
	CustomerUser    *EnterpriseIAMCustomerUserDTO `json:"customerUser"`
	InitialPassword string                        `json:"initialPassword"`
}

type EnterpriseIAMCustomerInviteDTO struct {
	GrantID         int64  `json:"grantId"`
	InviteCode      string `json:"inviteCode"`
	RegistrationURL string `json:"registrationUrl"`
	Email           string `json:"email"`
	CustomerOrgID   int64  `json:"customerOrgId"`
	CustomerOrgName string `json:"customerOrgName"`
	ExpiresAt       string `json:"expiresAt"`
	EmailSent       bool   `json:"emailSent"`
	EmailError      string `json:"emailError,omitempty"`
}

type EnterpriseIAMPartnerAdminInviteDTO struct {
	Partner         *EnterpriseIAMPartnerDTO `json:"partner"`
	InviteCode      string                   `json:"inviteCode"`
	RegistrationURL string                   `json:"registrationUrl"`
	Email           string                   `json:"email"`
	PartnerID       int64                    `json:"partnerId"`
	PartnerName     string                   `json:"partnerName"`
	ExpiresAt       string                   `json:"expiresAt"`
	EmailSent       bool                     `json:"emailSent"`
	EmailError      string                   `json:"emailError,omitempty"`
}
