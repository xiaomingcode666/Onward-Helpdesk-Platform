package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var QrLabelTemplateRepository = newQrLabelTemplateRepository()

func newQrLabelTemplateRepository() *qrLabelTemplateRepository {
	return &qrLabelTemplateRepository{}
}

type qrLabelTemplateRepository struct{}

func (r *qrLabelTemplateRepository) Get(db *gorm.DB, id int64) *models.QrLabelTemplate {
	ret := &models.QrLabelTemplate{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelTemplateRepository) Take(db *gorm.DB, where ...any) *models.QrLabelTemplate {
	ret := &models.QrLabelTemplate{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelTemplateRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrLabelTemplate) {
	cnd.Find(db, &list)
	return
}

func (r *qrLabelTemplateRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.QrLabelTemplate {
	ret := &models.QrLabelTemplate{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *qrLabelTemplateRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrLabelTemplate, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.QrLabelTemplate{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *qrLabelTemplateRepository) Create(db *gorm.DB, t *models.QrLabelTemplate) error {
	return db.Create(t).Error
}

func (r *qrLabelTemplateRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.QrLabelTemplate{}).Where("id = ?", id).Updates(columns).Error
}

func (r *qrLabelTemplateRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.QrLabelTemplate{}, "id = ?", id)
}

func (r *qrLabelTemplateRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.QrLabelTemplate{})
}
