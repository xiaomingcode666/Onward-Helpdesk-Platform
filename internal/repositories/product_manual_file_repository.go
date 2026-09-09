package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductManualFileRepository = newProductManualFileRepository()

func newProductManualFileRepository() *productManualFileRepository {
	return &productManualFileRepository{}
}

type productManualFileRepository struct{}

func (r *productManualFileRepository) Get(db *gorm.DB, id int64) *models.ProductManualFile {
	ret := &models.ProductManualFile{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productManualFileRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductManualFile) {
	cnd.Find(db, &list)
	return
}

func (r *productManualFileRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductManualFile {
	ret := &models.ProductManualFile{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productManualFileRepository) Create(db *gorm.DB, item *models.ProductManualFile) error {
	return db.Create(item).Error
}

func (r *productManualFileRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductManualFile{}).Where("id = ?", id).Updates(columns).Error
}
