package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(78, "relayout platform default workflow AI fallback handoff path", func() error {
		return relayoutPlatformDefaultHandoffWorkflow(sqls.DB())
	})
}

func relayoutPlatformDefaultHandoffWorkflow(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	return err
}
