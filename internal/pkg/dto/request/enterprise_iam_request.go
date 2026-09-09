package request

type EnterpriseMemberInviteRequest struct {
	Username        string   `json:"username" binding:"required"`
	DisplayName     string   `json:"displayName" binding:"required"`
	Password        string   `json:"password" binding:"required"`
	Email           string   `json:"email"`
	Mobile          string   `json:"mobile"`
	DepartmentID    int64    `json:"departmentId"`
	JobTitle        string   `json:"jobTitle"`
	MemberType      string   `json:"memberType"`
	RoleCodes       []string `json:"roleCodes"`
	DispatchEnabled bool     `json:"dispatchEnabled"`
}

type EnterpriseMemberUpdateRequest struct {
	DisplayName     *string  `json:"displayName"`
	Email           *string  `json:"email"`
	Mobile          *string  `json:"mobile"`
	DepartmentID    *int64   `json:"departmentId"`
	JobTitle        *string  `json:"jobTitle"`
	MemberType      *string  `json:"memberType"`
	RoleCodes       []string `json:"roleCodes"`
	DispatchEnabled *bool    `json:"dispatchEnabled"`
	Status          *int     `json:"status"`
}

type EnterpriseMemberStatusRequest struct {
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

type EnterpriseCustomerUserAuthorizeRequest struct {
	Username      string   `json:"username" binding:"required"`
	DisplayName   string   `json:"displayName" binding:"required"`
	Password      string   `json:"password" binding:"required"`
	Email         string   `json:"email"`
	Mobile        string   `json:"mobile"`
	CustomerOrgID int64    `json:"customerOrgId"`
	CustomerOrg   string   `json:"customerOrg"`
	RoleCodes     []string `json:"roleCodes"`
	Locale        string   `json:"locale"`
	Timezone      string   `json:"timezone"`
}

type EnterpriseCustomerUserUpdateRequest struct {
	DisplayName   *string  `json:"displayName"`
	Email         *string  `json:"email"`
	Mobile        *string  `json:"mobile"`
	CustomerOrgID *int64   `json:"customerOrgId"`
	CustomerOrg   *string  `json:"customerOrg"`
	RoleCodes     []string `json:"roleCodes"`
	Locale        *string  `json:"locale"`
	Timezone      *string  `json:"timezone"`
	Status        *int     `json:"status"`
}

type EnterpriseCustomerUserStatusRequest struct {
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

type EnterpriseCustomerUserInviteRequest struct {
	DisplayName   string   `json:"displayName"`
	Email         string   `json:"email" binding:"required"`
	CustomerOrgID int64    `json:"customerOrgId"`
	CustomerOrg   string   `json:"customerOrg"`
	RoleCodes     []string `json:"roleCodes"`
	Locale        string   `json:"locale"`
	Timezone      string   `json:"timezone"`
}

type EnterprisePartnerAdminInviteRequest struct {
	PartnerCompanyID int64  `json:"partnerCompanyId"`
	PartnerNo        string `json:"partnerNo"`
	PartnerName      string `json:"partnerName" binding:"required"`
	PartnerType      string `json:"partnerType"`
	CountryRegion    string `json:"countryRegion"`
	ContactName      string `json:"contactName"`
	Username         string `json:"username"`
	DisplayName      string `json:"displayName"`
	Password         string `json:"password"`
	Email            string `json:"email" binding:"required"`
	Mobile           string `json:"mobile"`
}

type EnterprisePartnerUpdateRequest struct {
	PartnerNo     *string `json:"partnerNo"`
	PartnerName   *string `json:"partnerName"`
	PartnerType   *string `json:"partnerType"`
	CountryRegion *string `json:"countryRegion"`
	ContactName   *string `json:"contactName"`
	Status        *int    `json:"status"`
}

type EnterprisePartnerStatusRequest struct {
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

type EnterpriseDepartmentCreateRequest struct {
	ParentID        int64  `json:"parentId"`
	Name            string `json:"name" binding:"required"`
	RegionCode      string `json:"regionCode"`
	ManagerMemberID int64  `json:"managerMemberId"`
}
