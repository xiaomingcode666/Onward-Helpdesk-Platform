package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ProductFaultStatsRepository = newProductFaultStatsRepository()

func newProductFaultStatsRepository() *productFaultStatsRepository {
	return &productFaultStatsRepository{}
}

type productFaultStatsRepository struct {
}

func (r *productFaultStatsRepository) Get(db *gorm.DB, id int64) *models.ProductFaultStatsDaily {
	ret := &models.ProductFaultStatsDaily{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productFaultStatsRepository) Take(db *gorm.DB, where ...any) *models.ProductFaultStatsDaily {
	ret := &models.ProductFaultStatsDaily{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productFaultStatsRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductFaultStatsDaily) {
	cnd.Find(db, &list)
	return
}

func (r *productFaultStatsRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductFaultStatsDaily {
	ret := &models.ProductFaultStatsDaily{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productFaultStatsRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductFaultStatsDaily, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductFaultStatsDaily{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productFaultStatsRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductFaultStatsDaily{})
}

func (r *productFaultStatsRepository) Create(db *gorm.DB, t *models.ProductFaultStatsDaily) error {
	return db.Create(t).Error
}

func (r *productFaultStatsRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductFaultStatsDaily{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productFaultStatsRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductFaultStatsDaily{}, "id = ?", id)
}

func (r *productFaultStatsRepository) FindByProductAndDateRange(db *gorm.DB, productID int64, startDate, endDate time.Time) []models.ProductFaultStatsDaily {
	var list []models.ProductFaultStatsDaily
	db.Where("product_id = ? AND bucket_date >= ? AND bucket_date <= ?", productID, startDate, endDate).Find(&list)
	return list
}

func (r *productFaultStatsRepository) FindByTenantProductAndDateRange(db *gorm.DB, tenantID, productID int64, startDate, endDate time.Time) []models.ProductFaultStatsDaily {
	var list []models.ProductFaultStatsDaily
	db.Where("tenant_id = ? AND product_id = ? AND bucket_date >= ? AND bucket_date <= ?", tenantID, productID, startDate, endDate).
		Order("ticket_count DESC").
		Order("id DESC").
		Find(&list)
	return list
}

// DeleteByTenantProductAndDateRange 清空产品某日期范围内的投影行，供历史重建使用。
func (r *productFaultStatsRepository) DeleteByTenantProductAndDateRange(db *gorm.DB, tenantID, productID int64, startDate, endDate time.Time) error {
	return db.Where("tenant_id = ? AND product_id = ? AND bucket_date >= ? AND bucket_date <= ?", tenantID, productID, startDate, endDate).
		Delete(&models.ProductFaultStatsDaily{}).Error
}

func (r *productFaultStatsRepository) UpsertDaily(db *gorm.DB, stat *models.ProductFaultStatsDaily) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "product_id"},
			{Name: "product_model_id"},
			{Name: "fault_code"},
			{Name: "fault_part"},
			{Name: "module_id"},
			{Name: "bucket_date"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"ticket_count":          gorm.Expr("product_fault_stats_daily.ticket_count + ?", stat.TicketCount),
			"repeat_count":          gorm.Expr("product_fault_stats_daily.repeat_count + ?", stat.RepeatCount),
			"low_score_count":       gorm.Expr("product_fault_stats_daily.low_score_count + ?", stat.LowScoreCount),
			"meeting_count":         gorm.Expr("product_fault_stats_daily.meeting_count + ?", stat.MeetingCount),
			"affected_device_count": gorm.Expr("product_fault_stats_daily.affected_device_count + ?", stat.AffectedDeviceCount),
			"updated_at":            stat.UpdatedAt,
		}),
	}).Create(stat).Error
}
