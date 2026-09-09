package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductKnowledgeLinkRepository = newProductKnowledgeLinkRepository()

func newProductKnowledgeLinkRepository() *productKnowledgeLinkRepository {
	return &productKnowledgeLinkRepository{}
}

type productKnowledgeLinkRepository struct {
}

func (r *productKnowledgeLinkRepository) Get(db *gorm.DB, id int64) *models.ProductKnowledgeLink {
	ret := &models.ProductKnowledgeLink{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productKnowledgeLinkRepository) Take(db *gorm.DB, where ...any) *models.ProductKnowledgeLink {
	ret := &models.ProductKnowledgeLink{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productKnowledgeLinkRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductKnowledgeLink) {
	cnd.Find(db, &list)
	return
}

func (r *productKnowledgeLinkRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductKnowledgeLink {
	ret := &models.ProductKnowledgeLink{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productKnowledgeLinkRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductKnowledgeLink, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductKnowledgeLink{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productKnowledgeLinkRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductKnowledgeLink{})
}

func (r *productKnowledgeLinkRepository) Create(db *gorm.DB, t *models.ProductKnowledgeLink) error {
	return db.Create(t).Error
}

func (r *productKnowledgeLinkRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductKnowledgeLink{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productKnowledgeLinkRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductKnowledgeLink{}, "id = ?", id)
}

func (r *productKnowledgeLinkRepository) FindByProductID(db *gorm.DB, productID int64) []models.ProductKnowledgeLink {
	var list []models.ProductKnowledgeLink
	db.Where("product_id = ?", productID).Find(&list)
	return list
}

func (r *productKnowledgeLinkRepository) FindByProductAndType(db *gorm.DB, productID int64, linkType string) []models.ProductKnowledgeLink {
	var list []models.ProductKnowledgeLink
	db.Where("product_id = ? AND link_type = ?", productID, linkType).Find(&list)
	return list
}
