package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(42, "close conversations linked to closed tickets", func() error {
		return services.TicketLifecycleService.BackfillClosedTicketConversations(sqls.DB())
	})
}
