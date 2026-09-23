package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(157, "add ticket quality scorecards and reviews", func() error {
		return sqls.DB().AutoMigrate(&models.TicketQualityScorecardVersion{}, &models.TicketQualityReview{})
	})
}
