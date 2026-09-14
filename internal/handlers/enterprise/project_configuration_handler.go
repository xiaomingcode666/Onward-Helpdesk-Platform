package enterprise

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func projectConfigOperator(ctx *gin.Context) (*dto.AuthPrincipal, int64, bool) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return nil, 0, false
	}
	id, ok := resolveEnterpriseTenantIDInt(ctx)
	return op, id, ok
}
func decodeConfigRequest(ctx *gin.Context, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 300*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("配置格式不正确、包含不支持的字段或超过大小限制"))
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("只能提交一份配置"))
		return false
	}
	return true
}
func writeProjectConfigResult(ctx *gin.Context, result any, err error) {
	if errors.Is(err, services.ErrProjectConfigConflict) {
		httpx.WriteHttpStatusJSON(ctx, http.StatusConflict, errorsx.InvalidParam(err.Error()))
		return
	}
	var invalid *services.ProjectConfigValidationError
	if errors.As(err, &invalid) {
		message := "配置校验未通过"
		if len(invalid.Report.Issues) > 0 {
			message = invalid.Report.Issues[0].Path + "：" + invalid.Report.Issues[0].Message
		}
		httpx.WriteHttpStatusJSON(ctx, http.StatusUnprocessableEntity, errorsx.InvalidParam(message))
		return
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}
func ProjectConfigurationGet(ctx *gin.Context) {
	op, id, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	var beforeID int64
	if raw := ctx.Query("before_id"); raw != "" {
		var err error
		beforeID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || beforeID < 0 {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("历史版本游标无效"))
			return
		}
	}
	view, err := services.GetProjectConfigurationHistory(id, beforeID)
	if err == nil && view.Document.Runtime != nil {
		err = services.RequireProjectRuntimeOperator(op)
	}
	if err == nil {
		for _, v := range view.Versions {
			if v.Document.Runtime != nil {
				err = services.RequireProjectRuntimeOperator(op)
				break
			}
		}
	}
	writeProjectConfigResult(ctx, view, err)
}

func ProjectConfigurationUpgrade(ctx *gin.Context) {
	op, id, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	if err := services.RequireProjectRuntimeOperator(op); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	doc, err := services.UpgradeProjectConfiguration(id)
	writeProjectConfigResult(ctx, doc, err)
}
func ProjectConfigurationDraft(ctx *gin.Context) {
	op, id, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	var input services.ProjectConfigDraft
	if !decodeConfigRequest(ctx, &input) {
		return
	}
	version, err := services.SaveProjectConfigurationDraft(id, input, op)
	writeProjectConfigResult(ctx, version, err)
}
func ProjectConfigurationValidate(ctx *gin.Context) {
	op, id, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	var doc projectconfig.Document
	if !decodeConfigRequest(ctx, &doc) {
		return
	}
	if doc.Runtime != nil {
		if err := services.RequireProjectRuntimeOperator(op); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
	}
	httpx.WriteJSON(ctx, projectconfig.Validate(doc, id, projectconfig.Environment(), projectconfig.RuntimeSecretCheck))
}
func ProjectConfigurationApply(ctx *gin.Context) {
	op, id, ok := projectConfigOperator(ctx)
	if !ok {
		return
	}
	versionID, err := strconv.ParseInt(ctx.Param("version"), 10, 64)
	if err != nil || versionID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("配置版本无效"))
		return
	}
	version, err := services.ApplyProjectConfiguration(id, versionID, op)
	writeProjectConfigResult(ctx, version, err)
}
