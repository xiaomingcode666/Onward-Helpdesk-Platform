package migration

import (
	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(39, "reconcile ai handoff ticket titles at creation time", func() error {
		return repairAIHandoffTicketTitles(sqls.DB())
	})
}
