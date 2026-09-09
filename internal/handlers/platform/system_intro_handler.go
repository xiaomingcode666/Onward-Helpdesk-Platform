package platform

import (
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// 平台系统介绍文档 Handler（系统配置 → 系统介绍）。
// 分层约束：Handler 只负责参数解析、鉴权、调用 Service 和 httpx.WriteJSON。

// GetPlatformSystemIntroList 系统介绍文档分页列表
// GET /api/platform/system-intro/list
func GetPlatformSystemIntroList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionSystemIntroView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	page := queryInt(ctx, "page", 1)
	limit := queryInt(ctx, "limit", 20)
	items, total, err := services.SystemIntroService.ListPage(ctx.Query("keyword"), page, limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, response.PlatformSystemIntroListResponse{Items: items, Total: total})
}

// PostPlatformSystemIntroCreate 上传系统介绍文档（multipart：file + title + sort_no）
// POST /api/platform/system-intro/create
func PostPlatformSystemIntroCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionSystemIntroCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0323"))
		return
	}
	title := ctx.PostForm("title")
	sortNo, _ := params.GetInt(ctx, "sort_no")
	item, err := services.SystemIntroService.UploadDoc(header, title, sortNo, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// PostPlatformSystemIntroUpdate 编辑系统介绍文档（标题/排序/上下架）
// POST /api/platform/system-intro/update
func PostPlatformSystemIntroUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionSystemIntroUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformSystemIntroUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.ID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("id is required"))
		return
	}
	item, err := services.SystemIntroService.UpdateDoc(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// PostPlatformSystemIntroDelete 删除系统介绍文档
// POST /api/platform/system-intro/delete
func PostPlatformSystemIntroDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionSystemIntroDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformSystemIntroDeleteRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.ID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("id is required"))
		return
	}
	if err := services.SystemIntroService.DeleteDoc(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
