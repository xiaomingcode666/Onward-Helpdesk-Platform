package platform

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func GetPlatformAIModelWorkspace(ctx *gin.Context) {
	aggregate, err := services.PlatformAIModelService.GetWorkspace()
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAIModelWorkspace(aggregate))
}

func PostPlatformAIProviderSave(ctx *gin.Context) {
	var req request.PlatformAIProviderSaveRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.PlatformAIModelService.UpdateProvider(req, services.AuthService.GetAuthPrincipal(ctx)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	aggregate, err := services.PlatformAIModelService.GetWorkspace()
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAIModelWorkspace(aggregate))
}

func PostPlatformAITenantProvision(ctx *gin.Context) {
	var req request.PlatformAITenantProvisionRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	account, err := services.PlatformAIModelService.ProvisionTenant(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_ = account
	aggregate, err := services.PlatformAIModelService.GetWorkspace()
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAIModelWorkspace(aggregate))
}

func GetPlatformAITenantBilling(ctx *gin.Context) {
	tenantID := queryInt64(ctx, "tenantId", 0)
	if tenantID <= 0 {
		httpx.WriteJSON(ctx, errors.New("tenantId is required"))
		return
	}
	timezone := strings.TrimSpace(ctx.Query("timezone"))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	aggregate, err := services.PlatformAIModelService.GetTenantBilling(tenantID, timezone, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAITenantBilling(aggregate))
}

func PostPlatformAITenantRecharge(ctx *gin.Context) {
	var req request.PlatformAITenantRechargeRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if strings.TrimSpace(req.Timezone) == "" {
		req.Timezone = "Asia/Shanghai"
	}
	aggregate, err := services.PlatformAIModelService.RechargeTenant(req, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAITenantBilling(aggregate))
}

func GetPlatformAITenantUsage(ctx *gin.Context) {
	tenantID := queryInt64(ctx, "tenantId", 0)
	if tenantID <= 0 {
		httpx.WriteJSON(ctx, errors.New("tenantId is required"))
		return
	}
	timezone := strings.TrimSpace(ctx.Query("timezone"))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	startDate := strings.TrimSpace(ctx.Query("startDate"))
	endDate := strings.TrimSpace(ctx.Query("endDate"))
	if startDate == "" || endDate == "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			loc = time.FixedZone("UTC+8", 8*3600)
		}
		now := time.Now().In(loc)
		if endDate == "" {
			endDate = now.Format("2006-01-02")
		}
		if startDate == "" {
			startDate = now.AddDate(0, 0, -1).Format("2006-01-02")
		}
	}
	req := request.PlatformAIUsageQueryRequest{
		TenantID:    tenantID,
		StartDate:   startDate,
		EndDate:     endDate,
		ModelSource: strings.TrimSpace(ctx.Query("modelSource")),
		Timezone:    timezone,
		Page:        queryInt(ctx, "page", 1),
		PageSize:    queryInt(ctx, "pageSize", 20),
		SortBy:      strings.TrimSpace(ctx.Query("sortBy")),
		SortOrder:   strings.TrimSpace(ctx.Query("sortOrder")),
	}
	aggregate, err := services.PlatformAIModelService.GetTenantUsage(tenantID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformAIUsage(aggregate))
}

func GetPlatformSub2APIUsers(ctx *gin.Context) {
	query := request.PlatformSub2APIUsersQueryRequest{
		Page:      queryInt(ctx, "page", 1),
		PageSize:  queryInt(ctx, "pageSize", 100),
		Status:    strings.TrimSpace(ctx.Query("status")),
		Role:      strings.TrimSpace(ctx.Query("role")),
		Search:    strings.TrimSpace(ctx.Query("search")),
		Timezone:  strings.TrimSpace(ctx.Query("timezone")),
		SortBy:    strings.TrimSpace(ctx.Query("sortBy")),
		SortOrder: strings.TrimSpace(ctx.Query("sortOrder")),
	}
	aggregate, err := services.PlatformAIModelService.ListSub2APIUsers(ctx.Request.Context(), query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSub2APIUserList(aggregate))
}

func GetPlatformSub2APIUserKeys(ctx *gin.Context) {
	userID, err := strconv.ParseInt(ctx.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		httpx.WriteJSON(ctx, errors.New("valid sub2api user id is required"))
		return
	}
	timezone := strings.TrimSpace(ctx.Query("timezone"))
	aggregate, err := services.PlatformAIModelService.ListSub2APIUserKeys(ctx.Request.Context(), userID, timezone)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformSub2APIUserKeyList(aggregate))
}
