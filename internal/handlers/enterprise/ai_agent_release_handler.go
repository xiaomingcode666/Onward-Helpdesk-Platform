package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func AIAgentReleaseCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentReleaseCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	agentID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	item, err := services.AIAgentReleaseService.CreateCandidate(agentID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAIAgentReleaseAudit(ctx, operator, item, "ai_agent_release.created", models.RiskLevelMedium)
	httpx.WriteJSON(ctx, builders.BuildAIAgentRelease(item))
}

func AIAgentReleaseList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	agentID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	items, err := services.AIAgentReleaseService.ListByAgent(agentID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIAgentReleaseList(items))
}

func AIAgentReleaseGet(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	releaseID, ok := httpx.GetPathInt64(ctx, "releaseId")
	if !ok {
		return
	}
	item := services.AIAgentReleaseService.Get(releaseID, operator)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIAgentRelease(item))
}

func AIAgentReleaseSubmitReview(ctx *gin.Context) {
	operator, releaseID, ok := requireReleaseAction(ctx, constants.PermissionAIAgentReleaseCreate)
	if !ok {
		return
	}
	if err := services.AIAgentReleaseService.SubmitReview(releaseID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAIAgentReleaseAudit(ctx, operator, services.AIAgentReleaseService.Get(releaseID, operator), "ai_agent_release.submitted", models.RiskLevelMedium)
	httpx.WriteJSON(ctx, nil)
}

func AIAgentReleaseApprove(ctx *gin.Context) {
	reviewRelease(ctx, true)
}

func AIAgentReleaseReject(ctx *gin.Context) {
	reviewRelease(ctx, false)
}

func reviewRelease(ctx *gin.Context, approved bool) {
	operator, releaseID, ok := requireReleaseAction(ctx, constants.PermissionAIAgentReleaseReview)
	if !ok {
		return
	}
	req := dto.AIAgentReleaseReviewRequest{}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentReleaseService.Review(releaseID, approved, req.Comment, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	action := "ai_agent_release.rejected"
	if approved {
		action = "ai_agent_release.approved"
	}
	recordAIAgentReleaseAudit(ctx, operator, services.AIAgentReleaseService.Get(releaseID, operator), action, models.RiskLevelHigh)
	httpx.WriteJSON(ctx, nil)
}

func AIAgentReleaseDeploy(ctx *gin.Context) {
	operator, releaseID, ok := requireReleaseAction(ctx, constants.PermissionAIAgentReleaseDeploy)
	if !ok {
		return
	}
	if err := services.AIAgentReleaseService.Deploy(releaseID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAIAgentReleaseAudit(ctx, operator, services.AIAgentReleaseService.Get(releaseID, operator), "ai_agent_release.deployed", models.RiskLevelHigh)
	httpx.WriteJSON(ctx, nil)
}

func AIAgentReleaseRollback(ctx *gin.Context) {
	operator, releaseID, ok := requireReleaseAction(ctx, constants.PermissionAIAgentReleaseDeploy)
	if !ok {
		return
	}
	if err := services.AIAgentReleaseService.Rollback(releaseID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordAIAgentReleaseAudit(ctx, operator, services.AIAgentReleaseService.Get(releaseID, operator), "ai_agent_release.rolled_back", models.RiskLevelCritical)
	httpx.WriteJSON(ctx, nil)
}

func recordAIAgentReleaseAudit(ctx *gin.Context, operator *dto.AuthPrincipal, item *models.AIAgentRelease, action, riskLevel string) {
	if ctx == nil || operator == nil || item == nil {
		return
	}
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:     item.TenantID,
		ActorID:      strconv.FormatInt(operator.UserID, 10),
		ActorType:    "user",
		Domain:       "ai_agent",
		ResourceType: "ai_agent_release",
		ResourceID:   strconv.FormatInt(item.ID, 10),
		Action:       action,
		AfterState: map[string]any{
			"agentId":                item.AgentID,
			"productId":              item.ProductID,
			"releaseNo":              item.ReleaseNo,
			"workflowVersionId":      item.WorkflowVersionID,
			"workflowDefinitionHash": item.WorkflowDefinitionHash,
			"agentConfigHash":        item.AgentConfigHash,
			"knowledgeScopeHash":     item.KnowledgeScopeHash,
			"reviewStatus":           item.ReviewStatus,
			"deploymentStatus":       item.DeploymentStatus,
		},
		IPAddress: ctx.ClientIP(),
		UserAgent: ctx.Request.UserAgent(),
		RequestID: ctx.GetHeader("X-Request-Id"),
		RiskLevel: riskLevel,
	})
}

func requireReleaseAction(ctx *gin.Context, permission constants.Permission) (*dto.AuthPrincipal, int64, bool) {
	operator, err := services.AuthService.RequirePermission(ctx, permission)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return nil, 0, false
	}
	releaseID, ok := httpx.GetPathInt64(ctx, "releaseId")
	if !ok {
		return nil, 0, false
	}
	return operator, releaseID, true
}
