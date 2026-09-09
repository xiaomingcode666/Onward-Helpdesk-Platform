package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func ProductKnowledgeBindingAnyList(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx, params.QueryFilter{ParamName: "productId"}, params.QueryFilter{ParamName: "productModelId"}, params.QueryFilter{ParamName: "knowledgeBaseId"}, params.QueryFilter{ParamName: "scopeType"}, params.QueryFilter{ParamName: "status"}).Where("status <> ?", enums.StatusDeleted).Desc("sort_no").Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}
	list, paging := services.ProductKnowledgeBindingService.FindPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{Results: builders.BuildProductKnowledgeBindingList(list), Page: paging})
}

func ProductKnowledgeBindingGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.ProductKnowledgeBindingService.Get(id)
	if item == nil || item.Status == enums.StatusDeleted || (principal.TenantID > 0 && item.TenantID != principal.TenantID) {
		httpx.WriteJSON(ctx, nil)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductKnowledgeBinding(item))
}

func ProductKnowledgeBindingPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateProductKnowledgeBindingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductKnowledgeBindingService.Create(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductKnowledgeBinding(item))
}

func ProductKnowledgeBindingPostUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductKnowledgeBindingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductKnowledgeBindingService.Update(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductKnowledgeBindingPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteProductKnowledgeBindingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductKnowledgeBindingService.Delete(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductKnowledgeBindingPostUpdate_status(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductKnowledgeBindingStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductKnowledgeBindingService.UpdateStatus(req.ID, req.Status, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductKnowledgeBindingPostResolve(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductKnowledgeBindingResolve)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.ResolveProductKnowledgeBindingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if principal.TenantID > 0 {
		req.TenantID = principal.TenantID
	}
	list, err := services.ProductKnowledgeBindingService.Resolve(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildResolvedKnowledgeBaseList(list))
}
