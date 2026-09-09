package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(93, "normalize workflow model credential chains", func() error {
		return normalizeWorkflowModelCredentialChains(sqls.DB())
	})
}

func normalizeWorkflowModelCredentialChains(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	return services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db)
}
