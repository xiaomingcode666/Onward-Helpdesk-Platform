package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(82, "make quick AI knowledge fallback deterministic", func() error {
		return upgradePlatformDefaultWorkflowQuickAISafeReply(sqls.DB())
	})
}

func upgradePlatformDefaultWorkflowQuickAISafeReply(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	return err
}
