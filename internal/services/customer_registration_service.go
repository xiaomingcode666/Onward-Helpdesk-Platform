package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const customerRegistrationInviteTTL = 7 * 24 * time.Hour

var CustomerRegistrationService = &customerRegistrationService{}

type customerRegistrationService struct{}

type CustomerRegistrationContext struct {
	Method         string
	DomainType     string
	Email          string
	DisplayName    string
	ExpiresAt      *time.Time
	Registered     bool
	Grant          *models.CustomerRegistrationGrant
	Tenant         *models.Tenant
	CustomerOrg    *models.CustomerOrg
	PartnerCompany *models.PartnerCompany
	Product        *models.Product
	Device         *models.Device
	ServiceCode    *models.ServiceCode
}

type CustomerRegistrationInviteResult struct {
	Grant           *models.CustomerRegistrationGrant
	CustomerOrg     *models.CustomerOrg
	InviteCode      string
	RegistrationURL string
	EmailSent       bool
	EmailError      string
}

type CustomerInvitationDraftTicketResult struct {
	Ticket     *models.Ticket
	Invitation *CustomerRegistrationInviteResult
}

type customerInvitationPlan struct {
	Tenant                *models.Tenant
	Email                 string
	RoleCodes             []string
	RoleCodesJSON         string
	InviteCode            string
	Now                   time.Time
	ExpiresAt             time.Time
	CustomerDefaultLocale string
}

func (s *customerRegistrationService) InviteCustomer(tenantID int64, req request.EnterpriseCustomerUserInviteRequest, operator *dto.AuthPrincipal) (*CustomerRegistrationInviteResult, error) {
	plan, err := s.prepareCustomerInvitation(tenantID, req)
	if err != nil {
		return nil, err
	}
	var grant *models.CustomerRegistrationGrant
	var customerOrg *models.CustomerOrg
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var createErr error
		grant, customerOrg, createErr = s.createCustomerInvitationDB(ctx.Tx, tenantID, req, operator, plan)
		return createErr
	})
	if err != nil {
		return nil, err
	}
	return s.completeCustomerInvitation(tenantID, req, operator, plan, grant, customerOrg), nil
}

func (s *customerRegistrationService) InviteCustomerWithDraftTicket(
	tenantID int64,
	inviteReq request.EnterpriseCustomerUserInviteRequest,
	ticketReq request.CreateTicketRequest,
	operator *dto.AuthPrincipal,
) (*CustomerInvitationDraftTicketResult, error) {
	plan, err := s.prepareCustomerInvitation(tenantID, inviteReq)
	if err != nil {
		return nil, err
	}
	var grant *models.CustomerRegistrationGrant
	var customerOrg *models.CustomerOrg
	var ticket *models.Ticket
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var createErr error
		grant, customerOrg, createErr = s.createCustomerInvitationDB(ctx.Tx, tenantID, inviteReq, operator, plan)
		if createErr != nil {
			return createErr
		}
		ticketReq.TenantID = tenantID
		ticketReq.CustomerID = 0
		ticketReq.ConversationID = 0
		ticketReq.CustomerRegistrationGrantID = grant.ID
		prepared, existing, prepareErr := TicketService.prepareTicketCreate(ctx.Tx, ticketReq, operator)
		if prepareErr != nil {
			return prepareErr
		}
		if existing != nil {
			ticket = existing
			return nil
		}
		if createErr := TicketService.createTicketPreparedTx(ctx, prepared, operator); createErr != nil {
			return createErr
		}
		ticket = prepared.ticket
		return nil
	})
	if err != nil {
		return nil, err
	}
	invitation := s.completeCustomerInvitation(tenantID, inviteReq, operator, plan, grant, customerOrg)
	return &CustomerInvitationDraftTicketResult{Ticket: TicketService.Get(ticket.ID), Invitation: invitation}, nil
}

func (s *customerRegistrationService) prepareCustomerInvitation(tenantID int64, req request.EnterpriseCustomerUserInviteRequest) (*customerInvitationPlan, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenantID <= 0 || tenant == nil {
		return nil, errorsx.InvalidParam("tenant is not available")
	}
	email, err := normalizeCustomerRegistrationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	roleCodes := normalizeIAMRoleCodes(req.RoleCodes)
	if len(roleCodes) == 0 {
		roleCodes = []string{CustomerRoleUser}
	}
	roleJSON, err := json.Marshal(roleCodes)
	if err != nil {
		return nil, err
	}
	inviteCode, err := generateCustomerRegistrationCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &customerInvitationPlan{
		Tenant: tenant, Email: email, RoleCodes: roleCodes, RoleCodesJSON: string(roleJSON), InviteCode: inviteCode,
		Now: now, ExpiresAt: now.Add(customerRegistrationInviteTTL),
		CustomerDefaultLocale: TenantPortalSettingsService.CustomerDefaultLocaleDB(sqls.DB(), tenant),
	}, nil
}

