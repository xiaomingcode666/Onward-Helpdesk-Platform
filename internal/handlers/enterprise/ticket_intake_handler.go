package enterprise

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
	"strconv"
)

func TicketIntakePolicyGet(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	if raw := ctx.Query("ticket_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("工单编号无效"))
			return
		}
		policy, err := services.GetTicketBoundIntakePolicy(id, operator)
		if err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		httpx.WriteJSON(ctx, policy)
		return
	}
	policy, err := services.GetTicketIntakePolicy(tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, policy)
}

func TicketIntakePolicyUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var policy dto.TicketIntakePolicy
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.UpdateTicketIntakePolicy(tenantID, policy, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, 0, "ticket.intake_policy.updated", models.RiskLevelMedium, map[string]any{"policy": policy})
	httpx.WriteJSON(ctx, policy)
}

func TicketIntakeComplete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	if _, err := services.EnterpriseTicketService.GetAggregateForOperator(tenantID, ticketID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var input dto.CompleteTicketIntakeRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.CompleteTicketIntake(ticketID, input, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	writeEnterpriseTicketAggregate(ctx, tenantID, ticketID)
}
