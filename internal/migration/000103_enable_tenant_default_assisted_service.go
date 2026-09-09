package migration

import "github.com/mlogclub/simple/sqls"

func init() {
	register(103, "enable assisted service for tenant default agents", func() error {
		return ensureTenantDefaultAIAgents(sqls.DB())
	})
}
