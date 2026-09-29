package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(159, "repair SLA deadlines collapsed by calendar timezone conversion", func() error {
		_, err := services.RepairCollapsedProjectSLADeadlinesDB(sqls.DB())
		return err
	})
}
