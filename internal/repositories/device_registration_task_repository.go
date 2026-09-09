package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var DeviceRegistrationTaskRepository = newDeviceRegistrationTaskRepository()

func newDeviceRegistrationTaskRepository() *deviceRegistrationTaskRepository {
	return &deviceRegistrationTaskRepository{}
}

type deviceRegistrationTaskRepository struct{}

func (r *deviceRegistrationTaskRepository) Get(db *gorm.DB, id int64) *models.DeviceRegistrationTask {
	ret := &models.DeviceRegistrationTask{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRegistrationTaskRepository) Take(db *gorm.DB, where ...any) *models.DeviceRegistrationTask {
	ret := &models.DeviceRegistrationTask{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRegistrationTaskRepository) FindByServiceCodeID(db *gorm.DB, serviceCodeID int64) *models.DeviceRegistrationTask {
	ret := &models.DeviceRegistrationTask{}
	if err := db.Where("service_code_id = ?", serviceCodeID).Order("id desc").First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceRegistrationTaskRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceRegistrationTask) {
	cnd.Find(db, &list)
	return
}

func (r *deviceRegistrationTaskRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.DeviceRegistrationTask {
	ret := &models.DeviceRegistrationTask{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *deviceRegistrationTaskRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceRegistrationTask, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.DeviceRegistrationTask{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *deviceRegistrationTaskRepository) Create(db *gorm.DB, t *models.DeviceRegistrationTask) error {
	return db.Create(t).Error
}

func (r *deviceRegistrationTaskRepository) CreateIfAbsent(db *gorm.DB, t *models.DeviceRegistrationTask) (bool, error) {
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "service_code_id"}},
		DoNothing: true,
	}).Create(t)
	return result.RowsAffected > 0, result.Error
}

func (r *deviceRegistrationTaskRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.DeviceRegistrationTask{}).Where("id = ?", id).Updates(columns).Error
}

func (r *deviceRegistrationTaskRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.DeviceRegistrationTask{})
}
