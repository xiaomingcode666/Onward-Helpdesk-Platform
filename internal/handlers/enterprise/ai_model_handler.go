package enterprise

import (
	"errors"
	"strconv"
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func AIModelWorkspace(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseAIModelService.GetWorkspace(ctx.Request.Context(), tenantID, enterpriseAIModelQuery(ctx), enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAIModelWorkspace(aggregate))
}

func AICapabilities(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	aggregate := services.EnterpriseAICapabilityService.GetCapabilities(ctx.Request.Context(), tenantID)
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAICapabilities(aggregate))
}

func AIModelDefaultLLMUpdate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseAIModelDefaultLLMUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	aggregate, err := services.EnterpriseAICapabilityService.UpdateDefaultLLMModel(ctx.Request.Context(), tenantID, req.ModelName, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAICapabilities(aggregate))
}

func AIModelKeyUpdateQuota(ctx *gin.Context) {
	tenantID, keyID, ok := resolveEnterpriseAIModelKeyRoute(ctx)
	if !ok {
		return
	}
	var req request.EnterpriseAIKeyQuotaUpdateRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.Quota == nil {
		httpx.WriteJSON(ctx, errors.New("quota is required"))
		return
	}
	aggregate, err := services.EnterpriseAIModelService.UpdateKeyQuota(ctx.Request.Context(), tenantID, keyID, *req.Quota, enterpriseAIModelQuery(ctx), enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAIModelWorkspace(aggregate))
}

func AIModelKeyResetQuota(ctx *gin.Context) {
	tenantID, keyID, ok := resolveEnterpriseAIModelKeyRoute(ctx)
	if !ok {
		return
	}
	aggregate, err := services.EnterpriseAIModelService.ResetKeyQuota(ctx.Request.Context(), tenantID, keyID, enterpriseAIModelQuery(ctx), enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseAIModelWorkspace(aggregate))
}

func resolveEnterpriseAIModelKeyRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, false
	}
	keyID, err := strconv.ParseInt(strings.TrimSpace(ctx.Param("keyId")), 10, 64)
	if err != nil || keyID <= 0 {
		httpx.WriteJSON(ctx, errors.New("keyId is required"))
		return 0, 0, false
	}
	return tenantID, keyID, true
}

func enterpriseAIModelQuery(ctx *gin.Context) request.EnterpriseAIModelWorkspaceQueryRequest {
	return request.EnterpriseAIModelWorkspaceQueryRequest{
		StartDate:   strings.TrimSpace(ctx.Query("startDate")),
		EndDate:     strings.TrimSpace(ctx.Query("endDate")),
		Granularity: strings.TrimSpace(ctx.Query("granularity")),
		Timezone:    strings.TrimSpace(ctx.Query("timezone")),
		Page:        enterpriseAIModelQueryInt(ctx, "page", 1),
		PageSize:    enterpriseAIModelQueryInt(ctx, "pageSize", enterpriseAIModelQueryInt(ctx, "page_size", 20)),
		SortBy:      strings.TrimSpace(ctx.Query("sortBy")),
		SortOrder:   strings.TrimSpace(ctx.Query("sortOrder")),
	}
}

func enterpriseAIModelQueryInt(ctx *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
