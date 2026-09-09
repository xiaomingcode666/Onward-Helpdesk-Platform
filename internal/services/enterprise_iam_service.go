package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var EnterpriseIAMService = &enterpriseIAMService{}

type enterpriseIAMService struct{}

type EnterpriseIAMQuery struct {
	Search string
	Status string
	Page   int
	Limit  int
}

type AuditLogQuery struct {
	Action     string
	RiskLevel  string
	Status     string
	TargetType string
	Source     string
	Search     string
	Period     string
	ActorID    int64
	Page       int
	Limit      int
}

type EnterpriseIAMMemberListAggregate struct {
	Items            []models.TenantMember
	Paging           *sqls.Paging
	Users            map[int64]*models.User
	Departments      map[int64]*models.Department
	EngineerProfiles map[int64]*models.EngineerProfile
	ProductGroups    map[int64][]dto.EnterpriseIAMProductGroupDTO
	RolesBySubject   map[int64][]string
}

type EnterpriseIAMCustomerUserListAggregate struct {
	Items          []models.CustomerUser
	Paging         *sqls.Paging
	CustomerOrgs   map[int64]*models.CustomerOrg
	RolesBySubject map[int64][]string
}

type EnterpriseIAMPartnerListAggregate struct {
	Items          []models.PartnerCompany
	Paging         *sqls.Paging
	AccountCounts  map[int64]int64
	ContractCounts map[int64]int64
}

type EnterpriseIAMDepartmentListAggregate struct {
	Items        []models.Department
	Paging       *sqls.Paging
	Managers     map[int64]*models.TenantMember
	MemberCounts map[int64]int64
}

type EnterpriseIAMMemberInviteResult struct {
	Member          *models.TenantMember
	User            *models.User
	Department      *models.Department
	Engineer        *models.EngineerProfile
	Roles           []string
	InitialPassword string
}

type EnterpriseIAMCustomerAuthorizeResult struct {
	CustomerUser    *models.CustomerUser
	CustomerOrg     *models.CustomerOrg
	Roles           []string
	InitialPassword string
}

type EnterpriseIAMPartnerAdminInviteResult struct {
	PartnerCompany  *models.PartnerCompany
	Grant           *models.CustomerRegistrationGrant
	InviteCode      string
	RegistrationURL string
	EmailSent       bool
	EmailError      string
}

type EnterpriseIAMPartnerUpdateResult struct {
	PartnerCompany *models.PartnerCompany
	AccountCount   int64
	ContractCount  int64
}

