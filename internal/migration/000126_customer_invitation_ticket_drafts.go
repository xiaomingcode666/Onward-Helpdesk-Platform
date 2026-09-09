package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(126, "add customer invitation draft tickets", func() error {
		return migrateCustomerInvitationDraftTickets(sqls.DB())
	})
}

func migrateCustomerInvitationDraftTickets(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasColumn(&models.Ticket{}, "CustomerRegistrationGrantID") {
		if err := db.Migrator().AddColumn(&models.Ticket{}, "CustomerRegistrationGrantID"); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasIndex(&models.Ticket{}, "CustomerRegistrationGrantID") {
		if err := db.Migrator().CreateIndex(&models.Ticket{}, "CustomerRegistrationGrantID"); err != nil {
			return err
		}
	}
	if _, err := ensurePermissions(db); err != nil {
		return err
	}
	return services.SyncDefaultIAMRolePoliciesDB(db)
}
