package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(80, "add portable workflow entry and service access policies", func() error {
		return upgradePlatformDefaultWorkflowServiceAccessPolicy(sqls.DB())
	})
}

func upgradePlatformDefaultWorkflowServiceAccessPolicy(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	return err
}
