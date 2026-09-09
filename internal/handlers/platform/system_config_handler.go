package platform

import (
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// GetPlatformSystemPolicy 返回平台系统级默认策略。
func GetPlatformSystemPolicy(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.PlatformSystemConfigService.GetSettings(services.AuthService.GetAuthPrincipal(ctx)))
}

// PostPlatformSystemPolicy 更新平台系统级默认策略。
func PostPlatformSystemPolicy(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTenantUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.PlatformSystemPolicyUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.PlatformSystemConfigService.UpdateSettings(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}
