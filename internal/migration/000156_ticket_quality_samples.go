package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(156, "add ticket quality samples", func() error {
		return sqls.DB().AutoMigrate(&models.TicketQualitySample{})
	})
}
