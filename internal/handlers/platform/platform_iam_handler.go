package platform

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
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func GetPlatformTenantList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	page := queryInt(ctx, "page", 1)
	limit := queryInt(ctx, "limit", 20)
	aggregate, err := services.PlatformIAMService.ListTenants(ctx.Query("search"), ctx.Query("status"), ctx.Query("lifecycle"), ctx.Query("billing"), page, limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, responseTenantList(aggregate))
}

func PostPlatformTenantCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformTenantCreateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenant, err := services.PlatformIAMService.CreateTenant(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildPlatformTenantWithPortalSettings(tenant))
}

func PostPlatformTenantUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformTenantUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenant, err := services.PlatformIAMService.UpdateTenant(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildPlatformTenantWithPortalSettings(tenant))
}

func PostPlatformTenantFreeze(ctx *gin.Context) {
	changeTenantState(ctx, services.PlatformIAMService.FreezeTenant)
}

func PostPlatformTenantUnfreeze(ctx *gin.Context) {
	changeTenantState(ctx, services.PlatformIAMService.UnfreezeTenant)
}

func PostPlatformTenantEnter(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformTenantEnterRequest
	if err = params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.PlatformIAMService.EnterTenant(req, operator, config.Current().Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func PostPlatformTenantAdminPassword(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformTenantAdminPasswordRequest
	if err = params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.PlatformIAMService.ResetTenantAdministratorPassword(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func PostPlatformTenantDecommission(ctx *gin.Context) {
	var req request.PlatformTenantDecommissionRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenant, err := services.PlatformIAMService.DecommissionTenant(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildPlatformTenantWithPortalSettings(tenant))
}

func PostPlatformTenantDelete(ctx *gin.Context) {
	var req request.PlatformTenantDeleteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenant, err := services.PlatformIAMService.DeleteTenant(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildPlatformTenantWithPortalSettings(tenant))
}

func PostPlatformTenantUndoDecommission(ctx *gin.Context) {
	changeTenantState(ctx, services.PlatformIAMService.UndoDecommissionTenant)
}

func PostPlatformTenantDataExport(ctx *gin.Context) {
	var req request.PlatformTenantDataExportRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.PlatformIAMService.BuildTenantDataExport(req))
}

func PostPlatformTenantRightToErasure(ctx *gin.Context) {
	var req request.PlatformTenantRightToErasureRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.PlatformIAMService.BuildRightToErasure(req))
}

func GetPlatformPlanList(ctx *gin.Context) {
	items, err := services.PlatformIAMService.ListPlans(ctx.Query("status"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformPlanList(items))
}

func PostPlatformPlanCreate(ctx *gin.Context) {
	savePlatformPlan(ctx)
}

func PostPlatformPlanUpdate(ctx *gin.Context) {
	savePlatformPlan(ctx)
}

func PostPlatformSubscriptionAssign(ctx *gin.Context) {
	var req request.PlatformSubscriptionAssignRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	subscription, err := services.PlatformIAMService.AssignSubscription(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSubscription(subscription))
}

func PostPlatformSubscriptionChangePlan(ctx *gin.Context) {
	PostPlatformSubscriptionAssign(ctx)
}

func PostPlatformQuotaOverride(ctx *gin.Context) {
	var req request.PlatformQuotaOverrideRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.OverrideQuota(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformQuotaOverride(item))
}

func GetPlatformSub2APIAccountList(ctx *gin.Context) {
	tenantID, _ := params.GetInt64(ctx, "tenantId")
	items, err := services.PlatformIAMService.ListSub2APIAccounts(tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSub2APIAccountList(items))
}

func PostPlatformSub2APIAccountBind(ctx *gin.Context) {
	var req request.PlatformSub2APIAccountBindRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.BindSub2APIAccount(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSub2APIAccount(item))
}

func PostPlatformSub2APIAccountTest(ctx *gin.Context) {
	touchSub2APIAccount(ctx, "test")
}

func PostPlatformSub2APIAccountSync(ctx *gin.Context) {
	touchSub2APIAccount(ctx, "sync")
}

func GetPlatformStaffList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	aggregate, err := services.PlatformIAMService.ListPlatformStaff(ctx.Query("status"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformStaffList(aggregate.Items, aggregate.Users))
}

func PostPlatformStaffInvite(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformStaffInviteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.InvitePlatformStaff(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformStaff(item, repositories.UserRepository.Get(sqls.DB(), item.UserID)))
}

func PostPlatformStaffUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformStaffUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.UpdatePlatformStaff(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformStaff(item, repositories.UserRepository.Get(sqls.DB(), item.UserID)))
}

func PostPlatformStaffGrantTenant(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserAssignRole)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformStaffGrantTenantRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.GrantPlatformTenant(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformTenantGrant(item))
}

func PostPlatformStaffDisable(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformStaffDisableRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.DisablePlatformStaff(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformStaff(item, repositories.UserRepository.Get(sqls.DB(), item.UserID)))
}

func PostPlatformStaffDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionUserDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformStaffDeleteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.PlatformIAMService.DeletePlatformStaff(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"success": true})
}

func GetPlatformPermissionRoleList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionRoleView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, _ := params.GetInt64(ctx, "tenantId")
	items, permissions, err := services.PlatformIAMService.ListAuthRoles(tenantID, ctx.Query("domainType"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRoleList(items, permissions))
}

