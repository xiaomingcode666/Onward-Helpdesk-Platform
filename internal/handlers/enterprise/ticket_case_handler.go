package enterprise

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

func TicketCaseTransition(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var command services.TicketCaseCommand
	if err = ctx.ShouldBindJSON(&command); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("工单操作参数无效"))
		return
	}
	if err = services.ExecuteTicketCaseCommand(ticketID, command, operator); err != nil {
		if errors.Is(err, services.ErrTicketCaseConflict) {
			httpx.WriteHttpStatusJSON(ctx, http.StatusConflict, errorsx.InvalidParam(err.Error()))
			return
		}
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.GetTicketCaseCommandResult(tenantID, ticketID, command.IdempotencyKey)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketCaseOwnerTransfer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		OwnerID         int64  `json:"owner_id"`
		ExpectedOwnerID *int64 `json:"expected_owner_id"`
		Reason          string `json:"reason"`
		IdempotencyKey  string `json:"idempotency_key"`
	}
	if err = ctx.ShouldBindJSON(&req); err != nil || req.ExpectedOwnerID == nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("请选择接替客服并提供当前负责人"))
		return
	}
	if err = services.TicketCaseOwnerService.TransferWithKey(ticketID, *req.ExpectedOwnerID, req.OwnerID, req.Reason, req.IdempotencyKey, operator); err != nil {
		if errors.Is(err, services.ErrTicketCaseConflict) {
			httpx.WriteHttpStatusJSON(ctx, http.StatusConflict, errorsx.InvalidParam(err.Error()))
			return
		}
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.GetTicketCaseCommandResult(tenantID, ticketID, req.IdempotencyKey)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketCaseOwnerOptions(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	if _, err = services.EnterpriseTicketService.GetAggregateForOperator(tenantID, ticketID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	items, err := services.TicketCaseOwnerService.ListCandidates(tenantID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	options := make([]map[string]any, 0, len(items))
	for _, item := range items {
		options = append(options, map[string]any{"id": item.UserID, "name": item.DisplayName})
	}
	httpx.WriteJSON(ctx, options)
}
