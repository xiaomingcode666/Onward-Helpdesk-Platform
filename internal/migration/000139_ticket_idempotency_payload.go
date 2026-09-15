package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(139, "store ticket idempotency payload hash", func() error {
		db := sqls.DB()
		if db == nil || !db.Migrator().HasTable(&models.Ticket{}) {
			return nil
		}
		if !db.Migrator().HasColumn(&models.Ticket{}, "idempotency_payload_hash") {
			return db.Migrator().AddColumn(&models.Ticket{}, "IdempotencyPayloadHash")
		}
		return nil
	})
}
