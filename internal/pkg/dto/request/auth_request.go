package request

type LoginRequest struct {
	Username              string `json:"username"`
	Password              string `json:"password"`
	DomainType            string `json:"domainType"`
	PortalChoiceConfirmed bool   `json:"portalChoiceConfirmed"`
}

type CustomerRegistrationVerifyRequest struct {
	Method     string `json:"method" binding:"required"`
	Credential string `json:"credential"`
}

type CustomerRegistrationRequest struct {
	Method           string `json:"method" binding:"required"`
	Credential       string `json:"credential"`
	Username         string `json:"username" binding:"required"`
	DisplayName      string `json:"displayName"`
	Email            string `json:"email" binding:"required"`
	Password         string `json:"password" binding:"required"`
	VerificationCode string `json:"verificationCode"`
}

type PortalInvitationVerifyRequest struct {
	DomainType string `json:"domainType" binding:"required"`
	Credential string `json:"credential" binding:"required"`
}

type PortalInvitationRegisterRequest struct {
	DomainType  string `json:"domainType" binding:"required"`
	Credential  string `json:"credential" binding:"required"`
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

type PortalInvitationBindRequest struct {
	DomainType string `json:"domainType" binding:"required"`
	Credential string `json:"credential" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type AuthVerificationCodeRequest struct {
	Purpose string `json:"purpose" binding:"required"`
	Email   string `json:"email" binding:"required"`
}

type PasswordResetRequest struct {
	Email            string `json:"email" binding:"required"`
	VerificationCode string `json:"verificationCode" binding:"required"`
	Password         string `json:"password" binding:"required"`
}

type EnterpriseRegistrationRequest struct {
	TenantName       string `json:"tenantName" binding:"required"`
	Industry         string `json:"industry"`
	CountryRegion    string `json:"countryRegion"`
	Username         string `json:"username" binding:"required"`
	DisplayName      string `json:"displayName"`
	Email            string `json:"email" binding:"required"`
	Mobile           string `json:"mobile"`
	Password         string `json:"password" binding:"required"`
	VerificationCode string `json:"verificationCode" binding:"required"`
}

type WxWorkExchangeRequest struct {
	Ticket string `json:"ticket"`
}

type OIDCExchangeRequest struct {
	Ticket string `json:"ticket"`
}
