package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	PlatformRoleAdmin      = "platform_admin"
	PlatformRoleOperations = "platform_operations"
	PlatformRoleFinance    = "platform_finance"
	PlatformRoleAuditor    = "platform_auditor"

	EnterpriseRoleOwner          = "tenant_owner"
	EnterpriseRoleAdmin          = "tenant_admin"
	EnterpriseRoleServiceManager = "service_manager"
	EnterpriseRoleEngineer       = "service_engineer"
	EnterpriseRoleKnowledge      = "knowledge_manager"
	EnterpriseRoleViewer         = "enterprise_viewer"

	CustomerRoleAdmin = "customer_admin"
	CustomerRoleUser  = "customer_user"
)

type iamDefaultRoleSpec struct {
	DomainType  string
	Code        string
	Name        string
	Description string
	SortNo      int
	Permissions []string
}

func EnsurePlatformDefaultIAMRolesDB(db *gorm.DB, operator *dto.AuthPrincipal) error {
	return ensureIAMRoleSpecsDB(db, 0, platformDefaultRoleSpecs(), operator)
}

func EnsureBootstrapPlatformAdministratorDB(db *gorm.DB, operator *dto.AuthPrincipal) error {
	if err := EnsurePlatformDefaultIAMRolesDB(db, operator); err != nil {
		return err
	}
	user := repositories.UserRepository.FindOne(db, sqls.NewCnd().Eq("username", constants.BootstrapAdminUsername))
	if user == nil {
		return fmt.Errorf("bootstrap administrator %s not found", constants.BootstrapAdminUsername)
	}
	profile := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, user.ID)
	if profile == nil {
		profile = &models.PlatformStaffProfile{
			UserID: user.ID, TeamCode: "platform", JobTitle: "平台管理员",
			SupportLevel: "global", EmploymentType: "employee", Status: enums.StatusOk,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreatePlatformStaff(db, profile); err != nil {
			return err
		}
	} else if profile.Status != enums.StatusOk {
		if err := repositories.PlatformIAMRepository.UpdatePlatformStaff(db, profile.ID, map[string]any{
			"status": enums.StatusOk,
		}); err != nil {
			return err
		}
	}
	return replaceIAMRoleBindingsDB(db, 0, models.DomainTypePlatform, models.SubjectTypePlatformStaff, profile.ID, []string{PlatformRoleAdmin}, PlatformRoleAdmin, operator)
}

func EnsureTenantDefaultIAMRolesDB(db *gorm.DB, tenantID int64, operator *dto.AuthPrincipal) error {
	if tenantID <= 0 {
		return fmt.Errorf("tenant id is required")
	}
	specs := tenantDefaultRoleSpecs()
	if tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID); tenant != nil && tenant.IsKnowledgeSupportScene() {
		for i := range specs {
			if specs[i].Code == EnterpriseRoleEngineer {
				specs[i].Permissions = uniqueSortedPermissionCodes(append(specs[i].Permissions, constants.PermissionTicketCreate.Code))
				break
			}
		}
	}
	return ensureIAMRoleSpecsDB(db, tenantID, specs, operator)
}

