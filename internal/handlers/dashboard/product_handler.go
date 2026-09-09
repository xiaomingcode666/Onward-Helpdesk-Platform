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
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func ProductAnyList(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "productLineId"},
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "code", Op: params.Like},
		params.QueryFilter{ParamName: "name", Op: params.Like},
		params.QueryFilter{ParamName: "category"},
	).Where("status <> ?", enums.StatusDeleted).Desc("id")

	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}

	list, paging := services.ProductService.FindPageByCnd(cnd)
	results := builders.BuildProductList(list)
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func ProductGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.ProductService.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		httpx.WriteJSON(ctx, nil)
		return
	}
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 && item.TenantID != principal.TenantID {
		httpx.WriteJSON(ctx, nil)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProduct(item))
}

func ProductGetList_all(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := sqls.NewCnd().Eq("status", enums.StatusOk).Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}
	list := services.ProductService.Find(cnd)
	httpx.WriteJSON(ctx, builders.BuildProductList(list))
}

func ProductPostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateProductRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductService.CreateProduct(req, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProduct(item))
}

func ProductPostUpdate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductService.UpdateProduct(req, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductPostDelete(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteProductRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductService.DeleteProduct(req.ID, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductPostUpdate_status(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductService.UpdateStatus(req.ID, req.Status, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
