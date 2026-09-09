package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var CustomerRegistrationRepository = &customerRegistrationRepository{}

type customerRegistrationRepository struct{}

func (r *customerRegistrationRepository) GetForTenant(db *gorm.DB, tenantID, id int64) *models.CustomerRegistrationGrant {
	if db == nil || tenantID <= 0 || id <= 0 {
		return nil
	}
	item := &models.CustomerRegistrationGrant{}
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerRegistrationRepository) Create(db *gorm.DB, item *models.CustomerRegistrationGrant) error {
	return db.Create(item).Error
}

func (r *customerRegistrationRepository) FindByTokenHash(db *gorm.DB, tokenHash string) *models.CustomerRegistrationGrant {
	item := &models.CustomerRegistrationGrant{}
	if err := db.Where("token_hash = ?", tokenHash).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerRegistrationRepository) FindByTokenHashForUpdate(db *gorm.DB, tokenHash string) *models.CustomerRegistrationGrant {
	item := &models.CustomerRegistrationGrant{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerRegistrationRepository) FindPendingIDsByEmailForUpdate(db *gorm.DB, tenantID int64, domainType, email string) ([]int64, error) {
	ids := make([]int64, 0)
	if db == nil || tenantID <= 0 {
		return ids, nil
	}
	err := db.Model(&models.CustomerRegistrationGrant{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND domain_type = ? AND email = ? AND status = ?", tenantID, domainType, email, models.CustomerRegistrationGrantPending).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *customerRegistrationRepository) RevokePendingByEmail(db *gorm.DB, tenantID int64, domainType, email string, now time.Time) error {
	return db.Model(&models.CustomerRegistrationGrant{}).
		Where("tenant_id = ? AND domain_type = ? AND email = ? AND status = ?", tenantID, domainType, email, models.CustomerRegistrationGrantPending).
		Updates(map[string]any{"status": models.CustomerRegistrationGrantRevoked, "updated_at": now}).Error
}

func (r *customerRegistrationRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.CustomerRegistrationGrant{}).Where("id = ?", id).Updates(columns).Error
}