func (s *customerRegistrationService) createCustomerInvitationDB(
	db *gorm.DB,
	tenantID int64,
	req request.EnterpriseCustomerUserInviteRequest,
	operator *dto.AuthPrincipal,
	plan *customerInvitationPlan,
) (*models.CustomerRegistrationGrant, *models.CustomerOrg, error) {
	if db == nil || plan == nil {
		return nil, nil, errorsx.InvalidParam("customer invitation transaction is not available")
	}
	if err := EnsureTenantDefaultIAMRolesDB(db, tenantID, operator); err != nil {
		return nil, nil, err
	}
	for _, roleCode := range plan.RoleCodes {
		role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenantID, models.DomainTypeCustomer, roleCode)
		if role == nil || role.Status != enums.StatusOk {
			return nil, nil, errorsx.InvalidParam("customer role is not available")
		}
	}
	customerOrg := repositories.EnterpriseIAMRepository.GetCustomerOrg(db, tenantID, req.CustomerOrgID)
	if req.CustomerOrgID > 0 && customerOrg == nil {
		return nil, nil, errorsx.InvalidParam("customer organization is not available")
	}
	orgName := strings.TrimSpace(req.CustomerOrg)
	if customerOrg == nil && orgName != "" {
		customerOrg = repositories.EnterpriseIAMRepository.FindCustomerOrgByName(db, tenantID, orgName)
	}
	if customerOrg == nil {
		if orgName == "" {
			return nil, nil, errorsx.InvalidParam("customer organization is required")
		}
		customerOrg = &models.CustomerOrg{
			TenantID: tenantID, CustomerNo: fmt.Sprintf("CUS-%d-%d", tenantID, plan.Now.UnixNano()), Name: orgName,
			DefaultLocale: defaultString(req.Locale, plan.CustomerDefaultLocale), Timezone: defaultString(req.Timezone, "Asia/Shanghai"),
			Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.EnterpriseIAMRepository.CreateCustomerOrg(db, customerOrg); err != nil {
			return nil, nil, err
		}
	}
	if customerOrg.Status != enums.StatusOk {
		return nil, nil, errorsx.InvalidParam("customer organization is not available")
	}
	pendingGrantIDs, err := repositories.CustomerRegistrationRepository.FindPendingIDsByEmailForUpdate(db, tenantID, models.DomainTypeCustomer, plan.Email)
	if err != nil {
		return nil, nil, err
	}
	if err := repositories.CustomerRegistrationRepository.RevokePendingByEmail(db, tenantID, models.DomainTypeCustomer, plan.Email, plan.Now); err != nil {
		return nil, nil, err
	}
	grant := &models.CustomerRegistrationGrant{
		TenantID: tenantID, DomainType: models.DomainTypeCustomer, CustomerOrgID: customerOrg.ID, Email: plan.Email,
		DisplayName: strings.TrimSpace(req.DisplayName), TokenHash: hashCustomerRegistrationCode(plan.InviteCode),
		RoleCodesJSON: plan.RoleCodesJSON, Status: models.CustomerRegistrationGrantPending, ExpiresAt: plan.ExpiresAt,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.CustomerRegistrationRepository.Create(db, grant); err != nil {
		return nil, nil, err
	}
	if err := repositories.TicketRepository.MoveInvitationDrafts(db, tenantID, pendingGrantIDs, grant.ID); err != nil {
		return nil, nil, err
	}
	return grant, customerOrg, nil
}

func (s *customerRegistrationService) completeCustomerInvitation(
	tenantID int64,
	req request.EnterpriseCustomerUserInviteRequest,
	operator *dto.AuthPrincipal,
	plan *customerInvitationPlan,
	grant *models.CustomerRegistrationGrant,
	customerOrg *models.CustomerOrg,
) *CustomerRegistrationInviteResult {
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeCustomer, "customer_registration_grant", fmt.Sprint(grant.ID), "customer_user.invited", nil, grant, models.RiskLevelMedium, "")
	params := url.Values{}
	params.Set("next", "/customer")
	params.Set("invite", plan.InviteCode)
	registrationURL := buildPublicAppURL("/customer/login", params)
	emailSent, emailError := s.sendPortalInvitationEmail(tenantID, models.DomainTypeCustomer, plan.Email, strings.TrimSpace(req.DisplayName), registrationURL, plan.InviteCode, plan.ExpiresAt, plan.Tenant, customerOrg)
	return &CustomerRegistrationInviteResult{
		Grant: grant, CustomerOrg: customerOrg, InviteCode: plan.InviteCode,
		RegistrationURL: registrationURL, EmailSent: emailSent, EmailError: emailError,
	}
}

