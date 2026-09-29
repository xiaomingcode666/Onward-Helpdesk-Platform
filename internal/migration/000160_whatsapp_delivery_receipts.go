package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(160, "add WhatsApp provider delivery receipt fields", func() error {
		return sqls.DB().AutoMigrate(&models.ChannelMessageOutbox{})
	})
}
