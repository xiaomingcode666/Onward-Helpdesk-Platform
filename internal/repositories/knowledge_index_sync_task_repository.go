package repositories

import (
	"remotehelpdesk/internal/models"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeIndexSyncTaskRepository = newKnowledgeIndexSyncTaskRepository()

func newKnowledgeIndexSyncTaskRepository() *knowledgeIndexSyncTaskRepository {
	return &knowledgeIndexSyncTaskRepository{}
}

type knowledgeIndexSyncTaskRepository struct{}

func (r *knowledgeIndexSyncTaskRepository) Get(db *gorm.DB, id int64) *models.KnowledgeIndexSyncTask {
	ret := &models.KnowledgeIndexSyncTask{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeIndexSyncTaskRepository) GetByIdempotencyKey(db *gorm.DB, key string) *models.KnowledgeIndexSyncTask {
	ret := &models.KnowledgeIndexSyncTask{}
	if err := db.First(ret, "idempotency_key = ?", key).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeIndexSyncTaskRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeIndexSyncTask) {
	cnd.Find(db, &list)
	return
}

func (r *knowledgeIndexSyncTaskRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeIndexSyncTask, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.KnowledgeIndexSyncTask{})
	paging = &sqls.Paging{Page: cnd.Paging.Page, Limit: cnd.Paging.Limit, Total: count}
	return
}

func (r *knowledgeIndexSyncTaskRepository) Create(db *gorm.DB, t *models.KnowledgeIndexSyncTask) error {
	return db.Create(t).Error
}

func (r *knowledgeIndexSyncTaskRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.KnowledgeIndexSyncTask{}).Where("id = ?", id).Updates(columns).Error
}

func (r *knowledgeIndexSyncTaskRepository) ClaimDueTasks(db *gorm.DB, now time.Time, limit int, lockOwner string, leaseUntil time.Time) ([]models.KnowledgeIndexSyncTask, error) {
	if limit <= 0 {
		limit = 20
	}
	var tasks []models.KnowledgeIndexSyncTask
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("(status = ? OR status = ?)", "pending", "waiting_retry").
			Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
			Order("id ASC").
			Limit(limit).
			Find(&tasks).Error; err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(tasks))
		for i := range tasks {
			ids = append(ids, tasks[i].ID)
		}
		if err := tx.Model(&models.KnowledgeIndexSyncTask{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":     "running",
				"locked_at":  now,
				"lock_owner": lockOwner,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		for i := range tasks {
			tasks[i].Status = "running"
			tasks[i].LockedAt = &now
			tasks[i].LockOwner = lockOwner
			nextLease := leaseUntil
			_ = nextLease
		}
		return nil
	})
	return tasks, err
}

func (r *knowledgeIndexSyncTaskRepository) RecoverExpiredRunningTasks(db *gorm.DB, expiredBefore time.Time, nextAttemptAt time.Time) (int64, error) {
	result := db.Model(&models.KnowledgeIndexSyncTask{}).
		Where("status = ?", "running").
		Where("locked_at IS NOT NULL AND locked_at <= ?", expiredBefore).
		Updates(map[string]any{
			"status":          "waiting_retry",
			"next_attempt_at": nextAttemptAt,
			"locked_at":       nil,
			"lock_owner":      "",
			"updated_at":      nextAttemptAt,
		})
	return result.RowsAffected, result.Error
}
