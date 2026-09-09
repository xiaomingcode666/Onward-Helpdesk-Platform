package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(88, "add workflow model credential policies", func() error {
		return addWorkflowModelCredentialPolicies(sqls.DB())
	})
}

func addWorkflowModelCredentialPolicies(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	return services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db)
}
