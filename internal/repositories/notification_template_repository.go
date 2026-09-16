package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var NotificationTemplateRepository = newNotificationTemplateRepository()

func newNotificationTemplateRepository() *notificationTemplateRepository {
	return &notificationTemplateRepository{}
}

type notificationTemplateRepository struct{}

func (r *notificationTemplateRepository) Get(db *gorm.DB, id int64) *models.NotificationTemplate {
	if db == nil || id <= 0 {
		return nil
	}
	item := &models.NotificationTemplate{}
	if err := db.First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *notificationTemplateRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.NotificationTemplate) {
	cnd.Find(db, &list)
	return
}

func (r *notificationTemplateRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.NotificationTemplate, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.NotificationTemplate{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *notificationTemplateRepository) Create(db *gorm.DB, item *models.NotificationTemplate) error {
	return db.Create(item).Error
}

func (r *notificationTemplateRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.NotificationTemplate{}).Where("id = ?", id).Updates(columns).Error
}

func (r *notificationTemplateRepository) Delete(db *gorm.DB, id int64) error {
	return db.Delete(&models.NotificationTemplate{}, "id = ?", id).Error
}
