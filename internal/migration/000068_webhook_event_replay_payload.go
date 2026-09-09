package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(68, "persist webhook replay payload and occurrence time", func() error {
		return sqls.DB().AutoMigrate(&models.WebhookEventInbox{})
	})
}
