package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const retiredBasicAIWorkflowCode = "aftersales_customer_service_ai_only"

func init() {
	register(100, "retire redundant basic AI workflow after consolidating platform templates", func() error {
		return retireRedundantPlatformAIWorkflow(sqls.DB())
	})
}

func retireRedundantPlatformAIWorkflow(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.AIWorkflow{}) ||
		!db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}

	return services.AIWorkflowService.RetireRemovedPlatformBuiltInWorkflowsDB(db)
}
