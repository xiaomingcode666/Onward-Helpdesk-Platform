package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(35, "repair enterprise engineer role bindings", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.ProductSupportOrganizationService.SyncEngineerAgentProfilesDB(ctx.Tx, nil)
		})
	})
}
