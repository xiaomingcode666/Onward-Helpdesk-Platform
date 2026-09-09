package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductKnowledgeBindingRepository = newProductKnowledgeBindingRepository()

func newProductKnowledgeBindingRepository() *productKnowledgeBindingRepository {
	return &productKnowledgeBindingRepository{}
}

type productKnowledgeBindingRepository struct{}

func (r *productKnowledgeBindingRepository) Get(db *gorm.DB, id int64) *models.ProductKnowledgeBinding {
	ret := &models.ProductKnowledgeBinding{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productKnowledgeBindingRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductKnowledgeBinding) {
	cnd.Find(db, &list)
	return
}

func (r *productKnowledgeBindingRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductKnowledgeBinding, paging *sqls.Paging) {
	cnd.Find(db, &list)
	paging = &sqls.Paging{Page: cnd.Paging.Page, Limit: cnd.Paging.Limit, Total: cnd.Count(db, &models.ProductKnowledgeBinding{})}
	return
}

func (r *productKnowledgeBindingRepository) Create(db *gorm.DB, item *models.ProductKnowledgeBinding) error {
	return db.Create(item).Error
}

func (r *productKnowledgeBindingRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductKnowledgeBinding{}).Where("id = ?", id).Updates(columns).Error
}
