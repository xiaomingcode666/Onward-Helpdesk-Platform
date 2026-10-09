package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(161, "add encrypted channel secrets", func() error {
		return sqls.DB().AutoMigrate(&models.ChannelSecret{})
	})
}
