package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func NotificationTemplateList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	result, err := services.NotificationTemplateService.List(services.NotificationTemplateListQuery{
		TenantID: tenantID,
		Code:     ctx.Query("code"),
		Channel:  ctx.Query("channel"),
		Language: ctx.Query("language"),
		Status:   ctx.Query("approval_status"),
		Keyword:  ctx.Query("keyword"),
		Page:     queryPositiveInt(ctx, "page", 1),
		PageSize: queryPositiveInt(ctx, "page_size", 20),
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func NotificationTemplateCreate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	var req request.SaveNotificationTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	item, err := services.NotificationTemplateService.Create(tenantID, notificationPrincipalUserID(principal), req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.created", "notification_template", strconv.FormatInt(item.ID, 10), item)
	httpx.WriteJSON(ctx, item)
}

func NotificationTemplateUpdate(ctx *gin.Context) {
	tenantID, templateID, ok := resolveNotificationTemplateTarget(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	var req request.SaveNotificationTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	item, err := services.NotificationTemplateService.Update(tenantID, notificationPrincipalUserID(principal), templateID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.updated", "notification_template", strconv.FormatInt(templateID, 10), item)
	httpx.WriteJSON(ctx, item)
}

func NotificationTemplateApprove(ctx *gin.Context) {
	tenantID, templateID, ok := resolveNotificationTemplateTarget(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	item, err := services.NotificationTemplateService.Approve(tenantID, notificationPrincipalUserID(principal), templateID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.approved", "notification_template", strconv.FormatInt(templateID, 10), item)
	httpx.WriteJSON(ctx, item)
}

func NotificationTemplateRetire(ctx *gin.Context) {
	tenantID, templateID, ok := resolveNotificationTemplateTarget(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	item, err := services.NotificationTemplateService.Retire(tenantID, notificationPrincipalUserID(principal), templateID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.retired", "notification_template", strconv.FormatInt(templateID, 10), item)
	httpx.WriteJSON(ctx, item)
}

func NotificationTemplateDelete(ctx *gin.Context) {
	tenantID, templateID, ok := resolveNotificationTemplateTarget(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if err := services.NotificationTemplateService.Delete(tenantID, templateID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.deleted", "notification_template", strconv.FormatInt(templateID, 10), map[string]any{"templateId": templateID})
	httpx.WriteJSON(ctx, nil)
}

func NotificationTemplatePreview(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.PreviewNotificationTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	result, err := services.NotificationTemplateService.Preview(tenantID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func NotificationTemplateSeedDefaults(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	created, err := services.NotificationTemplateService.SeedDrafts(tenantID, notificationPrincipalUserID(principal))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.template.seeded", "notification_template", "defaults", map[string]any{"created": created})
	httpx.WriteJSON(ctx, map[string]any{"created": created})
}

func NotificationDeliveryAttemptList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	result, err := services.NotificationTemplateService.ListAttempts(services.NotificationDeliveryAttemptListQuery{
		TenantID: tenantID,
		Channel:  ctx.Query("channel"),
		Status:   ctx.Query("status"),
		Page:     queryPositiveInt(ctx, "page", 1),
		PageSize: queryPositiveInt(ctx, "page_size", 20),
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func resolveNotificationTemplateTarget(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, false
	}
	templateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || templateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid notification template id"))
		return 0, 0, false
	}
	return tenantID, templateID, true
}

func notificationPrincipalUserID(principal *dto.AuthPrincipal) int64 {
	if principal == nil {
		return 0
	}
	return principal.UserID
}
