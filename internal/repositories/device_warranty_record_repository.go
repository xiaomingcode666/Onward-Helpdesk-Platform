package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var DeviceWarrantyRecordRepository = newDeviceWarrantyRecordRepository()

func newDeviceWarrantyRecordRepository() *deviceWarrantyRecordRepository {
	return &deviceWarrantyRecordRepository{}
}

type deviceWarrantyRecordRepository struct{}

func (r *deviceWarrantyRecordRepository) Get(db *gorm.DB, id int64) *models.DeviceWarrantyRecord {
	ret := &models.DeviceWarrantyRecord{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceWarrantyRecordRepository) Take(db *gorm.DB, where ...any) *models.DeviceWarrantyRecord {
	ret := &models.DeviceWarrantyRecord{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceWarrantyRecordRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceWarrantyRecord) {
	cnd.Find(db, &list)
	return
}

func (r *deviceWarrantyRecordRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.DeviceWarrantyRecord {
	ret := &models.DeviceWarrantyRecord{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *deviceWarrantyRecordRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceWarrantyRecord, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.DeviceWarrantyRecord{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *deviceWarrantyRecordRepository) FindByDeviceID(db *gorm.DB, deviceID int64) []models.DeviceWarrantyRecord {
	var list []models.DeviceWarrantyRecord
	db.Where("device_id = ?", deviceID).Order("end_at desc").Find(&list)
	return list
}

func (r *deviceWarrantyRecordRepository) FindByDeviceIDs(db *gorm.DB, deviceIDs []int64) []models.DeviceWarrantyRecord {
	if len(deviceIDs) == 0 {
		return []models.DeviceWarrantyRecord{}
	}
	var list []models.DeviceWarrantyRecord
	db.Where("device_id IN ?", uniqueRepositoryInt64s(deviceIDs)).
		Order("device_id ASC, end_at DESC, id DESC").
		Find(&list)
	return list
}

func (r *deviceWarrantyRecordRepository) Create(db *gorm.DB, t *models.DeviceWarrantyRecord) error {
	return db.Create(t).Error
}

func (r *deviceWarrantyRecordRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.DeviceWarrantyRecord{}).Where("id = ?", id).Updates(columns).Error
}

func (r *deviceWarrantyRecordRepository) Delete(db *gorm.DB, id int64) error {
	return db.Delete(&models.DeviceWarrantyRecord{}, "id = ?", id).Error
}

func (r *deviceWarrantyRecordRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.DeviceWarrantyRecord{})
}
