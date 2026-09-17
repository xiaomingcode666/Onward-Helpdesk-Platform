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
