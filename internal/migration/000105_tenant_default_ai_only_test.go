package migration

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMigrateTenantDefaultAgentsToAIOnlyActivatesCorrectedRelease(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "tenant-default-ai-only.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.KnowledgeBase{},
		&models.KnowledgeRevision{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tenant := &models.Tenant{
		Name:        "AI-only Migration Tenant",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{
		TenantID:   tenant.ID,
		Username:   "system",
		DomainType: models.DomainTypeEnterprise,
		Domain:     models.DomainTypeEnterprise,
	}
	agent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenant.ID, operator)
	if err != nil {
		t.Fatalf("seed tenant default agent: %v", err)
	}
	legacyReleaseID := agent.ActiveReleaseID
	assistedWorkflow, assistedVersion, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	if err != nil {
		t.Fatalf("seed assisted workflow: %v", err)
	}
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"workflow_id":         assistedWorkflow.ID,
		"workflow_version_id": assistedVersion.ID,
		"service_mode":        enums.IMConversationServiceModeAIFirst,
		"team_ids":            "99",
	}); err != nil {
		t.Fatalf("seed assisted tenant default draft: %v", err)
	}
	if err := repositories.AIAgentReleaseRepository.Updates(db, legacyReleaseID, map[string]any{
		"workflow_id":              assistedWorkflow.ID,
		"workflow_version_id":      assistedVersion.ID,
		"workflow_definition_hash": assistedVersion.DefinitionHash,
	}); err != nil {
		t.Fatalf("seed assisted active release: %v", err)
	}

	if err := migrateTenantDefaultAgentsToAIOnly(db); err != nil {
		t.Fatalf("migrateTenantDefaultAgentsToAIOnly() error = %v", err)
	}
	migrated := repositories.AIAgentRepository.Get(db, agent.ID)
	if migrated == nil || migrated.ActiveReleaseID == legacyReleaseID || migrated.ServiceMode != enums.IMConversationServiceModeAIOnly || migrated.TeamIDs != "" {
		t.Fatalf("tenant default agent was not migrated to AI-only: %+v", migrated)
	}
	workflow := repositories.AIWorkflowRepository.Get(db, migrated.WorkflowID)
	active := repositories.AIAgentReleaseRepository.Get(db, migrated.ActiveReleaseID)
	legacy := repositories.AIAgentReleaseRepository.Get(db, legacyReleaseID)
	if workflow == nil || workflow.Code != services.PlatformDeviceAIOnlyWorkflowCode {
		t.Fatalf("migrated workflow = %+v", workflow)
	}
	if active == nil || active.WorkflowID != workflow.ID || active.ReviewStatus != enums.AIAgentReviewStatusApproved || active.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("migrated active release = %+v", active)
	}
	if legacy == nil || legacy.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("legacy assisted release was not retired: %+v", legacy)
	}

	var releaseCount int64
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&releaseCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateTenantDefaultAgentsToAIOnly(db); err != nil {
		t.Fatalf("idempotent migration error = %v", err)
	}
	var repeatedCount int64
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&repeatedCount).Error; err != nil {
		t.Fatal(err)
	}
	if repeatedCount != releaseCount {
		t.Fatalf("idempotent migration created releases: before=%d after=%d", releaseCount, repeatedCount)
	}
}

func TestMigrateTenantDefaultAgentsToAIOnlyActivatesCurrentManagedConfigCandidate(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "tenant-default-general-reply.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.KnowledgeBase{},
		&models.KnowledgeRevision{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tenant := &models.Tenant{
		Name:        "General Reply Migration Tenant",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{
		TenantID:   tenant.ID,
		Username:   "system",
		DomainType: models.DomainTypeEnterprise,
		Domain:     models.DomainTypeEnterprise,
	}
	agent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenant.ID, operator)
	if err != nil {
		t.Fatalf("seed tenant default agent: %v", err)
	}
	originalReleaseID := agent.ActiveReleaseID
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"system_prompt":    "legacy general consultation prompt",
		"fallback_message": "legacy device fault fallback",
	}); err != nil {
		t.Fatalf("seed stale managed configuration: %v", err)
	}
	staged, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenant.ID, operator)
	if err != nil {
		t.Fatalf("stage managed configuration candidate: %v", err)
	}
	if staged.ActiveReleaseID != originalReleaseID {
		t.Fatalf("managed configuration was activated before migration: before=%d after=%d", originalReleaseID, staged.ActiveReleaseID)
	}

	if err := migrateTenantDefaultAgentsToAIOnly(db); err != nil {
		t.Fatalf("activate managed configuration candidate: %v", err)
	}
	activated := repositories.AIAgentRepository.Get(db, agent.ID)
	if activated == nil || activated.ActiveReleaseID == originalReleaseID {
		t.Fatalf("managed configuration candidate was not activated: %+v", activated)
	}
	release := repositories.AIAgentReleaseRepository.Get(db, activated.ActiveReleaseID)
	if release == nil || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive || release.ReviewStatus != enums.AIAgentReviewStatusApproved {
		t.Fatalf("managed configuration release is not active: %+v", release)
	}
	snapshot := dto.AIAgentReleaseConfigSnapshot{}
	if err := json.Unmarshal([]byte(release.AgentConfigSnapshot), &snapshot); err != nil {
		t.Fatalf("decode active configuration snapshot: %v", err)
	}
	if snapshot.SystemPrompt != activated.SystemPrompt || snapshot.FallbackMessage != workflowcapability.SafeKnowledgeFallbackMessage {
		t.Fatalf("active managed configuration is stale: %+v", snapshot)
	}
}