func (s *customerRegistrationService) Verify(req request.CustomerRegistrationVerifyRequest) (*CustomerRegistrationContext, error) {
	method, err := normalizeCustomerRegistrationMethod(req.Method)
	if err != nil {
		return nil, err
	}
	credential := strings.TrimSpace(req.Credential)
	if method == "invite" {
		if credential == "" {
			return nil, errorsx.InvalidParam("registration credential is required")
		}
		registration, err := s.resolveInviteContext(sqls.DB(), credential, false)
		if err != nil {
			return nil, err
		}
		if registration.DomainType != models.DomainTypeCustomer {
			return nil, errorsx.InvalidParam("invitation is not for customer portal")
		}
		return registration, nil
	}
	if method == "visitor" {
		email, err := normalizeCustomerRegistrationEmail(credential)
		if err != nil {
			return nil, err
		}
		return &CustomerRegistrationContext{Method: "visitor", DomainType: models.DomainTypeCustomer, Email: email}, nil
	}
	if credential == "" {
		return nil, errorsx.InvalidParam("registration credential is required")
	}
	return s.resolveServiceCodeContext(credential)
}

func (s *customerRegistrationService) Register(req request.CustomerRegistrationRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	method, err := normalizeCustomerRegistrationMethod(req.Method)
	if err != nil {
		return nil, err
	}
	if method == "visitor" {
		return s.registerVisitor(req, authCfg, clientIP, userAgent)
	}
	if method == "invite" {
		return s.registerInvitationAccount(req, models.DomainTypeCustomer, authCfg, clientIP, userAgent)
	}
	credential := strings.TrimSpace(req.Credential)
	username := strings.TrimSpace(req.Username)
	displayName := strings.TrimSpace(req.DisplayName)
	if username == "" || credential == "" {
		return nil, errorsx.InvalidParam("username and registration credential are required")
	}
	email, err := normalizeCustomerRegistrationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if len(req.Password) < 8 {
		return nil, errorsx.InvalidParam("password must contain at least 8 characters")
	}

	verified, err := s.Verify(request.CustomerRegistrationVerifyRequest{Method: method, Credential: credential})
	if err != nil {
		return nil, err
	}
	if verified.Email != "" && !strings.EqualFold(verified.Email, email) {
		return nil, errorsx.InvalidParam("registration email does not match the invitation")
	}
	if displayName == "" {
		displayName = verified.DisplayName
	}
	if displayName == "" {
		displayName = username
	}

	var user *models.User
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		registration := verified
		if method == "invite" {
			registration, err = s.resolveInviteContext(ctx.Tx, credential, true)
			if err != nil {
				return err
			}
			if !strings.EqualFold(registration.Email, email) {
				return errorsx.InvalidParam("registration email does not match the invitation")
			}
		} else {
			serviceCode := repositories.ServiceCodeRepository.Get(ctx.Tx, verified.ServiceCode.ID)
			if serviceCode == nil || !strings.EqualFold(strings.TrimSpace(serviceCode.ServiceCode), credential) || (serviceCode.Status != enums.ServiceCodeStatusActive && serviceCode.Status != enums.ServiceCodeStatusBound) {
				return errorsx.InvalidParam("device code is no longer available")
			}
			device := repositories.DeviceRepository.Get(ctx.Tx, verified.Device.ID)
			if device == nil || device.Status != enums.StatusOk || device.TenantID != verified.Tenant.ID || device.ProductID != verified.Product.ID {
				return errorsx.InvalidParam("device registration context has changed")
			}
			registration.ServiceCode = serviceCode
			registration.Device = device
		}

		user, _, err = UserService.CreateUserDB(ctx.Tx, request.CreateUserRequest{
			Username: username, Nickname: displayName, Email: &email, Password: req.Password,
			Remark: "Customer self-registration via " + method,
		}, nil)
		if err != nil {
			return err
		}

		customerDefaultLocale := TenantPortalSettingsService.CustomerDefaultLocaleDB(ctx.Tx, registration.Tenant)
		customerOrg := registration.CustomerOrg
		if method == "service_code" {
			if registration.Device.CustomerOrgID > 0 {
				customerOrg = repositories.EnterpriseIAMRepository.GetCustomerOrg(ctx.Tx, registration.Tenant.ID, registration.Device.CustomerOrgID)
				if customerOrg == nil || customerOrg.Status != enums.StatusOk {
					return errorsx.InvalidParam("device customer organization is not available")
				}
			} else {
				customerOrg = &models.CustomerOrg{
					TenantID: registration.Tenant.ID, CustomerNo: fmt.Sprintf("SELF-%d-%d", registration.Tenant.ID, user.ID),
					Name: displayName + "的设备账户", DefaultLocale: customerDefaultLocale,
					Timezone: defaultString(registration.Tenant.Timezone, "Asia/Shanghai"), Status: enums.StatusOk,
					AuditFields: utils.BuildAuditFields(nil),
				}
				if err := repositories.EnterpriseIAMRepository.CreateCustomerOrg(ctx.Tx, customerOrg); err != nil {
					return err
				}
			}
		}
		if customerOrg == nil || customerOrg.TenantID != registration.Tenant.ID {
			return errorsx.InvalidParam("customer organization is not available")
		}

		customerUser := &models.CustomerUser{
			TenantID: registration.Tenant.ID, CustomerOrgID: customerOrg.ID, UserID: user.ID,
			DisplayName: displayName, Email: email, Locale: customerDefaultLocale,
			Timezone: defaultString(registration.Tenant.Timezone, "Asia/Shanghai"), Status: enums.StatusOk,
			AuditFields: utils.BuildAuditFields(nil),
		}
		if err := repositories.EnterpriseIAMRepository.CreateCustomerUser(ctx.Tx, customerUser); err != nil {
			return err
		}
		customerID, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser, ExternalID: fmt.Sprint(user.ID), ExternalName: displayName,
		})
		if err != nil {
			return err
		}
		if err := EnsureTenantDefaultIAMRolesDB(ctx.Tx, registration.Tenant.ID, nil); err != nil {
			return err
		}
		roleCodes := []string{CustomerRoleUser}
		if registration.Grant != nil {
			if decoded := decodeCustomerRegistrationRoles(registration.Grant.RoleCodesJSON); len(decoded) > 0 {
				roleCodes = decoded
			}
		}
		if err := replaceIAMRoleBindingsDB(ctx.Tx, registration.Tenant.ID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUser.ID, roleCodes, CustomerRoleUser, nil); err != nil {
			return err
		}

		if method == "service_code" {
			now := time.Now()
			if registration.Device.CustomerOrgID == 0 {
				if err := repositories.DeviceRepository.Updates(ctx.Tx, registration.Device.ID, map[string]any{"customer_org_id": customerOrg.ID, "updated_at": now}); err != nil {
					return err
				}
			}
			binding := &models.CustomerDeviceBinding{
				TenantID: registration.Tenant.ID, CustomerOrgID: customerOrg.ID, CustomerUserID: customerUser.ID,
				DeviceID: registration.Device.ID, BindingRole: "owner", PermissionFlagsJSON: "{}",
				Source: "customer_registration", Status: enums.StatusOk, ConfirmedAt: &now, AuditFields: utils.BuildAuditFields(nil),
			}
			if err := repositories.CustomerDeviceBindingRepository.Create(ctx.Tx, binding); err != nil {
				return err
			}
			if registration.ServiceCode.Status == enums.ServiceCodeStatusActive {
				if _, err := repositories.ServiceCodeRepository.UpdateStatusIf(ctx.Tx, registration.ServiceCode.ID, string(enums.ServiceCodeStatusActive), map[string]any{
					"status": enums.ServiceCodeStatusBound, "bound_at": &now, "updated_at": now,
				}); err != nil {
					return err
				}
			}
		} else {
			now := time.Now()
			if err := TicketService.ActivateCustomerInvitationDrafts(ctx, registration.Grant, customerID); err != nil {
				return err
			}
			if err := repositories.CustomerRegistrationRepository.Updates(ctx.Tx, registration.Grant.ID, map[string]any{
				"status": models.CustomerRegistrationGrantConsumed, "consumed_at": &now,
				"consumed_user_id": user.ID, "updated_at": now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AuthService.Login(request.LoginRequest{Username: user.Username, Password: req.Password, DomainType: models.DomainTypeCustomer}, authCfg, clientIP, userAgent)
}

func (s *customerRegistrationService) VerifyPortalInvitation(req request.PortalInvitationVerifyRequest) (*CustomerRegistrationContext, error) {
	domainType, err := normalizePortalInvitationDomain(req.DomainType)
	if err != nil {
		return nil, err
	}
	registration, err := s.resolveInviteContext(sqls.DB(), req.Credential, false)
	if err != nil {
		return nil, err
	}
	if registration.DomainType != domainType {
		return nil, errorsx.InvalidParam("invitation does not match the requested portal")
	}
	return registration, nil
}

func (s *customerRegistrationService) RegisterPortalInvitation(req request.PortalInvitationRegisterRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	domainType, err := normalizePortalInvitationDomain(req.DomainType)
	if err != nil {
		return nil, err
	}
	return s.registerInvitationAccount(request.CustomerRegistrationRequest{
		Method:      "invite",
		Credential:  req.Credential,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Password:    req.Password,
	}, domainType, authCfg, clientIP, userAgent)
}

func (s *customerRegistrationService) BindPortalInvitation(req request.PortalInvitationBindRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	domainType, err := normalizePortalInvitationDomain(req.DomainType)
	if err != nil {
		return nil, err
	}
	credential := strings.TrimSpace(req.Credential)
	username := strings.TrimSpace(req.Username)
	password := req.Password
	if credential == "" || username == "" || password == "" {
		return nil, errorsx.InvalidParam("username, password and invitation credential are required")
	}
	registration, err := s.resolveInviteContext(sqls.DB(), credential, false)
	if err != nil {
		return nil, err
	}
	if registration.DomainType != domainType {
		return nil, errorsx.InvalidParam("invitation does not match the requested portal")
	}
	user := repositories.UserRepository.GetByUsername(sqls.DB(), username)
	if user == nil || user.Status != enums.StatusOk || strings.TrimSpace(user.Password) == "" {
		return nil, errorsx.InvalidAccountI18n("error.e0260")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil, errorsx.InvalidAccountI18n("error.e0260")
	}
	if user.Email == nil || !strings.EqualFold(strings.TrimSpace(*user.Email), registration.Email) {
		return nil, errorsx.InvalidParam("signed-in account email does not match the invitation")
	}
	var registrationForLogin *CustomerRegistrationContext
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked, err := s.resolveInviteContext(ctx.Tx, credential, true)
		if err != nil {
			return err
		}
		if locked.DomainType != domainType {
			return errorsx.InvalidParam("invitation does not match the requested portal")
		}
		if !strings.EqualFold(locked.Email, registration.Email) {
			return errorsx.InvalidParam("invitation context has changed")
		}
		displayName := strings.TrimSpace(locked.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(user.Nickname)
		}
		if displayName == "" {
			displayName = user.Username
		}
		if err := s.bindInvitationAccountDB(ctx, locked, user, displayName); err != nil {
			return err
		}
		if err := s.consumeInvitationGrantDB(ctx.Tx, locked.Grant, user.ID); err != nil {
			return err
		}
		registrationForLogin = locked
		return nil
	}); err != nil {
		return nil, err
	}
	return s.issueBoundInvitationLogin(registrationForLogin, user, authCfg, clientIP, userAgent)
}