func PostPlatformPermissionRoleSave(ctx *gin.Context) {
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
	item, err := services.PlatformIAMService.SaveAuthRole(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRole(item, nil))
}

func PostPlatformPermissionRoleDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionRoleDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformAuthRoleDeleteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.PlatformIAMService.DeleteAuthRole(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"success": true})
}

func PostPlatformPermissionPolicySave(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionRoleAssignPermission)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformAuthPolicySaveRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	role, permissions, err := services.PlatformIAMService.SaveAuthPolicy(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAuthRole(role, permissions))
}

func PostPlatformPermissionCheck(ctx *gin.Context) {
	var req request.PlatformAuthPermissionCheckRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.PlatformIAMService.CheckAuthPermission(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func GetPlatformAuditList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionSessionView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, _ := params.GetInt64(ctx, "tenantId")
	page := queryInt(ctx, "page", 1)
	limit := queryInt(ctx, "limit", 20)
	query := platformAuditLogQuery(ctx)
	query.Page = page
	query.Limit = limit
	items, paging, err := services.PlatformIAMService.ListAuditLogs(tenantID, query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(builders.BuildPlatformAuditLogList(items), paging))
}

func PostPlatformAuditExport(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionSessionView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, _ := params.GetInt64(ctx, "tenantId")
	items, err := services.PlatformIAMService.ExportAuditLogs(tenantID, platformAuditLogQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_ = services.PlatformIAMService.RecordAuthAudit(operator, tenantID, models.DomainTypePlatform, "audit_log", "export", "audit.exported", nil, map[string]any{"count": len(items)}, models.RiskLevelMedium, "")
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=platform-audit-%s.csv", time.Now().Format("20060102-150405")))
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

func GetPlatformAuditRetention(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionSessionView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.AuditRetentionService.GetSettings())
}

func PostPlatformAuditRetention(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAuditRetentionManage)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformAuditRetentionUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.AuditRetentionService.UpdateSettings(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
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

func platformAuditLogQuery(ctx *gin.Context) services.AuditLogQuery {
	actorID, _ := strconv.ParseInt(ctx.Query("actorId"), 10, 64)
	return services.AuditLogQuery{
		Action: ctx.Query("action"), RiskLevel: ctx.Query("riskLevel"), Status: ctx.Query("status"),
		TargetType: ctx.Query("targetType"), Source: ctx.Query("source"), Search: ctx.Query("search"), Period: ctx.Query("period"), ActorID: actorID,
		Page: 1, Limit: 20,
	}
}

func changeTenantState(ctx *gin.Context, fn func(request.PlatformTenantStateRequest, *dto.AuthPrincipal) (*models.Tenant, error)) {
	var req request.PlatformTenantStateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenant, err := fn(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildPlatformTenantWithPortalSettings(tenant))
}

func savePlatformPlan(ctx *gin.Context) {
	var req request.PlatformPlanSaveRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.SavePlan(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformPlan(item))
}

func touchSub2APIAccount(ctx *gin.Context, action string) {
	var req request.PlatformSub2APIAccountActionRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIAMService.TouchSub2APIAccount(req, action, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSub2APIAccount(item))
}

func responseTenantList(aggregate *services.PlatformTenantListAggregate) any {
	tenants := builders.BuildPlatformTenantList(aggregate.Items, aggregate.Subscriptions, aggregate.Plans, aggregate.DeviceCounts, aggregate.MemberCounts, aggregate.MonthlyUsage)
	builders.ApplyPlatformTenantAdministrators(tenants, aggregate.Administrators, aggregate.AdminUsers)
	for _, tenant := range tenants {
		builders.ApplyPlatformTenantPortalSettings(tenant, aggregate.Brandings[tenant.ID], aggregate.ServerConsoles[tenant.ID])
	}
	ret := &response.PlatformTenantListResponse{
		Tenants:    tenants,
		TotalCount: aggregate.Paging.Total,
	}
	if aggregate.Summary != nil {
		ret.Summary = response.PlatformTenantSummaryResponse{
			Total: aggregate.Summary.Total, Active: aggregate.Summary.Active,
			Trial: aggregate.Summary.Trial, Frozen: aggregate.Summary.Frozen,
			ExpiringSoon: aggregate.Summary.ExpiringSoon,
		}
	}
	return ret
}

func buildPlatformTenantWithPortalSettings(tenant *models.Tenant) *response.PlatformTenantResponse {
	ret := builders.BuildPlatformTenant(tenant, nil, nil, 0, 0)
	if tenant == nil {
		return ret
	}
	branding, serverConsole := services.TenantPortalSettingsService.GetDB(sqls.DB(), tenant.ID)
	builders.ApplyPlatformTenantPortalSettings(ret, branding, serverConsole)
	return ret
}

func queryInt(ctx *gin.Context, name string, fallback int) int {
	value, ok := params.GetInt(ctx, name)
	if !ok || value <= 0 {
		return fallback
	}
	return value
}

func queryInt64(ctx *gin.Context, name string, fallback int64) int64 {
	value, ok := params.GetInt64(ctx, name)
	if !ok {
		return fallback
	}
	return value
}
