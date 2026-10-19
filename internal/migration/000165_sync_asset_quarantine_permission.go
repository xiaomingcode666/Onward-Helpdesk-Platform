package migration

import (
	"github.com/mlogclub/simple/sqls"
)

// Builtin tenant and platform role policies are refreshed on every start by
// services.SyncDefaultIAMRolePoliciesDB; this keeps the legacy catalog in step.
func init() {
	register(165, "sync attachment quarantine management permission", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			permissions, err := ensurePermissions(ctx.Tx)
			if err != nil {
				return err
			}
			roles, err := ensureRoles(ctx.Tx)
			if err != nil {
				return err
			}
			return ensureRolePermissions(ctx.Tx, roles, permissions)
		})
	})
}
