package bootstrap

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"remotehelpdesk/internal/ai/mcps"
	_ "remotehelpdesk/internal/ai/runtime"
	"remotehelpdesk/internal/handlers/api"
	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/ginx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/observability"
	"remotehelpdesk/internal/pkg/tracex"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"

	_ "remotehelpdesk/internal/services/wx_callback_handlers"
)

func NewServer() (*gin.Engine, error) {
	cfg := config.Current()
	i18nx.SetDefaultLocale(cfg.LanguageOrDefault())

	gin.SetMode(gin.ReleaseMode)
	printBanner()

	app := gin.New()
	app.Use(requestIDMiddleware())
	app.Use(corsMiddleware())
	app.Use(gin.Recovery())
	app.Use(middleware.PrometheusMetricsMiddleware())
	app.Use(requestLogMiddleware())
	app.Use(maxBodySizeMiddleware())
	app.Use(i18nx.Middleware())

	addRouter(app)

	if baseURL := strings.TrimRight(cfg.Storage.Local.BaseURL, "/"); baseURL != "" {
		app.StaticFS(baseURL, ginx.StaticFiles(cfg.Storage.Local.Root))
	}
	app.NoRoute(func(ctx *gin.Context) {
		httpx.WriteHttpStatusJSON(ctx, http.StatusNotFound, web.JsonErrorCode(http.StatusNotFound, i18nx.T(ctx, "error.notFound")))
	})

	return app, nil
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := config.Current().Server.CORS.AllowedOrigins
	allowHeaders := "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-Tenant-Id, X-Request-Id, X-Locale, Idempotency-Key, X-Guest-Id, X-Channel-Id, X-External-Id, X-External-Name, X-Customer-Session-Token, X-Customer-Session-Expires-At, X-Customer-Entry-Visitor-Id, X-Customer-Entry-Visitor-Token"
	exposeHeaders := "Content-Length, Content-Type, Content-Disposition, Accept-Ranges, Content-Range, ETag, Authorization, X-Request-Id, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, X-Guest-Id, X-Channel-Id, X-External-Id, X-External-Name, X-Customer-Session-Token, X-Customer-Session-Expires-At"
	allowMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" {
			continue
		}
		allowedOriginSet[origin] = struct{}{}
	}
	return func(ctx *gin.Context) {
		if isWebsocketUpgrade(ctx) {
			ctx.Next()
			return
		}
		origin := strings.TrimRight(strings.TrimSpace(ctx.GetHeader("Origin")), "/")
		if origin != "" {
			ctx.Header("Vary", "Origin")
			if _, ok := allowedOriginSet[origin]; !ok {
				if ctx.Request.Method == http.MethodOptions {
					ctx.AbortWithStatus(http.StatusForbidden)
					return
				}
				ctx.Next()
				return
			}
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Methods", allowMethods)
			ctx.Header("Access-Control-Allow-Headers", allowHeaders)
			ctx.Header("Access-Control-Expose-Headers", exposeHeaders)
			ctx.Header("Access-Control-Max-Age", "600")
		}
		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := tracex.EnsureRequestID(ctx.GetHeader(tracex.RequestIDHeader))
		ctx.Set(tracex.GinRequestIDKey, requestID)
		if requestID != "" {
			ctx.Header(tracex.RequestIDHeader, requestID)
		}
		ctx.Next()
	}
}

func requestLogMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		rawPath := ctx.Request.URL.Path
		method := ctx.Request.Method
		requestID, _ := ctx.Get(tracex.GinRequestIDKey)
		ctx.Next()

		slog.Info("http request",
			"requestId", requestID,
			"method", method,
			"path", requestLogPath(ctx, rawPath),
			"status", ctx.Writer.Status(),
			"elapsed", time.Since(start).Milliseconds(),
			"clientIp", ctx.ClientIP(),
		)
	}
}

func requestLogPath(ctx *gin.Context, rawPath string) string {
	if routePath := strings.TrimSpace(ctx.FullPath()); routePath != "" {
		return routePath
	}
	return redactSensitiveRequestPath(rawPath)
}

func redactSensitiveRequestPath(rawPath string) string {
	path := strings.TrimSpace(rawPath)
	for _, prefix := range []string{
		"/api/third/jitsi/transcription/ws/",
		"/api/third/jitsi/transcription/events/",
	} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(path, prefix)
		_, suffix, hasSuffix := strings.Cut(remainder, "/")
		if hasSuffix && suffix != "" {
			return prefix + "<redacted>/" + suffix
		}
		return prefix + "<redacted>"
	}
	return path
}

