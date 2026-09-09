package platform

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func GetPlatformOverview(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOverview(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOverview(item))
}

func GetPlatformOverviewMetrics(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOverviewMetrics(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOverviewMetrics(item))
}

func GetPlatformOverviewTopTenants(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "5"))
	item, err := services.PlatformConsoleService.GetOverviewTopTenants(time.Now(), limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOverviewTopTenants(item))
}

func GetPlatformOverviewUsageTrend(ctx *gin.Context) {
	days, _ := strconv.Atoi(ctx.DefaultQuery("days", "7"))
	item, err := services.PlatformConsoleService.GetOverviewUsageTrend(time.Now(), days)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOverviewUsageTrend(item))
}

func GetPlatformOverviewEvents(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "8"))
	item, err := services.PlatformConsoleService.GetOverviewRecentEvents(time.Now(), limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOverviewEvents(item))
}

func GetPlatformTenants(ctx *gin.Context) {
	GetPlatformTenantList(ctx)
}

func GetPlatformGlobalization(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", ctx.DefaultQuery("limit", "20")))
	item, err := services.PlatformConsoleService.GetGlobalization(time.Now(), page, pageSize)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformGlobalization(item))
}

func GetPlatformUsage(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetUsageOverview(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformUsageOverview(item))
}

// GetPlatformOpsOverview is kept for clients migrating to the resource-specific ops endpoints.
// Deprecated: use the handlers below instead.
func GetPlatformOpsOverview(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsOverview(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsOverview(item))
}

func GetPlatformOpsRuntime(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsRuntime(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsRuntime(item))
}

func GetPlatformOpsDatabase(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsDatabase(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsDatabase(item))
}

func GetPlatformOpsInfrastructure(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsInfrastructure(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsInfrastructure(item))
}

func GetPlatformOpsJitsi(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsJitsi(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsJitsi(item))
}

func GetPlatformOpsSpeech(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsSpeech(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsSpeech(item))
}

func GetPlatformOpsTranslation(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsTranslation(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsTranslation(item))
}

func GetPlatformOpsAR(ctx *gin.Context) {
	item := services.PlatformConsoleService.GetOpsAR(time.Now())
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsAR(item))
}

func GetPlatformOpsSub2API(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsSub2API(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsSub2API(item))
}

func GetPlatformOpsAccess(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsAccess(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsAccess(item))
}

func GetPlatformOpsPipeline(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsPipeline(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsPipeline(item))
}

func GetPlatformOpsKnowledgeQueue(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsKnowledgeQueue(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsKnowledgeQueue(item))
}

func GetPlatformOpsNotificationQueue(ctx *gin.Context) {
	item, err := services.PlatformConsoleService.GetOpsNotificationQueue(time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformOpsNotificationQueue(item))
}

func GetPlatformIntegrationSettings(ctx *gin.Context) {
	item, err := services.PlatformIntegrationConfigService.GetSettings(services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformIntegrationSettings(item))
}

func PostPlatformIntegrationUpdate(ctx *gin.Context) {
	input := request.PlatformIntegrationUpdateRequest{}
	if err := params.ReadJSON(ctx, &input); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.PlatformIntegrationConfigService.Update(input, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPlatformIntegrationSettings(item))
}

func PostPlatformIntegrationTest(ctx *gin.Context) {
	input := request.PlatformIntegrationTestRequest{}
	if err := params.ReadJSON(ctx, &input); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.PlatformIntegrationConfigService.Test(input, services.AuthService.GetAuthPrincipal(ctx))
	httpx.WriteJSON(ctx, builders.BuildPlatformIntegrationTest(item))
}
