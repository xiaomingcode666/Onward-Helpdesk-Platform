package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var CustomerDeviceBindingRepository = newCustomerDeviceBindingRepository()

type customerDeviceBindingRepository struct{}

func newCustomerDeviceBindingRepository() *customerDeviceBindingRepository {
	return &customerDeviceBindingRepository{}
}

func (r *customerDeviceBindingRepository) FindByCustomer(db *gorm.DB, tenantID, deviceID, customerUserID, customerOrgID int64) *models.CustomerDeviceBinding {
	item := &models.CustomerDeviceBinding{}
	query := db.Where("tenant_id = ? and device_id = ?", tenantID, deviceID)
	if customerUserID > 0 && customerOrgID > 0 {
		query = query.Where("customer_user_id = ? and customer_org_id = ?", customerUserID, customerOrgID)
	} else if customerUserID > 0 {
		query = query.Where("customer_user_id = ?", customerUserID)
	} else {
		query = query.Where("customer_org_id = ?", customerOrgID)
	}
	if err := query.First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerDeviceBindingRepository) FindVisibleForCustomer(db *gorm.DB, tenantID, deviceID, customerUserID, customerOrgID int64) *models.CustomerDeviceBinding {
	if customerUserID <= 0 && customerOrgID <= 0 {
		return nil
	}
	item := &models.CustomerDeviceBinding{}
	query := visibleCustomerDeviceBindings(db, customerUserID, customerOrgID).
		Where("tenant_id = ? and device_id = ?", tenantID, deviceID)
	if err := query.Order("customer_user_id DESC").Order("confirmed_at DESC").Order("id DESC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func visibleCustomerDeviceBindings(db *gorm.DB, customerUserID, customerOrgID int64) *gorm.DB {
	query := db.Model(&models.CustomerDeviceBinding{}).Where("status = ?", enums.StatusOk)
	switch {
	case customerUserID > 0 && customerOrgID > 0:
		query = query.Where("(customer_user_id = ? OR (customer_user_id = 0 AND customer_org_id = ?))", customerUserID, customerOrgID)
	case customerUserID > 0:
		query = query.Where("customer_user_id = ?", customerUserID)
	case customerOrgID > 0:
		query = query.Where("customer_org_id = ?", customerOrgID)
	default:
		query = query.Where("1 = 0")
	}
	return query
}

func (r *customerDeviceBindingRepository) Create(db *gorm.DB, item *models.CustomerDeviceBinding) error {
	return db.Create(item).Error
}

func (r *customerDeviceBindingRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.CustomerDeviceBinding{}).Where("id = ?", id).Updates(columns).Error
}

func (r *customerDeviceBindingRepository) FindByDeviceID(db *gorm.DB, tenantID, deviceID int64) []models.CustomerDeviceBinding {
	var list []models.CustomerDeviceBinding
	db.Where("tenant_id = ? and device_id = ?", tenantID, deviceID).Find(&list)
	return list
}
