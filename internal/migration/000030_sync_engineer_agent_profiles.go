package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(30, "sync enterprise engineers into dispatch agent profiles", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.ProductSupportOrganizationService.SyncEngineerAgentProfilesDB(ctx.Tx, nil)
		})
	})
}
