package repositories

import (
	"time"

	"remotehelpdesk/internal/models"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var SystemConfigRepository = newSystemConfigRepository()

func newSystemConfigRepository() *systemConfigRepository {
	return &systemConfigRepository{}
}

type systemConfigRepository struct {
}

func (r *systemConfigRepository) Get(db *gorm.DB, id int64) *models.SystemConfig {
	ret := &models.SystemConfig{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *systemConfigRepository) Take(db *gorm.DB, where ...interface{}) *models.SystemConfig {
	ret := &models.SystemConfig{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *systemConfigRepository) FindByKey(db *gorm.DB, key string) *models.SystemConfig {
	ret := &models.SystemConfig{}
	if err := db.First(ret, "config_key = ?", key).Error; err != nil {
		return nil
	}
	return ret
}

func (r *systemConfigRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.SystemConfig) {
	cnd.Find(db, &list)
	return
}

func (r *systemConfigRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.SystemConfig {
	ret := &models.SystemConfig{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *systemConfigRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.SystemConfig, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *systemConfigRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.SystemConfig, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.SystemConfig{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *systemConfigRepository) FindBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (list []models.SystemConfig) {
	db.Raw(sqlStr, paramArr...).Scan(&list)
	return
}

func (r *systemConfigRepository) CountBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (count int64) {
	db.Raw(sqlStr, paramArr...).Count(&count)
	return
}

func (r *systemConfigRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.SystemConfig{})
}

func (r *systemConfigRepository) Create(db *gorm.DB, t *models.SystemConfig) (err error) {
	err = db.Create(t).Error
	return
}

func (r *systemConfigRepository) SaveByKey(db *gorm.DB, item *models.SystemConfig) error {
	existing := r.FindByKey(db, item.ConfigKey)
	if existing == nil {
		return db.Create(item).Error
	}
	item.ID = existing.ID
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now()
	}
	return db.Model(&models.SystemConfig{}).Where("config_key = ?", item.ConfigKey).Updates(map[string]any{
		"config_value":     item.ConfigValue,
		"group_code":       item.GroupCode,
		"title":            item.Title,
		"description":      item.Description,
		"status":           item.Status,
		"updated_at":       item.UpdatedAt,
		"update_user_id":   item.UpdateUserID,
		"update_user_name": item.UpdateUserName,
	}).Error
}

func (r *systemConfigRepository) Update(db *gorm.DB, t *models.SystemConfig) (err error) {
	err = db.Save(t).Error
	return
}

func (r *systemConfigRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.SystemConfig{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *systemConfigRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.SystemConfig{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *systemConfigRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.SystemConfig{}, "id = ?", id)
}
