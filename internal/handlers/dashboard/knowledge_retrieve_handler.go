package dashboard

import (
	"remotehelpdesk/internal/pkg/httpx"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
)

func KnowledgeRetrievePostDebugSearch(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.KnowledgeSearchRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	resp, err := services.KnowledgeDebugService.DebugSearch(ctx.Request.Context(), req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeRetrievePostDebugAnswer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.KnowledgeAnswerRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	resp, err := services.KnowledgeDebugService.DebugAnswer(ctx.Request.Context(), req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeRetrievePostBuild(ctx *gin.Context) {
	req := struct {
		DocumentID int64 `json:"documentId"`
		FAQID      int64 `json:"faqId"`
	}{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if req.DocumentID > 0 {
		operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentUpdate)
		if err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		document := services.KnowledgeDocumentService.Get(req.DocumentID)
		if document == nil || document.TenantID != operator.EffectiveTenantID() {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0218"))
			return
		}
		if _, err := services.KnowledgeIndexSyncService.RequestEntryReindex(ctx.Request.Context(), operator.EffectiveTenantID(), "document", req.DocumentID, operator); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		httpx.WriteJSON(ctx, nil)
		return
	}

	if req.FAQID > 0 {
		operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeFAQUpdate)
		if err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		faq := services.KnowledgeFAQService.Get(req.FAQID)
		if faq == nil || faq.TenantID != operator.EffectiveTenantID() {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0025"))
			return
		}
		if _, err := services.KnowledgeIndexSyncService.RequestEntryReindex(ctx.Request.Context(), operator.EffectiveTenantID(), "faq", req.FAQID, operator); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		httpx.WriteJSON(ctx, nil)
		return
	}

	httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0066"))
}
