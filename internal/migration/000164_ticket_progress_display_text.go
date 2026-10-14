package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(164, "add localized display text for system ticket progress, support reasons and priority explanations", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.BackfillTicketProgressDisplayTextDB(ctx.Tx)
		})
	})
}
