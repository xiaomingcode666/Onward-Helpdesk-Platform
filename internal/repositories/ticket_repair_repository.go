package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TicketRepairRepository = newTicketRepairRepository()

// TicketRepairRecordRepository 是 TicketRepairRepository 的别名，
// 供售后维修记录语义的调用方（测试与新服务）使用同一仓储实现。
var TicketRepairRecordRepository = TicketRepairRepository

func newTicketRepairRepository() *ticketRepairRepository {
	return &ticketRepairRepository{}
}

type ticketRepairRepository struct {
}

func (r *ticketRepairRepository) Get(db *gorm.DB, id int64) *models.TicketRepairRecord {
	ret := &models.TicketRepairRecord{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepairRepository) Take(db *gorm.DB, where ...any) *models.TicketRepairRecord {
	ret := &models.TicketRepairRecord{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepairRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketRepairRecord) {
	cnd.Find(db, &list)
	return
}

func (r *ticketRepairRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.TicketRepairRecord {
	ret := &models.TicketRepairRecord{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepairRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketRepairRecord, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.TicketRepairRecord{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *ticketRepairRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.TicketRepairRecord{})
}

func (r *ticketRepairRepository) Create(db *gorm.DB, t *models.TicketRepairRecord) error {
	return db.Create(t).Error
}

func (r *ticketRepairRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.TicketRepairRecord{}).Where("id = ?", id).Updates(columns).Error
}

func (r *ticketRepairRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.TicketRepairRecord{}, "id = ?", id)
}

func (r *ticketRepairRepository) FindByTicketID(db *gorm.DB, ticketID int64) []models.TicketRepairRecord {
	var list []models.TicketRepairRecord
	db.Where("ticket_id = ?", ticketID).Order("id ASC").Find(&list)
	return list
}

func (r *ticketRepairRepository) FindByDeviceID(db *gorm.DB, deviceID int64) []models.TicketRepairRecord {
	var list []models.TicketRepairRecord
	db.Where("device_id = ?", deviceID).Find(&list)
	return list
}
