package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductModelRepository = newProductModelRepository()

func newProductModelRepository() *productModelRepository {
	return &productModelRepository{}
}

type productModelRepository struct {
}

func (r *productModelRepository) GetByProductModelCode(db *gorm.DB, productID int64, modelCode string) *models.ProductModel {
	ret := &models.ProductModel{}
	if err := db.First(ret, "product_id = ? and model_code = ?", productID, modelCode).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModelRepository) Get(db *gorm.DB, id int64) *models.ProductModel {
	ret := &models.ProductModel{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModelRepository) FindByIDs(db *gorm.DB, ids []int64) []models.ProductModel {
	if len(ids) == 0 {
		return []models.ProductModel{}
	}
	var list []models.ProductModel
	db.Where("id IN ?", uniqueRepositoryInt64s(ids)).Find(&list)
	return list
}

func (r *productModelRepository) Take(db *gorm.DB, where ...any) *models.ProductModel {
	ret := &models.ProductModel{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productModelRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModel) {
	cnd.Find(db, &list)
	return
}

func (r *productModelRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ProductModel {
	ret := &models.ProductModel{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productModelRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ProductModel, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ProductModel{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productModelRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ProductModel{})
}

func (r *productModelRepository) Create(db *gorm.DB, t *models.ProductModel) error {
	return db.Create(t).Error
}

func (r *productModelRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ProductModel{}).Where("id = ?", id).Updates(columns).Error
}

func (r *productModelRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ProductModel{}, "id = ?", id)
}
