package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var NotificationRecipientSettingRepository = &notificationRecipientSettingRepository{}

type notificationRecipientSettingRepository struct{}

func (r *notificationRecipientSettingRepository) Get(db *gorm.DB, tenantID, userID int64) *models.NotificationRecipientSetting {
	item := &models.NotificationRecipientSetting{}
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *notificationRecipientSettingRepository) Upsert(db *gorm.DB, item *models.NotificationRecipientSetting) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"email_override": item.EmailOverride,
			"email_enabled":  item.EmailEnabled,
			"updated_at":     item.UpdatedAt,
		}),
	}).Select("tenant_id", "user_id", "email_override", "email_enabled", "created_at", "updated_at").Create(item).Error
}
