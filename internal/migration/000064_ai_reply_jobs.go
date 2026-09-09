package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(64, "add durable ai reply jobs", func() error {
		return sqls.DB().AutoMigrate(&models.AIReplyJob{})
	})
}
