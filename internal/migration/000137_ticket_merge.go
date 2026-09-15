package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	// 137 is already used by the idempotency payload migration. Keep this
	// follow-up migration ordered and uniquely versioned so startup can proceed.
	register(138, "add retained duplicate ticket merge references", func() error {
		return sqls.DB().AutoMigrate(&models.Ticket{})
	})
}
