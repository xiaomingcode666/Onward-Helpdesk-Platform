package dashboard

import (
	"remotehelpdesk/internal/pkg/httpx"
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func NotificationAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "type", ColumnName: "notification_type"},
	).Eq("recipient_user_id", operator.UserID).
		Eq("status", enums.StatusOk).
		Desc("id")
	if tenantID := operator.EffectiveTenantID(); tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}

	switch strings.TrimSpace(ctx.Query("readStatus")) {
	case "unread":
		cnd.Where("read_at IS NULL")
	case "read":
		cnd.Where("read_at IS NOT NULL")
	}

	list, paging := services.NotificationService.FindPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{
		Results: builders.BuildNotificationListWithLocale(list, i18nx.Locale(ctx)),
		Page:    paging,
	})
}

func NotificationGetUnread_count(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &response.NotificationUnreadCountResponse{
		UnreadCount: services.NotificationService.CountUnreadForTenant(operator.EffectiveTenantID(), operator.UserID),
	})
}

func NotificationPostMark_read(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.MarkNotificationReadRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.NotificationService.MarkReadForTenant(req.ID, operator.EffectiveTenantID(), operator.UserID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func NotificationPostMark_all_read(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.NotificationService.MarkAllReadForTenant(operator.EffectiveTenantID(), operator.UserID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
