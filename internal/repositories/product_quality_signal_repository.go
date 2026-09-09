package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductQualitySignalRepository = newProductQualitySignalRepository()

func newProductQualitySignalRepository() *productQualitySignalRepository {
	return &productQualitySignalRepository{}
}

type productQualitySignalRepository struct{}

func (r *productQualitySignalRepository) Get(db *gorm.DB, id int64) *models.ProductQualitySignal {
	ret := &models.ProductQualitySignal{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productQualitySignalRepository) Take(db *gorm.DB, where ...any) *models.ProductQualitySignal {
	ret := &models.ProductQualitySignal{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productQualitySignalRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductQualitySignal) {
	cnd.Find(db, &list)
	return
}

func (r *productQualitySignalRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductQualitySignal {
	ret := &models.ProductQualitySignal{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productQualitySignalRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductQualitySignal, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductQualitySignal{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productQualitySignalRepository) Create(db *gorm.DB, t *models.ProductQualitySignal) error {
	return db.Create(t).Error
}

func (r *productQualitySignalRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductQualitySignal{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productQualitySignalRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductQualitySignal{})
}
