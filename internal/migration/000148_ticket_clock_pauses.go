package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(148, "add DayPop accountable clock pauses", func() error {
		if err := sqls.DB().AutoMigrate(&models.TicketClockPause{}); err != nil {
			return err
		}
		return services.TicketClockService.Backfill()
	})
}
