package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(97, "harden platform workflow failure recovery", func() error {
		return hardenPlatformWorkflowFailureRecovery(sqls.DB())
	})
}

func hardenPlatformWorkflowFailureRecovery(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	return services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db)
}
