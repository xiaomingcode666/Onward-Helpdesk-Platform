package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ServiceCodeRepository = newServiceCodeRepository()

func newServiceCodeRepository() *serviceCodeRepository {
	return &serviceCodeRepository{}
}

type serviceCodeRepository struct {
}

func (r *serviceCodeRepository) GetByCode(db *gorm.DB, serviceCode string) *models.ServiceCode {
	ret := &models.ServiceCode{}
	if err := db.First(ret, "service_code = ?", serviceCode).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeRepository) GetActiveByDeviceID(db *gorm.DB, deviceID int64) *models.ServiceCode {
	ret := &models.ServiceCode{}
	if err := db.Where("device_id = ? and status in ?", deviceID, []string{"active", "bound"}).Order("id desc").First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeRepository) Get(db *gorm.DB, id int64) *models.ServiceCode {
	ret := &models.ServiceCode{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeRepository) Take(db *gorm.DB, where ...any) *models.ServiceCode {
	ret := &models.ServiceCode{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.ServiceCode) {
	cnd.Find(db, &list)
	return
}

func (r *serviceCodeRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.ServiceCode {
	ret := &models.ServiceCode{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *serviceCodeRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.ServiceCode, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.ServiceCode{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *serviceCodeRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.ServiceCode{})
}

func (r *serviceCodeRepository) Create(db *gorm.DB, t *models.ServiceCode) error {
	return db.Create(t).Error
}

func (r *serviceCodeRepository) BatchCreate(db *gorm.DB, list []models.ServiceCode) error {
	if len(list) == 0 {
		return nil
	}
	return db.Create(&list).Error
}

func (r *serviceCodeRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ServiceCode{}).Where("id = ?", id).Updates(columns).Error
}

func (r *serviceCodeRepository) UpdateStatusIf(db *gorm.DB, id int64, status string, columns map[string]any) (int64, error) {
	ret := db.Model(&models.ServiceCode{}).Where("id = ? and status = ?", id, status).Updates(columns)
	return ret.RowsAffected, ret.Error
}

func (r *serviceCodeRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.ServiceCode{}, "id = ?", id)
}
