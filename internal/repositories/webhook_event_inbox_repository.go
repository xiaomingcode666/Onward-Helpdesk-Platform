package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var WebhookEventInboxRepository = newWebhookEventInboxRepository()

type webhookEventInboxRepository struct{}

func newWebhookEventInboxRepository() *webhookEventInboxRepository {
	return &webhookEventInboxRepository{}
}

func (r *webhookEventInboxRepository) Claim(db *gorm.DB, item *models.WebhookEventInbox, now time.Time, lockTTL time.Duration) (*models.WebhookEventInbox, bool, error) {
	if lockTTL <= 0 {
		lockTTL = time.Minute
	}
	lockedUntil := now.Add(lockTTL)
	item.Status = "processing"
	item.AttemptCount = 1
	item.LockedUntil = &lockedUntil
	item.CreatedAt = now
	item.UpdatedAt = now
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return item, true, nil
	}

	var existing models.WebhookEventInbox
	if err := db.Where("provider = ? AND event_id = ?", item.Provider, item.EventID).First(&existing).Error; err != nil {
		return nil, false, err
	}
	if existing.PayloadHash != "" && existing.PayloadHash != item.PayloadHash {
		return &existing, false, nil
	}
	if existing.Status == "processed" {
		return &existing, false, nil
	}
	claim := db.Model(&models.WebhookEventInbox{}).
		Where("id = ? AND status <> ? AND (locked_until IS NULL OR locked_until <= ?)", existing.ID, "processed", now).
		Updates(map[string]any{
			"status":        "processing",
			"attempt_count": gorm.Expr("attempt_count + 1"),
			"locked_until":  lockedUntil,
			"last_error":    "",
			"updated_at":    now,
		})
	if claim.Error != nil {
		return nil, false, claim.Error
	}
	if claim.RowsAffected == 0 {
		return &existing, false, nil
	}
	if err := db.First(&existing, existing.ID).Error; err != nil {
		return nil, false, err
	}
	return &existing, true, nil
}

func (r *webhookEventInboxRepository) ClaimRetryable(db *gorm.DB, now time.Time, lockTTL time.Duration, maxAttempts, limit int) ([]models.WebhookEventInbox, error) {
	if lockTTL <= 0 {
		lockTTL = time.Minute
	}
	if maxAttempts <= 0 {
		maxAttempts = 12
	}
	if limit <= 0 {
		limit = 100
	}
	var candidates []models.WebhookEventInbox
	if err := db.Where(
		"status <> ? AND attempt_count < ? AND payload_json <> ? AND (locked_until IS NULL OR locked_until <= ?)",
		"processed", maxAttempts, "", now,
	).Order("received_at ASC, id ASC").Limit(limit).Find(&candidates).Error; err != nil {
		return nil, err
	}

	lockedUntil := now.Add(lockTTL)
	claimed := make([]models.WebhookEventInbox, 0, len(candidates))
	for i := range candidates {
		result := db.Model(&models.WebhookEventInbox{}).
			Where("id = ? AND status <> ? AND attempt_count < ? AND payload_json <> ? AND (locked_until IS NULL OR locked_until <= ?)",
				candidates[i].ID, "processed", maxAttempts, "", now).
			Updates(map[string]any{
				"status":        "processing",
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"locked_until":  lockedUntil,
				"last_error":    "",
				"updated_at":    now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		var item models.WebhookEventInbox
		if err := db.First(&item, candidates[i].ID).Error; err != nil {
			return nil, err
		}
		claimed = append(claimed, item)
	}
	return claimed, nil
}

func (r *webhookEventInboxRepository) MarkProcessed(db *gorm.DB, id int64, now time.Time) error {
	return db.Model(&models.WebhookEventInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":       "processed",
		"processed_at": now,
		"locked_until": nil,
		"last_error":   "",
		"updated_at":   now,
	}).Error
}

func (r *webhookEventInboxRepository) MarkFailed(db *gorm.DB, id int64, message string, now, retryAt time.Time) error {
	message = strings.TrimSpace(message)
	return db.Model(&models.WebhookEventInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":       "failed",
		"locked_until": retryAt,
		"last_error":   message,
		"updated_at":   now,
	}).Error
}
