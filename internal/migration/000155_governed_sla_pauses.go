package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(155, "add governed SLA pause evidence and approval fields", func() error {
		return sqls.DB().AutoMigrate(&services.SLAPauseRecord{})
	})
}