func (s *customerRegistrationService) registerInvitationAccount(req request.CustomerRegistrationRequest, domainType string, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	domainType, err := normalizePortalInvitationDomain(domainType)
	if err != nil {
		return nil, err
	}
	credential := strings.TrimSpace(req.Credential)
	username := strings.TrimSpace(req.Username)
	displayName := strings.TrimSpace(req.DisplayName)
	if username == "" || credential == "" {
		return nil, errorsx.InvalidParam("username and invitation credential are required")
	}
	email, err := normalizeCustomerRegistrationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if len(req.Password) < 8 {
		return nil, errorsx.InvalidParam("password must contain at least 8 characters")
	}
	var user *models.User
	var registrationForLogin *CustomerRegistrationContext
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		registration, err := s.resolveInviteContext(ctx.Tx, credential, true)
		if err != nil {
			return err
		}
		if registration.DomainType != domainType {
			return errorsx.InvalidParam("invitation does not match the requested portal")
		}
		if !strings.EqualFold(registration.Email, email) {
			return errorsx.InvalidParam("registration email does not match the invitation")
		}
		resolvedName := displayName
		if resolvedName == "" {
			resolvedName = registration.DisplayName
		}
		if resolvedName == "" {
			resolvedName = username
		}
		user, _, err = UserService.CreateUserDB(ctx.Tx, request.CreateUserRequest{
			Username: username, Nickname: resolvedName, Email: &email, Password: req.Password,
			Remark: "Portal self-registration via invitation",
		}, nil)
		if err != nil {
			return err
		}
		if err := s.bindInvitationAccountDB(ctx, registration, user, resolvedName); err != nil {
			return err
		}
		if err := s.consumeInvitationGrantDB(ctx.Tx, registration.Grant, user.ID); err != nil {
			return err
		}
		registrationForLogin = registration
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.issueBoundInvitationLogin(registrationForLogin, user, authCfg, clientIP, userAgent)
}

