package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var DeviceRepository = newDeviceRepository()

func newDeviceRepository() *deviceRepository {
	return &deviceRepository{}
}

type deviceRepository struct {
}

func (r *deviceRepository) GetByTenantDeviceNo(db *gorm.DB, tenantID int64, deviceNo string) *models.Device {
	ret := &models.Device{}
	if err := db.First(ret, "tenant_id = ? and device_no = ?", tenantID, deviceNo).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRepository) GetByDeviceNo(db *gorm.DB, deviceNo string) *models.Device {
	ret := &models.Device{}
	if err := db.Where("device_no = ?", deviceNo).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRepository) Get(db *gorm.DB, id int64) *models.Device {
	ret := &models.Device{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRepository) Take(db *gorm.DB, where ...any) *models.Device {
	ret := &models.Device{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Device) {
	cnd.Find(db, &list)
	return
}

func (r *deviceRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.Device {
	ret := &models.Device{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *deviceRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Device, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Device{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *deviceRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Device{})
}

func (r *deviceRepository) Create(db *gorm.DB, t *models.Device) error {
	return db.Create(t).Error
}

func (r *deviceRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.Device{}).Where("id = ?", id).Updates(columns).Error
}

func (r *deviceRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.Device{}, "id = ?", id)
}
