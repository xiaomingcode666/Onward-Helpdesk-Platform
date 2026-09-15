package enterprise

import (
	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
	"strconv"
)

func TicketStatusWorkflowGet(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	key := ctx.DefaultQuery("project_key", "*")
	beforeID := int64(0)
	if raw := ctx.Query("before_id"); raw != "" {
		beforeID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || beforeID < 0 {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("历史版本游标无效"))
			return
		}
	}
	view, err := services.GetTicketStatusWorkflow(tenantID, key, beforeID, op)
	writeProjectConfigResult(ctx, view, err)
}
func TicketStatusWorkflowDraft(ctx *gin.Context) {
	op, tenantID, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	var req services.TicketWorkflowDraft
	if !decodeConfigRequest(ctx, &req) {
		return
	}
	result, err := services.SaveTicketStatusWorkflow(tenantID, req, op)
	// Do not expose unrelated operational settings from the shared version.
	var response any
	if result != nil {
		response = map[string]any{"id": result.ID}
	}
	writeProjectConfigResult(ctx, response, err)
}
func TicketStatusWorkflowPublish(ctx *gin.Context) {
	op, tenantID, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	var req struct {
		VersionID int64 `json:"version_id"`
	}
	if !decodeConfigRequest(ctx, &req) {
		return
	}
	if req.VersionID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("请选择待发布的流程版本"))
		return
	}
	result, err := services.PublishTicketStatusWorkflow(tenantID, req.VersionID, op)
	var response any
	if result != nil {
		response = map[string]any{"id": result.ID}
	}
	writeProjectConfigResult(ctx, response, err)
}
