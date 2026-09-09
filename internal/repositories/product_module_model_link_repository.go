package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductModuleModelLinkRepository = newProductModuleModelLinkRepository()

func newProductModuleModelLinkRepository() *productModuleModelLinkRepository {
	return &productModuleModelLinkRepository{}
}

type productModuleModelLinkRepository struct {
}

func (r *productModuleModelLinkRepository) Get(db *gorm.DB, id int64) *models.ProductModuleModelLink {
	ret := &models.ProductModuleModelLink{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModuleModelLinkRepository) Take(db *gorm.DB, where ...any) *models.ProductModuleModelLink {
	ret := &models.ProductModuleModelLink{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModuleModelLinkRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModuleModelLink) {
	cnd.Find(db, &list)
	return
}

func (r *productModuleModelLinkRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductModuleModelLink {
	ret := &models.ProductModuleModelLink{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productModuleModelLinkRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModuleModelLink, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductModuleModelLink{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productModuleModelLinkRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductModuleModelLink{})
}

func (r *productModuleModelLinkRepository) Create(db *gorm.DB, t *models.ProductModuleModelLink) error {
	return db.Create(t).Error
}

func (r *productModuleModelLinkRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductModuleModelLink{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productModuleModelLinkRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductModuleModelLink{}, "id = ?", id)
}

func (r *productModuleModelLinkRepository) FindByModuleID(db *gorm.DB, moduleID int64) []models.ProductModuleModelLink {
	var list []models.ProductModuleModelLink
	db.Where("product_module_id = ?", moduleID).Find(&list)
	return list
}

func (r *productModuleModelLinkRepository) FindByModelID(db *gorm.DB, modelID int64) []models.ProductModuleModelLink {
	var list []models.ProductModuleModelLink
	db.Where("product_model_id = ?", modelID).Find(&list)
	return list
}

// DeleteByModuleID 物理删除模块的全部适用型号关联。
// 关联表采用物理删除，配合联合唯一索引支持整体替换（§10.3）。
func (r *productModuleModelLinkRepository) DeleteByModuleID(db *gorm.DB, moduleID int64) error {
	return db.Where("product_module_id = ?", moduleID).Delete(&models.ProductModuleModelLink{}).Error
}
