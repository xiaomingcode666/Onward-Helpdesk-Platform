package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(83, "split platform workflows by product service level", func() error {
		return splitPlatformProductWorkflows(sqls.DB())
	})
}

func splitPlatformProductWorkflows(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	return services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db)
}
