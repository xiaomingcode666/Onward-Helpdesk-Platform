package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func KnowledgeDocumentAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "knowledgeBaseId"},
		params.QueryFilter{ParamName: "title", Op: params.Like},
	).Desc("id")
	cnd.Eq("tenant_id", operator.EffectiveTenantID())
	knowledgeBaseID, _ := params.GetInt64(ctx, "knowledgeBaseId")
	if directoryID, ok := params.GetInt64(ctx, "directoryId"); ok {
		cnd.Where("directory_id = ?", directoryID)
	}

	if status, ok := params.GetInt64(ctx, "status"); ok {
		cnd.Where("status = ?", status)
	} else {
		cnd.Where("status != ?", enums.StatusDeleted)
	}
	if indexStatus, ok := params.Get(ctx, "indexStatus"); ok {
		if !enums.IsValidKnowledgeDocumentIndexStatus(indexStatus) {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0067"))
			return
		}
		cnd.Where("index_status = ?", indexStatus)
	}

	list, paging := services.KnowledgeDocumentService.FindPageListByCnd(cnd)
	directoryPaths := services.KnowledgeDirectoryService.PathMap(knowledgeBaseID)
	results := make([]response.KnowledgeDocumentListResponse, 0, len(list))
	for _, item := range list {
		resp := builders.BuildKnowledgeDocumentList(&item)
		fillKnowledgeDocumentDirectory(&resp, directoryPaths)
		results = append(results, resp)
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func KnowledgeDocumentGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	item := services.KnowledgeDocumentService.Get(id)
	if item == nil || item.TenantID != operator.EffectiveTenantID() {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0218"))
		return
	}
	resp := builders.BuildKnowledgeDocument(item)
	fillKnowledgeDocumentDirectory(&resp, services.KnowledgeDirectoryService.PathMap(item.KnowledgeBaseID))
	httpx.WriteJSON(ctx, resp)
}

func fillKnowledgeDocumentDirectory(resp any, directoryPaths map[int64]string) {
	switch item := resp.(type) {
	case *response.KnowledgeDocumentResponse:
		item.DirectoryPath = directoryPaths[item.DirectoryID]
		item.DirectoryName = item.DirectoryPath
	case *response.KnowledgeDocumentListResponse:
		item.DirectoryPath = directoryPaths[item.DirectoryID]
		item.DirectoryName = item.DirectoryPath
	}
}

func KnowledgeDocumentPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.CreateKnowledgeDocumentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.KnowledgeDocumentService.CreateKnowledgeDocument(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildKnowledgeDocument(item))
}

func KnowledgeDocumentPostUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.UpdateKnowledgeDocumentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeDocumentService.UpdateKnowledgeDocument(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeDocumentPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentDelete)
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

	if err := services.KnowledgeDocumentService.DeleteKnowledgeDocumentForOperator(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeDocumentPostBatch_move(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.BatchMoveKnowledgeDocumentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeDocumentService.BatchMoveKnowledgeDocuments(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func KnowledgeDocumentPostBatch_delete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.BatchDeleteKnowledgeDocumentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.KnowledgeDocumentService.BatchDeleteKnowledgeDocumentsForOperator(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
