package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var TenantRepository = newTenantRepository()

func newTenantRepository() *tenantRepository {
	return &tenantRepository{}
}

type tenantRepository struct {
}

func (r *tenantRepository) Get(db *gorm.DB, id int64) *models.Tenant {
	ret := &models.Tenant{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *tenantRepository) Updates(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&models.Tenant{}).Where("id = ?", id).Updates(updates).Error
}

func (r *tenantRepository) FindAutoCloseEnabled(db *gorm.DB) []models.Tenant {
	items := make([]models.Tenant, 0)
	db.Where("ticket_auto_close_enabled = ? AND status = ?", true, enums.StatusOk).Find(&items)
	return items
}
