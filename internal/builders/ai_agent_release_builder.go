package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

func BuildAIAgentRelease(item *models.AIAgentRelease, locale string) response.AIAgentReleaseResponse {
	if item == nil {
		return response.AIAgentReleaseResponse{}
	}
	return response.AIAgentReleaseResponse{
		ID:                     item.ID,
		TenantID:               item.TenantID,
		ProductID:              item.ProductID,
		AgentID:                item.AgentID,
		ReleaseNo:              item.ReleaseNo,
		WorkflowID:             item.WorkflowID,
		WorkflowVersionID:      item.WorkflowVersionID,
		WorkflowDefinitionHash: item.WorkflowDefinitionHash,
		AgentConfigHash:        item.AgentConfigHash,
		KnowledgeScopeHash:     item.KnowledgeScopeHash,
		ReviewStatus:           item.ReviewStatus,
		ReviewStatusName:       enums.GetAIAgentReviewStatusLabel(item.ReviewStatus, locale),
		ReviewComment:          services.LocalizeSystemComment(item.ReviewComment, locale),
		ReviewedAt:             utils.FormatTimePtr(item.ReviewedAt),
		ReviewedByName:         item.ReviewedByName,
		DeploymentStatus:       item.DeploymentStatus,
		DeployedAt:             utils.FormatTimePtr(item.DeployedAt),
		DeployedByName:         item.DeployedByName,
		RollbackFromReleaseID:  item.RollbackFromReleaseID,
		Status:                 item.Status,
		CreatedAt:              utils.FormatTime(item.CreatedAt),
		UpdatedAt:              utils.FormatTime(item.UpdatedAt),
	}
}

func BuildAIAgentReleaseList(items []models.AIAgentRelease, locale string) []response.AIAgentReleaseResponse {
	ret := make([]response.AIAgentReleaseResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildAIAgentRelease(&items[i], locale))
	}
	return ret
}