func (s *enterpriseIAMService) InviteMember(tenantID int64, req request.EnterpriseMemberInviteRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMMemberInviteResult, error) {
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, fmt.Errorf("tenant not found")
	}
	var result EnterpriseIAMMemberInviteResult
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := EnsureTenantDefaultIAMRolesDB(tx, tenantID, operator); err != nil {
			return err
		}
		department := repositories.EnterpriseIAMRepository.GetDepartment(tx, tenantID, req.DepartmentID)
		if req.DepartmentID > 0 && department == nil {
			return fmt.Errorf("department not found in current tenant")
		}
		if department == nil {
			department = repositories.EnterpriseIAMRepository.FindRootDepartment(tx, tenantID)
		}
		user, initialPassword, err := UserService.CreateUserDB(tx, request.CreateUserRequest{
			Username: req.Username, Nickname: req.DisplayName, Password: req.Password,
			Email: utils.NormalizeNullableString(&req.Email), Mobile: utils.NormalizeNullableString(&req.Mobile),
			Remark: "Enterprise member",
		}, operator)
		if err != nil {
			return err
		}
		now := time.Now()
		member := &models.TenantMember{
			TenantID: tenantID, UserID: user.ID, DisplayName: strings.TrimSpace(req.DisplayName),
			JobTitle: strings.TrimSpace(req.JobTitle), MemberType: defaultString(req.MemberType, "employee"),
			Status: enums.StatusOk, InvitedAt: &now, JoinedAt: &now, AuditFields: utils.BuildAuditFields(operator),
		}
		if department != nil {
			member.DepartmentID = department.ID
		}
		if err := repositories.EnterpriseIAMRepository.CreateTenantMember(tx, member); err != nil {
			return err
		}
		roleCodes := normalizeIAMRoleCodes(req.RoleCodes)
		if err := replaceIAMRoleBindingsDB(tx, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, member.ID, roleCodes, EnterpriseRoleViewer, operator); err != nil {
			return err
		}
		var engineer *models.EngineerProfile
		if req.DispatchEnabled || containsIAMRole(roleCodes, EnterpriseRoleEngineer) {
			engineer = &models.EngineerProfile{
				TenantID: tenantID, MemberID: member.ID, SkillTagsJSON: "[]", LanguagesJSON: "[]",
				ServiceRegionsJSON: "[]", Timezone: "UTC", DispatchEnabled: req.DispatchEnabled,
				Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.EnterpriseIAMRepository.CreateEngineerProfile(tx, engineer); err != nil {
				return err
			}
			if _, err := ProductSupportOrganizationService.EnsureEngineerAgentProfileDB(tx, member, engineer, operator); err != nil {
				return err
			}
		}
		result = EnterpriseIAMMemberInviteResult{Member: member, User: user, Department: department, Engineer: engineer, Roles: roleCodes, InitialPassword: initialPassword}
		if len(result.Roles) == 0 {
			result.Roles = []string{EnterpriseRoleViewer}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeEnterprise, "tenant_member", fmt.Sprint(result.Member.ID), "tenant_member.invited", nil, result.Member, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdateMember(tenantID, memberID int64, req request.EnterpriseMemberUpdateRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMMemberInviteResult, error) {
	if tenantID <= 0 || memberID <= 0 {
		return nil, errorsx.InvalidParam("enterprise member is required")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	existing := repositories.EnterpriseIAMRepository.GetTenantMemberAnyStatus(sqls.DB(), tenantID, memberID)
	if existing == nil {
		return nil, errorsx.InvalidParam("enterprise member not found")
	}
	var status *enums.Status
	if req.Status != nil {
		if *req.Status != int(enums.StatusOk) && *req.Status != int(enums.StatusDisabled) {
			return nil, errorsx.InvalidParam("enterprise member status is invalid")
		}
		next := enums.Status(*req.Status)
		status = &next
		if existing.UserID == auditUserID(operator) && next != enums.StatusOk {
			return nil, errorsx.InvalidParam("current member cannot disable itself")
		}
	}

	var result EnterpriseIAMMemberInviteResult
	revokeSessions := false
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		member := repositories.EnterpriseIAMRepository.GetTenantMemberAnyStatus(tx, tenantID, memberID)
		if member == nil {
			return errorsx.InvalidParam("enterprise member not found")
		}
		user := repositories.UserRepository.Get(tx, member.UserID)
		if user == nil {
			return errorsx.InvalidParam("enterprise member user not found")
		}
		now := time.Now()
		userColumns := map[string]any{}
		if req.DisplayName != nil {
			displayName := strings.TrimSpace(*req.DisplayName)
			if displayName == "" {
				return errorsx.InvalidParam("display name is required")
			}
			userColumns["nickname"] = displayName
		}
		if req.Email != nil {
			email := utils.NormalizeNullableString(req.Email)
			if email != nil {
				if existed := repositories.UserRepository.GetByEmail(tx, *email); existed != nil && existed.ID != user.ID {
					return errorsx.InvalidParam("email is already used")
				}
			}
			userColumns["email"] = email
		}
		if req.Mobile != nil {
			mobile := utils.NormalizeNullableString(req.Mobile)
			if mobile != nil {
				if existed := repositories.UserRepository.GetByMobile(tx, *mobile); existed != nil && existed.ID != user.ID {
					return errorsx.InvalidParam("mobile is already used")
				}
			}
			userColumns["mobile"] = mobile
		}
		if status != nil {
			userColumns["status"] = *status
			revokeSessions = *status != enums.StatusOk
		}
		if len(userColumns) > 0 {
			userColumns["updated_at"] = now
			userColumns["update_user_id"] = auditUserID(operator)
			userColumns["update_user_name"] = auditUserName(operator)
			if err := repositories.UserRepository.Updates(tx, user.ID, userColumns); err != nil {
				return err
			}
		}

		memberColumns := map[string]any{
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if req.DisplayName != nil {
			memberColumns["display_name"] = strings.TrimSpace(*req.DisplayName)
		}
		if req.DepartmentID != nil {
			if *req.DepartmentID > 0 {
				department := repositories.EnterpriseIAMRepository.GetDepartment(tx, tenantID, *req.DepartmentID)
				if department == nil || department.Status == enums.StatusDeleted {
					return errorsx.InvalidParam("department not found in current tenant")
				}
			}
			memberColumns["department_id"] = *req.DepartmentID
		}
		if req.JobTitle != nil {
			memberColumns["job_title"] = strings.TrimSpace(*req.JobTitle)
		}
		if req.MemberType != nil {
			memberColumns["member_type"] = defaultString(*req.MemberType, "employee")
		}
		if status != nil {
			memberColumns["status"] = *status
			if *status == enums.StatusDisabled {
				memberColumns["disabled_at"] = now
			} else {
				memberColumns["disabled_at"] = nil
			}
		}
		if err := repositories.EnterpriseIAMRepository.UpdateTenantMember(tx, tenantID, memberID, memberColumns); err != nil {
			return err
		}

		roleCodes := normalizeIAMRoleCodes(req.RoleCodes)
		if len(roleCodes) > 0 {
			if err := replaceIAMRoleBindingsDB(tx, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberID, roleCodes, EnterpriseRoleViewer, operator); err != nil {
				return err
			}
		} else {
			var err error
			roleCodes, err = s.roleCodesForSubjectDB(tx, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberID)
			if err != nil {
				return err
			}
		}

		engineer, err := repositories.EnterpriseIAMRepository.FindEngineerProfileByMemberID(tx, tenantID, memberID)
		if err != nil {
			return err
		}
		wantsEngineerProfile := containsIAMRole(roleCodes, EnterpriseRoleEngineer) || (req.DispatchEnabled != nil && *req.DispatchEnabled)
		if req.DispatchEnabled != nil || (engineer == nil && wantsEngineerProfile) {
			dispatchEnabled := false
			if req.DispatchEnabled != nil {
				dispatchEnabled = *req.DispatchEnabled
			}
			if engineer == nil {
				engineer = &models.EngineerProfile{
					TenantID: tenantID, MemberID: memberID, SkillTagsJSON: "[]", LanguagesJSON: "[]",
					ServiceRegionsJSON: "[]", Timezone: "UTC", DispatchEnabled: dispatchEnabled,
					Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
				}
				if status != nil && *status != enums.StatusOk {
					engineer.Status = *status
				}
				if err := repositories.EnterpriseIAMRepository.CreateEngineerProfile(tx, engineer); err != nil {
					return err
				}
			} else {
				engineerUpdates := map[string]any{
					"updated_at":       now,
					"update_user_id":   auditUserID(operator),
					"update_user_name": auditUserName(operator),
				}
				if req.DispatchEnabled != nil {
					engineerUpdates["dispatch_enabled"] = dispatchEnabled
				}
				if status != nil {
					engineerUpdates["status"] = *status
				}
				if err := repositories.EnterpriseIAMRepository.UpdateEngineerProfile(tx, tenantID, memberID, engineerUpdates); err != nil {
					return err
				}
			}
		} else if status != nil && engineer != nil {
			if err := repositories.EnterpriseIAMRepository.UpdateEngineerProfile(tx, tenantID, memberID, map[string]any{
				"status":           *status,
				"updated_at":       now,
				"update_user_id":   auditUserID(operator),
				"update_user_name": auditUserName(operator),
			}); err != nil {
				return err
			}
		}
		if req.DisplayName != nil || status != nil || req.DispatchEnabled != nil {
			dispatchEnabled := engineer != nil && engineer.DispatchEnabled
			if req.DispatchEnabled != nil {
				dispatchEnabled = *req.DispatchEnabled
			}
			if status != nil && *status != enums.StatusOk {
				dispatchEnabled = false
			}
			if profile := repositories.AgentProfileRepository.FindOne(tx, sqls.NewCnd().Eq("tenant_id", tenantID).Eq("user_id", member.UserID)); profile != nil {
				profileStatus := profile.Status
				if status != nil {
					profileStatus = *status
				}
				profileUpdates := map[string]any{
					"updated_at":       now,
					"update_user_id":   auditUserID(operator),
					"update_user_name": auditUserName(operator),
				}
				if req.DisplayName != nil {
					profileUpdates["display_name"] = strings.TrimSpace(*req.DisplayName)
				}
				if status != nil || req.DispatchEnabled != nil {
					profileUpdates["status"] = profileStatus
					profileUpdates["auto_assign_enabled"] = dispatchEnabled
				}
				if err := repositories.AgentProfileRepository.Updates(tx, profile.ID, profileUpdates); err != nil {
					return err
				}
			}
			if (status != nil || req.DispatchEnabled != nil) && tx.Migrator().HasTable(&models.AgentTeamMember{}) {
				teamMemberUpdates := map[string]any{
					"dispatch_enabled": dispatchEnabled,
					"updated_at":       now,
					"update_user_id":   auditUserID(operator),
					"update_user_name": auditUserName(operator),
				}
				if status != nil {
					teamMemberUpdates["status"] = *status
				}
				if err := tx.Model(&models.AgentTeamMember{}).
					Where("tenant_id = ? AND (member_id = ? OR user_id = ?)", tenantID, memberID, member.UserID).
					Updates(teamMemberUpdates).Error; err != nil {
					return err
				}
			}
		}

		member = repositories.EnterpriseIAMRepository.GetTenantMemberAnyStatus(tx, tenantID, memberID)
		user = repositories.UserRepository.Get(tx, member.UserID)
		department := repositories.EnterpriseIAMRepository.GetDepartment(tx, tenantID, member.DepartmentID)
		engineer, _ = repositories.EnterpriseIAMRepository.FindEngineerProfileByMemberID(tx, tenantID, memberID)
		if len(roleCodes) == 0 {
			roleCodes = []string{EnterpriseRoleViewer}
		}
		result = EnterpriseIAMMemberInviteResult{Member: member, User: user, Department: department, Engineer: engineer, Roles: roleCodes}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if revokeSessions {
		_ = LoginSessionService.RevokeByUser(result.User.ID, auditUserID(operator), auditUserName(operator))
	}
	if status != nil && *status != enums.StatusOk {
		AgentProfileService.recoverPendingAssignmentsForUser(tenantID, result.User.ID, "enterprise_member_disabled", time.Now())
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeEnterprise, "tenant_member", fmt.Sprint(memberID), "tenant_member.updated", existing, req, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdateMemberStatus(tenantID, memberID int64, req request.EnterpriseMemberStatusRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMMemberInviteResult, error) {
	status := req.Status
	return s.UpdateMember(tenantID, memberID, request.EnterpriseMemberUpdateRequest{Status: &status}, operator)
}

func (s *enterpriseIAMService) AuthorizeCustomerUser(tenantID int64, req request.EnterpriseCustomerUserAuthorizeRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMCustomerAuthorizeResult, error) {
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, fmt.Errorf("tenant not found")
	}
	var result EnterpriseIAMCustomerAuthorizeResult
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := EnsureTenantDefaultIAMRolesDB(tx, tenantID, operator); err != nil {
			return err
		}
		org := repositories.EnterpriseIAMRepository.GetCustomerOrg(tx, tenantID, req.CustomerOrgID)
		if req.CustomerOrgID > 0 && org == nil {
			return fmt.Errorf("customer organization not found in current tenant")
		}
		orgName := strings.TrimSpace(req.CustomerOrg)
		if org == nil && orgName != "" {
			org = repositories.EnterpriseIAMRepository.FindCustomerOrgByName(tx, tenantID, orgName)
		}
		if org == nil {
			if orgName == "" {
				return fmt.Errorf("customer organization is required")
			}
			org = &models.CustomerOrg{
				TenantID: tenantID, CustomerNo: fmt.Sprintf("CUS-%d-%d", tenantID, time.Now().UnixNano()),
				Name: orgName, DefaultLocale: defaultString(req.Locale, "en-US"), Timezone: defaultString(req.Timezone, "UTC"),
				Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.EnterpriseIAMRepository.CreateCustomerOrg(tx, org); err != nil {
				return err
			}
		}
		user, initialPassword, err := UserService.CreateUserDB(tx, request.CreateUserRequest{
			Username: req.Username, Nickname: req.DisplayName, Password: req.Password,
			Email: utils.NormalizeNullableString(&req.Email), Mobile: utils.NormalizeNullableString(&req.Mobile),
			Remark: "Customer portal user for " + org.Name,
		}, operator)
		if err != nil {
			return err
		}
		customerUser := &models.CustomerUser{
			TenantID: tenantID, CustomerOrgID: org.ID, UserID: user.ID,
			DisplayName: strings.TrimSpace(req.DisplayName), Email: strings.TrimSpace(req.Email), Phone: strings.TrimSpace(req.Mobile),
			Locale: defaultString(req.Locale, "en-US"), Timezone: defaultString(req.Timezone, "UTC"),
			Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.EnterpriseIAMRepository.CreateCustomerUser(tx, customerUser); err != nil {
			return err
		}
		legacyCustomer := &models.Customer{
			Name: strings.TrimSpace(req.DisplayName), PrimaryEmail: strings.TrimSpace(req.Email), PrimaryMobile: strings.TrimSpace(req.Mobile),
			Status: enums.StatusOk, Remark: "Formal customer portal identity", AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.CustomerRepository.Create(tx, legacyCustomer); err != nil {
			return err
		}
		if err := repositories.CustomerIdentityRepository.Create(tx, &models.CustomerIdentity{
			CustomerID: legacyCustomer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: fmt.Sprint(user.ID),
			RawProfile: "{}", Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
		}); err != nil {
			return err
		}
		roleCodes := normalizeIAMRoleCodes(req.RoleCodes)
		if err := replaceIAMRoleBindingsDB(tx, tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUser.ID, roleCodes, CustomerRoleUser, operator); err != nil {
			return err
		}
		result = EnterpriseIAMCustomerAuthorizeResult{CustomerUser: customerUser, CustomerOrg: org, Roles: roleCodes, InitialPassword: initialPassword}
		if len(result.Roles) == 0 {
			result.Roles = []string{CustomerRoleUser}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeCustomer, "customer_user", fmt.Sprint(result.CustomerUser.ID), "customer_user.authorized", nil, result.CustomerUser, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdateCustomerUser(tenantID, customerUserID int64, req request.EnterpriseCustomerUserUpdateRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMCustomerAuthorizeResult, error) {
	if tenantID <= 0 || customerUserID <= 0 {
		return nil, errorsx.InvalidParam("customer user is required")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	existing := repositories.EnterpriseIAMRepository.GetCustomerUser(sqls.DB(), tenantID, customerUserID)
	if existing == nil || existing.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("customer user not found")
	}
	var status *enums.Status
	if req.Status != nil {
		if *req.Status != int(enums.StatusOk) && *req.Status != int(enums.StatusDisabled) {
			return nil, errorsx.InvalidParam("customer user status is invalid")
		}
		next := enums.Status(*req.Status)
		status = &next
	}

	var result EnterpriseIAMCustomerAuthorizeResult
	revokeSessions := false
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(tx, tenantID, customerUserID)
		if customerUser == nil || customerUser.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("customer user not found")
		}
		user := repositories.UserRepository.Get(tx, customerUser.UserID)
		if user == nil {
			return errorsx.InvalidParam("customer login account not found")
		}
		now := time.Now()
		org := repositories.EnterpriseIAMRepository.GetCustomerOrg(tx, tenantID, customerUser.CustomerOrgID)
		if req.CustomerOrgID != nil && *req.CustomerOrgID > 0 {
			org = repositories.EnterpriseIAMRepository.GetCustomerOrg(tx, tenantID, *req.CustomerOrgID)
			if org == nil || org.Status == enums.StatusDeleted {
				return errorsx.InvalidParam("customer organization not found in current tenant")
			}
		}
		if req.CustomerOrg != nil {
			orgName := strings.TrimSpace(*req.CustomerOrg)
			if orgName != "" {
				org = repositories.EnterpriseIAMRepository.FindCustomerOrgByName(tx, tenantID, orgName)
				if org == nil {
					org = &models.CustomerOrg{
						TenantID: tenantID, CustomerNo: fmt.Sprintf("CUS-%d-%d", tenantID, time.Now().UnixNano()),
						Name: orgName, DefaultLocale: defaultString(customerUser.Locale, "en-US"), Timezone: defaultString(customerUser.Timezone, "UTC"),
						Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
					}
					if err := repositories.EnterpriseIAMRepository.CreateCustomerOrg(tx, org); err != nil {
						return err
					}
				}
			}
		}
		if org == nil {
			return errorsx.InvalidParam("customer organization is required")
		}

		userColumns := map[string]any{}
		if req.DisplayName != nil {
			displayName := strings.TrimSpace(*req.DisplayName)
			if displayName == "" {
				return errorsx.InvalidParam("display name is required")
			}
			userColumns["nickname"] = displayName
		}
		if req.Email != nil {
			email := utils.NormalizeNullableString(req.Email)
			if email != nil {
				if existed := repositories.UserRepository.GetByEmail(tx, *email); existed != nil && existed.ID != user.ID {
					return errorsx.InvalidParam("email is already used")
				}
			}
			userColumns["email"] = email
		}
		if req.Mobile != nil {
			mobile := utils.NormalizeNullableString(req.Mobile)
			if mobile != nil {
				if existed := repositories.UserRepository.GetByMobile(tx, *mobile); existed != nil && existed.ID != user.ID {
					return errorsx.InvalidParam("mobile is already used")
				}
			}
			userColumns["mobile"] = mobile
		}
		if status != nil {
			userColumns["status"] = *status
			revokeSessions = *status != enums.StatusOk
		}
		if len(userColumns) > 0 {
			userColumns["updated_at"] = now
			userColumns["update_user_id"] = auditUserID(operator)
			userColumns["update_user_name"] = auditUserName(operator)
			if err := repositories.UserRepository.Updates(tx, user.ID, userColumns); err != nil {
				return err
			}
		}

		customerColumns := map[string]any{
			"customer_org_id":  org.ID,
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if req.DisplayName != nil {
			customerColumns["display_name"] = strings.TrimSpace(*req.DisplayName)
		}
		if req.Email != nil {
			customerColumns["email"] = strings.TrimSpace(*req.Email)
		}
		if req.Mobile != nil {
			customerColumns["phone"] = strings.TrimSpace(*req.Mobile)
		}
		if req.Locale != nil {
			customerColumns["locale"] = defaultString(*req.Locale, "en-US")
		}
		if req.Timezone != nil {
			customerColumns["timezone"] = defaultString(*req.Timezone, "UTC")
		}
		if status != nil {
			customerColumns["status"] = *status
		}
		if err := repositories.EnterpriseIAMRepository.UpdateCustomerUser(tx, tenantID, customerUserID, customerColumns); err != nil {
			return err
		}

		roleCodes := normalizeIAMRoleCodes(req.RoleCodes)
		if len(roleCodes) > 0 {
			if err := replaceIAMRoleBindingsDB(tx, tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUserID, roleCodes, CustomerRoleUser, operator); err != nil {
				return err
			}
		} else {
			var err error
			roleCodes, err = s.roleCodesForSubjectDB(tx, tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUserID)
			if err != nil {
				return err
			}
		}
		customerUser = repositories.EnterpriseIAMRepository.GetCustomerUser(tx, tenantID, customerUserID)
		org = repositories.EnterpriseIAMRepository.GetCustomerOrg(tx, tenantID, customerUser.CustomerOrgID)
		if len(roleCodes) == 0 {
			roleCodes = []string{CustomerRoleUser}
		}
		result = EnterpriseIAMCustomerAuthorizeResult{CustomerUser: customerUser, CustomerOrg: org, Roles: roleCodes}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if revokeSessions {
		_ = LoginSessionService.RevokeByUser(result.CustomerUser.UserID, auditUserID(operator), auditUserName(operator))
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeCustomer, "customer_user", fmt.Sprint(customerUserID), "customer_user.updated", existing, req, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdateCustomerUserStatus(tenantID, customerUserID int64, req request.EnterpriseCustomerUserStatusRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMCustomerAuthorizeResult, error) {
	status := req.Status
	return s.UpdateCustomerUser(tenantID, customerUserID, request.EnterpriseCustomerUserUpdateRequest{Status: &status}, operator)
}

func (s *enterpriseIAMService) StartCustomerPortalSupportSession(tenantID, customerUserID int64, operator *dto.AuthPrincipal, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 {
		return nil, fmt.Errorf("operator is required")
	}
	if tenantID <= 0 || customerUserID <= 0 {
		return nil, fmt.Errorf("customer user is required")
	}
	var login *response.LoginResponse
	var grantID int64
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(tx, tenantID, customerUserID)
		if customerUser == nil || customerUser.Status != enums.StatusOk {
			return fmt.Errorf("customer user is not available")
		}
		user := repositories.UserRepository.Get(tx, customerUser.UserID)
		if user == nil || user.Status != enums.StatusOk {
			return fmt.Errorf("customer login account is not available")
		}
		now := time.Now()
		grant := &models.TemporaryAccessGrant{
			TenantID:      tenantID,
			GranteeType:   operator.SubjectType,
			GranteeID:     operator.SubjectID,
			ResourceType:  models.SubjectTypeCustomerUser,
			ResourceID:    fmt.Sprint(customerUser.ID),
			ScopeRuleJSON: `{"mode":"customer_portal","role":"customer_user"}`,
			Reason:        "企业客服协助客户门户",
			ApprovedBy:    operator.UserID,
			ApprovedAt:    &now,
			ExpiredAt:     now.Add(2 * time.Hour),
			Status:        models.AccessGrantStatusActive,
			AuditFields:   utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreateTemporaryAccessGrant(tx, grant); err != nil {
			return err
		}
		var err error
		login, err = AuthService.IssueEnterpriseCustomerSessionDB(tx, operator, customerUser, grant, clientIP, userAgent, authCfg)
		if err != nil {
			return err
		}
		grantID = grant.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeCustomer, "customer_user", fmt.Sprint(customerUserID), "customer_user.support_session_started", nil, map[string]any{
		"mode":       "customer_portal",
		"expires_at": login.ExpiresAt,
	}, models.RiskLevelHigh, fmt.Sprint(grantID))
	return login, nil
}

func (s *enterpriseIAMService) StartMemberPortalSupportSession(tenantID, memberID int64, operator *dto.AuthPrincipal, clientIP, userAgent string, authCfg config.AuthConfig) (*response.LoginResponse, error) {
	if operator == nil || operator.UserID <= 0 {
		return nil, fmt.Errorf("operator is required")
	}
	if tenantID <= 0 || memberID <= 0 {
		return nil, fmt.Errorf("enterprise member is required")
	}
	if operator.EffectiveTenantID() != tenantID {
		return nil, fmt.Errorf("operator is not in the current tenant")
	}
	var login *response.LoginResponse
	var grantID int64
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		member := repositories.EnterpriseIAMRepository.GetTenantMember(tx, tenantID, memberID)
		if member == nil || member.Status != enums.StatusOk {
			return fmt.Errorf("enterprise member is not available")
		}
		if member.UserID == operator.UserID {
			return fmt.Errorf("cannot enter the current account")
		}
		user := repositories.UserRepository.Get(tx, member.UserID)
		if user == nil || user.Status != enums.StatusOk {
			return fmt.Errorf("employee login account is not available")
		}
		now := time.Now()
		grant := &models.TemporaryAccessGrant{
			TenantID:      tenantID,
			GranteeType:   operator.SubjectType,
			GranteeID:     operator.SubjectID,
			ResourceType:  models.SubjectTypeTenantMember,
			ResourceID:    fmt.Sprint(member.ID),
			ScopeRuleJSON: `{"mode":"employee_portal","identity":"target_member"}`,
			Reason:        "企业管理员进入员工门户",
			ApprovedBy:    operator.UserID,
			ApprovedAt:    &now,
			ExpiredAt:     now.Add(2 * time.Hour),
			Status:        models.AccessGrantStatusActive,
			AuditFields:   utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreateTemporaryAccessGrant(tx, grant); err != nil {
			return err
		}
		var err error
		login, err = AuthService.IssueEnterpriseMemberSessionDB(tx, operator, member, grant, clientIP, userAgent, authCfg)
		if err != nil {
			return err
		}
		grantID = grant.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeEnterprise, "tenant_member", fmt.Sprint(memberID), "tenant_member.support_session_started", nil, map[string]any{
		"mode":       "employee_portal",
		"expires_at": login.ExpiresAt,
	}, models.RiskLevelHigh, fmt.Sprint(grantID))
	return login, nil
}

func (s *enterpriseIAMService) InvitePartnerAdmin(tenantID int64, req request.EnterprisePartnerAdminInviteRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMPartnerAdminInviteResult, error) {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant == nil {
		return nil, fmt.Errorf("tenant not found")
	}
	email, err := normalizeCustomerRegistrationEmail(req.Email)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(req.ContactName)
	}
	if displayName == "" {
		displayName = email
	}
	roleJSON, err := json.Marshal([]string{PartnerRoleAdmin})
	if err != nil {
		return nil, err
	}
	inviteCode, err := generateCustomerRegistrationCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(customerRegistrationInviteTTL)
	var result EnterpriseIAMPartnerAdminInviteResult
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := EnsureTenantDefaultIAMRolesDB(tx, tenantID, operator); err != nil {
			return err
		}
		company := repositories.EnterpriseIAMRepository.GetPartnerCompany(tx, tenantID, req.PartnerCompanyID)
		if req.PartnerCompanyID > 0 && company == nil {
			return fmt.Errorf("supplier company not found in current tenant")
		}
		if company == nil {
			if strings.TrimSpace(req.PartnerName) == "" {
				return errorsx.InvalidParam("supplier name is required")
			}
			company = &models.PartnerCompany{
				TenantID: tenantID, PartnerNo: defaultString(req.PartnerNo, fmt.Sprintf("SUP-%d-%d", tenantID, now.UnixNano())),
				Name: strings.TrimSpace(req.PartnerName), PartnerType: defaultString(req.PartnerType, "service_supplier"),
				CountryRegion: strings.TrimSpace(req.CountryRegion), ContactName: defaultString(req.ContactName, displayName),
				Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.EnterpriseIAMRepository.CreatePartnerCompany(tx, company); err != nil {
				return err
			}
		}
		if company.Status != enums.StatusOk {
			return errorsx.InvalidParam("supplier company is not available")
		}
		if err := repositories.CustomerRegistrationRepository.RevokePendingByEmail(tx, tenantID, models.DomainTypePartner, email, now); err != nil {
			return err
		}
		grant := &models.CustomerRegistrationGrant{
			TenantID: tenantID, DomainType: models.DomainTypePartner, PartnerCompanyID: company.ID,
			Email: email, DisplayName: displayName, TokenHash: hashCustomerRegistrationCode(inviteCode),
			RoleCodesJSON: string(roleJSON), Status: models.CustomerRegistrationGrantPending, ExpiresAt: expiresAt,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.CustomerRegistrationRepository.Create(tx, grant); err != nil {
			return err
		}
		result = EnterpriseIAMPartnerAdminInviteResult{PartnerCompany: company, Grant: grant, InviteCode: inviteCode}
		return nil
	})
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("portal", models.DomainTypePartner)
	params.Set("invite", inviteCode)
	params.Set("next", "/partner")
	result.RegistrationURL = buildPublicAppURL("/dashboard/login", params)
	result.EmailSent, result.EmailError = CustomerRegistrationService.sendPortalInvitationEmail(tenantID, models.DomainTypePartner, email, displayName, result.RegistrationURL, inviteCode, expiresAt, tenant, result.PartnerCompany)
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypePartner, "customer_registration_grant", fmt.Sprint(result.Grant.ID), "partner_admin.invited", nil, result.Grant, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdatePartner(tenantID, partnerCompanyID int64, req request.EnterprisePartnerUpdateRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMPartnerUpdateResult, error) {
	if tenantID <= 0 || partnerCompanyID <= 0 {
		return nil, errorsx.InvalidParam("supplier is required")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	existing := repositories.EnterpriseIAMRepository.GetPartnerCompany(sqls.DB(), tenantID, partnerCompanyID)
	if existing == nil || existing.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("supplier company not found in current tenant")
	}
	var status *enums.Status
	if req.Status != nil {
		if *req.Status != int(enums.StatusOk) && *req.Status != int(enums.StatusDisabled) {
			return nil, errorsx.InvalidParam("supplier status is invalid")
		}
		next := enums.Status(*req.Status)
		status = &next
	}

	var result EnterpriseIAMPartnerUpdateResult
	revokeUserIDs := make([]int64, 0)
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		company := repositories.EnterpriseIAMRepository.GetPartnerCompany(tx, tenantID, partnerCompanyID)
		if company == nil || company.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("supplier company not found in current tenant")
		}
		now := time.Now()
		columns := map[string]any{
			"updated_at":       now,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
		}
		if req.PartnerNo != nil {
			if partnerNo := strings.TrimSpace(*req.PartnerNo); partnerNo != "" {
				columns["partner_no"] = partnerNo
			}
		}
		if req.PartnerName != nil {
			name := strings.TrimSpace(*req.PartnerName)
			if name == "" {
				return errorsx.InvalidParam("supplier name is required")
			}
			columns["name"] = name
		}
		if req.PartnerType != nil {
			columns["partner_type"] = defaultString(*req.PartnerType, "service_supplier")
		}
		if req.CountryRegion != nil {
			columns["country_region"] = strings.TrimSpace(*req.CountryRegion)
		}
		if req.ContactName != nil {
			columns["contact_name"] = strings.TrimSpace(*req.ContactName)
		}
		if status != nil {
			columns["status"] = *status
		}
		if err := repositories.EnterpriseIAMRepository.UpdatePartnerCompany(tx, tenantID, partnerCompanyID, columns); err != nil {
			return err
		}
		if status != nil && *status == enums.StatusDisabled {
			accounts, err := repositories.EnterpriseIAMRepository.FindPartnerAccountsByCompany(tx, tenantID, partnerCompanyID)
			if err != nil {
				return err
			}
			for _, account := range accounts {
				if account.UserID > 0 {
					revokeUserIDs = append(revokeUserIDs, account.UserID)
				}
			}
			if err := repositories.EnterpriseIAMRepository.UpdatePartnerAccountsByCompany(tx, tenantID, partnerCompanyID, map[string]any{
				"status":           enums.StatusDisabled,
				"updated_at":       now,
				"update_user_id":   auditUserID(operator),
				"update_user_name": auditUserName(operator),
			}); err != nil {
				return err
			}
			if err := repositories.EnterpriseIAMRepository.UpdatePartnerAuthorizationScopesByCompany(tx, tenantID, partnerCompanyID, map[string]any{
				"status":           enums.StatusDisabled,
				"updated_at":       now,
				"update_user_id":   auditUserID(operator),
				"update_user_name": auditUserName(operator),
			}); err != nil {
				return err
			}
		}
		company = repositories.EnterpriseIAMRepository.GetPartnerCompany(tx, tenantID, partnerCompanyID)
		accountCounts, err := repositories.EnterpriseIAMRepository.CountPartnerAccountsByCompanyIDs(tx, tenantID, []int64{partnerCompanyID})
		if err != nil {
			return err
		}
		contractCounts, err := repositories.EnterpriseIAMRepository.CountPartnerContractsByCompanyIDs(tx, tenantID, []int64{partnerCompanyID})
		if err != nil {
			return err
		}
		result = EnterpriseIAMPartnerUpdateResult{PartnerCompany: company, AccountCount: accountCounts[partnerCompanyID], ContractCount: contractCounts[partnerCompanyID]}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, userID := range uniqueServiceInt64s(revokeUserIDs) {
		_ = LoginSessionService.RevokeByUser(userID, auditUserID(operator), auditUserName(operator))
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypePartner, "partner_company", fmt.Sprint(partnerCompanyID), "partner_company.updated", existing, req, models.RiskLevelMedium, "")
	return &result, nil
}

func (s *enterpriseIAMService) UpdatePartnerStatus(tenantID, partnerCompanyID int64, req request.EnterprisePartnerStatusRequest, operator *dto.AuthPrincipal) (*EnterpriseIAMPartnerUpdateResult, error) {
	status := req.Status
	return s.UpdatePartner(tenantID, partnerCompanyID, request.EnterprisePartnerUpdateRequest{Status: &status}, operator)
}

func (s *enterpriseIAMService) ListMembers(tenantID int64, query EnterpriseIAMQuery) (*EnterpriseIAMMemberListAggregate, error) {
	items, paging, err := repositories.EnterpriseIAMRepository.FindTenantMembers(sqls.DB(), tenantID, enterpriseIAMFilter(query), query.Page, query.Limit)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(items))
	departmentIDs := make([]int64, 0, len(items))
	memberIDs := make([]int64, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
		departmentIDs = append(departmentIDs, item.DepartmentID)
		memberIDs = append(memberIDs, item.ID)
	}
	users := userMap(repositories.UserRepository.FindByIds(sqls.DB(), uniqueServiceInt64s(userIDs)))
	departments, err := repositories.EnterpriseIAMRepository.FindDepartmentsByIDs(sqls.DB(), tenantID, departmentIDs)
	if err != nil {
		return nil, err
	}
	engineers, err := repositories.EnterpriseIAMRepository.FindEngineerProfilesByMemberIDs(sqls.DB(), tenantID, memberIDs)
	if err != nil {
		return nil, err
	}
	rolesBySubject, err := s.rolesBySubject(tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberIDs)
	if err != nil {
		return nil, err
	}
	return &EnterpriseIAMMemberListAggregate{
		Items:            items,
		Paging:           paging,
		Users:            users,
		Departments:      departments,
		EngineerProfiles: engineers,
		ProductGroups:    s.memberProductGroups(tenantID, items),
		RolesBySubject:   rolesBySubject,
	}, nil
}

func (s *enterpriseIAMService) memberProductGroups(tenantID int64, members []models.TenantMember) map[int64][]dto.EnterpriseIAMProductGroupDTO {
	ret := make(map[int64][]dto.EnterpriseIAMProductGroupDTO)
	if tenantID <= 0 || len(members) == 0 || !sqls.DB().Migrator().HasTable(&models.AgentTeamMember{}) {
		return ret
	}
	memberByID := make(map[int64]models.TenantMember, len(members))
	memberByUserID := make(map[int64]models.TenantMember, len(members))
	memberIDs := make([]int64, 0, len(members))
	userIDs := make([]int64, 0, len(members))
	for _, item := range members {
		memberByID[item.ID] = item
		memberByUserID[item.UserID] = item
		memberIDs = append(memberIDs, item.ID)
		userIDs = append(userIDs, item.UserID)
	}
	memberships := repositories.AgentTeamMemberRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("status", enums.StatusOk).
		Where("member_id IN ? OR user_id IN ?", uniqueServiceInt64s(memberIDs), uniqueServiceInt64s(userIDs)).
		Asc("team_id"))
	teamIDs := make([]int64, 0, len(memberships))
	for _, item := range memberships {
		if item.TeamID > 0 {
			teamIDs = append(teamIDs, item.TeamID)
		}
	}
	teams := repositories.AgentTeamRepository.FindByIds(sqls.DB(), uniqueServiceInt64s(teamIDs))
	teamByID := make(map[int64]models.AgentTeam, len(teams))
	productIDs := make([]int64, 0, len(teams))
	for _, team := range teams {
		if team.TenantID != tenantID || team.TeamType != AgentTeamTypeProductRepair || team.Status != enums.StatusOk || team.ProductID <= 0 {
			continue
		}
		teamByID[team.ID] = team
		productIDs = append(productIDs, team.ProductID)
	}
	productIDs = uniqueServiceInt64s(productIDs)
	productNameByID := make(map[int64]string, len(productIDs))
	if len(productIDs) > 0 {
		products := repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("status", enums.StatusOk).
			In("id", productIDs))
		for _, product := range products {
			productNameByID[product.ID] = product.Name
		}
	}
	seen := make(map[int64]map[int64]struct{})
	for _, membership := range memberships {
		team, ok := teamByID[membership.TeamID]
		if !ok {
			continue
		}
		member, ok := memberByID[membership.MemberID]
		if !ok {
			member, ok = memberByUserID[membership.UserID]
		}
		if !ok {
			continue
		}
		productName, ok := productNameByID[team.ProductID]
		if !ok {
			continue
		}
		if seen[member.ID] == nil {
			seen[member.ID] = make(map[int64]struct{})
		}
		if _, exists := seen[member.ID][team.ID]; exists {
			continue
		}
		seen[member.ID][team.ID] = struct{}{}
		ret[member.ID] = append(ret[member.ID], dto.EnterpriseIAMProductGroupDTO{
			TeamID:      team.ID,
			TeamName:    team.Name,
			ProductID:   team.ProductID,
			ProductName: productName,
		})
	}
	return ret
}

func (s *enterpriseIAMService) ListCustomerUsers(tenantID int64, query EnterpriseIAMQuery) (*EnterpriseIAMCustomerUserListAggregate, error) {
	items, paging, err := repositories.EnterpriseIAMRepository.FindCustomerUsers(sqls.DB(), tenantID, enterpriseIAMFilter(query), query.Page, query.Limit)
	if err != nil {
		return nil, err
	}
	orgIDs := make([]int64, 0, len(items))
	subjectIDs := make([]int64, 0, len(items))
	for _, item := range items {
		orgIDs = append(orgIDs, item.CustomerOrgID)
		subjectIDs = append(subjectIDs, item.ID)
	}
	orgs, err := repositories.EnterpriseIAMRepository.FindCustomerOrgsByIDs(sqls.DB(), tenantID, orgIDs)
	if err != nil {
		return nil, err
	}
	rolesBySubject, err := s.rolesBySubject(tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, subjectIDs)
	if err != nil {
		return nil, err
	}
	return &EnterpriseIAMCustomerUserListAggregate{Items: items, Paging: paging, CustomerOrgs: orgs, RolesBySubject: rolesBySubject}, nil
}

func (s *enterpriseIAMService) ListPartners(tenantID int64, query EnterpriseIAMQuery) (*EnterpriseIAMPartnerListAggregate, error) {
	items, paging, err := repositories.EnterpriseIAMRepository.FindPartnerCompanies(sqls.DB(), tenantID, enterpriseIAMFilter(query), query.Page, query.Limit)
	if err != nil {
		return nil, err
	}
	companyIDs := make([]int64, 0, len(items))
	for _, item := range items {
		companyIDs = append(companyIDs, item.ID)
	}
	accountCounts, err := repositories.EnterpriseIAMRepository.CountPartnerAccountsByCompanyIDs(sqls.DB(), tenantID, companyIDs)
	if err != nil {
		return nil, err
	}
	contractCounts, err := repositories.EnterpriseIAMRepository.CountPartnerContractsByCompanyIDs(sqls.DB(), tenantID, companyIDs)
	if err != nil {
		return nil, err
	}
	return &EnterpriseIAMPartnerListAggregate{Items: items, Paging: paging, AccountCounts: accountCounts, ContractCounts: contractCounts}, nil
}

func (s *enterpriseIAMService) ListDepartments(tenantID int64, query EnterpriseIAMQuery) (*EnterpriseIAMDepartmentListAggregate, error) {
	items, paging, err := repositories.EnterpriseIAMRepository.FindDepartments(sqls.DB(), tenantID, enterpriseIAMFilter(query), query.Page, query.Limit)
	if err != nil {
		return nil, err
	}
	managerIDs := make([]int64, 0, len(items))
	departmentIDs := make([]int64, 0, len(items))
	for _, item := range items {
		managerIDs = append(managerIDs, item.ManagerMemberID)
		departmentIDs = append(departmentIDs, item.ID)
	}
	managers, err := repositories.EnterpriseIAMRepository.FindTenantMembersByIDs(sqls.DB(), tenantID, managerIDs)
	if err != nil {
		return nil, err
	}
	memberCounts, err := repositories.EnterpriseIAMRepository.CountMembersByDepartmentIDs(sqls.DB(), tenantID, departmentIDs)
	if err != nil {
		return nil, err
	}
	return &EnterpriseIAMDepartmentListAggregate{Items: items, Paging: paging, Managers: managers, MemberCounts: memberCounts}, nil
}

func (s *enterpriseIAMService) CreateDepartment(tenantID int64, req request.EnterpriseDepartmentCreateRequest, operator *dto.AuthPrincipal) (*models.Department, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID) == nil {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParam("department name is required")
	}
	if len([]rune(name)) > 64 {
		return nil, errorsx.InvalidParam("department name is too long")
	}

	var created *models.Department
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if _, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(tx, tenantID, operator); err != nil {
			return err
		}
		parent := repositories.EnterpriseIAMRepository.GetDepartment(tx, tenantID, req.ParentID)
		if req.ParentID > 0 && parent == nil {
			return errorsx.InvalidParam("parent department not found")
		}
		if parent == nil {
			parent = repositories.EnterpriseIAMRepository.FindRootDepartment(tx, tenantID)
		}
		if parent == nil {
			return errorsx.BusinessError(1, "tenant root department not found")
		}
		if parent.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("parent department is deleted")
		}
		if s.isProtectedSupportDepartmentDB(tx, tenantID, parent) {
			return errorsx.Forbidden("技术售后组和产品节点由系统维护，不能在这里创建子组织")
		}
		if existing := repositories.EnterpriseIAMRepository.FindDepartmentByParentAndName(tx, tenantID, parent.ID, name); existing != nil {
			return errorsx.BusinessError(2, "同级组织名称已存在")
		}
		if req.ManagerMemberID > 0 {
			if member := repositories.EnterpriseIAMRepository.GetTenantMember(tx, tenantID, req.ManagerMemberID); member == nil {
				return errorsx.InvalidParam("manager member not found")
			}
		}

		parentPath := strings.TrimSpace(parent.Path)
		if parentPath == "" || parentPath == "/" {
			parentPath = ""
		}
		item := &models.Department{
			TenantID:        tenantID,
			ParentID:        parent.ID,
			DepartmentCode:  fmt.Sprintf("org-%d-%d", tenantID, time.Now().UnixNano()),
			Name:            name,
			Path:            parentPath + "/" + departmentPathSegment(name),
			Depth:           parent.Depth + 1,
			ManagerMemberID: req.ManagerMemberID,
			RegionCode:      strings.TrimSpace(req.RegionCode),
			Status:          enums.StatusOk,
			AuditFields:     utils.BuildAuditFields(operator),
		}
		if err := repositories.EnterpriseIAMRepository.CreateDepartment(tx, item); err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypeEnterprise, "department", fmt.Sprint(created.ID), "department.created", nil, created, models.RiskLevelMedium, "")
	return created, nil
}

func (s *enterpriseIAMService) isProtectedSupportDepartmentDB(db *gorm.DB, tenantID int64, department *models.Department) bool {
	if department == nil || tenantID <= 0 {
		return false
	}
	code := strings.TrimSpace(department.DepartmentCode)
	path := strings.TrimSpace(department.Path)
	if code == technicalRepairDepartmentCode ||
		path == TechnicalAfterSalesTeamPath || strings.HasPrefix(path, TechnicalAfterSalesTeamPath+"/") ||
		path == TechnicalMaintenanceTeamPath || strings.HasPrefix(path, TechnicalMaintenanceTeamPath+"/") {
		return true
	}
	team := repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("department_id", department.ID).
		Eq("system_managed", true).
		Where("team_type IN ?", []string{AgentTeamTypeTechnicalRepair, AgentTeamTypeProductRepair}).
		NotEq("status", enums.StatusDeleted))
	return team != nil
}

func departmentPathSegment(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "/", "-"))
	if value == "" {
		return "未命名组织"
	}
	return value
}

func (s *enterpriseIAMService) ListRoles(tenantID int64, domainType string) ([]models.AuthRole, map[int64][]string, error) {
	items, err := repositories.PlatformIAMRepository.FindAuthRoles(sqls.DB(), tenantID, domainType)
	if err != nil {
		return nil, nil, err
	}
	roleIDs := make([]int64, 0, len(items))
	for _, item := range items {
		roleIDs = append(roleIDs, item.ID)
	}
	permissions, err := repositories.PlatformIAMRepository.FindPermissionsByRoleIDs(sqls.DB(), roleIDs)
	if err != nil {
		return nil, nil, err
	}
	return items, permissions, nil
}

func (s *enterpriseIAMService) ListAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, *sqls.Paging, error) {
	return listAuditLogs(tenantID, query)
}

func listAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, *sqls.Paging, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 20
	}
	filter := auditRepositoryFilter(tenantID, query)
	switch normalizedAuditLogSource(query.Source) {
	case "auth":
		items, paging, err := repositories.PlatformIAMRepository.FindAuthAuditLogs(sqls.DB(), filter, query.Page, query.Limit)
		if err != nil {
			return nil, nil, err
		}
		return enrichAuditTicketMetadata(items), paging, nil
	case "business":
		businessItems, paging, err := repositories.PlatformIAMRepository.FindBusinessAuditLogs(sqls.DB(), filter, query.Page, query.Limit)
		if err != nil {
			return nil, nil, err
		}
		items := make([]models.AuthAuditLog, 0, len(businessItems))
		for i := range businessItems {
			items = append(items, enterpriseBusinessAuditAsAuth(businessItems[i]))
		}
		return enrichAuditTicketMetadata(items), paging, nil
	}

	headLimit := query.Page * query.Limit
	authItems, authTotal, err := repositories.PlatformIAMRepository.FindAuthAuditLogHead(sqls.DB(), filter, headLimit)
	if err != nil {
		return nil, nil, err
	}
	businessItems, businessTotal, err := repositories.PlatformIAMRepository.FindBusinessAuditLogHead(sqls.DB(), filter, headLimit)
	if err != nil {
		return nil, nil, err
	}

	items := make([]models.AuthAuditLog, 0, len(authItems)+len(businessItems))
	items = append(items, authItems...)
	for i := range businessItems {
		items = append(items, enterpriseBusinessAuditAsAuth(businessItems[i]))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OccurredAt.Equal(items[j].OccurredAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].OccurredAt.After(items[j].OccurredAt)
	})
	offset := (query.Page - 1) * query.Limit
	if offset >= len(items) {
		items = []models.AuthAuditLog{}
	} else {
		end := offset + query.Limit
		if end > len(items) {
			end = len(items)
		}
		items = items[offset:end]
	}
	return enrichAuditTicketMetadata(items), &sqls.Paging{Page: query.Page, Limit: query.Limit, Total: authTotal + businessTotal}, nil
}

func normalizedAuditLogSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "auth", "iam", "identity":
		return "auth"
	case "business", "operation", "ops":
		return "business"
	default:
		return ""
	}
}

func (s *enterpriseIAMService) ExportAuditLogs(tenantID int64, query AuditLogQuery) ([]models.AuthAuditLog, error) {
	query.Page = 1
	query.Limit = 200
	items := make([]models.AuthAuditLog, 0, 200)
	for {
		pageItems, paging, err := s.ListAuditLogs(tenantID, query)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(items) > 10000 {
			return nil, errorsx.InvalidParam("审计记录超过 10000 条，请缩小时间或风险范围后导出")
		}
		if len(pageItems) == 0 || int64(query.Page*query.Limit) >= paging.Total {
			break
		}
		query.Page++
	}
	return items, nil
}

func auditRepositoryFilter(tenantID int64, query AuditLogQuery) repositories.AuditLogFilter {
	filter := repositories.AuditLogFilter{
		TenantID:   tenantID,
		Action:     query.Action,
		RiskLevel:  query.RiskLevel,
		Status:     query.Status,
		TargetType: query.TargetType,
		Search:     query.Search,
		ActorID:    query.ActorID,
	}
	now := time.Now()
	switch strings.TrimSpace(query.Period) {
	case "24h":
		start := now.Add(-24 * time.Hour)
		filter.StartAt = &start
	case "7d":
		start := now.AddDate(0, 0, -7)
		filter.StartAt = &start
	case "14d":
		start := now.AddDate(0, 0, -14)
		filter.StartAt = &start
	case "30d":
		start := now.AddDate(0, 0, -30)
		filter.StartAt = &start
	case "90d":
		start := now.AddDate(0, 0, -90)
		filter.StartAt = &start
	case "180d":
		start := now.AddDate(0, 0, -180)
		filter.StartAt = &start
	}
	return filter
}

