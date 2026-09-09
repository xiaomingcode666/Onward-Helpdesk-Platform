package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var SystemIntroRepository = newSystemIntroRepository()

func newSystemIntroRepository() *systemIntroRepository {
	return &systemIntroRepository{}
}

type systemIntroRepository struct{}

func (r *systemIntroRepository) Get(db *gorm.DB, id int64) *models.SystemIntroDoc {
	ret := &models.SystemIntroDoc{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *systemIntroRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.SystemIntroDoc) {
	cnd.Find(db, &list)
	return
}

func (r *systemIntroRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.SystemIntroDoc {
	ret := &models.SystemIntroDoc{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *systemIntroRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.SystemIntroDoc, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.SystemIntroDoc{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *systemIntroRepository) Create(db *gorm.DB, item *models.SystemIntroDoc) error {
	return db.Create(item).Error
}

func (r *systemIntroRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.SystemIntroDoc{}).Where("id = ?", id).Updates(columns).Error
}

func (r *systemIntroRepository) Delete(db *gorm.DB, id int64) error {
	return db.Where("id = ?", id).Delete(&models.SystemIntroDoc{}).Error
}
