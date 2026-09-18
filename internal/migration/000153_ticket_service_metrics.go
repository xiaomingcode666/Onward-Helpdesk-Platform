package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(153, "add ticket service metrics", func() error {
		db := sqls.DB()
		if err := db.AutoMigrate(&models.TicketServiceMetric{}); err != nil {
			return err
		}
		for _, column := range []string{"FirstRespondedAt", "LastCustomerUpdateAt"} {
			if !db.Migrator().HasColumn(&models.Ticket{}, column) {
				if err := db.Migrator().AddColumn(&models.Ticket{}, column); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
