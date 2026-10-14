package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(163, "add localized display text for built-in knowledge bases, AI agents, teams and departments", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.BackfillBuiltinDisplayTextDB(ctx.Tx)
		})
	})
}
