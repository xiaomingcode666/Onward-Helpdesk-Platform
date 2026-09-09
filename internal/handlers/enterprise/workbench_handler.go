package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func WorkbenchGetQueue(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	userID := int64(0)
	if principal := services.AuthService.GetAuthPrincipal(ctx); principal != nil {
		userID = principal.UserID
	}
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(firstNonEmptyQuery(ctx, "page_size", "pageSize"))
	aggregate, err := services.EnterpriseWorkbenchService.Queue(tenantID, userID, services.EnterpriseWorkbenchQueueQuery{
		Page:     page,
		PageSize: pageSize,
		Limit:    limit,
		QueueKey: firstNonEmptyQuery(ctx, "queue_key", "queueKey", "queue"),
	}, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, &dto.EnterpriseWorkbenchQueueDTO{
		Conversations:           builders.BuildEnterpriseWorkbenchConversations(aggregate.Conversations),
		ConversationsPagination: aggregate.ConversationPagination,
		Tickets:                 aggregate.Tickets,
		TicketsPagination:       aggregate.TicketPagination,
	})
}

func firstNonEmptyQuery(ctx *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := ctx.Query(key); value != "" {
			return value
		}
	}
	return ""
}

func WorkbenchGetOverview(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	userID := int64(0)
	if principal := services.AuthService.GetAuthPrincipal(ctx); principal != nil {
		userID = principal.UserID
	}
	result, err := services.EnterpriseWorkbenchService.Overview(ctx.Request.Context(), tenantID, userID, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func WorkbenchGetCore(ctx *gin.Context) {
	tenantID, userID, principal, ok := resolveWorkbenchIdentity(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseWorkbenchService.Core(tenantID, userID, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func WorkbenchGetCollaboration(ctx *gin.Context) {
	tenantID, userID, principal, ok := resolveWorkbenchIdentity(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseWorkbenchService.Collaboration(tenantID, userID, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func WorkbenchGetNotifications(ctx *gin.Context) {
	tenantID, userID, _, ok := resolveWorkbenchIdentity(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseWorkbenchService.Notifications(tenantID, userID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func WorkbenchGetResources(ctx *gin.Context) {
	tenantID, userID, principal, ok := resolveWorkbenchIdentity(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseWorkbenchService.Resources(tenantID, userID, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func resolveWorkbenchIdentity(ctx *gin.Context) (int64, int64, *dto.AuthPrincipal, bool) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView); err != nil {
		httpx.WriteJSON(ctx, err)
		return 0, 0, nil, false
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, nil, false
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	userID := int64(0)
	if principal != nil {
		userID = principal.UserID
	}
	return tenantID, userID, principal, true
}
