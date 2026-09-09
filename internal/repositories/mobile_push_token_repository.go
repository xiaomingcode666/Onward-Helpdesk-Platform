package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var MobilePushTokenRepository = &mobilePushTokenRepository{}

type mobilePushTokenRepository struct{}

func (r *mobilePushTokenRepository) Upsert(db *gorm.DB, item *models.MobilePushToken) error {
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "user_id"},
			{Name: "platform"},
			{Name: "token_fingerprint"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"token_ciphertext": item.TokenCiphertext,
			"device_id":        item.DeviceID,
			"app_version":      item.AppVersion,
			"last_seen_at":     item.LastSeenAt,
			"revoked_at":       nil,
			"updated_at":       item.UpdatedAt,
		}),
	}).Select(
		"tenant_id", "user_id", "platform", "token_fingerprint", "token_ciphertext",
		"device_id", "app_version", "last_seen_at", "revoked_at", "created_at", "updated_at",
	).Create(item).Error
	if result != nil {
		return result
	}
	return db.Where(
		"tenant_id = ? AND user_id = ? AND platform = ? AND token_fingerprint = ?",
		item.TenantID, item.UserID, item.Platform, item.TokenFingerprint,
	).First(item).Error
}

func (r *mobilePushTokenRepository) Revoke(db *gorm.DB, tenantID, userID, id int64, now time.Time) error {
	return db.Model(&models.MobilePushToken{}).
		Where("tenant_id = ? AND user_id = ? AND id = ? AND revoked_at IS NULL", tenantID, userID, id).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (r *mobilePushTokenRepository) FindActiveByUser(db *gorm.DB, tenantID, userID int64) ([]models.MobilePushToken, error) {
	if db == nil || tenantID <= 0 || userID <= 0 {
		return []models.MobilePushToken{}, nil
	}
	var items []models.MobilePushToken
	err := db.Where("tenant_id = ? AND user_id = ? AND revoked_at IS NULL", tenantID, userID).
		Order("last_seen_at DESC").Order("id DESC").Find(&items).Error
	return items, err
}
