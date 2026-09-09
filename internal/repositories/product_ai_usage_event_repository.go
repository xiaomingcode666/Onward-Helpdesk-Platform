package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductAIUsageEventRepository = newProductAIUsageEventRepository()

func newProductAIUsageEventRepository() *productAIUsageEventRepository {
	return &productAIUsageEventRepository{}
}

type productAIUsageEventRepository struct{}

func (r *productAIUsageEventRepository) Get(db *gorm.DB, id int64) *models.ProductAIUsageEvent {
	ret := &models.ProductAIUsageEvent{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productAIUsageEventRepository) Take(db *gorm.DB, where ...any) *models.ProductAIUsageEvent {
	ret := &models.ProductAIUsageEvent{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productAIUsageEventRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductAIUsageEvent) {
	cnd.Find(db, &list)
	return
}

func (r *productAIUsageEventRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductAIUsageEvent {
	ret := &models.ProductAIUsageEvent{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productAIUsageEventRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductAIUsageEvent, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductAIUsageEvent{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productAIUsageEventRepository) Create(db *gorm.DB, t *models.ProductAIUsageEvent) error {
	return db.Create(t).Error
}

func (r *productAIUsageEventRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductAIUsageEvent{})
}
