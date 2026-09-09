package enterprise

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func IAMMemberInvite(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseMemberInviteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.InviteMember(tenantID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &dto.EnterpriseIAMMemberInviteDTO{
		Member:          builders.BuildEnterpriseIAMMember(result.Member, result.User, result.Department, result.Engineer, nil, result.Roles),
		InitialPassword: result.InitialPassword,
	})
}

func IAMCustomerUserAuthorize(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseCustomerUserAuthorizeRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.AuthorizeCustomerUser(tenantID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &dto.EnterpriseIAMCustomerAuthorizeDTO{
		CustomerUser:    builders.BuildEnterpriseIAMCustomerUser(result.CustomerUser, result.CustomerOrg, result.Roles),
		InitialPassword: result.InitialPassword,
	})
}

func IAMCustomerUserInvite(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseCustomerUserInviteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.CustomerRegistrationService.InviteCustomer(tenantID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &dto.EnterpriseIAMCustomerInviteDTO{
		GrantID:    result.Grant.ID,
		InviteCode: result.InviteCode, RegistrationURL: result.RegistrationURL, Email: result.Grant.Email,
		CustomerOrgID: result.CustomerOrg.ID, CustomerOrgName: result.CustomerOrg.Name,
		ExpiresAt: result.Grant.ExpiresAt.Format(time.RFC3339), EmailSent: result.EmailSent, EmailError: result.EmailError,
	})
}

func IAMCustomerUserPortalSession(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	customerUserID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || customerUserID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid customer user id"))
		return
	}
	login, err := services.EnterpriseIAMService.StartCustomerPortalSupportSession(
		tenantID,
		customerUserID,
		operator,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
		config.Current().Auth,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func IAMMemberPortalSession(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || memberID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid enterprise member id"))
		return
	}
	login, err := services.EnterpriseIAMService.StartMemberPortalSupportSession(
		tenantID,
		memberID,
		operator,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
		config.Current().Auth,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func IAMMemberUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || memberID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid enterprise member id"))
		return
	}
	var req request.EnterpriseMemberUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdateMember(tenantID, memberID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMMember(result.Member, result.User, result.Department, result.Engineer, nil, result.Roles))
}

func IAMMemberStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || memberID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid enterprise member id"))
		return
	}
	var req request.EnterpriseMemberStatusRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdateMemberStatus(tenantID, memberID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMMember(result.Member, result.User, result.Department, result.Engineer, nil, result.Roles))
}

func IAMPartnerAdminInvite(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterprisePartnerAdminInviteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.InvitePartnerAdmin(tenantID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &dto.EnterpriseIAMPartnerAdminInviteDTO{
		Partner:    builders.BuildEnterpriseIAMPartner(result.PartnerCompany, 0, 0),
		InviteCode: result.InviteCode, RegistrationURL: result.RegistrationURL, Email: result.Grant.Email,
		PartnerID: result.PartnerCompany.ID, PartnerName: result.PartnerCompany.Name,
		ExpiresAt: result.Grant.ExpiresAt.Format(time.RFC3339), EmailSent: result.EmailSent, EmailError: result.EmailError,
	})
}

func IAMCustomerUserUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	customerUserID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || customerUserID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid customer user id"))
		return
	}
	var req request.EnterpriseCustomerUserUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdateCustomerUser(tenantID, customerUserID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMCustomerUser(result.CustomerUser, result.CustomerOrg, result.Roles))
}

func IAMCustomerUserStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	customerUserID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || customerUserID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid customer user id"))
		return
	}
	var req request.EnterpriseCustomerUserStatusRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdateCustomerUserStatus(tenantID, customerUserID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMCustomerUser(result.CustomerUser, result.CustomerOrg, result.Roles))
}

func IAMPartnerUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	partnerID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || partnerID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid supplier id"))
		return
	}
	var req request.EnterprisePartnerUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdatePartner(tenantID, partnerID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMPartner(result.PartnerCompany, result.AccountCount, result.ContractCount))
}

func IAMPartnerStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	partnerID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || partnerID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid supplier id"))
		return
	}
	var req request.EnterprisePartnerStatusRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseIAMService.UpdatePartnerStatus(tenantID, partnerID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMPartner(result.PartnerCompany, result.AccountCount, result.ContractCount))
}

func IAMMemberList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseIAMService.ListMembers(tenantID, enterpriseIAMQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(
		builders.BuildEnterpriseIAMMemberList(aggregate.Items, aggregate.Users, aggregate.Departments, aggregate.EngineerProfiles, aggregate.ProductGroups, aggregate.RolesBySubject),
		aggregate.Paging,
	))
}

func IAMCustomerUserList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseIAMService.ListCustomerUsers(tenantID, enterpriseIAMQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(
		builders.BuildEnterpriseIAMCustomerUserList(aggregate.Items, aggregate.CustomerOrgs, aggregate.RolesBySubject),
		aggregate.Paging,
	))
}

func IAMPartnerList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseIAMService.ListPartners(tenantID, enterpriseIAMQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(
		builders.BuildEnterpriseIAMPartnerList(aggregate.Items, aggregate.AccountCounts, aggregate.ContractCounts),
		aggregate.Paging,
	))
}

