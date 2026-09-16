package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(143, "store inbound email thread records", func() error {
		return sqls.DB().AutoMigrate(&models.InboundEmail{})
	})
}