func enrichAuditTicketMetadata(items []models.AuthAuditLog) []models.AuthAuditLog {
	if len(items) == 0 {
		return items
	}
	ticketIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for i := range items {
		for _, id := range auditLogTicketIDs(items[i]) {
			if id > 0 && !seen[id] {
				seen[id] = true
				ticketIDs = append(ticketIDs, id)
			}
		}
	}
	if len(ticketIDs) == 0 {
		return items
	}
	tickets := make([]models.Ticket, 0, len(ticketIDs))
	if err := sqls.DB().Where("id IN ?", ticketIDs).Find(&tickets).Error; err != nil {
		return items
	}
	ticketNoByID := make(map[int64]string, len(tickets))
	for i := range tickets {
		ticketNoByID[tickets[i].ID] = strings.TrimSpace(tickets[i].TicketNo)
	}
	for i := range items {
		for _, id := range auditLogTicketIDs(items[i]) {
			if ticketNo := ticketNoByID[id]; ticketNo != "" {
				if next, ok := auditJSONWithValue(items[i].AfterStateJSON, "ticketNo", ticketNo); ok {
					items[i].AfterStateJSON = next
				} else if next, ok := auditJSONWithValue(items[i].BeforeStateJSON, "ticketNo", ticketNo); ok {
					items[i].BeforeStateJSON = next
				}
				break
			}
		}
	}
	return items
}

