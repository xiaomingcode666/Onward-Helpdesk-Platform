package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductRepository = newProductRepository()

func newProductRepository() *productRepository {
	return &productRepository{}
}

type productRepository struct {
}

func (r *productRepository) GetByTenantCode(db *gorm.DB, tenantID int64, code string) *models.Product {
	ret := &models.Product{}
	if err := db.First(ret, "tenant_id = ? and code = ?", tenantID, code).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productRepository) Get(db *gorm.DB, id int64) *models.Product {
	ret := &models.Product{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productRepository) FindByIDs(db *gorm.DB, ids []int64) []models.Product {
	if len(ids) == 0 {
		return []models.Product{}
	}
	var list []models.Product
	db.Where("id IN ?", uniqueRepositoryInt64s(ids)).Find(&list)
	return list
}

// GetByTenant 按 id 和 tenant_id 查询产品。
// 多租户环境下强制 tenant_id 过滤。
func (r *productRepository) GetByTenant(db *gorm.DB, id int64, tenantID int64) *models.Product {
	ret := &models.Product{}
	if err := db.First(ret, "id = ? and tenant_id = ?", id, tenantID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productRepository) Take(db *gorm.DB, where ...any) *models.Product {
	ret := &models.Product{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Product) {
	cnd.Find(db, &list)
	return
}

func (r *productRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.Product {
	ret := &models.Product{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *productRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Product, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Product{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *productRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Product{})
}

func (r *productRepository) Create(db *gorm.DB, t *models.Product) error {
	return db.Create(t).Error
}

func (r *productRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.Product{}).Where("id = ?", id).Updates(columns).Error
}

// UpdatesByTenant 按 id 和 tenant_id 更新产品字段。多租户下强制 tenant_id 过滤。
func (r *productRepository) UpdatesByTenant(db *gorm.DB, id int64, tenantID int64, columns map[string]any) error {
	return db.Model(&models.Product{}).Where("id = ? and tenant_id = ?", id, tenantID).Updates(columns).Error
}

func (r *productRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.Product{}, "id = ?", id)
}

// DeleteByTenant 按 id 和 tenant_id 删除产品。多租户下强制 tenant_id 过滤。
func (r *productRepository) DeleteByTenant(db *gorm.DB, id int64, tenantID int64) {
	db.Delete(&models.Product{}, "id = ? and tenant_id = ?", id, tenantID)
}

var ProductLineRepository = newProductLineRepository()

func newProductLineRepository() *productLineRepository {
	return &productLineRepository{}
}

type productLineRepository struct{}

func (r *productLineRepository) Get(db *gorm.DB, id int64) *models.ProductLine {
	ret := &models.ProductLine{}
	if err := db.First(ret, id).Error; err != nil {
		return nil
	}
	return ret
}

// GetByTenant 按 id 和 tenant_id 查询产品线。
func (r *productLineRepository) GetByTenant(db *gorm.DB, id int64, tenantID int64) *models.ProductLine {
	ret := &models.ProductLine{}
	if err := db.First(ret, "id = ? and tenant_id = ?", id, tenantID).Error; err != nil {
		return nil
	}
	return ret
}

// GetByTenantCode 按租户内编码查询产品线。
func (r *productLineRepository) GetByTenantCode(db *gorm.DB, tenantID int64, code string) *models.ProductLine {
	ret := &models.ProductLine{}
	if err := db.First(ret, "tenant_id = ? and code = ?", tenantID, code).Error; err != nil {
		return nil
	}
	return ret
}

// GetByTenantName 按租户内名称查询产品线。
func (r *productLineRepository) GetByTenantName(db *gorm.DB, tenantID int64, name string) *models.ProductLine {
	ret := &models.ProductLine{}
	if err := db.First(ret, "tenant_id = ? and name = ?", tenantID, name).Error; err != nil {
		return nil
	}
	return ret
}

func (r *productLineRepository) Create(db *gorm.DB, t *models.ProductLine) error {
	return db.Create(t).Error
}
