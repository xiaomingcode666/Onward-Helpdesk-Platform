package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(81, "add quick AI knowledge retrieval fallback", func() error {
		return upgradePlatformDefaultWorkflowQuickAIFallback(sqls.DB())
	})
}

func upgradePlatformDefaultWorkflowQuickAIFallback(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	return err
}
