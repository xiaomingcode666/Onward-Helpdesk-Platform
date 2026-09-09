package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(22, "bind bootstrap administrator to platform IAM", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return services.EnsureBootstrapPlatformAdministratorDB(ctx.Tx, nil)
		})
	})
}
