package builders

import (
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

func BuildEnterpriseSLAPolicy(policy *services.SLAPolicy) *dto.EnterpriseSLAPolicyDTO {
	if policy == nil {
		return nil
	}
	return &dto.EnterpriseSLAPolicyDTO{
		ID:                policy.ID,
		TenantID:          policy.TenantID,
		Name:              policy.Name,
		Priority:          policy.Priority,
		FRTMinutes:        policy.FRTMinutes,
		AssignmentMinutes: policy.AssignmentMinutes,
		ResolutionMinutes: policy.ResolutionMinutes,
		CalendarID:        policy.CalendarID,
		Status:            policy.Status,
		Active:            policy.Status == "active",
		CreatedAt:         policy.CreatedAt,
		UpdatedAt:         policy.UpdatedAt,
	}
}

func BuildEnterpriseSLAPolicies(policies []services.SLAPolicy) []dto.EnterpriseSLAPolicyDTO {
	result := make([]dto.EnterpriseSLAPolicyDTO, 0, len(policies))
	for i := range policies {
		if item := BuildEnterpriseSLAPolicy(&policies[i]); item != nil {
			result = append(result, *item)
		}
	}
	return result
}
