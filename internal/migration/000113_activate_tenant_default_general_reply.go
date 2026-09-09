package migration

import "github.com/mlogclub/simple/sqls"

func init() {
	register(113, "activate tenant default general consultation hardening", func() error {
		return migrateTenantDefaultAgentsToAIOnly(sqls.DB())
	})
}
