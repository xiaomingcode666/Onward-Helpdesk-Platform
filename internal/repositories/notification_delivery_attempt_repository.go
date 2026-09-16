package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var NotificationDeliveryAttemptRepository = newNotificationDeliveryAttemptRepository()

func newNotificationDeliveryAttemptRepository() *notificationDeliveryAttemptRepository {
	return &notificationDeliveryAttemptRepository{}
}

type notificationDeliveryAttemptRepository struct{}

func (r *notificationDeliveryAttemptRepository) Create(db *gorm.DB, item *models.NotificationDeliveryAttempt) error {
	return db.Create(item).Error
}

func (r *notificationDeliveryAttemptRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.NotificationDeliveryAttempt) {
	cnd.Find(db, &list)
	return
}

func (r *notificationDeliveryAttemptRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.NotificationDeliveryAttempt, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.NotificationDeliveryAttempt{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}
