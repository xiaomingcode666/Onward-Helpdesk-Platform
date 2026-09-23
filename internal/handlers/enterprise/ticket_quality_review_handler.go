package enterprise

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

func TicketQualityScorecardActive(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	value, err := services.GetActiveTicketQualityScorecard(operator.EffectiveTenantID())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, value)
}

func TicketQualityScorecardVersionCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req struct {
		Version string                                `json:"version"`
		Name    string                                `json:"name"`
		Items   []services.TicketQualityScorecardItem `json:"items"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("评分表参数无效"))
		return
	}
	value, err := services.CreateTicketQualityScorecardVersion(strings.TrimSpace(req.Version), strings.TrimSpace(req.Name), req.Items, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, value)
}

func TicketQualityScorecardVersionPublish(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	if err := services.PublishTicketQualityScorecardVersion(id, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"published": true, "id": id})
}

func TicketQualityReviewList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	value, err := services.ListTicketQualityReviews(ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, value)
}

func TicketQualityReviewCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		Answers        map[string]int `json:"answers"`
		Remark         string         `json:"remark"`
		DefectCodes    []string       `json:"defect_codes"`
		Evidence       string         `json:"evidence"`
		DisputeStatus  string         `json:"dispute_status"`
		DisputeNote    string         `json:"dispute_note"`
		Outcome        string         `json:"outcome"`
		CoachingAction string         `json:"coaching_action"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("评分参数无效"))
		return
	}
	value, err := services.CreateTicketQualityReviewWithMetadata(ticketID, req.Answers, strings.TrimSpace(req.Remark), services.TicketQualityReviewMetadata{
		DefectCodes: req.DefectCodes, Evidence: req.Evidence, DisputeStatus: req.DisputeStatus, DisputeNote: req.DisputeNote, Outcome: req.Outcome, CoachingAction: req.CoachingAction,
	}, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, value)
}

func TicketQualityAnalysisGet(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	parseDate := func(value string, end bool) time.Time {
		value = strings.TrimSpace(value)
		if value == "" {
			return time.Time{}
		}
		if parsed, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
			return parsed
		}
		parsed, parseErr := time.Parse("2006-01-02", value)
		if parseErr != nil {
			return time.Time{}
		}
		if end {
			return parsed.Add(24 * time.Hour)
		}
		return parsed
	}
	teamID, _ := strconv.ParseInt(ctx.Query("team_id"), 10, 64)
	agentID, _ := strconv.ParseInt(ctx.Query("agent_id"), 10, 64)
	filter := services.TicketQualityAnalysisFilter{Project: strings.TrimSpace(ctx.Query("project")), TeamID: teamID, AgentID: agentID, Category: strings.TrimSpace(ctx.Query("category")), Channel: strings.TrimSpace(ctx.Query("channel")), From: parseDate(ctx.Query("from"), false), To: parseDate(ctx.Query("to"), true)}
	value, err := services.AnalyzeTicketQuality(operator.EffectiveTenantID(), filter, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, value)
}
