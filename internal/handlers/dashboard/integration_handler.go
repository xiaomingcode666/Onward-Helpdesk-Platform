package dashboard

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"
)

func TenantIntegrationConfigList(c *gin.Context) {
	if _, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigView); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	q := params.NewPagedSqlCnd(c, params.QueryFilter{ParamName: "tenantId"}, params.QueryFilter{ParamName: "provider"}, params.QueryFilter{ParamName: "status"}).Where("status <> ?", enums.StatusDeleted).Desc("id")
	list, page := services.TenantIntegrationConfigService.FindPage(q)
	httpx.WriteJSON(c, &web.PageResult{Results: builders.BuildTenantIntegrationConfigList(list), Page: page})
}
func TenantIntegrationConfigGetBy(c *gin.Context) {
	id, ok := httpx.GetPathInt64(c, "id")
	if !ok {
		return
	}
	if _, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigView); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	v := services.TenantIntegrationConfigService.Get(id)
	if v == nil || v.Status == enums.StatusDeleted {
		httpx.WriteJSON(c, nil)
		return
	}
	httpx.WriteJSON(c, builders.BuildTenantIntegrationConfig(v))
}
func TenantIntegrationConfigPostCreate(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigCreate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.CreateTenantIntegrationConfigRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	v, e := services.TenantIntegrationConfigService.Create(req, op)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, builders.BuildTenantIntegrationConfig(v))
}
func TenantIntegrationConfigPostUpdate(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigUpdate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.UpdateTenantIntegrationConfigRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.TenantIntegrationConfigService.Update(req, op))
}
func TenantIntegrationConfigPostDelete(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigDelete)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.DeleteIntegrationRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.TenantIntegrationConfigService.Delete(req.ID, op))
}
func TenantIntegrationConfigPostUpdate_status(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionTenantIntegrationConfigUpdate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.UpdateIntegrationStatusRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.TenantIntegrationConfigService.UpdateStatus(req.ID, req.Status, op))
}

func ProductAIUsageCredentialList(c *gin.Context) {
	if _, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialView); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	q := params.NewPagedSqlCnd(c, params.QueryFilter{ParamName: "tenantId"}, params.QueryFilter{ParamName: "productId"}, params.QueryFilter{ParamName: "status"}).Where("status <> ?", enums.StatusDeleted).Desc("id")
	list, page := services.ProductAIUsageCredentialService.FindPage(q)
	httpx.WriteJSON(c, &web.PageResult{Results: builders.BuildProductAIUsageCredentialList(list), Page: page})
}
func ProductAIUsageCredentialGetBy(c *gin.Context) {
	id, ok := httpx.GetPathInt64(c, "id")
	if !ok {
		return
	}
	if _, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialView); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	v := services.ProductAIUsageCredentialService.Get(id)
	if v == nil || v.Status == enums.StatusDeleted {
		httpx.WriteJSON(c, nil)
		return
	}
	httpx.WriteJSON(c, builders.BuildProductAIUsageCredential(v))
}
func ProductAIUsageCredentialPostCreate(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialCreate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.CreateProductAIUsageCredentialRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	v, e := services.ProductAIUsageCredentialService.Create(req, op)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, builders.BuildProductAIUsageCredential(v))
}
func ProductAIUsageCredentialPostUpdate(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialUpdate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.UpdateProductAIUsageCredentialRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.ProductAIUsageCredentialService.Update(req, op))
}
func ProductAIUsageCredentialPostDelete(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialDelete)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.DeleteIntegrationRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.ProductAIUsageCredentialService.Delete(req.ID, op))
}
func ProductAIUsageCredentialPostUpdate_status(c *gin.Context) {
	op, e := services.AuthService.RequirePermission(c, constants.PermissionProductAIUsageCredentialUpdate)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.UpdateIntegrationStatusRequest
	if e = params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, services.ProductAIUsageCredentialService.UpdateStatus(req.ID, req.Status, op))
}

func MeetingPostPreview_create(c *gin.Context) {
	if _, e := services.AuthService.RequirePermission(c, constants.PermissionMeetingPreview); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	var req request.MeetingPreviewCreateRequest
	if e := params.ReadJSON(c, &req); e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	v, e := services.MeetingService.Preview(req)
	if e != nil {
		httpx.WriteJSON(c, e)
		return
	}
	httpx.WriteJSON(c, &response.MeetingPreviewResponse{
		RoomName:  v.RoomName,
		JoinURL:   v.JoinURL,
		ExpiresAt: v.ExpiresAt.Format(time.RFC3339),
	})
}
