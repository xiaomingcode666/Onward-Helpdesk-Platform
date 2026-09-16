package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(145, "durable mailbox cursor and ticket email replies", func() error {
		return sqls.DB().AutoMigrate(&models.InboundEmail{}, &models.MailboxSyncState{}, &models.TicketEmailReply{})
	})
}
