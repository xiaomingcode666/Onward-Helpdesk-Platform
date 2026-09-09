package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(50, "retire legacy AI workflow versions after platform default consolidation", func() error {
		return retireLegacyAIWorkflowVersions(sqls.DB())
	})
}

func retireLegacyAIWorkflowVersions(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		workflow, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(tx)
		if err != nil {
			return err
		}
		if workflow == nil || workflow.ID <= 0 {
			return nil
		}
		return retireNonDefaultAIWorkflowsDB(tx, workflow.ID, time.Now(), "migration-50")
	})
}

func retireNonDefaultAIWorkflowsDB(tx *gorm.DB, keepWorkflowID int64, now time.Time, operatorName string) error {
	if tx == nil || keepWorkflowID <= 0 {
		return nil
	}

	var workflowIDs []int64
	if err := tx.Model(&models.AIWorkflow{}).
		Where("id <> ?", keepWorkflowID).
		Pluck("id", &workflowIDs).Error; err != nil {
		return err
	}
	if len(workflowIDs) == 0 {
		return nil
	}

	if err := tx.Model(&models.AIWorkflow{}).
		Where("id IN ? AND status <> ?", workflowIDs, enums.StatusDeleted).
		Updates(map[string]any{
			"status":           enums.StatusDeleted,
			"updated_at":       now,
			"update_user_name": operatorName,
		}).Error; err != nil {
		return err
	}

	if tx.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		if err := tx.Model(&models.AIWorkflowVersion{}).
			Where("workflow_id IN ? AND status <> ?", workflowIDs, enums.StatusDeleted).
			Updates(map[string]any{
				"status":           enums.StatusDeleted,
				"updated_at":       now,
				"update_user_name": operatorName,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}
