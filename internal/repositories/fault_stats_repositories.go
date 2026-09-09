package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// FaultStatsEventInboxRepository 故障统计事件消费幂等仓储。
var FaultStatsEventInboxRepository = newFaultStatsEventInboxRepository()

func newFaultStatsEventInboxRepository() *faultStatsEventInboxRepository {
	return &faultStatsEventInboxRepository{}
}

type faultStatsEventInboxRepository struct{}

// Exists 判断事件是否已消费（幂等检查）。
func (r *faultStatsEventInboxRepository) Exists(db *gorm.DB, eventID string) bool {
	var count int64
	db.Model(&models.FaultStatsEventInbox{}).Where("event_id = ?", eventID).Count(&count)
	return count > 0
}

func (r *faultStatsEventInboxRepository) Create(db *gorm.DB, t *models.FaultStatsEventInbox) error {
	return db.Create(t).Error
}

// ProductFaultStatsRebuildJobRepository 故障统计重建任务仓储。
var ProductFaultStatsRebuildJobRepository = newProductFaultStatsRebuildJobRepository()

func newProductFaultStatsRebuildJobRepository() *productFaultStatsRebuildJobRepository {
	return &productFaultStatsRebuildJobRepository{}
}

type productFaultStatsRebuildJobRepository struct{}

func (r *productFaultStatsRebuildJobRepository) Get(db *gorm.DB, id int64) *models.ProductFaultStatsRebuildJob {
	ret := &models.ProductFaultStatsRebuildJob{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productFaultStatsRebuildJobRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductFaultStatsRebuildJob) {
	cnd.Find(db, &list)
	return
}

func (r *productFaultStatsRebuildJobRepository) Create(db *gorm.DB, t *models.ProductFaultStatsRebuildJob) error {
	return db.Create(t).Error
}

func (r *productFaultStatsRebuildJobRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductFaultStatsRebuildJob{}).Where("id = ?", id).Updates(columns).Error
}
