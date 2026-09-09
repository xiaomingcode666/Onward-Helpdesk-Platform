package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func AIAgentGetCapabilities(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	aggregate := services.EnterpriseAICapabilityService.GetCapabilities(ctx.Request.Context(), operator.TenantID)
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAICapabilities(aggregate))
}

func AIAgentAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "name", Op: params.Like},
		params.QueryFilter{ParamName: "code", Op: params.Like},
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "productScoped", ColumnName: "product_id", Op: params.Gt},
		params.QueryFilter{ParamName: "reviewStatus", ColumnName: "review_status"},
	).Asc("sort_no").Desc("id")
	if operator.TenantID > 0 {
		cnd.Eq("tenant_id", operator.TenantID)
	}
	list, paging := services.AIAgentService.FindPageByCnd(cnd)
	results := make([]response.AIAgentResponse, 0, len(list))
	for _, item := range list {
		results = append(results, builders.BuildAIAgentWithLocale(&item, i18nx.Locale(ctx)))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func AIAgentGetList_all(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := sqls.NewCnd().Where("status = ?", enums.StatusOk).Desc("sort_no").Desc("id")
	if operator.TenantID > 0 {
		cnd.Eq("tenant_id", operator.TenantID)
	}
	list := services.AIAgentService.Find(cnd)
	results := make([]response.AIAgentResponse, 0, len(list))
	for _, item := range list {
		results = append(results, builders.BuildAIAgentWithLocale(&item, i18nx.Locale(ctx)))
	}
	httpx.WriteJSON(ctx, results)
}

func AIAgentGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AIAgentService.GetForOperator(id, operator)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIAgentWithLocale(item, i18nx.Locale(ctx)))
}

func AIAgentPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateAIAgentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIAgentService.CreateAIAgent(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIAgentWithLocale(item, i18nx.Locale(ctx)))
}

func AIAgentPostUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAIAgentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.UpdateAIAgent(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIAgentPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteAIAgentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.DeleteAIAgent(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIAgentPostUpdate_sort(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var ids []int64
	if err := params.ReadJSON(ctx, &ids); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.UpdateSort(ids, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIAgentPostUpdate_status(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAIAgentStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.UpdateStatus(req.ID, req.Status, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
