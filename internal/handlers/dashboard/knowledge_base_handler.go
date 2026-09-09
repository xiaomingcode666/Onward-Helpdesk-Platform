package dashboard

import (
	"remotehelpdesk/internal/pkg/httpx"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func KnowledgeBaseAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "name", Op: params.Like},
	).Asc("sort_no").Desc("id")
	cnd.Eq("tenant_id", operator.EffectiveTenantID())
	list, paging := services.KnowledgeBaseService.FindPageByCnd(cnd)
	results := make([]response.KnowledgeBaseResponse, 0, len(list))
	for _, item := range list {
		docCount := repositories.KnowledgeDocumentRepository.CountByKnowledgeBaseID(sqls.DB(), item.ID)
		faqCount := repositories.KnowledgeFAQRepository.CountByKnowledgeBaseID(sqls.DB(), item.ID)
		resp := builders.BuildKnowledgeBase(&item)
		resp.DocumentCount = docCount
		resp.FAQCount = faqCount
		results = append(results, resp)
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func KnowledgeBaseAnyList_all(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	list := services.KnowledgeBaseService.Find(params.NewSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
	).Eq("tenant_id", operator.EffectiveTenantID()).Asc("sort_no").Desc("id"))
	results := make([]response.KnowledgeBaseResponse, 0, len(list))
	for _, item := range list {
		resp := builders.BuildKnowledgeBase(&item)
		results = append(results, resp)
	}
	httpx.WriteJSON(ctx, results)
}

func KnowledgeBaseGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	item := services.KnowledgeBaseService.GetForOperator(id, operator)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0283"))
		return
	}
	resp := builders.BuildKnowledgeBase(item)
	resp.DocumentCount = repositories.KnowledgeDocumentRepository.CountByKnowledgeBaseID(sqls.DB(), item.ID)
	resp.FAQCount = repositories.KnowledgeFAQRepository.CountByKnowledgeBaseID(sqls.DB(), item.ID)
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeBasePostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.CreateKnowledgeBaseRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.KnowledgeBaseService.CreateKnowledgeBase(req, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildKnowledgeBase(item))
}

func KnowledgeBasePostUpdate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.UpdateKnowledgeBaseRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeBaseService.UpdateKnowledgeBase(req, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeBasePostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeBaseService.DeleteKnowledgeBaseForOperator(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeBasePostUpdate_sort(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var ids []int64
	if err := params.ReadJSON(ctx, &ids); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeBaseService.UpdateSortForOperator(ids, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeBasePostRebuild_index(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	knowledgeBase := services.KnowledgeBaseService.GetForOperator(req.ID, operator)
	if knowledgeBase == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0283"))
		return
	}

	if _, err := services.KnowledgeIndexSyncService.RequestKnowledgeBaseReindex(ctx.Request.Context(), operator.EffectiveTenantID(), req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, nil)
}