func auditLogTicketIDs(item models.AuthAuditLog) []int64 {
	ids := make([]int64, 0, 2)
	if item.TargetType == "ticket" {
		if id := parseInt64OrZero(item.TargetID); id > 0 {
			ids = append(ids, id)
		}
	}
	state := parseAuditJSONMap(item.AfterStateJSON)
	if len(state) == 0 {
		state = parseAuditJSONMap(item.BeforeStateJSON)
	}
	if id := auditStateInt64(state["ticketId"]); id > 0 {
		ids = append(ids, id)
	}
	return ids
}

func auditJSONWithValue(raw, key, value string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return raw, false
	}
	state := parseAuditJSONMap(raw)
	if state == nil {
		state = make(map[string]any)
	}
	if strings.TrimSpace(auditStateStringForService(state[key])) != "" {
		return raw, true
	}
	state[key] = value
	next, err := json.Marshal(state)
	if err != nil {
		return raw, false
	}
	return string(next), true
}

func parseAuditJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return nil
	}
	var ret map[string]any
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		return nil
	}
	return ret
}

func auditStateInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		return parseInt64OrZero(typed)
	default:
		return 0
	}
}

func auditStateStringForService(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func enterpriseBusinessAuditAsAuth(item models.AuditLog) models.AuthAuditLog {
	actorID := parseInt64OrZero(item.ActorID)
	actorType := strings.TrimSpace(item.ActorType)
	if actorType == "" {
		actorType = "system"
	}
	return models.AuthAuditLog{
		ID:               -item.ID,
		TenantID:         item.TenantID,
		DomainType:       models.DomainTypeEnterprise,
		ActorUserID:      actorID,
		ActorSubjectType: actorType,
		ActorSubjectID:   actorID,
		TargetType:       item.ResourceType,
		TargetID:         item.ResourceID,
		Action:           item.Action,
		BeforeStateJSON:  item.BeforeState,
		AfterStateJSON:   item.AfterState,
		RequestID:        item.RequestID,
		SupportGrantID:   item.SupportGrantID,
		RiskLevel:        item.RiskLevel,
		Status:           item.Status,
		IPAddress:        item.IPAddress,
		UserAgent:        item.UserAgent,
		OccurredAt:       item.CreatedAt,
	}
}

func (s *enterpriseIAMService) rolesBySubject(tenantID int64, domainType, subjectType string, subjectIDs []int64) (map[int64][]string, error) {
	return s.rolesBySubjectDB(sqls.DB(), tenantID, domainType, subjectType, subjectIDs)
}

func (s *enterpriseIAMService) roleCodesForSubjectDB(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64) ([]string, error) {
	roles, err := s.rolesBySubjectDB(db, tenantID, domainType, subjectType, []int64{subjectID})
	if err != nil {
		return nil, err
	}
	return roles[subjectID], nil
}

func (s *enterpriseIAMService) rolesBySubjectDB(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectIDs []int64) (map[int64][]string, error) {
	ret := make(map[int64][]string, len(subjectIDs))
	bindings, err := repositories.EnterpriseIAMRepository.FindRoleBindingsBySubjects(db, tenantID, domainType, subjectType, subjectIDs)
	if err != nil {
		return nil, err
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
	}
	roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, uniqueServiceInt64s(roleIDs))
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if role := roles[binding.RoleID]; role != nil {
			ret[binding.SubjectID] = append(ret[binding.SubjectID], role.Code)
		}
	}
	return ret, nil
}

func enterpriseIAMFilter(query EnterpriseIAMQuery) repositories.EnterpriseIAMFilter {
	return repositories.EnterpriseIAMFilter{Search: query.Search, Status: query.Status}
}

func userMap(items []models.User) map[int64]*models.User {
	ret := make(map[int64]*models.User, len(items))
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret
}

func uniqueServiceInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	ret := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}

func containsIAMRole(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
