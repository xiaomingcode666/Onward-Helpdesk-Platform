package migration

import (
	"remotehelpdesk/internal/services"
)

func init() {
	register(149, "reconcile terminal ticket clock pauses", func() error {
		return services.TicketClockService.Backfill()
	})
}
