package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ProductResourceProvisioningJobRepository = &productResourceProvisioningJobRepository{}

type productResourceProvisioningJobRepository struct{}

func (r *productResourceProvisioningJobRepository) Get(db *gorm.DB, id int64) *models.ProductResourceProvisioningJob {
	ret := &models.ProductResourceProvisioningJob{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productResourceProvisioningJobRepository) GetByIdempotencyKey(db *gorm.DB, key string) *models.ProductResourceProvisioningJob {
	ret := &models.ProductResourceProvisioningJob{}
	if err := db.First(ret, "idempotency_key = ?", key).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productResourceProvisioningJobRepository) LatestByProduct(db *gorm.DB, tenantID, productID int64) *models.ProductResourceProvisioningJob {
	ret := &models.ProductResourceProvisioningJob{}
	if err := db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).Order("id DESC").First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productResourceProvisioningJobRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductResourceProvisioningJob) {
	cnd.Find(db, &list)
	return
}

func (r *productResourceProvisioningJobRepository) Create(db *gorm.DB, item *models.ProductResourceProvisioningJob) error {
	return db.Create(item).Error
}

func (r *productResourceProvisioningJobRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductResourceProvisioningJob{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productResourceProvisioningJobRepository) ClaimDueJobs(db *gorm.DB, now time.Time, limit int, lockOwner string) ([]models.ProductResourceProvisioningJob, error) {
	if limit <= 0 {
		limit = 20
	}
	var jobs []models.ProductResourceProvisioningJob
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("(status = ? OR status = ?)", "pending", "waiting_retry").
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
		if err := tx.Model(&models.ProductResourceProvisioningJob{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":     "running",
				"locked_at":  now,
				"lock_owner": lockOwner,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		for i := range jobs {
			jobs[i].Status = "running"
			jobs[i].LockedAt = &now
			jobs[i].LockOwner = lockOwner
		}
		return nil
	})
	return jobs, err
}

func (r *productResourceProvisioningJobRepository) RecoverExpiredRunningJobs(db *gorm.DB, expiredBefore time.Time, nextAttemptAt time.Time) (int64, error) {
	result := db.Model(&models.ProductResourceProvisioningJob{}).
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
