package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(134, "add standard case lifecycle and administration owner", func() error {
		// Intentionally do not backfill acceptance, restoration, or an owner from
		// the creator/engineer: those historical facts were never recorded.
		return sqls.DB().AutoMigrate(&models.Ticket{}, &models.TicketCaseOperation{})
	})
}
