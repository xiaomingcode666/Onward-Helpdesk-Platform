package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AIReplyJobRepository = &aiReplyJobRepository{}

type aiReplyJobRepository struct{}

func (r *aiReplyJobRepository) Create(db *gorm.DB, item *models.AIReplyJob) error {
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "message_id"}}, DoNothing: true}).Create(item).Error
}

func (r *aiReplyJobRepository) GetByMessageID(db *gorm.DB, messageID int64) *models.AIReplyJob {
	ret := &models.AIReplyJob{}
	if err := db.First(ret, "message_id = ?", messageID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiReplyJobRepository) HasEarlierUnfinishedJob(db *gorm.DB, conversationID, jobID int64) (bool, error) {
	if db == nil || conversationID <= 0 || jobID <= 0 {
		return false, nil
	}
	var count int64
	err := db.Model(&models.AIReplyJob{}).
		Where("conversation_id = ? AND id < ?", conversationID, jobID).
		Where("status IN ?", []string{
			models.AIReplyJobStatusPending,
			models.AIReplyJobStatusRunning,
			models.AIReplyJobStatusWaitingRetry,
		}).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func (r *aiReplyJobRepository) ClaimByMessageID(db *gorm.DB, messageID int64, now time.Time, lockOwner string) (*models.AIReplyJob, error) {
	result := db.Model(&models.AIReplyJob{}).
		Where("message_id = ?", messageID).
		Where("status IN ?", []string{models.AIReplyJobStatusPending, models.AIReplyJobStatusWaitingRetry}).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Updates(map[string]any{
			"status":          models.AIReplyJobStatusRunning,
			"locked_at":       now,
			"lock_owner":      lockOwner,
			"last_attempt_at": now,
			"started_at":      gorm.Expr("COALESCE(started_at, ?)", now),
			"updated_at":      now,
		})
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	job := r.GetByMessageID(db, messageID)
	if job == nil || job.LockOwner != lockOwner {
		return nil, nil
	}
	return job, nil
}

func (r *aiReplyJobRepository) ClaimDueJobs(db *gorm.DB, now time.Time, limit int, lockOwner string) ([]models.AIReplyJob, error) {
	if limit <= 0 {
		limit = 20
	}
	var jobs []models.AIReplyJob
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("status IN ?", []string{models.AIReplyJobStatusPending, models.AIReplyJobStatusWaitingRetry}).
			Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
			Order("id ASC").
			Limit(limit)
		if tx.Dialector != nil && tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := query.Find(&jobs).Error; err != nil {
			return err
		}
		if len(jobs) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(jobs))
		for i := range jobs {
			ids = append(ids, jobs[i].ID)
		}
		if err := tx.Model(&models.AIReplyJob{}).
			Where("id IN ?", ids).
			Where("status IN ?", []string{models.AIReplyJobStatusPending, models.AIReplyJobStatusWaitingRetry}).
			Updates(map[string]any{
				"status":          models.AIReplyJobStatusRunning,
				"locked_at":       now,
				"lock_owner":      lockOwner,
				"last_attempt_at": now,
				"started_at":      gorm.Expr("COALESCE(started_at, ?)", now),
				"updated_at":      now,
			}).Error; err != nil {
			return err
		}
		for i := range jobs {
			jobs[i].Status = models.AIReplyJobStatusRunning
			jobs[i].LockedAt = &now
			jobs[i].LastAttemptAt = &now
			jobs[i].LockOwner = lockOwner
		}
		return nil
	})
	return jobs, err
}

func (r *aiReplyJobRepository) Finish(db *gorm.DB, jobID int64, lockOwner, status, lastError string, retryCount int, nextAttemptAt *time.Time) (bool, error) {
	now := time.Now()
	updates := map[string]any{
		"status":          status,
		"retry_count":     retryCount,
		"next_attempt_at": nextAttemptAt,
		"locked_at":       nil,
		"lock_owner":      "",
		"last_error":      lastError,
		"updated_at":      now,
	}
	if status == models.AIReplyJobStatusSucceeded || status == models.AIReplyJobStatusFailed || status == models.AIReplyJobStatusCancelled {
		updates["finished_at"] = now
	}
	result := db.Model(&models.AIReplyJob{}).
		Where("id = ? AND status = ? AND lock_owner = ?", jobID, models.AIReplyJobStatusRunning, lockOwner).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}

func (r *aiReplyJobRepository) Heartbeat(db *gorm.DB, jobID int64, lockOwner string, now time.Time) (bool, error) {
	result := db.Model(&models.AIReplyJob{}).
		Where("id = ? AND status = ? AND lock_owner = ?", jobID, models.AIReplyJobStatusRunning, lockOwner).
		Updates(map[string]any{
			"locked_at":  now,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *aiReplyJobRepository) RecoverExpiredRunningJobs(db *gorm.DB, expiredBefore, nextAttemptAt time.Time) (int64, error) {
	result := db.Model(&models.AIReplyJob{}).
		Where("status = ?", models.AIReplyJobStatusRunning).
		Where("locked_at IS NOT NULL AND locked_at <= ?", expiredBefore).
		Updates(map[string]any{
			"status":          models.AIReplyJobStatusWaitingRetry,
			"next_attempt_at": nextAttemptAt,
			"locked_at":       nil,
			"lock_owner":      "",
			"last_error":      "worker lease expired",
			"updated_at":      nextAttemptAt,
		})
	return result.RowsAffected, result.Error
}
