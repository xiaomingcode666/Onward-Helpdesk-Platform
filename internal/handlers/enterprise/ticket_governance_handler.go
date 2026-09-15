package enterprise

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"net/http"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

func TicketClassificationPolicy(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	p, err := services.TicketPriorityPolicyForTenant(sqls.DB(), op.EffectiveTenantID())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, p)
}

func TicketGovernance(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, id, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	if ctx.Request.Method == http.MethodGet {
		if priority := ctx.Query("preview_priority"); priority != "" {
			value, err := services.PreviewTicketPriorityChange(id, priority, op)
			if err != nil {
				httpx.WriteJSON(ctx, err)
			} else {
				httpx.WriteJSON(ctx, value)
			}
			return
		}
		if ctx.Query("merge_candidates") == "1" {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("重复问题已改为父子工单，请刷新页面"))
			return
		}
		if ctx.Query("duplicate_candidates") == "1" {
			value, err := services.SearchTicketDuplicateCandidates(id, ctx.Query("search"), op)
			if err != nil {
				httpx.WriteJSON(ctx, err)
			} else {
				httpx.WriteJSON(ctx, value)
			}
			return
		}
		if ctx.Query("candidates") == "1" {
			value, err := services.SearchTicketRelationCandidates(id, ctx.Query("search"), op)
			if err != nil {
				httpx.WriteJSON(ctx, err)
			} else {
				httpx.WriteJSON(ctx, value)
			}
			return
		}
		value, err := services.GetTicketGovernance(id, op)
		if err != nil {
			httpx.WriteJSON(ctx, err)
		} else {
			httpx.WriteJSON(ctx, value)
		}
		return
	}
	var cmd services.TicketGovernanceCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("分类、等级或关联参数无效"))
		return
	}
	value, err := services.ExecuteTicketGovernance(id, cmd, op)
	if errors.Is(err, services.ErrTicketCaseConflict) {
		httpx.WriteHttpStatusJSON(ctx, http.StatusConflict, errorsx.InvalidParam(err.Error()))
		return
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
	} else {
		httpx.WriteJSON(ctx, value)
	}
}
