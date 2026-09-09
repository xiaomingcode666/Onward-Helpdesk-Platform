package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(105, "restore tenant default agents to AI-only diagnosis", func() error {
		return migrateTenantDefaultAgentsToAIOnly(sqls.DB())
	})
}

func migrateTenantDefaultAgentsToAIOnly(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	var tenants []models.Tenant
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		if !tenants[i].IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   tenants[i].ID,
			Username:   "system",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		agent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenants[i].ID, operator)
		if err != nil {
			return err
		}
		if agent == nil {
			continue
		}
		if err := activateTenantDefaultAIOnlyRelease(db, agent.ID, operator); err != nil {
			return fmt.Errorf("activate tenant %d default AI diagnosis release: %w", tenants[i].ID, err)
		}
	}
	return nil
}

func activateTenantDefaultAIOnlyRelease(db *gorm.DB, agentID int64, operator *dto.AuthPrincipal) error {
	return db.Transaction(func(tx *gorm.DB) error {
		agent := repositories.AIAgentRepository.GetForUpdate(tx, agentID)
		if agent == nil || agent.Source != services.TenantDefaultAIAgentSource || agent.ProductID != 0 {
			return fmt.Errorf("tenant default agent not found")
		}
		workflow := repositories.AIWorkflowRepository.Get(tx, agent.WorkflowID)
		version := repositories.AIWorkflowVersionRepository.Get(tx, agent.WorkflowVersionID)
		if workflow == nil || workflow.Code != services.PlatformDeviceAIOnlyWorkflowCode || version == nil || version.WorkflowID != workflow.ID || version.Status != enums.StatusOk || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
			return fmt.Errorf("tenant default draft is not bound to the stable AI-only workflow")
		}
		definition := dsl.Definition{}
		if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
			return fmt.Errorf("decode AI-only workflow definition: %w", err)
		}
		capabilities := workflowcapability.FromDefinition(definition)
		if capabilities.HumanHandoff || capabilities.TicketCreation {
			return fmt.Errorf("tenant default workflow still exposes assisted-service actions")
		}

		active := repositories.AIAgentReleaseRepository.Get(tx, agent.ActiveReleaseID)
		candidates := repositories.AIAgentReleaseRepository.Find(tx, sqls.NewCnd().
			Eq("agent_id", agent.ID).
			Eq("workflow_id", workflow.ID).
			Eq("workflow_version_id", version.ID).
			Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
			NotEq("status", enums.StatusDeleted).
			Desc("release_no"))
		if len(candidates) == 0 {
			if tenantDefaultMigrationActiveReleaseIsCurrent(active, agent, workflow, version) {
				return nil
			}
			return fmt.Errorf("AI-only candidate release not found")
		}
		if tenantDefaultMigrationHash(version.Definition) != version.DefinitionHash {
			return fmt.Errorf("AI-only candidate workflow fingerprint is stale")
		}
		var candidate *models.AIAgentRelease
		for i := range candidates {
			item := &candidates[i]
			if item.TenantID != agent.TenantID || item.ProductID != 0 || item.Status != enums.StatusOk || item.WorkflowDefinitionHash != version.DefinitionHash {
				continue
			}
			if tenantDefaultMigrationHash(item.AgentConfigSnapshot) != item.AgentConfigHash || tenantDefaultMigrationHash(item.KnowledgeScopeSnapshot) != item.KnowledgeScopeHash {
				continue
			}
			snapshot := dto.AIAgentReleaseConfigSnapshot{}
			if err := json.Unmarshal([]byte(item.AgentConfigSnapshot), &snapshot); err != nil {
				continue
			}
			if snapshot.DraftRevision != agent.DraftRevision || snapshot.ServiceMode != enums.IMConversationServiceModeAIOnly || len(snapshot.TeamIDs) > 0 || len(snapshot.SkillIDs) > 0 || len(snapshot.AllowedGraphTools) > 0 || !tenantDefaultMigrationToolsEmpty(snapshot.AllowedMCPTools) {
				continue
			}
			if active != nil && !tenantDefaultMigrationPreservesModelSelection(active, item) {
				continue
			}
			candidate = item
			break
		}
		if candidate == nil {
			if tenantDefaultMigrationActiveReleaseIsCurrent(active, agent, workflow, version) {
				return nil
			}
			return fmt.Errorf("current AI-only candidate snapshot is invalid")
		}

		now := time.Now()
		activeReleases := repositories.AIAgentReleaseRepository.Find(tx, sqls.NewCnd().
			Eq("agent_id", agent.ID).
			Eq("deployment_status", models.AIAgentReleaseDeploymentActive).
			NotEq("id", candidate.ID).
			NotEq("status", enums.StatusDeleted))
		for i := range activeReleases {
			if err := repositories.AIAgentReleaseRepository.Updates(tx, activeReleases[i].ID, map[string]any{
				"deployment_status": models.AIAgentReleaseDeploymentRetired,
				"update_user_id":    operator.UserID,
				"update_user_name":  operator.Username,
				"updated_at":        now,
			}); err != nil {
				return err
			}
		}
		comment := "系统修正企业默认接待为基础 AI 问诊"
		if err := repositories.AIAgentReleaseRepository.Updates(tx, candidate.ID, map[string]any{
			"review_status":     enums.AIAgentReviewStatusApproved,
			"review_comment":    comment,
			"reviewed_at":       &now,
			"reviewed_by_id":    operator.UserID,
			"reviewed_by_name":  operator.Username,
			"deployment_status": models.AIAgentReleaseDeploymentActive,
			"deployed_at":       &now,
			"deployed_by_id":    operator.UserID,
			"deployed_by_name":  operator.Username,
			"update_user_id":    operator.UserID,
			"update_user_name":  operator.Username,
			"updated_at":        now,
		}); err != nil {
			return err
		}
		return repositories.AIAgentRepository.Updates(tx, agent.ID, map[string]any{
			"active_release_id": candidate.ID,
			"status":            enums.StatusOk,
			"review_status":     enums.AIAgentReviewStatusApproved,
			"review_comment":    comment,
			"reviewed_at":       &now,
			"reviewed_by_id":    operator.UserID,
			"reviewed_by_name":  operator.Username,
			"update_user_id":    operator.UserID,
			"update_user_name":  operator.Username,
			"updated_at":        now,
		})
	})
}

