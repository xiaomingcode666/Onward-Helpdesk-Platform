package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ServiceCodeBatchRepository = newServiceCodeBatchRepository()

func newServiceCodeBatchRepository() *serviceCodeBatchRepository {
	return &serviceCodeBatchRepository{}
}

type serviceCodeBatchRepository struct {
}

func (r *serviceCodeBatchRepository) GetByTenantBatchNo(db *gorm.DB, tenantID int64, batchNo string) *models.ServiceCodeBatch {
	ret := &models.ServiceCodeBatch{}
	if err := db.First(ret, "tenant_id = ? and batch_no = ?", tenantID, batchNo).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeBatchRepository) Get(db *gorm.DB, id int64) *models.ServiceCodeBatch {
	ret := &models.ServiceCodeBatch{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeBatchRepository) Take(db *gorm.DB, where ...any) *models.ServiceCodeBatch {
	ret := &models.ServiceCodeBatch{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeBatchRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ServiceCodeBatch) {
	cnd.Find(db, &list)
	return
}

func (r *serviceCodeBatchRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ServiceCodeBatch {
	ret := &models.ServiceCodeBatch{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeBatchRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ServiceCodeBatch, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ServiceCodeBatch{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *serviceCodeBatchRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ServiceCodeBatch{})
}

func (r *serviceCodeBatchRepository) Create(db *gorm.DB, t *models.ServiceCodeBatch) error {
	return db.Create(t).Error
}

func (r *serviceCodeBatchRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ServiceCodeBatch{}).Where("id = ?", id).Updates(columns).Error
}

func (r *serviceCodeBatchRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ServiceCodeBatch{}, "id = ?", id)
}