func (s *customerRegistrationService) issueBoundInvitationLogin(registration *CustomerRegistrationContext, user *models.User, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	if registration == nil || registration.Tenant == nil || user == nil {
		return nil, errorsx.InvalidParam("invitation login context is invalid")
	}
	switch registration.DomainType {
	case models.DomainTypeCustomer:
		if registration.CustomerOrg == nil {
			return nil, errorsx.InvalidParam("customer organization is not available")
		}
		customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantOrgAndUserID(sqls.DB(), registration.Tenant.ID, registration.CustomerOrg.ID, user.ID)
		if customerUser == nil || customerUser.Status != enums.StatusOk {
			return nil, errorsx.InvalidAccountI18n("error.e0260")
		}
		return AuthService.IssuePortalInvitationSession(models.DomainTypeCustomer, user.ID, registration.Tenant.ID, customerUser.ID, clientIP, userAgent, authCfg)
	case models.DomainTypePartner:
		if registration.PartnerCompany == nil {
			return nil, errorsx.InvalidParam("supplier company is not available")
		}
		account := repositories.EnterpriseIAMRepository.FindPartnerAccountByUser(sqls.DB(), registration.Tenant.ID, registration.PartnerCompany.ID, user.ID)
		if account == nil || account.Status != enums.StatusOk {
			return nil, errorsx.InvalidAccountI18n("error.e0260")
		}
		return AuthService.IssuePortalInvitationSession(models.DomainTypePartner, user.ID, registration.Tenant.ID, account.ID, clientIP, userAgent, authCfg)
	default:
		return nil, errorsx.InvalidParam("unsupported invitation portal")
	}
}

