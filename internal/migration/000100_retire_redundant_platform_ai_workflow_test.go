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

func TestRetireRedundantPlatformAIWorkflowPreservesAuditHistory(t *testing.T) {
	db := setupRetiredPlatformAIWorkflowTestDB(t)
	workflow, version := seedRetiredPlatformAIWorkflow(t, db)

	if err := retireRedundantPlatformAIWorkflow(db); err != nil {
		t.Fatalf("retireRedundantPlatformAIWorkflow() error = %v", err)
	}
	if err := db.First(workflow, workflow.ID).Error; err != nil {
		t.Fatalf("reload workflow: %v", err)
	}
	if workflow.Status != enums.StatusDeleted {
		t.Fatalf("retired workflow status = %d, want deleted", workflow.Status)
	}
	if err := db.First(version, version.ID).Error; err != nil {
		t.Fatalf("reload version: %v", err)
	}
	if version.ReleaseChannel != models.AIWorkflowReleaseChannelDeprecated {
		t.Fatalf("retired version channel = %q, want deprecated", version.ReleaseChannel)
	}
	if err := retireRedundantPlatformAIWorkflow(db); err != nil {
		t.Fatalf("idempotent retirement error = %v", err)
	}
}

func TestRetireRedundantPlatformAIWorkflowRebindsLiveConfigurationWithoutAutoDeployment(t *testing.T) {
	db := setupRetiredPlatformAIWorkflowTestDB(t)
	workflow, version := seedRetiredPlatformAIWorkflow(t, db)
	agent := &models.AIAgent{
		TenantID: 7, ProductID: 70, Name: "legacy", WorkflowID: workflow.ID, WorkflowVersionID: version.ID,
		ActiveReleaseID: 999, DraftRevision: 3, ServiceMode: enums.IMConversationServiceModeAIFirst,
		ReviewStatus: enums.AIAgentReviewStatusApproved, Status: enums.StatusOk,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	release := &models.AIAgentRelease{TenantID: 7, ProductID: 70, AgentID: agent.ID, ReleaseNo: 1, WorkflowID: workflow.ID, WorkflowVersionID: version.ID, ReviewStatus: enums.AIAgentReviewStatusApproved, DeploymentStatus: models.AIAgentReleaseDeploymentActive, Status: enums.StatusOk}
	if err := db.Create(release).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(agent).Update("active_release_id", release.ID).Error; err != nil {
		t.Fatal(err)
	}
	historical := &models.AIAgentRelease{TenantID: 7, ProductID: 70, AgentID: agent.ID, ReleaseNo: 2, WorkflowID: workflow.ID, WorkflowVersionID: version.ID, ReviewStatus: enums.AIAgentReviewStatusApproved, DeploymentStatus: models.AIAgentReleaseDeploymentRetired, Status: enums.StatusOk}
	if err := db.Create(historical).Error; err != nil {
		t.Fatal(err)
	}
	profile := &models.ProductServiceProfile{TenantID: 9, ProductID: 90, DefaultFlowTemplateID: workflow.ID, Status: enums.StatusOk}
	if err := db.Create(profile).Error; err != nil {
		t.Fatal(err)
	}
	fork := &models.AIWorkflow{TenantID: 10, Scope: models.AIWorkflowScopeTenant, Code: "legacy-fork", Name: "legacy fork", SourceWorkflowID: workflow.ID, SourceVersionID: version.ID, Status: enums.StatusOk}
	if err := db.Create(fork).Error; err != nil {
		t.Fatal(err)
	}

	if err := retireRedundantPlatformAIWorkflow(db); err != nil {
		t.Fatalf("retireRedundantPlatformAIWorkflow() error = %v", err)
	}
	replacement := &models.AIWorkflow{}
	if err := db.Where("code = ? AND scope = ?", "aftersales_device_diagnosis_ai_only", models.AIWorkflowScopePlatform).First(replacement).Error; err != nil {
		t.Fatalf("load replacement workflow: %v", err)
	}
	if replacement.CurrentStableVersionID <= 0 {
		t.Fatalf("replacement has no stable version: %+v", replacement)
	}
	if err := db.First(agent, agent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if agent.WorkflowID != replacement.ID || agent.WorkflowVersionID != replacement.CurrentStableVersionID || agent.ActiveReleaseID != 0 || agent.DraftRevision != 4 {
		t.Fatalf("agent was not rebound to replacement draft: %+v", agent)
	}
	if agent.Status != enums.StatusDisabled || agent.ReviewStatus != enums.AIAgentReviewStatusUnreviewed || agent.ServiceMode != enums.IMConversationServiceModeAIOnly {
		t.Fatalf("agent should require manual review and deployment: %+v", agent)
	}
	if err := db.First(release, release.ID).Error; err != nil {
		t.Fatal(err)
	}
	if release.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("active legacy release status = %q, want retired", release.DeploymentStatus)
	}
	if err := db.First(historical, historical.ID).Error; err != nil {
		t.Fatal(err)
	}
	if historical.WorkflowID != workflow.ID || historical.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("historical release was rewritten: %+v", historical)
	}
	if err := db.First(profile, profile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if profile.DefaultFlowTemplateID != replacement.ID {
		t.Fatalf("profile workflow = %d, want %d", profile.DefaultFlowTemplateID, replacement.ID)
	}
	if err := db.First(fork, fork.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fork.SourceWorkflowID != replacement.ID || fork.SourceVersionID != replacement.CurrentStableVersionID {
		t.Fatalf("fork provenance was not rebound: %+v", fork)
	}
	if err := db.First(workflow, workflow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if workflow.Status != enums.StatusDeleted {
		t.Fatalf("legacy workflow status = %d, want deleted", workflow.Status)
	}
	if err := retireRedundantPlatformAIWorkflow(db); err != nil {
		t.Fatalf("idempotent retirement error = %v", err)
	}
	if err := db.First(agent, agent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if agent.DraftRevision != 4 {
		t.Fatalf("idempotent retirement changed draft revision to %d", agent.DraftRevision)
	}
}

func setupRetiredPlatformAIWorkflowTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "retire-redundant-platform-ai.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}, &models.AIAgent{}, &models.AIAgentRelease{}, &models.ProductServiceProfile{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedRetiredPlatformAIWorkflow(t *testing.T, db *gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion) {
	t.Helper()
	now := time.Now()
	workflow := &models.AIWorkflow{Code: retiredBasicAIWorkflowCode, Scope: models.AIWorkflowScopePlatform, Name: "retired basic AI", Locked: true, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatal(err)
	}
	version := &models.AIWorkflowVersion{WorkflowID: workflow.ID, Version: 1, Status: enums.StatusOk, Definition: `{}`, DefinitionHash: "retired-basic-ai", ReleaseChannel: models.AIWorkflowReleaseChannelStable, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(workflow).Updates(map[string]any{"current_stable_version_id": version.ID, "published_version_id": version.ID}).Error; err != nil {
		t.Fatal(err)
	}
	return workflow, version
}
