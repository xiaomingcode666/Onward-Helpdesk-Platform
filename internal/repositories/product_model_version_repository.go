package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductModelVersionRepository = newProductModelVersionRepository()

func newProductModelVersionRepository() *productModelVersionRepository {
	return &productModelVersionRepository{}
}

type productModelVersionRepository struct{}

func (r *productModelVersionRepository) Get(db *gorm.DB, id int64) *models.ProductModelVersion {
	ret := &models.ProductModelVersion{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModelVersionRepository) Take(db *gorm.DB, where ...any) *models.ProductModelVersion {
	ret := &models.ProductModelVersion{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModelVersionRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModelVersion) {
	cnd.Find(db, &list)
	return
}

func (r *productModelVersionRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductModelVersion {
	ret := &models.ProductModelVersion{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productModelVersionRepository) Create(db *gorm.DB, t *models.ProductModelVersion) error {
	return db.Create(t).Error
}

func (r *productModelVersionRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductModelVersion{}).Where("id = ?", id).Updates(columns).Error
}
