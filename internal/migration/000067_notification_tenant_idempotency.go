package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(67, "scope notification idempotency by tenant", func() error {
		return ensureNotificationTenantIdempotencyIndex(sqls.DB())
	})
}

func ensureNotificationTenantIdempotencyIndex(db *gorm.DB) error {
	if db.Migrator().HasIndex(&models.Notification{}, "uk_notification_idempotency_key") {
		if err := db.Migrator().DropIndex(&models.Notification{}, "uk_notification_idempotency_key"); err != nil {
			return err
		}
	}
	if db.Migrator().HasIndex(&models.Notification{}, "uk_notification_tenant_idempotency") {
		return nil
	}
	return db.Migrator().CreateIndex(&models.Notification{}, "uk_notification_tenant_idempotency")
}
