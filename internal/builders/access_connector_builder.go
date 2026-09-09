package builders

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func BuildAccessConnectorList(items []*models.AccessConnector) []*dto.AccessConnectorListItemDTO {
	result := make([]*dto.AccessConnectorListItemDTO, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		lastChecked := ""
		if item.LastTestedAt != nil {
			lastChecked = item.LastTestedAt.Format(time.RFC3339)
		}
		result = append(result, &dto.AccessConnectorListItemDTO{
			ID:                item.ID,
			Name:              item.Name,
			ConnectorType:     item.ConnectorType,
			BaseURL:           item.BaseURL,
			AuthType:          item.AuthType,
			Active:            item.Status == "active",
			HealthStatus:      item.HealthStatus,
			LastHealthCheckAt: lastChecked,
			CreatedAt:         item.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}
