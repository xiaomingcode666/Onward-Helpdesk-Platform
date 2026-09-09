package builders

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func BuildEnterpriseWorkbenchConversations(items []models.Conversation) []dto.EnterpriseWorkbenchConversationDTO {
	result := make([]dto.EnterpriseWorkbenchConversationDTO, 0, len(items))
	now := time.Now()
	customerLastSeenByConversationID := services.ConversationService.BuildConversationCustomerLastSeenAtMap(items)
	for i := range items {
		item := items[i]
		customerLastSeenAt := customerLastSeenByConversationID[item.ID]
		row := dto.EnterpriseWorkbenchConversationDTO{
			ID:                  item.ID,
			CustomerID:          item.CustomerID,
			CustomerName:        item.CustomerName,
			Status:              int(item.Status),
			ServiceMode:         int(item.ServiceMode),
			Priority:            item.Priority,
			CurrentAssigneeID:   item.CurrentAssigneeID,
			CurrentTeamID:       item.CurrentTeamID,
			ChannelID:           item.ChannelID,
			LastMessageAt:       utils.FormatTime(item.LastMessageAt),
			LastActiveAt:        utils.FormatTime(item.LastActiveAt),
			LastMessageSummary:  item.LastMessageSummary,
			AgentUnreadCount:    item.AgentUnreadCount,
			CustomerUnreadCount: item.CustomerUnreadCount,
			CustomerOnline:      services.ConversationService.IsCustomerLastSeenOnline(customerLastSeenAt, now),
			CustomerLastSeenAt:  utils.FormatTimePtr(customerLastSeenAt),
		}
		if item.CurrentAssigneeID > 0 {
			if user := repositories.UserRepository.Get(sqls.DB(), item.CurrentAssigneeID); user != nil {
				row.CurrentAssigneeName = user.Nickname
				if row.CurrentAssigneeName == "" {
					row.CurrentAssigneeName = user.Username
				}
			}
		}
		if item.CurrentTeamID > 0 {
			if team := repositories.AgentTeamRepository.Get(sqls.DB(), item.CurrentTeamID); team != nil {
				row.CurrentTeamName = team.Name
			}
		}
		if item.ProductID > 0 {
			if product := repositories.ProductRepository.Get(sqls.DB(), item.ProductID); product != nil && product.TenantID == item.TenantID {
				row.ProductName = product.Name
			}
		}
		if item.DeviceID > 0 {
			if device := repositories.DeviceRepository.Get(sqls.DB(), item.DeviceID); device != nil && device.TenantID == item.TenantID {
				row.DeviceNo = device.DeviceNo
			}
		}
		result = append(result, row)
	}
	return result
}
