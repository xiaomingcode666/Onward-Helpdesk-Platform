package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(98, "remove legacy workflow routing and retire platform-derived enterprise templates", func() error {
		return removeLegacyWorkflowRouting(sqls.DB())
	})
}

func removeLegacyWorkflowRouting(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.AIWorkflow{}) ||
		!db.Migrator().HasTable(&models.AIWorkflowVersion{}) ||
		!db.Migrator().HasTable(&models.AIAgent{}) ||
		!db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}

	type workflowTarget struct {
		workflow *models.AIWorkflow
		version  *models.AIWorkflowVersion
	}
	targets := make([]workflowTarget, 0, len(services.PlatformBuiltInWorkflowCodes()))
	for _, ensure := range []func(*gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion, error){
		services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB,
		services.AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB,
		services.AIWorkflowService.EnsurePlatformDispatchOnlyWorkflowDB,
	} {
		workflow, version, err := ensure(db)
		if err != nil {
			return err
		}
		targets = append(targets, workflowTarget{workflow: workflow, version: version})
	}

	platformIDs := make([]int64, 0, len(targets))
	for _, target := range targets {
		platformIDs = append(platformIDs, target.workflow.ID)
	}
	var forks []models.AIWorkflow
	if err := db.Where(
		"scope = ? AND agent_id = 0 AND source_workflow_id IN ? AND status <> ?",
		models.AIWorkflowScopeTenant,
		platformIDs,
		enums.StatusDeleted,
	).Order("id ASC").Find(&forks).Error; err != nil {
		return err
	}
	targetByWorkflowID := make(map[int64]workflowTarget, len(targets)+len(forks))
	for _, target := range targets {
		targetByWorkflowID[target.workflow.ID] = target
	}
	for i := range forks {
		target, ok := targetByWorkflowID[forks[i].SourceWorkflowID]
		if ok {
			targetByWorkflowID[forks[i].ID] = target
		}
	}

	var activeReleases []models.AIAgentRelease
	if err := db.Where(
		"deployment_status = ? AND review_status = ? AND status <> ?",
		models.AIAgentReleaseDeploymentActive,
		enums.AIAgentReviewStatusApproved,
		enums.StatusDeleted,
	).Order("id ASC").Find(&activeReleases).Error; err != nil {
		return err
	}
	for i := range activeReleases {
		target, managed := targetByWorkflowID[activeReleases[i].WorkflowID]
		if !managed || (activeReleases[i].WorkflowID == target.workflow.ID && activeReleases[i].WorkflowVersionID == target.version.ID) {
			continue
		}
		release := activeReleases[i]
		operator := &dto.AuthPrincipal{
			TenantID:   release.TenantID,
			Username:   "system",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if err := services.AIAgentReleaseService.AlignAgentWorkflowToCurrentStableDB(db, release.AgentID, operator); err != nil {
			return fmt.Errorf("align legacy release for agent %d: %w", release.AgentID, err)
		}
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
			if err := tx.Model(&models.AIAgent{}).
				Where("id = ? AND active_release_id = ?", release.AgentID, release.ID).
				Updates(map[string]any{
					"active_release_id": 0,
					"status":            enums.StatusDisabled,
					"review_status":     enums.AIAgentReviewStatusUnreviewed,
					"review_comment":    "旧流程已退役，请基于最新稳定版重新审核部署",
					"reviewed_at":       nil,
					"reviewed_by_id":    0,
					"reviewed_by_name":  "",
					"update_user_name":  "system",
					"updated_at":        now,
				}).Error; err != nil {
				return err
			}
			if tx.Migrator().HasTable(&models.ProductServiceProfile{}) {
				return tx.Model(&models.ProductServiceProfile{}).
					Where("tenant_id = ? AND product_id = ? AND status <> ?", release.TenantID, release.ProductID, enums.StatusDeleted).
					Updates(map[string]any{
						"default_flow_template_id": target.workflow.ID,
						"update_user_name":         "system",
						"updated_at":               now,
					}).Error
			}
			return nil
		}); err != nil {
			return fmt.Errorf("retire legacy release %d: %w", release.ID, err)
		}
	}

	managedWorkflowIDs := make([]int64, 0, len(targetByWorkflowID))
	for workflowID := range targetByWorkflowID {
		managedWorkflowIDs = append(managedWorkflowIDs, workflowID)
	}
	var agents []models.AIAgent
	if err := db.Where("workflow_id IN ? AND status <> ?", managedWorkflowIDs, enums.StatusDeleted).Order("id ASC").Find(&agents).Error; err != nil {
		return err
	}
	for i := range agents {
		operator := &dto.AuthPrincipal{
			TenantID:   agents[i].TenantID,
			Username:   "system",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if err := services.AIAgentReleaseService.AlignAgentWorkflowToCurrentStableDB(db, agents[i].ID, operator); err != nil {
			return fmt.Errorf("align governed workflow for agent %d: %w", agents[i].ID, err)
		}
		current := models.AIAgent{}
		if err := db.First(&current, agents[i].ID).Error; err != nil {
			return err
		}
		if current.ActiveReleaseID <= 0 {
			if err := db.Model(&models.AIAgent{}).Where("id = ?", current.ID).Updates(map[string]any{
				"status":           enums.StatusDisabled,
				"review_status":    enums.AIAgentReviewStatusUnreviewed,
				"review_comment":   "请基于最新稳定版完成人工审核与部署",
				"reviewed_at":      nil,
				"reviewed_by_id":   0,
				"reviewed_by_name": "",
				"update_user_name": "system",
				"updated_at":       time.Now(),
			}).Error; err != nil {
				return err
			}
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for i := range forks {
			fork := forks[i]
			var activeReleaseCount int64
			if err := tx.Model(&models.AIAgentRelease{}).
				Where("workflow_id = ? AND deployment_status = ? AND status <> ?", fork.ID, models.AIAgentReleaseDeploymentActive, enums.StatusDeleted).
				Count(&activeReleaseCount).Error; err != nil {
				return err
			}
			if activeReleaseCount > 0 {
				return fmt.Errorf("legacy workflow %d still has %d active release(s)", fork.ID, activeReleaseCount)
			}
			if tx.Migrator().HasTable(&models.ProductServiceProfile{}) {
				if err := tx.Model(&models.ProductServiceProfile{}).
					Where("default_flow_template_id = ? AND status <> ?", fork.ID, enums.StatusDeleted).
					Updates(map[string]any{
						"default_flow_template_id": fork.SourceWorkflowID,
						"update_user_name":         "system",
						"updated_at":               now,
					}).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&models.AIWorkflowVersion{}).
				Where("workflow_id = ? AND release_channel <> ?", fork.ID, models.AIWorkflowReleaseChannelRevoked).
				Updates(map[string]any{
					"release_channel":  models.AIWorkflowReleaseChannelDeprecated,
					"update_user_name": "system",
					"updated_at":       now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.AIWorkflow{}).Where("id = ?", fork.ID).Updates(map[string]any{
				"status":           enums.StatusDeleted,
				"update_user_name": "system",
				"updated_at":       now,
			}).Error; err != nil {
				return err
			}
		}
		for _, target := range targets {
			if err := tx.Model(&models.AIWorkflowVersion{}).
				Where("workflow_id = ? AND id <> ? AND release_channel <> ?", target.workflow.ID, target.version.ID, models.AIWorkflowReleaseChannelRevoked).
				Updates(map[string]any{
					"release_channel":  models.AIWorkflowReleaseChannelDeprecated,
					"update_user_name": "system",
					"updated_at":       now,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
