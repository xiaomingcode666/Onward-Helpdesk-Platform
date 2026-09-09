package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(66, "add DSAR execution idempotency", func() error {
		return sqls.DB().AutoMigrate(&models.DSARExecutionLog{})
	})
}
