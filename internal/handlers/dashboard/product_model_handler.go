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

func ProductModelAnyList(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "modelCode", Op: params.Like},
		params.QueryFilter{ParamName: "name", Op: params.Like},
	).Where("status <> ?", enums.StatusDeleted).Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}

	list, paging := services.ProductModelService.FindPageByCnd(cnd)
	results := builders.BuildProductModelList(list)
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func ProductModelGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.ProductModelService.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		httpx.WriteJSON(ctx, nil)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductModel(item))
}

func ProductModelGetList_all(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := sqls.NewCnd().Eq("status", enums.StatusOk).Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}
	if productID, ok := params.GetInt64(ctx, "productId"); ok {
		cnd.Eq("product_id", productID)
	}
	list := services.ProductModelService.Find(cnd)
	httpx.WriteJSON(ctx, builders.BuildProductModelList(list))
}

func ProductModelPostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateProductModelRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductModelService.CreateProductModel(req, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductModel(item))
}

func ProductModelPostUpdate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductModelRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductModelService.UpdateProductModel(req, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductModelPostDelete(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteProductModelRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductModelService.DeleteProductModel(req.ID, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductModelPostUpdate_status(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductModelUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductModelStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductModelService.UpdateStatus(req.ID, req.Status, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