func (s *customerRegistrationService) registerVisitor(req request.CustomerRegistrationRequest, authCfg config.AuthConfig, clientIP, userAgent string) (*response.LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	displayName := strings.TrimSpace(req.DisplayName)
	if username == "" {
		return nil, errorsx.InvalidParam("username is required")
	}
	email, err := normalizeCustomerRegistrationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	credentialEmail, err := normalizeCustomerRegistrationEmail(req.Credential)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(credentialEmail, email) {
		return nil, errorsx.InvalidParam("registration email does not match the verified email")
	}
	if len(req.Password) < 8 {
		return nil, errorsx.InvalidParam("password must contain at least 8 characters")
	}
	if displayName == "" {
		displayName = username
	}
	var user *models.User
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := AuthVerificationService.VerifyDB(ctx.Tx, models.AuthVerificationPurposeCustomerRegister, email, req.VerificationCode, true); err != nil {
			return err
		}
		created, _, err := UserService.CreateUserDB(ctx.Tx, request.CreateUserRequest{
			Username: username, Nickname: displayName, Email: &email, Password: req.Password,
			Remark: "Customer visitor self-registration",
		}, nil)
		if err != nil {
			return err
		}
		user = created
		customerID, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     fmt.Sprint(created.ID),
			ExternalName:   displayName,
		})
		if err != nil {
			return err
		}
		return repositories.CustomerRepository.Updates(ctx.Tx, customerID, map[string]interface{}{
			"primary_email": email,
			"updated_at":    time.Now(),
		})
	}); err != nil {
		return nil, err
	}
	return AuthService.Login(request.LoginRequest{Username: user.Username, Password: req.Password, DomainType: models.DomainTypeCustomer}, authCfg, clientIP, userAgent)
}

func (s *customerRegistrationService) bindInvitationAccountDB(ctx *sqls.TxContext, registration *CustomerRegistrationContext, user *models.User, displayName string) error {
	if ctx == nil || ctx.Tx == nil || registration == nil || registration.Grant == nil || user == nil {
		return errorsx.InvalidParam("invitation binding context is invalid")
	}
	if registration.Tenant == nil || registration.Tenant.ID <= 0 {
		return errorsx.InvalidParam("invitation enterprise is not available")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Nickname)
	}
	if displayName == "" {
		displayName = user.Username
	}
	if err := EnsureTenantDefaultIAMRolesDB(ctx.Tx, registration.Tenant.ID, nil); err != nil {
		return err
	}
	switch registration.DomainType {
	case models.DomainTypeCustomer:
		return s.bindCustomerInvitationAccountDB(ctx, registration, user, displayName)
	case models.DomainTypePartner:
		return s.bindPartnerInvitationAccountDB(ctx.Tx, registration, user, displayName)
	default:
		return errorsx.InvalidParam("unsupported invitation portal")
	}
}

