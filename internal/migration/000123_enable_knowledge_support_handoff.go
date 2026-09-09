package migration

import (
	"encoding/json"
	"fmt"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(123, "enable knowledge support handoff and default maintenance teams", func() error {
		return enableKnowledgeSupportTenantHumanHandoff(sqls.DB())
	})
}

func enableKnowledgeSupportTenantHumanHandoff(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	var tenants []models.Tenant
	if err := db.Where("service_scene = ? AND status <> ?", models.TenantServiceSceneKnowledgeSupport, enums.StatusDeleted).
		Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		tenant := &tenants[i]
		operator := &dto.AuthPrincipal{
			TenantID:   tenant.ID,
			Username:   "migration-123",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		technicalTeam, err := services.ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
		if err != nil {
			return fmt.Errorf("ensure tenant %d default maintenance team: %w", tenant.ID, err)
		}
		if technicalTeam == nil || technicalTeam.Name != services.TechnicalMaintenanceTeamName || technicalTeam.TeamType != services.AgentTeamTypeTechnicalRepair || technicalTeam.ProductID != 0 {
			return fmt.Errorf("tenant %d default maintenance team is invalid", tenant.ID)
		}
		if !tenant.IsAIEnabled() {
			continue
		}
		agent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenant.ID, operator)
		if err != nil {
			return fmt.Errorf("enable tenant %d knowledge support handoff: %w", tenant.ID, err)
		}
		if agent == nil || agent.ActiveReleaseID <= 0 || agent.ServiceMode != enums.IMConversationServiceModeAIFirst ||
			agent.HandoffMode != enums.AIAgentHandoffModeDefaultTeamPool ||
			len(utils.SplitInt64s(agent.TeamIDs)) != 1 || utils.SplitInt64s(agent.TeamIDs)[0] != technicalTeam.ID {
			return fmt.Errorf("tenant %d knowledge support handoff release is not active", tenant.ID)
		}
		activeRelease := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
		if activeRelease == nil || activeRelease.AgentID != agent.ID || activeRelease.ReviewStatus != enums.AIAgentReviewStatusApproved || activeRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
			return fmt.Errorf("tenant %d knowledge support handoff release is invalid", tenant.ID)
		}
		activeVersion := repositories.AIWorkflowVersionRepository.Get(db, activeRelease.WorkflowVersionID)
		definition := dsl.Definition{}
		if activeVersion == nil || activeVersion.WorkflowID != activeRelease.WorkflowID || json.Unmarshal([]byte(activeVersion.Definition), &definition) != nil {
			return fmt.Errorf("tenant %d knowledge support handoff workflow is invalid", tenant.ID)
		}
		capabilities := workflowcapability.FromDefinition(definition)
		activeSnapshot := dto.AIAgentReleaseConfigSnapshot{}
		if json.Unmarshal([]byte(activeRelease.AgentConfigSnapshot), &activeSnapshot) != nil ||
			activeSnapshot.ServiceMode != enums.IMConversationServiceModeAIFirst ||
			activeSnapshot.HandoffMode != enums.AIAgentHandoffModeDefaultTeamPool ||
			len(activeSnapshot.TeamIDs) != 1 || activeSnapshot.TeamIDs[0] != technicalTeam.ID ||
			!capabilities.HumanHandoff || capabilities.TicketCreation {
			return fmt.Errorf("tenant %d knowledge support handoff runtime is not active", tenant.ID)
		}
	}
	return nil
}
