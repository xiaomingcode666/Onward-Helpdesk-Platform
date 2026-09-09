package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var IndustrySolutionPackRepository = &industrySolutionPackRepository{}

type industrySolutionPackRepository struct{}

func (r *industrySolutionPackRepository) GetPack(db *gorm.DB, tenantID, packID int64) (*models.IndustrySolutionPack, error) {
	pack := &models.IndustrySolutionPack{}
	err := db.Where("tenant_id = ? AND id = ?", tenantID, packID).First(pack).Error
	if err != nil {
		return nil, err
	}
	return pack, nil
}

func (r *industrySolutionPackRepository) ListResources(db *gorm.DB, tenantID, packID int64) ([]models.IndustrySolutionPackResource, error) {
	var resources []models.IndustrySolutionPackResource
	err := db.Where("tenant_id = ? AND pack_id = ?", tenantID, packID).
		Order("apply_order ASC, resource_type ASC, resource_key ASC, id ASC").
		Find(&resources).Error
	return resources, err
}

func (r *industrySolutionPackRepository) FindApplicationByKey(db *gorm.DB, tenantID int64, idempotencyKey string) (*models.IndustrySolutionPackApplication, error) {
	application := &models.IndustrySolutionPackApplication{}
	err := db.Where("tenant_id = ? AND idempotency_key = ?", tenantID, idempotencyKey).First(application).Error
	if err != nil {
		return nil, err
	}
	return application, nil
}

func (r *industrySolutionPackRepository) GetApplication(db *gorm.DB, tenantID, applicationID int64) (*models.IndustrySolutionPackApplication, error) {
	application := &models.IndustrySolutionPackApplication{}
	err := db.Where("tenant_id = ? AND id = ?", tenantID, applicationID).First(application).Error
	if err != nil {
		return nil, err
	}
	return application, nil
}

func (r *industrySolutionPackRepository) ListApplicationItems(db *gorm.DB, tenantID, applicationID int64) ([]models.IndustrySolutionPackApplicationItem, error) {
	var items []models.IndustrySolutionPackApplicationItem
	err := db.Where("tenant_id = ? AND application_id = ?", tenantID, applicationID).
		Order("apply_order ASC, resource_type ASC, resource_key ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *industrySolutionPackRepository) CreateApplicationWithItems(
	db *gorm.DB,
	application *models.IndustrySolutionPackApplication,
	items []models.IndustrySolutionPackApplicationItem,
) (bool, error) {
	created := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(application)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		created = true
		for i := range items {
			items[i].ApplicationID = application.ID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return created, err
}

func (r *industrySolutionPackRepository) UpdateApplication(db *gorm.DB, tenantID, applicationID int64, updates map[string]any) error {
	result := db.Model(&models.IndustrySolutionPackApplication{}).
		Where("tenant_id = ? AND id = ?", tenantID, applicationID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *industrySolutionPackRepository) UpdateApplicationItem(db *gorm.DB, tenantID, applicationID, itemID int64, updates map[string]any) error {
	result := db.Model(&models.IndustrySolutionPackApplicationItem{}).
		Where("tenant_id = ? AND application_id = ? AND id = ?", tenantID, applicationID, itemID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *industrySolutionPackRepository) ClaimApplication(
	db *gorm.DB,
	tenantID, applicationID int64,
	leaseToken string,
	operatorID int64,
	operatorName string,
	now, leaseUntil time.Time,
) (bool, error) {
	claimableStatuses := []string{
		models.IndustrySolutionApplicationStatusPending,
		models.IndustrySolutionApplicationStatusRunning,
		models.IndustrySolutionApplicationStatusFailed,
		models.IndustrySolutionApplicationStatusPartial,
	}
	result := db.Model(&models.IndustrySolutionPackApplication{}).
		Where("tenant_id = ? AND id = ?", tenantID, applicationID).
		Where("status IN ?", claimableStatuses).
		Where("lease_until IS NULL OR lease_until < ?", now).
		Updates(map[string]any{
			"status":           models.IndustrySolutionApplicationStatusRunning,
			"lease_token":      leaseToken,
			"lease_until":      leaseUntil,
			"started_at":       gorm.Expr("COALESCE(started_at, ?)", now),
			"completed_at":     nil,
			"last_error":       "",
			"update_user_id":   operatorID,
			"update_user_name": operatorName,
			"updated_at":       now,
		})
	return result.RowsAffected == 1, result.Error
}
