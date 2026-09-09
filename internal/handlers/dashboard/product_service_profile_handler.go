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

func ProductServiceProfileAnyList(ctx *gin.Context) {
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "safetyLevel"},
	).Where("status <> ?", enums.StatusDeleted).Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 {
		cnd.Eq("tenant_id", principal.TenantID)
	}

	list, paging := services.ProductServiceProfileService.FindPageByCnd(cnd)
	results := builders.BuildProductServiceProfileList(list)
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func ProductServiceProfileGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.ProductServiceProfileService.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		httpx.WriteJSON(ctx, nil)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductServiceProfile(item))
}

func ProductServiceProfilePostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateProductServiceProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductServiceProfileService.CreateProductServiceProfile(req, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildProductServiceProfile(item))
}

func ProductServiceProfilePostUpdate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductServiceProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductServiceProfileService.UpdateProductServiceProfile(req, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductServiceProfilePostDelete(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteProductServiceProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductServiceProfileService.DeleteProductServiceProfile(req.ID, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductServiceProfilePostUpdate_status(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionProductServiceProfileUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateProductServiceProfileStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductServiceProfileService.UpdateStatus(req.ID, req.Status, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
