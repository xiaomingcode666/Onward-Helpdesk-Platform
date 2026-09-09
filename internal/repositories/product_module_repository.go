package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductModuleRepository = newProductModuleRepository()

func newProductModuleRepository() *productModuleRepository {
	return &productModuleRepository{}
}

type productModuleRepository struct {
}

func (r *productModuleRepository) GetByTenantProductCode(db *gorm.DB, tenantID, productID int64, moduleCode string) *models.ProductModule {
	ret := &models.ProductModule{}
	if err := db.First(ret, "tenant_id = ? and product_id = ? and module_code = ?", tenantID, productID, moduleCode).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModuleRepository) FindByProductID(db *gorm.DB, productID int64) (list []models.ProductModule) {
	db.Find(&list, "product_id = ?", productID)
	return
}

func (r *productModuleRepository) Get(db *gorm.DB, id int64) *models.ProductModule {
	ret := &models.ProductModule{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModuleRepository) Take(db *gorm.DB, where ...any) *models.ProductModule {
	ret := &models.ProductModule{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModuleRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModule) {
	cnd.Find(db, &list)
	return
}

func (r *productModuleRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductModule {
	ret := &models.ProductModule{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productModuleRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModule, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductModule{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productModuleRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductModule{})
}

func (r *productModuleRepository) Create(db *gorm.DB, t *models.ProductModule) error {
	return db.Create(t).Error
}

func (r *productModuleRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductModule{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productModuleRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductModule{}, "id = ?", id)
}
