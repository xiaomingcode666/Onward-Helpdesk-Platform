package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func TicketSupplierCollaborationList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.List(tenantID, ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildTicketSupplierCollaborations(items))
}

func TicketSupplierOptionList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListAvailablePartners(ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildTicketSupplierOptions(items))
}

func TicketSupplierCollaborationInvite(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req dto.TicketSupplierInviteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.Invite(ticketID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	after := map[string]any{
		"productModuleId":  req.ProductModuleID,
		"partnerCompanyId": req.PartnerCompanyID,
	}
	if item != nil && item.Collaboration != nil {
		after["collaborationId"] = item.Collaboration.ID
		after["partnerCompanyId"] = item.Collaboration.PartnerCompanyID
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.supplier_collaboration.invited", models.RiskLevelMedium, after)
	httpx.WriteJSON(ctx, builders.BuildTicketSupplierCollaboration(*item))
}

func TicketSupplierCollaborationDetail(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	id, ok := supplierCollaborationID(ctx)
	if !ok {
		return
	}
	item, err := services.TicketSupplierCollaborationService.EnterpriseTicketDetail(ticketID, id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketSupplierCollaborationProgress(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	id, ok := supplierCollaborationID(ctx)
	if !ok {
		return
	}
	var req dto.PartnerTicketProgressCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.AddEnterpriseProgress(ticketID, id, req.Content, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.supplier_collaboration.progress.created", models.RiskLevelLow, map[string]any{
		"collaborationId": id,
	})
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketSupplierCollaborationResolve(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	id, ok := supplierCollaborationID(ctx)
	if !ok {
		return
	}
	var req dto.TicketSupplierResolveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.ResolveForTicket(ticketID, id, req.Resolution, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.supplier_collaboration.resolved", models.RiskLevelMedium, map[string]any{
		"collaborationId": id,
	})
	httpx.WriteJSON(ctx, builders.BuildTicketSupplierCollaboration(*item))
}

func supplierCollaborationID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("collaborationId"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid supplier collaboration id"))
		return 0, false
	}
	return id, true
}
