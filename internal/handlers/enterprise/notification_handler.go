package enterprise

import (
	"fmt"
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func NotificationList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "50"))
	category := ctx.Query("category")
	if category == "" {
		category = ctx.Query("type") // 兼容旧参数名
	}
	result, err := services.EnterpriseNotificationService.ListForOperator(tenantID, principal, services.EnterpriseNotificationQuery{
		ReadStatus: ctx.Query("read_status"),
		Category:   category,
		Search:     ctx.Query("search"),
		Scope:      ctx.Query("scope"),
		Page:       queryPositiveInt(ctx, "page", 1),
		PageSize:   queryPositiveInt(ctx, "page_size", limit),
		Limit:      limit,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func NotificationMarkRead(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	notificationID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || notificationID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid notification id"))
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	userID := int64(0)
	if principal != nil {
		userID = principal.UserID
	}
	if err := services.EnterpriseNotificationService.MarkRead(tenantID, userID, notificationID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.read", "notification", strconv.FormatInt(notificationID, 10), map[string]any{
		"notificationId": notificationID,
	})
	httpx.WriteJSON(ctx, nil)
}

func NotificationMarkAllRead(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	userID := int64(0)
	if principal != nil {
		userID = principal.UserID
	}
	if err := services.EnterpriseNotificationService.MarkAllRead(tenantID, userID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.read_all", "notification", "all", map[string]any{
		"recipientUserId": userID,
	})
	httpx.WriteJSON(ctx, nil)
}

func NotificationRecipientSettingGet(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	setting, err := services.NotificationRecipientSettingService.Get(tenantID, principal.UserID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, setting)
}

func NotificationRecipientSettingSave(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	var req request.UpdateNotificationRecipientSettingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	setting, err := services.NotificationRecipientSettingService.Save(tenantID, principal.UserID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.preferences.updated", "notification_recipient_setting", fmt.Sprint(principal.UserID), setting)
	httpx.WriteJSON(ctx, setting)
}

func NotificationRecipientSettingTest(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	to := services.NotificationRecipientSettingService.ResolveEmail(tenantID, principal.UserID)
	if to == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("recipient email is unavailable or disabled"))
		return
	}
	if err := services.MailSettingService.SendTest(tenantID, to); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.recipient_test.sent", "notification_recipient_setting", fmt.Sprint(principal.UserID), map[string]any{"recipient": to})
	httpx.WriteJSON(ctx, nil)
}

func MobilePushTokenRegister(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil || principal.UserID <= 0 {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	var req request.RegisterMobilePushTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	item, err := services.MobilePushTokenService.Register(tenantID, principal.UserID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.push_token.registered", "mobile_push_token", fmt.Sprint(item.ID), map[string]any{
		"platform": req.Platform,
		"deviceId": req.DeviceID,
	})
	httpx.WriteJSON(ctx, item)
}

func MobilePushTokenRevoke(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil || principal.UserID <= 0 {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	tokenID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || tokenID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid mobile push token id"))
		return
	}
	if err := services.MobilePushTokenService.Revoke(tenantID, principal.UserID, tokenID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, principal, tenantID, "notification.push_token.revoked", "mobile_push_token", fmt.Sprint(tokenID), nil)
	httpx.WriteJSON(ctx, nil)
}

func NotificationMailSettingGet(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	setting, err := services.MailSettingService.Get(tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, setting)
}

func NotificationMailSettingSave(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.UpdateMailSettingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	setting, err := services.MailSettingService.Save(tenantID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordNotificationAudit(ctx, services.AuthService.GetAuthPrincipal(ctx), tenantID, "notification.mail_settings.updated", "tenant_mail_setting", fmt.Sprint(tenantID), setting)
	httpx.WriteJSON(ctx, setting)
}

func NotificationMailSettingTest(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.TestMailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	if err := services.MailSettingService.SendTest(tenantID, req.To); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	recordNotificationAudit(ctx, services.AuthService.GetAuthPrincipal(ctx), tenantID, "notification.test_email.sent", "tenant_mail_setting", fmt.Sprint(tenantID), map[string]any{
		"recipient": strings.TrimSpace(req.To),
	})
	httpx.WriteJSON(ctx, nil)
}

func recordNotificationAudit(ctx *gin.Context, operator *dto.AuthPrincipal, tenantID int64, action, resourceType, resourceID string, afterState any) {
	if ctx == nil || operator == nil || tenantID <= 0 {
		return
	}
	actorType := strings.TrimSpace(operator.SubjectType)
	if actorType == "" {
		actorType = "user"
	}
	riskLevel := models.RiskLevelLow
	if action == "notification.mail_settings.updated" {
		riskLevel = models.RiskLevelMedium
	}
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       tenantID,
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      actorType,
		Domain:         "notification",
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Action:         action,
		AfterState:     afterState,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
		RiskLevel:      riskLevel,
	})
}
