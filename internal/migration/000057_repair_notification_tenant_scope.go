package migration

import (
	"fmt"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/routes"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(57, "repair notification tenant scope and enterprise action urls", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return repairNotificationTenantScope(ctx.Tx)
		})
	})
}

func repairNotificationTenantScope(tx *gorm.DB) error {
	var notifications []models.Notification
	if err := tx.Where("tenant_id = ?", 0).Order("id ASC").Find(&notifications).Error; err != nil {
		return err
	}
	for _, notification := range notifications {
		tenantID := notificationResourceTenantID(tx, notification)
		if tenantID <= 0 && notification.RecipientUserID > 0 {
			var member models.TenantMember
			if err := tx.Where("user_id = ? AND status = ?", notification.RecipientUserID, enums.StatusOk).
				Order("id ASC").First(&member).Error; err == nil {
				tenantID = member.TenantID
			}
		}
		if tenantID <= 0 {
			continue
		}
		category, level := deriveNotificationCategoryLevel(notification.NotificationType, notification.BizType)
		updates := map[string]any{
			"tenant_id": tenantID,
			"category":  category,
			"level":     level,
		}
		if strings.TrimSpace(notification.RecipientName) == "" && notification.RecipientUserID > 0 {
			updates["recipient_name"] = notificationRecipientName(tx, tenantID, notification.RecipientUserID)
		}
		if actionURL := enterpriseNotificationActionURL(notification); actionURL != "" {
			updates["action_url"] = actionURL
		}
		if err := tx.Model(&models.Notification{}).Where("id = ? AND tenant_id = ?", notification.ID, 0).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func notificationResourceTenantID(tx *gorm.DB, notification models.Notification) int64 {
	if notification.BizID <= 0 {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(notification.BizType)) {
	case "product":
		var item models.Product
		if err := tx.Select("tenant_id").First(&item, "id = ?", notification.BizID).Error; err == nil {
			return item.TenantID
		}
	case "ticket":
		var item models.Ticket
		if err := tx.Select("tenant_id").First(&item, "id = ?", notification.BizID).Error; err == nil {
			return item.TenantID
		}
	case "conversation":
		var item models.Conversation
		if err := tx.Select("tenant_id").First(&item, "id = ?", notification.BizID).Error; err == nil {
			return item.TenantID
		}
	}
	return 0
}

func notificationRecipientName(tx *gorm.DB, tenantID, userID int64) string {
	var member models.TenantMember
	if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error; err == nil {
		if name := strings.TrimSpace(member.DisplayName); name != "" {
			return name
		}
	}
	var user models.User
	if err := tx.First(&user, "id = ?", userID).Error; err == nil {
		if name := strings.TrimSpace(user.Nickname); name != "" {
			return name
		}
		return strings.TrimSpace(user.Username)
	}
	return ""
}

func enterpriseNotificationActionURL(notification models.Notification) string {
	if notification.BizID <= 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(notification.BizType)) {
	case "product":
		return fmt.Sprintf("/enterprise/products/%d", notification.BizID)
	case "ticket":
		return routes.EnterpriseTicketWorkbenchPath(notification.BizID)
	case "conversation":
		return routes.EnterpriseConversationWorkbenchPath(notification.BizID)
	default:
		return ""
	}
}
