package builders

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func BuildCustomerPortalDeviceAccess(deviceID int64, accessible bool) *dto.CustomerPortalDeviceAccessDTO {
	return &dto.CustomerPortalDeviceAccessDTO{
		DeviceID:   deviceID,
		Accessible: accessible,
	}
}

func BuildCustomerAccountDeletion(item *models.DSARRequest) *dto.CustomerAccountDeletionDTO {
	if item == nil {
		return &dto.CustomerAccountDeletionDTO{Status: "not_requested"}
	}
	completedAt := ""
	if item.CompletedAt != nil {
		completedAt = item.CompletedAt.Format(time.RFC3339)
	}
	return &dto.CustomerAccountDeletionDTO{
		RequestID:            item.ID,
		Status:               item.Status,
		RequestedAt:          item.RequestedAt.Format(time.RFC3339),
		DeadlineAt:           item.DeadlineAt.Format(time.RFC3339),
		CompletedAt:          completedAt,
		AccountAccessRevoked: item.Status == "completed",
	}
}
