package migration

import (
	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(32, "enforce product fault statistics composite uniqueness", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return ensureProductFaultStatsUniqueIndex(ctx.Tx)
		})
	})
}
