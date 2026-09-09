package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(24, "backfill product repair conversation and ticket queues", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.ProductSupportOrganizationService.BackfillAllDB(ctx.Tx, nil)
		})
	})
}
