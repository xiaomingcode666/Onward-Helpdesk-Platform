package migration

import (
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
)

func init() {
	register(130, "add manual phone intake provenance and context policy", func() error {
		return migrateTicketIntake(sqls.DB())
	})
}

func migrateTicketIntake(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, entry := range []struct {
		model  any
		fields []string
	}{
		{&models.Ticket{}, []string{"SourceRecordID", "SourceRecordKey", "ProjectKey", "TicketType", "CallerName", "CallerPhone", "ReceivedAt", "ContextStatus", "MissingContextJSON"}},
		{&models.Tenant{}, []string{"TicketIntakePolicyJSON"}},
	} {
		if !db.Migrator().HasTable(entry.model) {
			continue
		}
		for _, field := range entry.fields {
			if !db.Migrator().HasColumn(entry.model, field) {
				if err := db.Migrator().AddColumn(entry.model, field); err != nil {
					return err
				}
			}
		}
	}
	if db.Migrator().HasTable(&models.Ticket{}) {
		for _, index := range []string{"SourceRecordID", "uk_ticket_intake_source"} {
			if !db.Migrator().HasIndex(&models.Ticket{}, index) {
				if err := db.Migrator().CreateIndex(&models.Ticket{}, index); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
