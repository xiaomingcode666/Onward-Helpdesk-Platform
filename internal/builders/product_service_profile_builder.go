package builders

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildProductServiceProfile(item *models.ProductServiceProfile) *response.ProductServiceProfileResponse {
	if item == nil {
		return nil
	}
	return &response.ProductServiceProfileResponse{
		ID:                     item.ID,
		TenantID:               item.TenantID,
		ProductID:              item.ProductID,
		SupportLocalesJSON:     strings.TrimSpace(item.SupportLocalesJSON),
		SupportRegionsJSON:     strings.TrimSpace(item.SupportRegionsJSON),
		WarrantyPolicyJSON:     strings.TrimSpace(item.WarrantyPolicyJSON),
		SafetyLevel:            strings.TrimSpace(item.SafetyLevel),
		DefaultFlowTemplateID:  item.DefaultFlowTemplateID,
		DefaultKnowledgeBaseID: item.DefaultKnowledgeBaseID,
		DefaultAIAgentID:       item.DefaultAIAgentID,
		MeetingEnabled:         item.MeetingEnabled,
		ServicePolicyJSON:      strings.TrimSpace(item.ServicePolicyJSON),
		Status:                 int(item.Status),
		CreatedAt:              utils.FormatTime(item.CreatedAt),
		UpdatedAt:              utils.FormatTime(item.UpdatedAt),
	}
}

func BuildProductServiceProfileList(list []models.ProductServiceProfile) []response.ProductServiceProfileResponse {
	results := make([]response.ProductServiceProfileResponse, 0, len(list))
	for _, item := range list {
		if ret := BuildProductServiceProfile(&item); ret != nil {
			results = append(results, *ret)
		}
	}
	return results
}
