package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(41, "backfill supplier participants and conversation messages", func() error {
		return services.TicketSupplierCollaborationService.BackfillPartnerConversationMessages(sqls.DB())
	})
}
