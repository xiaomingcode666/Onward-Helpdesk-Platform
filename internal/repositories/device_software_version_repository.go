package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var DeviceSoftwareVersionRepository = newDeviceSoftwareVersionRepository()

func newDeviceSoftwareVersionRepository() *deviceSoftwareVersionRepository {
	return &deviceSoftwareVersionRepository{}
}

type deviceSoftwareVersionRepository struct {
}

func (r *deviceSoftwareVersionRepository) Get(db *gorm.DB, id int64) *models.DeviceSoftwareVersion {
	ret := &models.DeviceSoftwareVersion{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceSoftwareVersionRepository) Take(db *gorm.DB, where ...any) *models.DeviceSoftwareVersion {
	ret := &models.DeviceSoftwareVersion{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *deviceSoftwareVersionRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceSoftwareVersion) {
	cnd.Find(db, &list)
	return
}

func (r *deviceSoftwareVersionRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.DeviceSoftwareVersion {
	ret := &models.DeviceSoftwareVersion{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *deviceSoftwareVersionRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.DeviceSoftwareVersion, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.DeviceSoftwareVersion{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *deviceSoftwareVersionRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.DeviceSoftwareVersion{})
}

func (r *deviceSoftwareVersionRepository) Create(db *gorm.DB, t *models.DeviceSoftwareVersion) error {
	return db.Create(t).Error
}

func (r *deviceSoftwareVersionRepository) CreateInBatches(db *gorm.DB, list []models.DeviceSoftwareVersion, batchSize int) error {
	if len(list) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return db.CreateInBatches(list, batchSize).Error
}

func (r *deviceSoftwareVersionRepository) GetLatestByDevice(db *gorm.DB, tenantID, deviceID int64) []models.DeviceSoftwareVersion {
	var list []models.DeviceSoftwareVersion
	db.Where("tenant_id = ? and device_id = ?", tenantID, deviceID).
		Order("installed_at desc").
		Find(&list)
	return list
}

func (r *deviceSoftwareVersionRepository) GetHistoryByDeviceComponent(db *gorm.DB, tenantID, deviceID int64, componentType string) []models.DeviceSoftwareVersion {
	var list []models.DeviceSoftwareVersion
	db.Where("tenant_id = ? and device_id = ? and component_type = ?", tenantID, deviceID, componentType).
		Order("installed_at desc").
		Find(&list)
	return list
}

func (r *deviceSoftwareVersionRepository) GetLatestByDeviceComponent(db *gorm.DB, tenantID, deviceID int64, componentType string) *models.DeviceSoftwareVersion {
	ret := &models.DeviceSoftwareVersion{}
	if err := db.Where("tenant_id = ? and device_id = ? and component_type = ?", tenantID, deviceID, componentType).
		Order("installed_at desc").
		First(ret).Error; err != nil {
		return nil
	}
	return ret
}
