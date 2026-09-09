package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(49, "consolidate AI agents onto the single platform default workflow", func() error {
		return consolidateAIWorkflowsToPlatformDefault(sqls.DB())
	})
}

func consolidateAIWorkflowsToPlatformDefault(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIAgent{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		workflow, version, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(tx)
		if err != nil {
			return err
		}
		if workflow == nil || version == nil || version.Status != enums.StatusOk || workflow.ID <= 0 || version.WorkflowID != workflow.ID {
			return fmt.Errorf("platform default workflow has no valid stable version")
		}
		if workflowDefinitionHashForMigration(version.Definition) != version.DefinitionHash {
			return fmt.Errorf("platform default workflow definition hash is invalid")
		}

		now := time.Now()
		var agents []models.AIAgent
		if err := tx.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&agents).Error; err != nil {
			return err
		}
		for i := range agents {
			agent := agents[i]
			activeReleaseID := agent.ActiveReleaseID
			if activeReleaseID > 0 {
				activeRelease := &models.AIAgentRelease{}
				if err := tx.First(activeRelease, "id = ?", activeReleaseID).Error; err != nil {
					return fmt.Errorf("load active release %d for agent %d: %w", activeReleaseID, agent.ID, err)
				}
				if activeRelease.AgentID != agent.ID || activeRelease.TenantID != agent.TenantID || activeRelease.ProductID != agent.ProductID || activeRelease.Status != enums.StatusOk || activeRelease.ReviewStatus != enums.AIAgentReviewStatusApproved || activeRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
					return fmt.Errorf("agent %d active release is not deployable", agent.ID)
				}
				if activeRelease.WorkflowID != workflow.ID || activeRelease.WorkflowVersionID != version.ID {
					if err := tx.Model(&models.AIAgentRelease{}).Where("id = ?", activeRelease.ID).Updates(map[string]any{
						"deployment_status": models.AIAgentReleaseDeploymentRetired,
						"updated_at":        now,
						"update_user_name":  "migration-49",
					}).Error; err != nil {
						return err
					}
					var maxReleaseNo int
					if err := tx.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).
						Select("COALESCE(MAX(release_no), 0)").Scan(&maxReleaseNo).Error; err != nil {
						return err
					}
					replacement := &models.AIAgentRelease{
						TenantID:               agent.TenantID,
						ProductID:              agent.ProductID,
						AgentID:                agent.ID,
						ReleaseNo:              maxReleaseNo + 1,
						WorkflowID:             workflow.ID,
						WorkflowVersionID:      version.ID,
						WorkflowDefinitionHash: version.DefinitionHash,
						AgentConfigSnapshot:    activeRelease.AgentConfigSnapshot,
						AgentConfigHash:        activeRelease.AgentConfigHash,
						KnowledgeScopeSnapshot: activeRelease.KnowledgeScopeSnapshot,
						KnowledgeScopeHash:     activeRelease.KnowledgeScopeHash,
						ReviewStatus:           enums.AIAgentReviewStatusApproved,
						ReviewComment:          "统一切换到平台默认售后会话流程",
						ReviewedAt:             &now,
						ReviewedByName:         "migration-49",
						DeploymentStatus:       models.AIAgentReleaseDeploymentActive,
						DeployedAt:             &now,
						DeployedByName:         "migration-49",
						Status:                 enums.StatusOk,
						AuditFields: models.AuditFields{
							CreatedAt:      now,
							CreateUserName: "migration-49",
							UpdatedAt:      now,
							UpdateUserName: "migration-49",
						},
					}
					if err := tx.Create(replacement).Error; err != nil {
						return err
					}
					activeReleaseID = replacement.ID
				}
			}

			updates := map[string]any{
				"workflow_id":         workflow.ID,
				"workflow_version_id": version.ID,
				"active_release_id":   activeReleaseID,
				"updated_at":          now,
				"update_user_name":    "migration-49",
			}
			if agent.WorkflowID != workflow.ID || agent.WorkflowVersionID != version.ID {
				updates["draft_revision"] = agent.DraftRevision + 1
			}
			if err := tx.Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		if tx.Migrator().HasTable(&models.ProductServiceProfile{}) {
			if err := tx.Model(&models.ProductServiceProfile{}).Where("status <> ?", enums.StatusDeleted).Updates(map[string]any{
				"default_flow_template_id": workflow.ID,
				"updated_at":               now,
				"update_user_name":         "migration-49",
			}).Error; err != nil {
				return err
			}
		}
		return retireNonDefaultAIWorkflowsDB(tx, workflow.ID, now, "migration-49")
	})
}

func workflowDefinitionHashForMigration(definition string) string {
	sum := sha256.Sum256([]byte(definition))
	return hex.EncodeToString(sum[:])
}
