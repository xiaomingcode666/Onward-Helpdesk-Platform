package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(107, "add ticket acceptance timestamp and dispatch attempts", func() error {
		db := sqls.DB()
		if db == nil {
			return nil
		}
		if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasColumn(&models.Ticket{}, "accepted_at") {
			if err := db.Migrator().AddColumn(&models.Ticket{}, "AcceptedAt"); err != nil {
				return err
			}
		}
		return db.AutoMigrate(&models.TicketDispatchAttempt{})
	})
}
