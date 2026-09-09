package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var DeviceServiceRecordRepository = newDeviceServiceRecordRepository()

func newDeviceServiceRecordRepository() *deviceServiceRecordRepository {
	return &deviceServiceRecordRepository{}
}

type deviceServiceRecordRepository struct {
}

func (r *deviceServiceRecordRepository) Get(db *gorm.DB, id int64) *models.DeviceServiceRecord {
	ret := &models.DeviceServiceRecord{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceServiceRecordRepository) Take(db *gorm.DB, where ...any) *models.DeviceServiceRecord {
	ret := &models.DeviceServiceRecord{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceServiceRecordRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceServiceRecord) {
	cnd.Find(db, &list)
	return
}

func (r *deviceServiceRecordRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.DeviceServiceRecord {
	ret := &models.DeviceServiceRecord{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *deviceServiceRecordRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceServiceRecord, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.DeviceServiceRecord{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *deviceServiceRecordRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.DeviceServiceRecord{})
}

func (r *deviceServiceRecordRepository) Create(db *gorm.DB, t *models.DeviceServiceRecord) error {
	return db.Create(t).Error
}

func (r *deviceServiceRecordRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.DeviceServiceRecord{}).Where("id = ?", id).Updates(columns).Error
}

func (r *deviceServiceRecordRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.DeviceServiceRecord{}, "id = ?", id)
}

func (r *deviceServiceRecordRepository) FindByDeviceID(db *gorm.DB, deviceID int64) []models.DeviceServiceRecord {
	var list []models.DeviceServiceRecord
	db.Where("device_id = ?", deviceID).Order("occurred_at desc").Find(&list)
	return list
}

func (r *deviceServiceRecordRepository) FindByProductID(db *gorm.DB, productID int64) []models.DeviceServiceRecord {
	var list []models.DeviceServiceRecord
	db.Where("product_id = ?", productID).Order("occurred_at desc").Find(&list)
	return list
}

func (r *deviceServiceRecordRepository) FindByTicketID(db *gorm.DB, ticketID int64) []models.DeviceServiceRecord {
	var list []models.DeviceServiceRecord
	db.Where("ticket_id = ?", ticketID).Order("occurred_at desc").Find(&list)
	return list
}