func (s *customerRegistrationService) bindCustomerInvitationAccountDB(ctx *sqls.TxContext, registration *CustomerRegistrationContext, user *models.User, displayName string) error {
	customerOrg := registration.CustomerOrg
	if customerOrg == nil || customerOrg.TenantID != registration.Tenant.ID || customerOrg.Status != enums.StatusOk {
		return errorsx.InvalidParam("customer organization is not available")
	}
	now := time.Now()
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantOrgAndUserID(ctx.Tx, registration.Tenant.ID, customerOrg.ID, user.ID)
	if customerUser == nil || customerUser.Status == enums.StatusDeleted {
		customerDefaultLocale := TenantPortalSettingsService.CustomerDefaultLocaleDB(ctx.Tx, registration.Tenant)
		customerUser = &models.CustomerUser{
			TenantID: registration.Tenant.ID, CustomerOrgID: customerOrg.ID, UserID: user.ID,
			DisplayName: displayName, Email: registration.Email, Locale: customerDefaultLocale,
			Timezone: defaultString(registration.Tenant.Timezone, "Asia/Shanghai"), Status: enums.StatusOk,
			AuditFields: utils.BuildAuditFields(nil),
		}
		if err := repositories.EnterpriseIAMRepository.CreateCustomerUser(ctx.Tx, customerUser); err != nil {
			return err
		}
	} else if err := repositories.EnterpriseIAMRepository.UpdateCustomerUser(ctx.Tx, registration.Tenant.ID, customerUser.ID, map[string]any{
		"customer_org_id": customerOrg.ID,
		"display_name":    displayName,
		"email":           registration.Email,
		"status":          enums.StatusOk,
		"updated_at":      now,
	}); err != nil {
		return err
	}
	customerID, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser, ExternalID: fmt.Sprint(user.ID), ExternalName: displayName,
	})
	if err != nil {
		return err
	}
	roleCodes := []string{CustomerRoleUser}
	if decoded := decodeCustomerRegistrationRoles(registration.Grant.RoleCodesJSON); len(decoded) > 0 {
		roleCodes = decoded
	}
	if err := replaceIAMRoleBindingsDB(ctx.Tx, registration.Tenant.ID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUser.ID, roleCodes, CustomerRoleUser, nil); err != nil {
		return err
	}
	return TicketService.ActivateCustomerInvitationDrafts(ctx, registration.Grant, customerID)
}

func (s *customerRegistrationService) bindPartnerInvitationAccountDB(db *gorm.DB, registration *CustomerRegistrationContext, user *models.User, displayName string) error {
	company := registration.PartnerCompany
	if company == nil || company.TenantID != registration.Tenant.ID || company.Status != enums.StatusOk {
		return errorsx.InvalidParam("supplier company is not available")
	}
	now := time.Now()
	account := repositories.EnterpriseIAMRepository.FindPartnerAccountByUser(db, registration.Tenant.ID, company.ID, user.ID)
	if account == nil || account.Status == enums.StatusDeleted {
		account = &models.PartnerAccount{
			TenantID: registration.Tenant.ID, PartnerCompanyID: company.ID, UserID: user.ID,
			DisplayName: displayName, Email: registration.Email, LanguagesJSON: "[]",
			Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(nil),
		}
		if err := repositories.EnterpriseIAMRepository.CreatePartnerAccount(db, account); err != nil {
			return err
		}
	} else if err := repositories.EnterpriseIAMRepository.UpdatePartnerAccount(db, registration.Tenant.ID, company.ID, account.ID, map[string]any{
		"display_name": displayName,
		"email":        registration.Email,
		"status":       enums.StatusOk,
		"updated_at":   now,
	}); err != nil {
		return err
	}
	return replaceIAMRoleBindingsDB(db, registration.Tenant.ID, models.DomainTypePartner, models.SubjectTypePartnerAccount, account.ID, []string{PartnerRoleAdmin}, PartnerRoleAdmin, nil)
}

func (s *customerRegistrationService) consumeInvitationGrantDB(db *gorm.DB, grant *models.CustomerRegistrationGrant, userID int64) error {
	if grant == nil || grant.ID <= 0 {
		return errorsx.InvalidParam("invitation grant is not available")
	}
	now := time.Now()
	return repositories.CustomerRegistrationRepository.Updates(db, grant.ID, map[string]any{
		"status":           models.CustomerRegistrationGrantConsumed,
		"consumed_at":      &now,
		"consumed_user_id": userID,
		"updated_at":       now,
	})
}

