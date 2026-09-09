package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductServiceProfileRepository = newProductServiceProfileRepository()

func newProductServiceProfileRepository() *productServiceProfileRepository {
	return &productServiceProfileRepository{}
}

type productServiceProfileRepository struct {
}

func (r *productServiceProfileRepository) GetByProductID(db *gorm.DB, productID int64) *models.ProductServiceProfile {
	ret := &models.ProductServiceProfile{}
	if err := db.First(ret, "product_id = ?", productID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productServiceProfileRepository) Get(db *gorm.DB, id int64) *models.ProductServiceProfile {
	ret := &models.ProductServiceProfile{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productServiceProfileRepository) Take(db *gorm.DB, where ...any) *models.ProductServiceProfile {
	ret := &models.ProductServiceProfile{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productServiceProfileRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductServiceProfile) {
	cnd.Find(db, &list)
	return
}

func (r *productServiceProfileRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductServiceProfile {
	ret := &models.ProductServiceProfile{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productServiceProfileRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductServiceProfile, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductServiceProfile{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productServiceProfileRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductServiceProfile{})
}

func (r *productServiceProfileRepository) ExistsByDefaultFlowTemplateID(db *gorm.DB, workflowID int64) bool {
	if db == nil || workflowID <= 0 {
		return false
	}
	var count int64
	db.Model(&models.ProductServiceProfile{}).
		Where("default_flow_template_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
		Limit(1).
		Count(&count)
	return count > 0
}

func (r *productServiceProfileRepository) Create(db *gorm.DB, t *models.ProductServiceProfile) error {
	return db.Create(t).Error
}

func (r *productServiceProfileRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductServiceProfile{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productServiceProfileRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductServiceProfile{}, "id = ?", id)
}
