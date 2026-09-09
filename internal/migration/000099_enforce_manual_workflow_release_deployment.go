package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	platformWorkflowAutomaticUpgradeComment = "平台工作流稳定版本自动升级"
	legacyAgentBackfillComment              = "存量已审核机器人上线快照回填"
)

func init() {
	register(99, "retire automatically activated workflow releases and require manual review deployment", func() error {
		return enforceManualWorkflowReleaseDeployment(sqls.DB())
	})
}

func enforceManualWorkflowReleaseDeployment(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.AIAgent{}) ||
		!db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	if err := removeLegacyWorkflowRouting(db); err != nil {
		return err
	}

	var releases []models.AIAgentRelease
	if err := db.Where(
		"deployment_status = ? AND review_status = ? AND review_comment IN ? AND reviewed_by_name = ? AND deployed_by_name = ? AND status <> ?",
		models.AIAgentReleaseDeploymentActive,
		enums.AIAgentReviewStatusApproved,
		[]string{platformWorkflowAutomaticUpgradeComment, legacyAgentBackfillComment},
		"system",
		"system",
		enums.StatusDeleted,
	).Order("id ASC").Find(&releases).Error; err != nil {
		return err
	}
	for i := range releases {
		release := releases[i]
		if err := db.Transaction(func(tx *gorm.DB) error {
			now := time.Now()
			result := tx.Model(&models.AIAgentRelease{}).
				Where("id = ? AND deployment_status = ? AND status <> ?", release.ID, models.AIAgentReleaseDeploymentActive, enums.StatusDeleted).
				Updates(map[string]any{
					"deployment_status": models.AIAgentReleaseDeploymentRetired,
					"update_user_name":  "system",
					"updated_at":        now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			return tx.Model(&models.AIAgent{}).
				Where("id = ? AND active_release_id = ?", release.AgentID, release.ID).
				Updates(map[string]any{
					"active_release_id": 0,
					"status":            enums.StatusDisabled,
					"review_status":     enums.AIAgentReviewStatusUnreviewed,
					"review_comment":    "平台稳定版需重新完成人工审核与部署",
					"reviewed_at":       nil,
					"reviewed_by_id":    0,
					"reviewed_by_name":  "",
					"update_user_name":  "system",
					"updated_at":        now,
				}).Error
		}); err != nil {
			return fmt.Errorf("retire automatically activated release %d: %w", release.ID, err)
		}
	}
	return nil
}
