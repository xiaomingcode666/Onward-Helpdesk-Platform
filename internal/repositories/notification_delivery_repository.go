package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var NotificationDeliveryRepository = newNotificationDeliveryRepository()

type notificationDeliveryRepository struct{}

func newNotificationDeliveryRepository() *notificationDeliveryRepository {
	return &notificationDeliveryRepository{}
}

func (r *notificationDeliveryRepository) Get(db *gorm.DB, id int64) *models.DeliveryLog {
	if db == nil || id <= 0 {
		return nil
	}
	var item models.DeliveryLog
	if err := db.First(&item, "id = ?", id).Error; err != nil {
		return nil
	}
	return &item
}

func (r *notificationDeliveryRepository) FindByIdempotencyKey(db *gorm.DB, tenantID int64, key string) *models.DeliveryLog {
	key = strings.TrimSpace(key)
	if db == nil || tenantID <= 0 || key == "" {
		return nil
	}
	var item models.DeliveryLog
	if err := db.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *notificationDeliveryRepository) Create(db *gorm.DB, item *models.DeliveryLog) error {
	return db.Create(item).Error
}

func (r *notificationDeliveryRepository) ClaimDue(db *gorm.DB, now time.Time, limit int, staleBefore time.Time) ([]models.DeliveryLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var candidates []models.DeliveryLog
	if err := db.Where(
		"((status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND updated_at <= ?))",
		[]string{"pending", "waiting_retry"}, now, "processing", staleBefore,
	).Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
		return nil, err
	}

	claimed := make([]models.DeliveryLog, 0, len(candidates))
	for i := range candidates {
		result := db.Model(&models.DeliveryLog{}).
			Where("id = ?", candidates[i].ID).
			Where(
				"((status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND updated_at <= ?))",
				[]string{"pending", "waiting_retry"}, now, "processing", staleBefore,
			).
			Updates(map[string]any{
				"status":          "processing",
				"last_attempt_at": now,
				"updated_at":      now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		item := r.Get(db, candidates[i].ID)
		if item != nil {
			claimed = append(claimed, *item)
		}
	}
	return claimed, nil
}

func (r *notificationDeliveryRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.DeliveryLog{}).Where("id = ?", id).Updates(columns).Error
}
