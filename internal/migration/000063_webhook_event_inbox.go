package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(63, "add durable inbound webhook idempotency inbox", func() error {
		return sqls.DB().AutoMigrate(&models.WebhookEventInbox{})
	})
}