// SyncDefaultIAMRolePoliciesDB keeps code-owned builtin roles consistent across upgrades.
// Tenant-defined custom roles and their bindings are not modified.
func SyncDefaultIAMRolePoliciesDB(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := EnsurePlatformDefaultIAMRolesDB(tx, nil); err != nil {
			return err
		}
		var tenants []models.Tenant
		if err := tx.Find(&tenants).Error; err != nil {
			return err
		}
		for _, tenant := range tenants {
			if err := EnsureTenantDefaultIAMRolesDB(tx, tenant.ID, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func replaceIAMRoleBindingsDB(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64, roleCodes []string, fallbackRole string, operator *dto.AuthPrincipal) error {
	roleCodes = normalizeIAMRoleCodes(roleCodes)
	if len(roleCodes) == 0 && strings.TrimSpace(fallbackRole) != "" {
		roleCodes = []string{strings.TrimSpace(fallbackRole)}
	}
	if len(roleCodes) == 0 {
		return fmt.Errorf("at least one role is required")
	}
	now := time.Now()
	bindings := make([]models.AuthRoleBinding, 0, len(roleCodes))
	for _, code := range roleCodes {
		role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenantID, domainType, code)
		if role == nil || role.Status != enums.StatusOk {
			return fmt.Errorf("role %s is not available in %s domain", code, domainType)
		}
		bindings = append(bindings, models.AuthRoleBinding{
			TenantID: tenantID, DomainType: domainType, RoleID: role.ID,
			SubjectType: subjectType, SubjectID: subjectID, Status: enums.StatusOk,
			EffectiveAt: &now, AuditFields: utils.BuildAuditFields(operator),
		})
	}
	return repositories.PlatformIAMRepository.ReplaceRoleBindings(db, tenantID, domainType, subjectType, subjectID, bindings)
}

func ensureIAMRoleBindingDB(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64, roleCode string, operator *dto.AuthPrincipal) error {
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenantID, domainType, strings.TrimSpace(roleCode))
	if role == nil || role.Status != enums.StatusOk {
		return fmt.Errorf("role %s is not available in %s domain", roleCode, domainType)
	}
	binding := &models.AuthRoleBinding{}
	err := db.Where(
		"tenant_id = ? AND domain_type = ? AND role_id = ? AND subject_type = ? AND subject_id = ?",
		tenantID, domainType, role.ID, subjectType, subjectID,
	).First(binding).Error
	now := time.Now()
	if err == nil {
		return db.Model(&models.AuthRoleBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
			"status":           enums.StatusOk,
			"effective_at":     &now,
			"expired_at":       nil,
			"update_user_id":   auditOperatorID(operator),
			"update_user_name": auditOperatorName(operator),
			"updated_at":       now,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return db.Create(&models.AuthRoleBinding{
		TenantID: tenantID, DomainType: domainType, RoleID: role.ID,
		SubjectType: subjectType, SubjectID: subjectID, Status: enums.StatusOk,
		EffectiveAt: &now, AuditFields: utils.BuildAuditFields(operator),
	}).Error
}

func ensureIAMRoleSpecsDB(db *gorm.DB, tenantID int64, specs []iamDefaultRoleSpec, operator *dto.AuthPrincipal) error {
	for _, spec := range specs {
		role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, tenantID, spec.DomainType, spec.Code)
		if role == nil {
			role = &models.AuthRole{
				TenantID: tenantID, DomainType: spec.DomainType, Code: spec.Code,
				Name: spec.Name, Description: spec.Description, IsBuiltin: true,
				Status: enums.StatusOk, SortNo: spec.SortNo, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.PlatformIAMRepository.CreateAuthRole(db, role); err != nil {
				return err
			}
		} else if err := repositories.PlatformIAMRepository.UpdateAuthRole(db, role.ID, map[string]any{
			"name":        spec.Name,
			"description": spec.Description,
			"is_builtin":  true,
			"sort_no":     spec.SortNo,
			"status":      enums.StatusOk,
		}); err != nil {
			return err
		}
		permissions := make([]models.AuthRolePermission, 0, len(spec.Permissions))
		for _, code := range spec.Permissions {
			permissions = append(permissions, models.AuthRolePermission{
				TenantID: tenantID, RoleID: role.ID, PermissionCode: code,
				Effect: "allow", Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			})
		}
		if err := repositories.PlatformIAMRepository.ReplaceAuthRolePermissions(db, tenantID, role.ID, permissions); err != nil {
			return err
		}
	}
	return nil
}

func platformDefaultRoleSpecs() []iamDefaultRoleSpec {
	return []iamDefaultRoleSpec{
		{models.DomainTypePlatform, PlatformRoleAdmin, "平台管理员", "管理平台人员、租户、套餐、授权和审计", 10, allIAMPermissionCodes()},
		{models.DomainTypePlatform, PlatformRoleOperations, "平台运营", "负责租户开通、状态维护、用量支持和跨租户协作，不管理平台人员与角色", 20, permissionCodesByExactOrPrefix([]string{"tenant.view", "tenant.create", "tenant.update", "tenant.export", "report.view", "product.view", "productModel.view", "device.view", "ticket.view", "notification.view", "aiConfig.view", "productAIUsageCredential.view"})},
		{models.DomainTypePlatform, PlatformRoleFinance, "平台财务", "查看租户付费状态、用量成本，并处理充值、套餐和额度调整", 25, platformFinancePermissionCodes()},
		{models.DomainTypePlatform, PlatformRoleAuditor, "平台审计员", "只读查看平台配置、人员、角色、会话和业务审计数据", 30, permissionCodesByExactOrPrefix([]string{"tenant.view", "report.view", "user.view", "role.view", "permission.view", "session.view", "product.view", "device.view", "ticket.view", "notification.view"})},
	}
}

func tenantDefaultRoleSpecs() []iamDefaultRoleSpec {
	tenantAdminPermissions := uniqueSortedPermissionCodes(append(
		permissionCodesForLegacyRole(constants.RoleCodeAdmin),
		"mcp.view",
		"mcp.call",
	))
	serviceManagerPermissions := permissionCodesForPrefixes("conversation.", "ticket.", "meeting.", "notification.", "customer.", "product.", "productModel.", "device.", "serviceCode.", "serviceCodeBatch.", "knowledgeBase.", "knowledgeDocument.", "knowledgeFAQ.", "report.")
	engineerPermissions := permissionCodesByExactOrPrefix(
		[]string{"ticket.view", "ticket.update", "ticket.changeStatus", "ticket.progress", "ticket.repairHistory.view", "conversation.view", "conversation.send", "meeting.preview", "meeting.create", "meeting.view", "meeting.update", "device.view", "product.view", "productModel.view", "notification.view", "notification.update"},
	)
	knowledgeManagerPermissions := uniqueSortedPermissionCodes(append(
		permissionCodesByExactOrPrefix([]string{"product.view", "productModel.view", "aiConfig.view"}),
		permissionCodesForPrefixes("productKnowledgeBinding.", "knowledgeBase.", "knowledgeDocument.", "knowledgeFAQ.", "asset.", "aiAgent.")...,
	))
	customerPermissions := permissionCodesByExactOrPrefix([]string{"device.view", "device.create", "device.update", "ticket.view", "ticket.create", "ticket.progress", "conversation.view", "conversation.send", "meeting.view", "notification.view"})
	return []iamDefaultRoleSpec{
		{models.DomainTypeEnterprise, EnterpriseRoleOwner, "租户所有者", "租户最高权限，负责组织、人员、供应商与安全设置", 10, enterpriseAllIAMPermissionCodes()},
		{models.DomainTypeEnterprise, EnterpriseRoleAdmin, "企业管理员", "管理企业人员、客户、供应商和业务配置", 20, tenantAdminPermissions},
		{models.DomainTypeEnterprise, EnterpriseRoleServiceManager, "服务负责人", "管理客户服务、会话、工单、派工和维修协作", 30, serviceManagerPermissions},
		{models.DomainTypeEnterprise, EnterpriseRoleEngineer, "服务工程师", "处理分配给自己的会话、工单、诊断和视频协作", 40, engineerPermissions},
		{models.DomainTypeEnterprise, EnterpriseRoleKnowledge, "知识管理员", "维护产品知识、手册和 AI 诊断内容", 50, knowledgeManagerPermissions},
		{models.DomainTypeEnterprise, EnterpriseRoleViewer, "企业只读成员", "只读查看企业获授权的业务数据", 60, viewIAMPermissionCodes()},
		{models.DomainTypePartner, PartnerRoleAdmin, "供应商管理员", "管理供应商员工并分配协作工单", 10, permissionCodesByExactOrPrefix([]string{"partnerMember.view", "partnerMember.invite", "partnerMember.update", "ticket.view", "ticket.update", "ticket.assign", "ticket.changeStatus", "ticket.progress", "meeting.view", "meeting.create", "meeting.update", "notification.view"})},
		{models.DomainTypePartner, PartnerRoleEngineer, "供应商工程师", "处理协作工单并可邀请本供应商的工程师同事", 20, permissionCodesByExactOrPrefix([]string{"partnerMember.view", "partnerMember.invite", "ticket.view", "ticket.progress", "ticket.changeStatus", "meeting.view", "notification.view"})},
		{models.DomainTypeCustomer, CustomerRoleAdmin, "客户管理员", "管理本公司用户、设备归属和售后协作", 10, uniqueSortedPermissionCodes(append(customerPermissions, "customerMember.view", "customerMember.invite", "customerMember.update"))},
		{models.DomainTypeCustomer, CustomerRoleUser, "客户用户", "添加和查看自己的设备并发起售后服务", 20, customerPermissions},
	}
}

func tenantDefaultRolePermissions(roleCode string) []string {
	for _, spec := range tenantDefaultRoleSpecs() {
		if spec.Code == roleCode {
			return append([]string{}, spec.Permissions...)
		}
	}
	return nil
}

func allIAMPermissionCodes() []string {
	return uniqueSortedPermissionCodes(constants.PermissionCodes())
}

func enterpriseAllIAMPermissionCodes() []string {
	ret := make([]string, 0, len(constants.Permissions))
	for _, permission := range constants.Permissions {
		if strings.HasPrefix(permission.Code, "tenant.") ||
			strings.HasPrefix(permission.Code, "partnerMember.") ||
			strings.HasPrefix(permission.Code, "customerMember.") ||
			strings.HasPrefix(permission.Code, "finance.") {
			continue
		}
		ret = append(ret, permission.Code)
	}
	return uniqueSortedPermissionCodes(ret)
}

func viewIAMPermissionCodes() []string {
	ret := make([]string, 0)
	for _, permission := range constants.Permissions {
		if strings.HasPrefix(permission.Code, "finance.") {
			continue
		}
		if strings.HasSuffix(permission.Code, ".view") || permission.Code == "permission.view" || permission.Code == "role.view" || permission.Code == "user.view" {
			ret = append(ret, permission.Code)
		}
	}
	return uniqueSortedPermissionCodes(ret)
}

func platformFinancePermissionCodes() []string {
	return permissionCodesByExactOrPrefix([]string{
		"tenant.view",
		"report.view",
		"report.export",
		"finance.view",
		"finance.manage",
		"aiConfig.view",
		"productAIUsageCredential.view",
		"notification.view",
	})
}

func permissionCodesForLegacyRole(roleCode string) []string {
	ret := make([]string, 0)
	for _, permission := range constants.RolePermissions[roleCode] {
		ret = append(ret, permission.Code)
	}
	return uniqueSortedPermissionCodes(ret)
}

func permissionCodesForPrefixes(prefixes ...string) []string {
	ret := make([]string, 0)
	for _, permission := range constants.Permissions {
		for _, prefix := range prefixes {
			if strings.HasPrefix(permission.Code, prefix) {
				ret = append(ret, permission.Code)
				break
			}
		}
	}
	return uniqueSortedPermissionCodes(ret)
}

func permissionCodesByExactOrPrefix(codes []string) []string {
	allowed := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		allowed[code] = struct{}{}
	}
	ret := make([]string, 0, len(codes))
	for _, permission := range constants.Permissions {
		if _, ok := allowed[permission.Code]; ok {
			ret = append(ret, permission.Code)
		}
	}
	return uniqueSortedPermissionCodes(ret)
}

func uniqueSortedPermissionCodes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	ret := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	sort.Strings(ret)
	return ret
}

func normalizeIAMRoleCodes(values []string) []string {
	return uniqueSortedPermissionCodes(values)
}
