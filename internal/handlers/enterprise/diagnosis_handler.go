package enterprise

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

type startDiagnosisRequest struct {
	CustomerID     string `json:"customer_id"`
	DeviceID       string `json:"device_id"`
	ProductID      string `json:"product_id" binding:"required"`
	TicketID       string `json:"ticket_id"`
	ServiceCodeID  string `json:"service_code_id"`
	ConversationID string `json:"conversation_id"`
	Language       string `json:"language"`
	Symptoms       string `json:"symptom"`
	MaxRounds      int    `json:"max_rounds"`
}

type submitStepRequest struct {
	UserInput string `json:"answer" binding:"required"`
}

// DiagnosisFaultTreeNodeList 获取当前租户下指定产品的故障树节点。
func DiagnosisFaultTreeNodeList(ctx *gin.Context) {
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	productID, err := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid product id"))
		return
	}
	rows, err := services.FaultTreeService.ListManagedNodes(tenantID, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseFaultTreeNodes(rows))
}

// DiagnosisFaultTreeNodeCreate 创建故障树节点。
func DiagnosisFaultTreeNodeCreate(ctx *gin.Context) {
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseFaultTreeNodeCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	row, err := services.FaultTreeService.CreateManagedNode(tenantID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseFaultTreeNode(row))
}

// DiagnosisFaultTreeNodeUpdate 更新节点内容、层级或发布状态。
func DiagnosisFaultTreeNodeUpdate(ctx *gin.Context) {
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseFaultTreeNodeUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	row, err := services.FaultTreeService.UpdateManagedNode(tenantID, ctx.Param("nodeId"), req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseFaultTreeNode(row))
}

// DiagnosisPostStart 发起诊断
func DiagnosisPostStart(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	var req startDiagnosisRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	tenantID, _ := parsePrincipalTenantID(principal)

	session, err := services.DiagnosisService.CreateDiagnosisSessionWithContext(ctx, services.DiagnosisSessionCreateRequest{
		TenantID:       tenantID,
		CustomerID:     parseDiagnosisID(req.CustomerID),
		DeviceID:       parseDiagnosisID(req.DeviceID),
		ProductID:      parseDiagnosisID(req.ProductID),
		ServiceCodeID:  parseDiagnosisID(req.ServiceCodeID),
		ConversationID: parseDiagnosisID(req.ConversationID),
		Language:       req.Language,
		Audience:       resolveDiagnosisAudience(principal),
		Symptoms:       req.Symptoms,
		MaxRounds:      req.MaxRounds,
	})
	if err != nil {
		slog.Error("failed to start diagnosis", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, map[string]any{
		"sessionId":  session.ID,
		"status":     session.Status,
		"createdAt":  session.CreatedAt,
		"maxRounds":  session.MaxRounds,
		"confidence": session.ConfidenceScore,
	})
}

// DiagnosisPostStep 提交诊断步骤
func DiagnosisPostStep(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	tenantID, _ := parsePrincipalTenantID(principal)

	sessionID := ctx.Param("id")
	if sessionID == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "session id is required"))
		return
	}

	var req submitStepRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	step, err := services.DiagnosisService.DiagnoseForTenant(ctx, tenantID, sessionID, req.UserInput)
	if err != nil {
		slog.Error("diagnosis step failed", "sessionID", sessionID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	shouldEscalate, reason := services.DiagnosisService.ShouldEscalateToHumanForTenant(tenantID, sessionID)
	if shouldEscalate {
		services.DiagnosisService.EndDiagnosisSessionForTenant(tenantID, sessionID, "escalated", reason)
	}

	httpx.WriteJSON(ctx, map[string]any{
		"stepId":           step.ID,
		"outputContent":    step.OutputContent,
		"confidenceScore":  step.ConfidenceScore,
		"sequenceNo":       step.SequenceNo,
		"shouldEscalate":   shouldEscalate,
		"escalationReason": reason,
	})
}

// DiagnosisGetSummary 获取诊断摘要
func DiagnosisGetSummary(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	tenantID, _ := parsePrincipalTenantID(principal)

	sessionID := ctx.Param("id")
	if sessionID == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "session id is required"))
		return
	}

	summary, err := services.DiagnosisService.GetDiagnosisSummaryForTenant(tenantID, sessionID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, summary)
}

