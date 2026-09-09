package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var QrLabelExportJobRepository = newQrLabelExportJobRepository()

func newQrLabelExportJobRepository() *qrLabelExportJobRepository {
	return &qrLabelExportJobRepository{}
}

type qrLabelExportJobRepository struct{}

func (r *qrLabelExportJobRepository) Get(db *gorm.DB, id int64) *models.QrLabelExportJob {
	ret := &models.QrLabelExportJob{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelExportJobRepository) Take(db *gorm.DB, where ...any) *models.QrLabelExportJob {
	ret := &models.QrLabelExportJob{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelExportJobRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrLabelExportJob) {
	cnd.Find(db, &list)
	return
}

func (r *qrLabelExportJobRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.QrLabelExportJob {
	ret := &models.QrLabelExportJob{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelExportJobRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrLabelExportJob, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.QrLabelExportJob{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *qrLabelExportJobRepository) Create(db *gorm.DB, t *models.QrLabelExportJob) error {
	return db.Create(t).Error
}

func (r *qrLabelExportJobRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.QrLabelExportJob{}).Where("id = ?", id).Updates(columns).Error
}

func (r *qrLabelExportJobRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.QrLabelExportJob{}, "id = ?", id)
}

func (r *qrLabelExportJobRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.QrLabelExportJob{})
}
