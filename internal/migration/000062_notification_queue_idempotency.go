package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(62, "add notification queue idempotency key", func() error {
		db := sqls.DB()
		if !db.Migrator().HasColumn(&models.Notification{}, "idempotency_key") {
			if err := db.Migrator().AddColumn(&models.Notification{}, "IdempotencyKey"); err != nil {
				return err
			}
		}
		return ensureNotificationTenantIdempotencyIndex(db)
	})
}
