package migration

import "github.com/mlogclub/simple/sqls"

func init() {
	register(102, "ensure tenant default general knowledge bases", func() error {
		return ensureTenantDefaultAIAgents(sqls.DB())
	})
}
