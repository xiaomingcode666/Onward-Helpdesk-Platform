package migration

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConsolidateAIWorkflowsToPlatformDefaultPreservesReleaseHistory(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-consolidation.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
		&models.ProductServiceProfile{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	legacy := &models.AIWorkflow{
		TenantID: 1, Code: "legacy_agent_1", Scope: models.AIWorkflowScopeTenant,
		Name: "历史私有流程", AgentID: 1, Status: enums.StatusOk, DraftDefinition: `{}`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy workflow: %v", err)
	}
	legacyVersion := &models.AIWorkflowVersion{
		WorkflowID: legacy.ID, Version: 1, Status: enums.StatusOk, Definition: `{}`,
		DefinitionHash: workflowDefinitionHashForMigration(`{}`), ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		SchemaVersion: 1, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacyVersion).Error; err != nil {
		t.Fatalf("create legacy version: %v", err)
	}
	agent := &models.AIAgent{
		TenantID: 1, ProductID: 7, Name: "测试机器人", WorkflowID: legacy.ID,
		WorkflowVersionID: legacyVersion.ID, DraftRevision: 1, Status: enums.StatusOk,
		ReviewStatus: enums.AIAgentReviewStatusApproved,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	active := &models.AIAgentRelease{
		TenantID: 1, ProductID: 7, AgentID: agent.ID, ReleaseNo: 1,
		WorkflowID: legacy.ID, WorkflowVersionID: legacyVersion.ID,
		WorkflowDefinitionHash: legacyVersion.DefinitionHash,
		AgentConfigSnapshot:    `{}`, AgentConfigHash: "agent-hash",
		KnowledgeScopeSnapshot: `{}`, KnowledgeScopeHash: "knowledge-hash",
		ReviewStatus: enums.AIAgentReviewStatusApproved, DeploymentStatus: models.AIAgentReleaseDeploymentActive,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create active release: %v", err)
	}
	if err := db.Model(agent).Update("active_release_id", active.ID).Error; err != nil {
		t.Fatalf("bind active release: %v", err)
	}
	profile := &models.ProductServiceProfile{TenantID: 1, ProductID: 7, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(profile).Error; err != nil {
		t.Fatalf("create product profile: %v", err)
	}

	if err := consolidateAIWorkflowsToPlatformDefault(db); err != nil {
		t.Fatalf("consolidateAIWorkflowsToPlatformDefault() error = %v", err)
	}
	if err := consolidateAIWorkflowsToPlatformDefault(db); err != nil {
		t.Fatalf("idempotent consolidation error = %v", err)
	}

	var platform models.AIWorkflow
	if err := db.Where("scope = ? AND code = ?", models.AIWorkflowScopePlatform, "aftersales_customer_service_default").First(&platform).Error; err != nil {
		t.Fatalf("load platform workflow: %v", err)
	}
	var refreshedAgent models.AIAgent
	if err := db.First(&refreshedAgent, agent.ID).Error; err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if refreshedAgent.WorkflowID != platform.ID || refreshedAgent.WorkflowVersionID != platform.CurrentStableVersionID || refreshedAgent.ActiveReleaseID == active.ID {
		t.Fatalf("agent was not consolidated: %+v", refreshedAgent)
	}
	var oldRelease, newRelease models.AIAgentRelease
	if err := db.First(&oldRelease, active.ID).Error; err != nil {
		t.Fatalf("load old release: %v", err)
	}
	if err := db.First(&newRelease, refreshedAgent.ActiveReleaseID).Error; err != nil {
		t.Fatalf("load replacement release: %v", err)
	}
	if oldRelease.DeploymentStatus != models.AIAgentReleaseDeploymentRetired || newRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive || newRelease.WorkflowID != platform.ID {
		t.Fatalf("unexpected release transition: old=%+v new=%+v", oldRelease, newRelease)
	}
	var refreshedLegacy models.AIWorkflow
	if err := db.First(&refreshedLegacy, legacy.ID).Error; err != nil {
		t.Fatalf("load legacy workflow: %v", err)
	}
	if refreshedLegacy.Status != enums.StatusDeleted {
		t.Fatalf("legacy workflow remains active: %+v", refreshedLegacy)
	}
	var refreshedLegacyVersion models.AIWorkflowVersion
	if err := db.First(&refreshedLegacyVersion, legacyVersion.ID).Error; err != nil {
		t.Fatalf("load legacy workflow version: %v", err)
	}
	if refreshedLegacyVersion.Status != enums.StatusDeleted {
		t.Fatalf("legacy workflow version remains active: %+v", refreshedLegacyVersion)
	}
	var refreshedProfile models.ProductServiceProfile
	if err := db.First(&refreshedProfile, profile.ID).Error; err != nil {
		t.Fatalf("load product profile: %v", err)
	}
	if refreshedProfile.DefaultFlowTemplateID != platform.ID {
		t.Fatalf("product profile default flow = %d, want %d", refreshedProfile.DefaultFlowTemplateID, platform.ID)
	}
}
