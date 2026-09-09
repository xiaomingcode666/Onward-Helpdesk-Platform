package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(19, "clear stale error details from successful migrations", func() error {
		return sqls.DB().Model(&models.Migration{}).
			Where("success = ? AND error_info <> ?", true, "").
			Update("error_info", "").Error
	})
}
