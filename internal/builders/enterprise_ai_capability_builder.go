package builders

import (
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/services"
)

func BuildEnterpriseAICapabilities(agg *services.EnterpriseAICapabilityAggregate) *response.EnterpriseAICapabilitiesResponse {
	if agg == nil {
		return nil
	}
	ret := &response.EnterpriseAICapabilitiesResponse{
		GeneratedAt:  formatTime(agg.GeneratedAt),
		Capabilities: make([]response.EnterpriseAICapabilityResponse, 0, len(agg.Capabilities)),
	}
	if agg.DefaultCredential != nil {
		ret.DefaultCredential = &response.EnterpriseAIDefaultCredentialResponse{
			KeyName:         agg.DefaultCredential.KeyName,
			KeyStatus:       agg.DefaultCredential.KeyStatus,
			ProvisionStatus: agg.DefaultCredential.ProvisionStatus,
			Ready:           agg.DefaultCredential.Ready,
		}
	}
	for _, item := range agg.Capabilities {
		ret.Capabilities = append(ret.Capabilities, response.EnterpriseAICapabilityResponse{
			Type:      string(item.Type),
			Available: item.Available,
			ModelName: item.ModelName,
			Models:    append([]string(nil), item.Models...),
			Dimension: item.Dimension,
		})
	}
	return ret
}