func IAMDepartmentList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseIAMService.ListDepartments(tenantID, enterpriseIAMQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(
		builders.BuildEnterpriseIAMDepartmentList(aggregate.Items, aggregate.Managers, aggregate.MemberCounts),
		aggregate.Paging,
	))
}

func IAMDepartmentCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseDepartmentCreateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.EnterpriseIAMService.CreateDepartment(tenantID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseIAMDepartment(item, nil, 0))
}

func IAMRoleList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionRoleView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	domainType := ctx.DefaultQuery("domainType", models.DomainTypeEnterprise)
	if domainType != models.DomainTypeEnterprise && domainType != models.DomainTypeCustomer && domainType != models.DomainTypePartner {
		domainType = models.DomainTypeEnterprise
	}
	items, permissions, err := services.EnterpriseIAMService.ListRoles(tenantID, domainType)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRoleList(items, permissions))
}

func IAMRoleSave(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.PlatformAuthRoleSaveRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	permission := constants.PermissionRoleCreate
	if req.ID > 0 {
		permission = constants.PermissionRoleUpdate
	}
	operator, err := services.AuthService.RequirePermission(ctx, permission)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req.TenantID = tenantID
	req.DomainType = models.DomainTypeEnterprise
	req.IsBuiltin = false
	item, err := services.PlatformIAMService.SaveAuthRole(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRole(item, nil))
}

func IAMPolicySave(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionRoleAssignPermission)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.PlatformAuthPolicySaveRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req.TenantID = tenantID
	role, permissions, err := services.PlatformIAMService.SaveAuthPolicy(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRole(role, permissions))
}

func IAMAuditList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionSessionView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	items, paging, err := services.EnterpriseIAMService.ListAuditLogs(tenantID, enterpriseAuditLogQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(builders.BuildPlatformAuditLogList(items), paging))
}

func IAMAuditExport(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionSessionView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	query := enterpriseAuditLogQuery(ctx)
	items, err := services.EnterpriseIAMService.ExportAuditLogs(tenantID, query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID: tenantID, ActorID: strconv.FormatInt(operator.UserID, 10), ActorType: operator.SubjectType,
		Domain: "audit", ResourceType: "audit_log", ResourceID: "export", Action: "audit.exported",
		AfterState: map[string]any{"count": len(items), "period": query.Period, "riskLevel": query.RiskLevel, "status": query.Status, "targetType": query.TargetType, "source": query.Source},
		IPAddress:  ctx.ClientIP(), UserAgent: ctx.Request.UserAgent(), RequestID: httpx.GetRequestID(ctx), SupportGrantID: operator.SupportGrantID,
		RiskLevel: models.RiskLevelMedium,
	})

	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=enterprise-audit-%s.csv", time.Now().Format("20060102-150405")))
	_, _ = ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(ctx.Writer)
	_ = writer.Write([]string{"发生时间", "操作", "摘要", "风险", "结果", "操作人类型", "操作人ID", "对象类型", "对象ID", "会话ID", "消息ID", "发送方", "消息类型", "请求ID", "协助授权", "IP地址", "用户代理"})
	for i := range items {
		item := items[i]
		display := builders.BuildPlatformAuditLog(&item)
		metadata := map[string]string{}
		if display != nil && display.Metadata != nil {
			metadata = display.Metadata
		}
		summary := ""
		if display != nil {
			summary = display.Summary
		}
		_ = writer.Write([]string{
			item.OccurredAt.Format(time.RFC3339), item.Action, summary, item.RiskLevel, item.Status,
			item.ActorSubjectType, strconv.FormatInt(item.ActorUserID, 10), item.TargetType, item.TargetID,
			metadata["conversationId"], metadata["messageId"], auditExportSender(metadata), metadata["messageType"],
			item.RequestID, strconv.FormatInt(item.SupportGrantID, 10), item.IPAddress, item.UserAgent,
		})
	}
	writer.Flush()
}

func auditExportSender(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	sender := metadata["senderType"]
	if name := metadata["senderName"]; name != "" {
		if sender != "" {
			return sender + ":" + name
		}
		return name
	}
	if id := metadata["senderId"]; id != "" {
		if sender != "" {
			return sender + ":" + id
		}
		return id
	}
	return sender
}

func enterpriseAuditLogQuery(ctx *gin.Context) services.AuditLogQuery {
	actorID, _ := strconv.ParseInt(ctx.Query("actorId"), 10, 64)
	return services.AuditLogQuery{
		Action: ctx.Query("action"), RiskLevel: ctx.Query("riskLevel"), Status: ctx.Query("status"),
		TargetType: ctx.Query("targetType"), Source: ctx.Query("source"), Search: ctx.Query("search"), Period: ctx.Query("period"), ActorID: actorID,
		Page: queryPositiveInt(ctx, "page", 1), Limit: queryPositiveInt(ctx, "limit", 20),
	}
}

func enterpriseIAMQuery(ctx *gin.Context) services.EnterpriseIAMQuery {
	return services.EnterpriseIAMQuery{
		Search: ctx.Query("search"),
		Status: ctx.Query("status"),
		Page:   queryPositiveInt(ctx, "page", 1),
		Limit:  queryPositiveInt(ctx, "limit", 20),
	}
}

func queryPositiveInt(ctx *gin.Context, name string, fallback int) int {
	value, ok := params.GetInt(ctx, name)
	if !ok || value <= 0 {
		return fallback
	}
	return value
}
