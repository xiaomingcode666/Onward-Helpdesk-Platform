package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(79, "add portable workflow node failure fallback paths", func() error {
		return upgradePlatformDefaultWorkflowFailureFallback(sqls.DB())
	})
}

func upgradePlatformDefaultWorkflowFailureFallback(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	return err
}
