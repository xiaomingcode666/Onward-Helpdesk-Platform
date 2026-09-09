package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(108, "add ticket dispatch defer fields", func() error {
		return migrateTicketDispatchDefer(sqls.DB())
	})
}

func migrateTicketDispatchDefer(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	for _, column := range []string{"dispatch_deferred_until", "last_dispatch_failure_reason"} {
		if db.Migrator().HasColumn(&models.Ticket{}, column) {
			continue
		}
		if err := db.Migrator().AddColumn(&models.Ticket{}, column); err != nil {
			return err
		}
	}
	for _, index := range []string{"DispatchDeferredUntil", "LastDispatchFailureReason"} {
		if db.Migrator().HasIndex(&models.Ticket{}, index) {
			continue
		}
		if err := db.Migrator().CreateIndex(&models.Ticket{}, index); err != nil {
			return err
		}
	}
	return nil
}