func tenantDefaultMigrationActiveReleaseIsCurrent(active *models.AIAgentRelease, agent *models.AIAgent, workflow *models.AIWorkflow, version *models.AIWorkflowVersion) bool {
	if active == nil || agent == nil || workflow == nil || version == nil ||
		active.WorkflowID != workflow.ID || active.WorkflowVersionID != version.ID ||
		active.WorkflowDefinitionHash != version.DefinitionHash ||
		active.ReviewStatus != enums.AIAgentReviewStatusApproved ||
		active.DeploymentStatus != models.AIAgentReleaseDeploymentActive ||
		active.Status != enums.StatusOk ||
		tenantDefaultMigrationHash(active.AgentConfigSnapshot) != active.AgentConfigHash ||
		tenantDefaultMigrationHash(active.KnowledgeScopeSnapshot) != active.KnowledgeScopeHash {
		return false
	}
	snapshot := dto.AIAgentReleaseConfigSnapshot{}
	if err := json.Unmarshal([]byte(active.AgentConfigSnapshot), &snapshot); err != nil {
		return false
	}
	return snapshot.DraftRevision == agent.DraftRevision
}

func tenantDefaultMigrationPreservesModelSelection(active, candidate *models.AIAgentRelease) bool {
	if active == nil || candidate == nil {
		return true
	}
	activeSnapshot := dto.AIAgentReleaseConfigSnapshot{}
	candidateSnapshot := dto.AIAgentReleaseConfigSnapshot{}
	if json.Unmarshal([]byte(active.AgentConfigSnapshot), &activeSnapshot) != nil ||
		json.Unmarshal([]byte(candidate.AgentConfigSnapshot), &candidateSnapshot) != nil {
		return false
	}
	return activeSnapshot.AIConfigID == candidateSnapshot.AIConfigID &&
		activeSnapshot.LLMModelName == candidateSnapshot.LLMModelName
}

func tenantDefaultMigrationHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func tenantDefaultMigrationToolsEmpty(value any) bool {
	switch items := value.(type) {
	case nil:
		return true
	case []any:
		return len(items) == 0
	case map[string]any:
		return len(items) == 0
	default:
		return false
	}
}
