package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func KnowledgeRetrieveLogAnyList(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "tenantId"},
		params.QueryFilter{ParamName: "knowledgeBaseId"},
		params.QueryFilter{ParamName: "conversationId"},
		params.QueryFilter{ParamName: "indexGenerationId"},
		params.QueryFilter{ParamName: "question", Op: params.Like},
		params.QueryFilter{ParamName: "channel"},
		params.QueryFilter{ParamName: "scene"},
		params.QueryFilter{ParamName: "chunkProvider"},
		params.QueryFilter{ParamName: "sessionId"},
		params.QueryFilter{ParamName: "requestId"},
		params.QueryFilter{ParamName: "collectionName"},
		params.QueryFilter{ParamName: "embeddingModel"},
		params.QueryFilter{ParamName: "noAnswerReason"},
	).Desc("id")
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}

	if answerStatus, ok := params.GetInt64(ctx, "answerStatus"); ok && answerStatus > 0 {
		cnd.Where("answer_status = ?", answerStatus)
	}
	if rerankEnabled, ok := params.GetInt64(ctx, "rerankEnabled"); ok {
		cnd.Where("rerank_enabled = ?", rerankEnabled > 0)
	}

	queryParams := params.NewQueryParams(ctx)
	queryParams.Cnd = *cnd
	list, paging := services.KnowledgeRetrieveLogService.FindPageByParams(queryParams)
	results := make([]response.KnowledgeRetrieveLogResponse, 0, len(list))
	for _, item := range list {
		resp := builders.BuildKnowledgeRetrieveLog(&item)
		if knowledgeBase := services.KnowledgeBaseService.Get(item.KnowledgeBaseID); knowledgeBase != nil {
			resp.KnowledgeBaseName = knowledgeBase.Name
		}
		results = append(results, resp)
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func KnowledgeRetrieveLogGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	logItem := services.KnowledgeRetrieveLogService.Get(id)
	if logItem == nil || (principal.TenantID > 0 && logItem.TenantID != principal.TenantID) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0241"))
		return
	}

	logResp := builders.BuildKnowledgeRetrieveLog(logItem)
	if knowledgeBase := services.KnowledgeBaseService.Get(logItem.KnowledgeBaseID); knowledgeBase != nil {
		logResp.KnowledgeBaseName = knowledgeBase.Name
	}

	hits := services.KnowledgeRetrieveLogService.FindHitsByRetrieveLogID(id)
	hitResults := make([]response.KnowledgeRetrieveHitResponse, 0, len(hits))
	for _, item := range hits {
		hitResults = append(hitResults, builders.BuildKnowledgeRetrieveHitResponse(&item))
	}

	httpx.WriteJSON(ctx, response.KnowledgeRetrieveLogDetailResponse{
		Log:  logResp,
		Hits: hitResults,
	})
}
