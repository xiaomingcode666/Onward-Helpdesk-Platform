package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(158, "add structured ticket quality review fields", func() error {
		return sqls.DB().AutoMigrate(&models.TicketQualityReview{})
	})
}
