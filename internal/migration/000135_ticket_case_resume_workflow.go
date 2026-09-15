package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(135, "preserve technical work step while a case waits", func() error {
		// Version 134 may already be applied in a development checkout.
		if sqls.DB().Migrator().HasColumn(&models.Ticket{}, "CaseResumeTechnicalStatus") {
			return nil
		}
		return sqls.DB().Migrator().AddColumn(&models.Ticket{}, "CaseResumeTechnicalStatus")
	})
}