func maxBodySizeMiddleware() gin.HandlerFunc {
	limit := config.Current().Storage.MaxRequestBodySizeBytes()
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		ctx.Next()
	}
}

func isWebsocketUpgrade(ctx *gin.Context) bool {
	if !strings.EqualFold(ctx.GetHeader("Upgrade"), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(ctx.GetHeader("Connection")), "upgrade")
}

func addRouter(app *gin.Engine) {
	app.GET("/metrics", gin.WrapH(observability.Handler()))
	if config.Current().MCP.Enabled {
		app.Any("/api/mcp", mcpAccessMiddleware(), gin.WrapH(mcps.NewHTTPHandler()))
	}

	apiGroup := app.Group("/api")
	apiGroup.GET("/health", api.Health)
	apiGroup.GET("/health/ready", api.Ready)
	apiGroup.GET("/health/live", api.Live)
	apiGroup.GET("/config", api.PublicConfig)
	apiGroup.POST("/marketing/demo-request", middleware.MarketingDemoRequestRateLimitMiddleware(), api.MarketingDemoRequest)
	apiGroup.GET("/tenant-branding/logo/:assetId", api.TenantBrandingLogoGet)
	registerApiAuthRoutes(apiGroup.Group("/auth"))
	registerApiChannelRoutes(apiGroup.Group("/channel"))
	registerApiCustomerRoutes(apiGroup.Group("/customer", middleware.CustomerEntryRateLimitMiddleware()))
	registerApiCustomerPortalRoutes(apiGroup.Group("/customer/v1", middleware.CustomerPortalAccountMiddleware, middleware.CustomerPortalRateLimitMiddleware()))
	registerApiConversationRoutes(apiGroup.Group("/conversation", middleware.ExternalUserMiddleware))
	registerApiMessageRoutes(apiGroup.Group("/message", middleware.ExternalUserMiddleware))
	apiGroup.GET("/media/:assetId", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), api.ConversationMediaGetForOperator)

	wsGroup := app.Group("/api/ws")
	wsGroup.GET("/dashboard", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), services.WsService.HandleDashboardWS)
	wsGroup.GET("/dashboard/notification", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), services.WsService.HandleDashboardNotificationWS)
	wsGroup.GET("/open", middleware.CustomerEntryRateLimitMiddleware(), services.WsService.HandleOpenWS)

	dashboardGroup := app.Group("/api/dashboard", middleware.AuthMiddleware, middleware.TenantContextMiddleware())
	registerDashboardDashboardRoutes(dashboardGroup.Group("/dashboard"))
	registerDashboardUserRoutes(dashboardGroup.Group("/user"))
	registerDashboardCompanyRoutes(dashboardGroup.Group("/company"))
	registerDashboardProductRoutes(dashboardGroup.Group("/product"))
	registerDashboardProductServiceProfileRoutes(dashboardGroup.Group("/product-service-profile"))
	registerDashboardProductKnowledgeBindingRoutes(dashboardGroup.Group("/product-knowledge-binding"))
	registerDashboardTenantIntegrationConfigRoutes(dashboardGroup.Group("/tenant-integration-config"))
	registerDashboardProductAIUsageCredentialRoutes(dashboardGroup.Group("/product-ai-usage-credential"))
	registerDashboardMeetingRoutes(dashboardGroup.Group("/meeting"))
	registerDashboardProductModelRoutes(dashboardGroup.Group("/product-model"))
	registerDashboardDeviceRoutes(dashboardGroup.Group("/device"))
	registerDashboardServiceCodeBatchRoutes(dashboardGroup.Group("/service-code-batch"))
	registerDashboardServiceCodeRoutes(dashboardGroup.Group("/service-code"))
	registerDashboardCustomerRoutes(dashboardGroup.Group("/customer"))
	registerDashboardCustomerContactRoutes(dashboardGroup.Group("/customer-contact"))
	registerDashboardRoleRoutes(dashboardGroup.Group("/role"))
	registerDashboardPermissionRoutes(dashboardGroup.Group("/permission"))
	registerDashboardSessionRoutes(dashboardGroup.Group("/session"))
	registerDashboardTagRoutes(dashboardGroup.Group("/tag"))
	registerDashboardConversationRoutes(dashboardGroup.Group("/conversation"))
	registerDashboardTicketRoutes(dashboardGroup.Group("/ticket"))
	registerDashboardTicketAliasRoutes(dashboardGroup.Group("/tickets"))
	registerDashboardNotificationRoutes(dashboardGroup.Group("/notification"))
	registerDashboardQuickReplyRoutes(dashboardGroup.Group("/quick-reply"))
	registerDashboardChannelRoutes(dashboardGroup.Group("/channel"))
	registerDashboardAgentRoutes(dashboardGroup.Group("/agent"))
	registerDashboardAgentTeamRoutes(dashboardGroup.Group("/agent-team"))
	registerDashboardAgentTeamScheduleRoutes(dashboardGroup.Group("/agent-team-schedule"))
	registerDashboardAIAgentRoutes(dashboardGroup.Group("/ai-agent"))
	registerDashboardAIWorkflowRoutes(dashboardGroup.Group("/ai-workflow"))
	registerDashboardAIConfigRoutes(dashboardGroup.Group("/ai-config"))
	registerDashboardAssetRoutes(dashboardGroup.Group("/asset"))
	registerDashboardKnowledgeBaseRoutes(dashboardGroup.Group("/knowledge-base"))
	registerDashboardKnowledgeDirectoryRoutes(dashboardGroup.Group("/knowledge-directory"))
	registerDashboardKnowledgeDocumentRoutes(dashboardGroup.Group("/knowledge-document"))
	registerDashboardKnowledgeFAQRoutes(dashboardGroup.Group("/knowledge-faq"))
	registerDashboardKnowledgeRetrieveRoutes(dashboardGroup.Group("/knowledge-retrieve"))
	registerDashboardKnowledgeRetrieveLogRoutes(dashboardGroup.Group("/knowledge-retrieve-log"))
	registerDashboardKnowledgeIndexGenerationRoutes(dashboardGroup.Group("/knowledge-index-generation"))
	registerDashboardSkillDefinitionRoutes(dashboardGroup.Group("/skill-definition"))
	registerDashboardMCPRoutes(dashboardGroup.Group("/mcp"))

	platformGroup := app.Group("/api/platform", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), middleware.PlatformOnlyMiddleware())
	registerPlatformRoutes(platformGroup)

	enterpriseGroup := app.Group("/api/enterprise/v1", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), middleware.EnterpriseOnlyMiddleware())
	registerEnterpriseSLARoutes(enterpriseGroup)
	registerEnterpriseDiagnosisRoutes(enterpriseGroup)
	registerEnterpriseReportRoutes(enterpriseGroup)
	registerEnterpriseConversationRoutes(enterpriseGroup)
	registerEnterpriseWebhookRoutes(enterpriseGroup)
	registerEnterpriseGDPRRoutes(enterpriseGroup)
	registerEnterpriseAccessConnectorRoutes(enterpriseGroup)
	registerEnterpriseAIWorkflowRoutes(enterpriseGroup)
	registerEnterpriseProductRoutes(enterpriseGroup)
	registerEnterpriseTicketRoutes(enterpriseGroup)
	registerEnterpriseDeviceRoutes(enterpriseGroup)
	registerEnterpriseServiceCodeRoutes(enterpriseGroup)
	registerEnterpriseKnowledgeRoutes(enterpriseGroup)
	registerEnterpriseSearchRoutes(enterpriseGroup)
	registerEnterpriseMeetingRoutes(enterpriseGroup)
	registerEnterpriseNotificationRoutes(enterpriseGroup)
	registerEnterpriseIAMRoutes(enterpriseGroup)

	partnerGroup := app.Group("/api/partner/v1", middleware.AuthMiddleware, middleware.TenantContextMiddleware(), middleware.PartnerOnlyMiddleware())
	registerPartnerRoutes(partnerGroup)

	thirdGroup := app.Group("/api/third")
	registerThirdWechatRoutes(thirdGroup.Group("/wechat"))
	registerThirdJitsiRoutes(thirdGroup.Group("/jitsi"))
}

func mcpAccessMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		configuredToken := strings.TrimSpace(config.Current().MCP.ServerToken)
		if configuredToken != "" {
			providedToken := strings.TrimSpace(ctx.GetHeader("Authorization"))
			if scheme, value, found := strings.Cut(providedToken, " "); found && strings.EqualFold(scheme, "Bearer") {
				providedToken = strings.TrimSpace(value)
			} else {
				providedToken = ""
			}
			if len(providedToken) != len(configuredToken) || subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredToken)) != 1 {
				ctx.AbortWithStatus(http.StatusNotFound)
				return
			}
			ctx.Next()
			return
		}

		host, _, err := net.SplitHostPort(strings.TrimSpace(ctx.Request.RemoteAddr))
		if err != nil {
			host = strings.TrimSpace(ctx.Request.RemoteAddr)
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		ctx.Next()
	}
}
