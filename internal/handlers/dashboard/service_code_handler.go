package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func ServiceCodeAnyList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionServiceCodeView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "tenantId"},
		params.QueryFilter{ParamName: "batchId"},
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "productModelId"},
		params.QueryFilter{ParamName: "deviceId"},
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "mode"},
		params.QueryFilter{ParamName: "serviceCode", Op: params.Like},
	).Desc("id")

	list, paging := services.ServiceCodeManagementService.FindPageByCnd(cnd)
	results := builders.BuildServiceCodeList(list)
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func ServiceCodeGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionServiceCodeView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.ServiceCodeManagementService.Get(id)
	if item == nil {
		httpx.WriteJSON(ctx, nil)
		return
	}
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if principal.TenantID > 0 && item.TenantID != principal.TenantID {
		httpx.WriteJSON(ctx, nil)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildServiceCode(item))
}

func ServiceCodePostGenerate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionServiceCodeGenerate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.GenerateServiceCodesRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	list, err := services.ServiceCodeManagementService.GenerateCodes(req.BatchID, req.Count, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildServiceCodeList(list))
}

func ServiceCodePostRevoke(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionServiceCodeRevoke)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.RevokeServiceCodeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ServiceCodeManagementService.RevokeCode(req.ID, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