// DiagnosisGetByTicket 获取工单关联的诊断快照
func DiagnosisGetByTicket(ctx *gin.Context) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}

	tenantID, _ := parsePrincipalTenantID(principal)

	ticketID := ctx.Param("id")
	if ticketID == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "ticket id is required"))
		return
	}

	// 根据工单查询关联的诊断会话（带租户过滤）
	var sessions []models.DiagnosisSession
	sqls.DB().Where("tenant_id = ? AND id IN (SELECT diagnosis_session_id FROM tickets WHERE id = ? AND tenant_id = ?)", tenantID, ticketID, tenantID).
		Or("tenant_id = ? AND conversation_ref_id IN (SELECT conversation_id FROM tickets WHERE id = ? AND tenant_id = ?)", tenantID, ticketID, tenantID).
		Order("created_at desc").
		Find(&sessions)

	if len(sessions) == 0 {
		var ticket models.Ticket
		if err := sqls.DB().Where("id = ? AND tenant_id = ?", ticketID, tenantID).First(&ticket).Error; err == nil {
			sqls.DB().Where("tenant_id = ? AND (conversation_ref_id = ? OR device_ref_id = ?)", tenantID, ticket.ConversationID, ticket.DeviceID).
				Order("created_at desc").
				Find(&sessions)
		}
	}

	if len(sessions) == 0 {
		httpx.WriteJSON(ctx, []models.DiagnosisSession{})
		return
	}

	type DiagnosisSnapshot struct {
		Session   models.DiagnosisSession `json:"session"`
		StepCount int64                   `json:"stepCount"`
		Summary   map[string]any          `json:"summary,omitempty"`
	}

	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}

	// 批量统计步骤数(一条 GROUP BY 替代逐条 COUNT)
	stepCounts := map[string]int64{}
	if len(sessionIDs) > 0 {
		var counts []struct {
			SessionID string `gorm:"column:session_id"`
			Cnt       int64  `gorm:"column:cnt"`
		}
		sqls.DB().Model(&models.DiagnosisStep{}).
			Where("tenant_id = ? AND session_id IN ?", tenantID, sessionIDs).
			Select("session_id, COUNT(*) as cnt").
			Group("session_id").
			Scan(&counts)
		for _, c := range counts {
			stepCounts[c.SessionID] = c.Cnt
		}
	}

	// 批量拉取所有会话的诊断步骤,内存分组构造 summary
	stepsBySession := map[string][]models.DiagnosisStep{}
	if len(sessionIDs) > 0 {
		var allSteps []models.DiagnosisStep
		sqls.DB().Where("tenant_id = ? AND session_id IN ?", tenantID, sessionIDs).
			Order("sequence_no asc, id asc").
			Find(&allSteps)
		for _, step := range allSteps {
			stepsBySession[step.SessionID] = append(stepsBySession[step.SessionID], step)
		}
	}

	var snapshots []DiagnosisSnapshot
	for _, session := range sessions {
		session.Symptoms = maskJSONArray(session.Symptoms)

		summary := map[string]any{
			"session_id":         session.ID,
			"tenant_id":          session.TenantID,
			"customer_id":        session.CustomerID,
			"device_id":          session.DeviceID,
			"product_id":         session.ProductID,
			"product_model_id":   session.ProductModelID,
			"service_code_id":    session.ServiceCodeID,
			"conversation_id":    session.ConversationID,
			"region_code":        session.RegionCode,
			"audience":           session.Audience,
			"knowledge_scope":    session.KnowledgeScopeSnapshot,
			"status":             session.Status,
			"symptoms":           session.Symptoms,
			"fault_codes":        session.FaultCodes,
			"total_rounds":       session.TotalRounds,
			"confidence_score":   session.ConfidenceScore,
			"retrieve_count":     session.RetrieveCount,
			"retrieve_hit_count": session.RetrieveHitCount,
			"retrieve_top_score": session.RetrieveTopScore,
			"created_at":         session.CreatedAt,
			"steps":              stepsBySession[session.ID],
		}

		if nodes, err := services.FaultTreeService.GetDiagnosisPath(context.Background(), session.ID); err == nil {
			summary["fault_tree_path"] = nodes
		}

		snapshots = append(snapshots, DiagnosisSnapshot{
			Session:   session,
			StepCount: stepCounts[session.ID],
			Summary:   summary,
		})
	}

	httpx.WriteJSON(ctx, snapshots)
}

// maskJSONArray 对 JSON 数组做脱敏处理
func maskJSONArray(raw string) string {
	if raw == "" || raw == "[]" {
		return raw
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return raw
	}
	return raw
}

// parsePrincipalTenantID 从 AuthPrincipal 中解析 tenantID 为 int64
func parsePrincipalTenantID(principal *middleware.AuthPrincipal) (int64, bool) {
	if principal == nil || principal.TenantID <= 0 {
		return 0, false
	}
	return principal.TenantID, true
}

func parseDiagnosisID(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func resolveDiagnosisAudience(principal *dto.AuthPrincipal) string {
	if principal == nil {
		return "agent"
	}
	if principal.IsCustomer() {
		return "customer"
	}
	for _, role := range principal.Roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if strings.Contains(role, "engineer") {
			return "engineer"
		}
	}
	return "agent"
}
