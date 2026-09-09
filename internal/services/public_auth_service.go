package services

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var PublicAuthService = &publicAuthService{}

type publicAuthService struct{}

func (s *publicAuthService) RegisterEnterprise(req request.EnterpriseRegistrationRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	tenantName := strings.TrimSpace(req.TenantName)
	username := strings.TrimSpace(req.Username)
	displayName := strings.TrimSpace(req.DisplayName)
	if tenantName == "" || username == "" {
		return nil, errorsx.InvalidParam("tenant name and username are required")
	}
	email, err := normalizeAuthVerificationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if len(req.Password) < 8 {
		return nil, errorsx.InvalidParam("password must contain at least 8 characters")
	}
	if displayName == "" {
		displayName = username
	}
	if err := AuthVerificationService.Verify(models.AuthVerificationPurposeEnterpriseRegister, email, req.VerificationCode, true); err != nil {
		return nil, err
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name:          tenantName,
		Industry:      strings.TrimSpace(req.Industry),
		CountryRegion: strings.TrimSpace(req.CountryRegion),
		DefaultLocale: "zh-CN",
		Timezone:      "Asia/Shanghai",
		AdminUsername: username,
		AdminNickname: displayName,
		AdminEmail:    email,
		AdminMobile:   strings.TrimSpace(req.Mobile),
		AdminPassword: req.Password,
	}, nil)
	if err != nil {
		return nil, err
	}
	login, err := AuthService.Login(request.LoginRequest{
		Username: username, Password: req.Password, DomainType: models.DomainTypeEnterprise, PortalChoiceConfirmed: true,
	}, authCfg, clientIP, userAgent)
	if err != nil {
		return nil, err
	}
	if login.Membership == nil {
		login.Membership = &response.TenantMembershipResponse{Paid: false}
	}
	login.TenantID = tenant.ID
	return login, nil
}

func (s *publicAuthService) ResetPassword(req request.PasswordResetRequest) error {
	email, err := normalizeAuthVerificationEmail(req.Email)
	if err != nil {
		return err
	}
	if len(req.Password) < 8 {
		return errorsx.InvalidParam("password must contain at least 8 characters")
	}
	user := repositories.UserRepository.GetByEmail(sqls.DB(), email)
	if user == nil {
		return errorsx.InvalidParam("account email is not registered")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := AuthVerificationService.VerifyDB(ctx.Tx, models.AuthVerificationPurposePasswordReset, email, req.VerificationCode, true); err != nil {
			return err
		}
		return UserService.SetPassword(user.ID, req.Password, &dto.AuthPrincipal{UserID: user.ID, Username: user.Username})
	})
}
