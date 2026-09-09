package dashboard

import (
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// TicketPostAccept - POST /api/dashboard/ticket/:id/accept
func TicketPostAccept(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req request.AcceptTicketRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if err := services.TicketLifecycleService.Accept(id, req.AssigneeID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

// TicketPostTakeover - POST /api/dashboard/tickets/:id/takeover
func TicketPostTakeover(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.TicketLifecycleService.TakeoverWithAudit(id, operator, services.RecordAuditInput{
		TenantID:       operator.EffectiveTenantID(),
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
	}); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID <= 0 {
		if ticket := services.TicketService.Get(id); ticket != nil {
			tenantID = ticket.TenantID
		}
	}
	result, err := services.EnterpriseTicketService.GetAggregateForOperator(tenantID, id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// TicketPostTransfer - POST /api/dashboard/ticket/:id/transfer
func TicketPostTransfer(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketAssign)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		ToUserID int64  `json:"toUserId"`
		Reason   string `json:"reason"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if err := services.TicketLifecycleService.Transfer(id, req.ToUserID, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

// TicketPostEscalate - POST /api/dashboard/ticket/:id/escalate
func TicketPostEscalate(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if err := services.TicketLifecycleService.Escalate(id, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

// TicketPostClose - POST /api/dashboard/ticket/:id/close (uses TicketLifecycleService.Close which creates repair records)
func TicketPostClose(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if err := services.TicketLifecycleService.Close(id, req.Resolution, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

// TicketPostRepair - POST /api/dashboard/ticket/:id/repair
func TicketPostRepair(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req request.CreateTicketRepairRecordRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req.TicketID = id
	record, err := services.TicketService.CreateRepairRecord(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, record)
}

// TicketPostReopen - POST /api/dashboard/ticket/:id/reopen
func TicketPostReopen(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if err := services.TicketLifecycleService.Reopen(id, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
