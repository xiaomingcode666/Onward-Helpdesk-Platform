package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(141, "bind tickets to versioned lifecycle workflows", func() error {
		if !sqls.DB().Migrator().HasColumn(&models.Ticket{}, "CaseWorkflowVersionID") {
			if err := sqls.DB().Migrator().AddColumn(&models.Ticket{}, "CaseWorkflowVersionID"); err != nil {
				return err
			}
		}
		if !sqls.DB().Migrator().HasColumn(&models.Ticket{}, "CaseWorkflowKey") {
			return sqls.DB().Migrator().AddColumn(&models.Ticket{}, "CaseWorkflowKey")
		}
		return nil
	})
}
