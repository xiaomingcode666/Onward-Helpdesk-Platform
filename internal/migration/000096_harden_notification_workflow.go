package migration

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var notificationRecipientSuffixPattern = regexp.MustCompile(`:recipient:[0-9]+$`)

func init() {
	register(96, "group tenant notifications and initialize durable delivery state", func() error {
		return hardenNotificationWorkflow(sqls.DB())
	})
}

func hardenNotificationWorkflow(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Notification{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := backfillNotificationEventKeys(tx); err != nil {
			return err
		}
		if err := expandLegacyBroadcastNotifications(tx); err != nil {
			return err
		}
		if err := tx.Model(&models.Notification{}).
			Where("channels LIKE ? AND external_channel_status = ?", "%email%", "").
			Update("external_channel_status", "email_skipped").Error; err != nil {
			return err
		}
		if err := disablePrototypeMailSetting(tx); err != nil {
			return err
		}
		return backfillDeliveryLogScope(tx)
	})
}

func backfillNotificationEventKeys(tx *gorm.DB) error {
	var items []models.Notification
	if err := tx.Where("event_key = ?", "").Order("id ASC").Find(&items).Error; err != nil {
		return err
	}
	for i := range items {
		eventKey := ""
		if items[i].IdempotencyKey != nil {
			eventKey = notificationRecipientSuffixPattern.ReplaceAllString(strings.TrimSpace(*items[i].IdempotencyKey), "")
		}
		if eventKey == "" {
			eventKey = fmt.Sprintf("legacy:notification:%d", items[i].ID)
		}
		if err := tx.Model(&models.Notification{}).Where("id = ?", items[i].ID).Update("event_key", eventKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func expandLegacyBroadcastNotifications(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&models.TenantMember{}) {
		return nil
	}
	var broadcasts []models.Notification
	if err := tx.Where("recipient_user_id = ? AND status = ?", 0, enums.StatusOk).Order("id ASC").Find(&broadcasts).Error; err != nil {
		return err
	}
	for i := range broadcasts {
		var members []models.TenantMember
		if err := tx.Where("tenant_id = ? AND user_id > ? AND status = ?", broadcasts[i].TenantID, 0, enums.StatusOk).Find(&members).Error; err != nil {
			return err
		}
		if len(members) == 0 {
			continue
		}
		for j := range members {
			copy := broadcasts[i]
			copy.ID = 0
			copy.RecipientUserID = members[j].UserID
			if displayName := strings.TrimSpace(members[j].DisplayName); displayName != "" {
				copy.RecipientName = displayName
			}
			copy.EventKey = fmt.Sprintf("legacy:broadcast:%d", broadcasts[i].ID)
			key := fmt.Sprintf("legacy:broadcast:%d:recipient:%d", broadcasts[i].ID, members[j].UserID)
			copy.IdempotencyKey = &key
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&copy).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.Notification{}).Where("id = ?", broadcasts[i].ID).Update("status", enums.StatusDisabled).Error; err != nil {
			return err
		}
	}
	return nil
}

func disablePrototypeMailSetting(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&models.TenantMailSetting{}) {
		return nil
	}
	return tx.Model(&models.TenantMailSetting{}).
		Where("smtp_host = ? AND from_address = ? AND username = ? AND password = ?", "smtp.enterprise-mail.com", "service-notice@hdjg.com", "", "").
		Updates(map[string]any{
			"smtp_host":    "",
			"from_address": "",
			"from_name":    "",
			"reply_to":     "",
			"status":       enums.StatusDisabled,
			"updated_at":   time.Now(),
		}).Error
}

func backfillDeliveryLogScope(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&models.DeliveryLog{}) {
		return nil
	}
	var logs []models.DeliveryLog
	if err := tx.Where("notification_id > ? AND tenant_id = ?", 0, 0).Find(&logs).Error; err != nil {
		return err
	}
	for i := range logs {
		var notification models.Notification
		if err := tx.Select("tenant_id").First(&notification, "id = ?", logs[i].NotificationID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		updatedAt := logs[i].CreatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}
		if err := tx.Model(&models.DeliveryLog{}).Where("id = ?", logs[i].ID).Updates(map[string]any{
			"tenant_id":  notification.TenantID,
			"updated_at": updatedAt,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