func (s *customerRegistrationService) resolveInviteContext(db *gorm.DB, credential string, forUpdate bool) (*CustomerRegistrationContext, error) {
	tokenHash := hashCustomerRegistrationCode(credential)
	var grant *models.CustomerRegistrationGrant
	if forUpdate {
		grant = repositories.CustomerRegistrationRepository.FindByTokenHashForUpdate(db, tokenHash)
	} else {
		grant = repositories.CustomerRegistrationRepository.FindByTokenHash(db, tokenHash)
	}
	if grant == nil || grant.Status != models.CustomerRegistrationGrantPending {
		return nil, errorsx.InvalidParam("invitation is invalid or has already been used")
	}
	if !grant.ExpiresAt.After(time.Now()) {
		return nil, errorsx.InvalidParam("invitation has expired")
	}
	tenant := repositories.TenantRepository.Get(db, grant.TenantID)
	if tenant == nil || tenant.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("invitation enterprise is not available")
	}
	domainType := strings.TrimSpace(grant.DomainType)
	if domainType == "" {
		domainType = models.DomainTypeCustomer
	}
	var org *models.CustomerOrg
	var company *models.PartnerCompany
	switch domainType {
	case models.DomainTypeCustomer:
		org = repositories.EnterpriseIAMRepository.GetCustomerOrg(db, grant.TenantID, grant.CustomerOrgID)
		if org == nil || org.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("invitation customer organization is not available")
		}
	case models.DomainTypePartner:
		company = repositories.EnterpriseIAMRepository.GetPartnerCompany(db, grant.TenantID, grant.PartnerCompanyID)
		if company == nil || company.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("invitation supplier company is not available")
		}
	default:
		return nil, errorsx.InvalidParam("unsupported invitation portal")
	}
	expiresAt := grant.ExpiresAt
	registered := repositories.UserRepository.GetByEmail(db, grant.Email) != nil
	return &CustomerRegistrationContext{
		Method: "invite", DomainType: domainType, Email: grant.Email, DisplayName: grant.DisplayName, ExpiresAt: &expiresAt,
		Registered: registered, Grant: grant, Tenant: tenant, CustomerOrg: org, PartnerCompany: company,
	}, nil
}

func (s *customerRegistrationService) resolveServiceCodeContext(credential string) (*CustomerRegistrationContext, error) {
	resolved, err := ServiceCodeResolveService.Resolve(credential)
	if err != nil {
		return nil, err
	}
	if resolved == nil || !resolved.Valid || resolved.ServiceCode == nil || resolved.Tenant == nil || resolved.Product == nil {
		return nil, errorsx.InvalidParam("device code is invalid")
	}
	if resolved.Device == nil || resolved.ServiceCode.DeviceID <= 0 {
		return nil, errorsx.InvalidParam("device code is not bound to a registered device")
	}
	if resolved.Device.Status != enums.StatusOk || resolved.Device.TenantID != resolved.Tenant.ID || resolved.Device.ProductID != resolved.Product.ID {
		return nil, errorsx.InvalidParam("device, product and enterprise context do not match")
	}
	return &CustomerRegistrationContext{
		Method: "service_code", DomainType: models.DomainTypeCustomer, Tenant: resolved.Tenant, Product: resolved.Product,
		Device: resolved.Device, ServiceCode: resolved.ServiceCode,
	}, nil
}

func normalizeCustomerRegistrationMethod(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "invite", "invitation":
		return "invite", nil
	case "service_code", "servicecode", "device_code", "devicecode":
		return "service_code", nil
	case "visitor", "account", "email":
		return "visitor", nil
	default:
		return "", errorsx.InvalidParam("unsupported customer registration method")
	}
}

func normalizePortalInvitationDomain(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.DomainTypeCustomer:
		return models.DomainTypeCustomer, nil
	case models.DomainTypePartner:
		return models.DomainTypePartner, nil
	default:
		return "", errorsx.InvalidParam("unsupported invitation portal")
	}
}

func normalizeCustomerRegistrationEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsed.Address, email) {
		return "", errorsx.InvalidParam("a valid email address is required")
	}
	return email, nil
}

func generateCustomerRegistrationCode() (string, error) {
	return generateNumericVerificationCode(8)
}

func hashCustomerRegistrationCode(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func decodeCustomerRegistrationRoles(value string) []string {
	var roles []string
	if json.Unmarshal([]byte(value), &roles) != nil {
		return nil
	}
	return normalizeIAMRoleCodes(roles)
}

func buildPublicAppURL(path string, params url.Values) string {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	baseURL := config.CurrentOrDefault().Public.NormalizedBaseURL()
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

func (s *customerRegistrationService) sendPortalInvitationEmail(tenantID int64, domainType, email, targetName, actionURL, inviteCode string, expiresAt time.Time, tenant *models.Tenant, org any) (bool, string) {
	portalName := "用户端"
	if domainType == models.DomainTypePartner {
		portalName = "供应商端"
	}
	tenantName := ""
	if tenant != nil {
		tenantName = strings.TrimSpace(tenant.Name)
	}
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		targetName = strings.TrimSpace(email)
	}
	orgName := ""
	switch value := org.(type) {
	case *models.CustomerOrg:
		if value != nil {
			orgName = strings.TrimSpace(value.Name)
		}
	case *models.PartnerCompany:
		if value != nil {
			orgName = strings.TrimSpace(value.Name)
		}
	}
	err := EmailNotificationService.SendEmail(tenantID, email, "", "portal_invitation", map[string]interface{}{
		"PortalName": portalName,
		"TenantName": tenantName,
		"TargetName": targetName,
		"OrgName":    orgName,
		"InviteCode": inviteCode,
		"ActionURL":  actionURL,
		"ExpiresAt":  expiresAt.Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}
