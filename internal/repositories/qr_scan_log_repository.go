package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var QrScanLogRepository = newQrScanLogRepository()

func newQrScanLogRepository() *qrScanLogRepository {
	return &qrScanLogRepository{}
}

type qrScanLogRepository struct{}

func (r *qrScanLogRepository) Get(db *gorm.DB, id int64) *models.QrScanLog {
	ret := &models.QrScanLog{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrScanLogRepository) Take(db *gorm.DB, where ...any) *models.QrScanLog {
	ret := &models.QrScanLog{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *qrScanLogRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrScanLog) {
	cnd.Find(db, &list)
	return
}

func (r *qrScanLogRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.QrScanLog {
	ret := &models.QrScanLog{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *qrScanLogRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.QrScanLog, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.QrScanLog{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *qrScanLogRepository) Create(db *gorm.DB, t *models.QrScanLog) error {
	return db.Create(t).Error
}

func (r *qrScanLogRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.QrScanLog{})
}
