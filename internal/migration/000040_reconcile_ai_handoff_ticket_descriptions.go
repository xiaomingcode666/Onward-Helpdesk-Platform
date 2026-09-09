package migration

import "github.com/mlogclub/simple/sqls"

func init() {
	register(40, "reconcile ai handoff ticket descriptions", func() error {
		return repairAIHandoffTicketTitles(sqls.DB())
	})
}
