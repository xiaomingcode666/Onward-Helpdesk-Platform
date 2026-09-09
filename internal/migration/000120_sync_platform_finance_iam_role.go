package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(120, "sync platform finance IAM role and permissions", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if _, err := ensurePermissions(ctx.Tx); err != nil {
				return err
			}
			return services.EnsurePlatformDefaultIAMRolesDB(ctx.Tx, nil)
		})
	})
}
