package enterprise

import (
	"log/slog"
	"strconv"
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// AccessConnectorPostCreate 创建连接器
// POST /api/enterprise/access/connector/create
func AccessConnectorPostCreate(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	var req request.AccessConnectorCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if req.TemplateCode != "" {
		template := services.GetConnectorTemplateByCode(req.TemplateCode)
		if template != nil {
			if req.BaseURL == "" {
				req.BaseURL = template.BaseURL
			}
			if req.AuthType == "" {
				req.AuthType = template.AuthType
			}
		}
	}

	tenantIDInt := principal.TenantID

	connector := &models.AccessConnector{
		TenantID:      tenantIDInt,
		Name:          req.Name,
		ConnectorType: req.ConnectorType,
		BaseURL:       req.BaseURL,
		AuthType:      req.AuthType,
		AuthConfig:    req.AuthConfig,
		FieldMapping:  req.FieldMapping,
		TemplateCode:  req.TemplateCode,
	}

	if err := services.AccessConnectorService.CreateConnector(ctx, connector); err != nil {
		slog.Error("failed to create connector", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAccessConnectorAudit(ctx, principal, "access.connector.created", connector, map[string]any{
		"name": connector.Name, "connectorType": connector.ConnectorType, "baseUrl": connector.BaseURL, "authType": connector.AuthType,
	})

	httpx.WriteJSON(ctx, map[string]any{
		"id":     connector.ID,
		"name":   connector.Name,
		"status": connector.Status,
	})
}

// AccessConnectorPostUpdate 更新租户连接器配置。
func AccessConnectorPostUpdate(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	var req request.AccessConnectorUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	connector := &models.AccessConnector{
		ID: req.ID, TenantID: principal.TenantID, Name: req.Name, ConnectorType: req.ConnectorType,
		BaseURL: req.BaseURL, AuthType: req.AuthType, AuthConfig: req.AuthConfig,
		FieldMapping: req.FieldMapping, TemplateCode: req.TemplateCode,
	}
	if err := services.AccessConnectorService.UpdateConnector(ctx, connector); err != nil {
		slog.Error("failed to update connector", "id", req.ID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAccessConnectorAudit(ctx, principal, "access.connector.updated", connector, map[string]any{
		"name": connector.Name, "connectorType": connector.ConnectorType, "baseUrl": connector.BaseURL, "authType": connector.AuthType,
	})
	httpx.WriteJSON(ctx, map[string]any{"id": connector.ID, "name": connector.Name, "status": connector.Status})
}

// AccessConnectorPostTest 测试连接器连通性
// POST /api/enterprise/access/connector/:id/test
func AccessConnectorPostTest(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	connectorID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || connectorID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "invalid connector id"))
		return
	}

	tenantIDInt := principal.TenantID

	if err := services.AccessConnectorService.TestConnector(ctx, tenantIDInt, connectorID); err != nil {
		slog.Error("connector test failed", "id", connectorID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAccessConnectorAudit(ctx, principal, "access.connector.tested", &models.AccessConnector{ID: connectorID, TenantID: tenantIDInt}, map[string]any{"result": "success"})

	httpx.WriteJSON(ctx, map[string]any{
		"success": true,
	})
}

// AccessConnectorPostCall 调用连接器
// POST /api/enterprise/access/connector/:id/call
func AccessConnectorPostCall(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	connectorID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || connectorID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "invalid connector id"))
		return
	}

	type callConnectorRequest struct {
		Method      string            `json:"method"`
		Path        string            `json:"path"`
		QueryParams map[string]string `json:"queryParams"`
		Body        interface{}       `json:"body"`
		Headers     map[string]string `json:"headers"`
	}

	var req callConnectorRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	tenantIDInt := principal.TenantID

	connectorReq := services.ConnectorRequest{
		Method:      req.Method,
		Path:        req.Path,
		QueryParams: req.QueryParams,
		Body:        req.Body,
		Headers:     req.Headers,
	}

	resp, err := services.AccessConnectorService.CallConnector(ctx, tenantIDInt, connectorID, connectorReq)
	if err != nil {
		slog.Error("connector call failed", "id", connectorID, "error", err)
		httpx.WriteJSON(ctx, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	httpx.WriteJSON(ctx, map[string]any{
		"success":    true,
		"statusCode": resp.StatusCode,
		"body":       string(resp.Body),
		"headers":    resp.Headers,
		"durationMs": resp.DurationMs,
	})
}

// AccessConnectorGetList 获取连接器列表
// GET /api/enterprise/access/connectors
func AccessConnectorGetList(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	tenantIDInt := principal.TenantID
	if tenantIDInt <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "tenant is required"))
		return
	}

	connectorType := ctx.Query("connectorType")

	connectors, err := services.AccessConnectorService.ListConnectors(ctx, tenantIDInt, connectorType)
	if err != nil {
		slog.Error("failed to list connectors", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, builders.BuildAccessConnectorList(connectors))
}

// AccessConnectorGetStatus 获取连接器健康状态
// GET /api/enterprise/access/connector/:id/status
func AccessConnectorGetStatus(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	connectorID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || connectorID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "invalid connector id"))
		return
	}

	tenantIDInt := principal.TenantID
	if tenantIDInt <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "tenant is required"))
		return
	}

	health, err := services.AccessConnectorService.GetConnectorStatus(ctx, tenantIDInt, connectorID)
	if err != nil {
		slog.Error("failed to get connector status", "id", connectorID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, health)
}

// AccessConnectorGetLogs 获取连接器调用日志
// GET /api/enterprise/access/connector/:id/logs
func AccessConnectorGetLogs(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	connectorID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || connectorID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "invalid connector id"))
		return
	}

	tenantIDInt := principal.TenantID
	if tenantIDInt <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "tenant is required"))
		return
	}

	limitStr := ctx.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	logs, err := services.AccessConnectorService.GetCallLogs(ctx, tenantIDInt, connectorID, limit)
	if err != nil {
		slog.Error("failed to get call logs", "id", connectorID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, logs)
}

// AccessConnectorGetTemplates 获取连接器模板列表
// GET /api/enterprise/access/connector/templates
func AccessConnectorGetTemplates(ctx *gin.Context) {
	templates := services.GetConnectorTemplates()
	httpx.WriteJSON(ctx, templates)
}

func recordAccessConnectorAudit(ctx *gin.Context, principal *dto.AuthPrincipal, action string, connector *models.AccessConnector, afterState any) {
	if ctx == nil || principal == nil || connector == nil || connector.TenantID <= 0 {
		return
	}
	actorType := strings.TrimSpace(principal.SubjectType)
	if actorType == "" {
		actorType = "user"
	}
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       connector.TenantID,
		ActorID:        strconv.FormatInt(principal.UserID, 10),
		ActorType:      actorType,
		Domain:         "access",
		ResourceType:   "access_connector",
		ResourceID:     strconv.FormatInt(connector.ID, 10),
		Action:         action,
		AfterState:     afterState,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: principal.SupportGrantID,
		RiskLevel:      models.RiskLevelMedium,
	})
}
