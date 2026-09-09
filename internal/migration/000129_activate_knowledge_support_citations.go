package migration

import (
	"encoding/json"
	"fmt"

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
	register(129, "activate knowledge support citation releases", func() error {
		return activateKnowledgeSupportCitationReleases(sqls.DB())
	})
}

func activateKnowledgeSupportCitationReleases(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	if err := services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db); err != nil {
		return fmt.Errorf("materialize knowledge workflow: %w", err)
	}
	var tenants []models.Tenant
	if err := db.Where("service_scene = ? AND status <> ?", models.TenantServiceSceneKnowledgeSupport, enums.StatusDeleted).
		Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		if !tenants[i].IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   tenants[i].ID,
			Username:   "migration-129",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		agent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenants[i].ID, operator)
		if err != nil {
			return fmt.Errorf("activate tenant %d knowledge citations: %w", tenants[i].ID, err)
		}
		if err := validateKnowledgeSupportCitationRelease(db, agent); err != nil {
			return fmt.Errorf("validate tenant %d knowledge citations: %w", tenants[i].ID, err)
		}
	}
	return nil
}

func validateKnowledgeSupportCitationRelease(db *gorm.DB, agent *models.AIAgent) error {
	if agent == nil || agent.ActiveReleaseID <= 0 {
		return fmt.Errorf("active release is missing")
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if release == nil || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive || release.ReviewStatus != enums.AIAgentReviewStatusApproved {
		return fmt.Errorf("active release is not approved and deployed")
	}
	workflow := repositories.AIWorkflowRepository.Get(db, release.WorkflowID)
	version := repositories.AIWorkflowVersionRepository.Get(db, release.WorkflowVersionID)
	if workflow == nil || workflow.Code != services.PlatformKnowledgeSupportWorkflowCode || version == nil || version.WorkflowID != workflow.ID {
		return fmt.Errorf("active workflow is not the knowledge support workflow")
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
		return fmt.Errorf("decode active workflow definition: %w", err)
	}
	if !knowledgeSupportDefinitionCarriesCitations(definition) {
		return fmt.Errorf("active workflow does not forward knowledge citations")
	}
	return nil
}

func knowledgeSupportDefinitionCarriesCitations(definition dsl.Definition) bool {
	knowledgeNodes := make(map[string]struct{})
	for _, node := range definition.Nodes {
		if node.Type == "knowledge_retrieve" {
			knowledgeNodes[node.ID] = struct{}{}
		}
	}
	for _, node := range definition.Nodes {
		if node.Type != "send_reply" {
			continue
		}
		selector, ok := node.Inputs["citations"]
		if !ok || selector.Field != "citations" {
			continue
		}
		if _, ok := knowledgeNodes[selector.NodeID]; ok {
			return true
		}
	}
	return false
}
