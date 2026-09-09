package repositories

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var AIWorkflowEffectRepository = &aiWorkflowEffectRepository{}

type aiWorkflowEffectRepository struct{}

func (r *aiWorkflowEffectRepository) GetByIdempotencyKey(db *gorm.DB, key string) *models.AIWorkflowEffect {
	ret := &models.AIWorkflowEffect{}
	if err := db.First(ret, "idempotency_key = ?", key).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiWorkflowEffectRepository) Create(db *gorm.DB, item *models.AIWorkflowEffect) error {
	return db.Create(item).Error
}

func (r *aiWorkflowEffectRepository) TryAcquire(db *gorm.DB, id int64, now time.Time, lockedUntil time.Time, requestData string) (bool, error) {
	result := db.Model(&models.AIWorkflowEffect{}).
		Where("id = ? AND status <> ? AND (locked_until IS NULL OR locked_until < ?)", id, models.AIWorkflowEffectStatusSucceeded, now).
		Updates(map[string]any{
			"status":       models.AIWorkflowEffectStatusRunning,
			"attempt":      gorm.Expr("attempt + 1"),
			"request_data": requestData,
			"last_error":   "",
			"locked_until": lockedUntil,
			"updated_at":   now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *aiWorkflowEffectRepository) MarkSucceeded(db *gorm.DB, id int64, attempt int, resultData string, completedAt time.Time) error {
	result := db.Model(&models.AIWorkflowEffect{}).
		Where("id = ? AND status = ? AND attempt = ?", id, models.AIWorkflowEffectStatusRunning, attempt).
		Updates(map[string]any{
			"status":       models.AIWorkflowEffectStatusSucceeded,
			"result_data":  resultData,
			"last_error":   "",
			"locked_until": nil,
			"completed_at": completedAt,
			"updated_at":   completedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("workflow effect lease is stale")
	}
	return nil
}

func (r *aiWorkflowEffectRepository) MarkFailed(db *gorm.DB, id int64, attempt int, errorMessage string, failedAt time.Time) error {
	result := db.Model(&models.AIWorkflowEffect{}).
		Where("id = ? AND status = ? AND attempt = ?", id, models.AIWorkflowEffectStatusRunning, attempt).
		Updates(map[string]any{
			"status":       models.AIWorkflowEffectStatusFailed,
			"last_error":   errorMessage,
			"locked_until": nil,
			"updated_at":   failedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("workflow effect lease is stale")
	}
	return nil
}
