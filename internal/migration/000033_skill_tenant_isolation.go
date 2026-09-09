package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(33, "add tenant isolation to AI skills and run logs", func() error {
		return sqls.DB().AutoMigrate(&models.SkillDefinition{}, &models.SkillRunLog{})
	})
}
