package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func migrationTestHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestRemoveLegacyWorkflowRoutingMigratesActiveUseAndRetainsAuditHistory(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "remove-legacy-workflows.db")), &gorm.Config{
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
	legacyDefinition := `{"schemaVersion":1}`
	platform := &models.AIWorkflow{
		Code:            services.PlatformDefaultAfterSalesWorkflowCode,
		Scope:           models.AIWorkflowScopePlatform,
		Name:            "Legacy platform workflow",
		DraftDefinition: legacyDefinition,
		Locked:          true,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(platform).Error; err != nil {
		t.Fatalf("create platform workflow: %v", err)
	}
	legacyPlatformVersion := &models.AIWorkflowVersion{
		WorkflowID:      platform.ID,
		Version:         1,
		Status:          enums.StatusOk,
		Definition:      legacyDefinition,
		DefinitionHash:  migrationTestHash(legacyDefinition),
		ReleaseChannel:  models.AIWorkflowReleaseChannelStable,
		PublishedAt:     &now,
		PublishedByName: "system",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacyPlatformVersion).Error; err != nil {
		t.Fatalf("create legacy platform version: %v", err)
	}
	if err := db.Model(platform).Updates(map[string]any{
		"current_stable_version_id": legacyPlatformVersion.ID,
		"published_version_id":      legacyPlatformVersion.ID,
	}).Error; err != nil {
		t.Fatalf("bind legacy platform version: %v", err)
	}

	fork := &models.AIWorkflow{
		TenantID:               1,
		Code:                   "legacy-enterprise-flow",
		Scope:                  models.AIWorkflowScopeTenant,
		Name:                   "Legacy enterprise workflow",
		SourceWorkflowID:       platform.ID,
		SourceVersionID:        legacyPlatformVersion.ID,
		Status:                 enums.StatusOk,
		DraftDefinition:        legacyDefinition,
		CurrentStableVersionID: 0,
		AuditFields:            models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(fork).Error; err != nil {
		t.Fatalf("create enterprise fork: %v", err)
	}
	forkVersion := &models.AIWorkflowVersion{
		WorkflowID:      fork.ID,
		Version:         1,
		Status:          enums.StatusOk,
		Definition:      legacyDefinition,
		DefinitionHash:  migrationTestHash(legacyDefinition),
		ReleaseChannel:  models.AIWorkflowReleaseChannelStable,
		PublishedAt:     &now,
		PublishedByName: "enterprise",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(forkVersion).Error; err != nil {
		t.Fatalf("create enterprise fork version: %v", err)
	}
	if err := db.Model(fork).Updates(map[string]any{
		"current_stable_version_id": forkVersion.ID,
		"published_version_id":      forkVersion.ID,
	}).Error; err != nil {
		t.Fatalf("bind enterprise fork version: %v", err)
	}

	agent := &models.AIAgent{
		TenantID:          1,
		ProductID:         10,
		Name:              "Legacy enterprise agent",
		Status:            enums.StatusOk,
		ServiceMode:       enums.IMConversationServiceModeAIFirst,
		WorkflowID:        fork.ID,
		WorkflowVersionID: forkVersion.ID,
		DraftRevision:     3,
		ReviewStatus:      enums.AIAgentReviewStatusApproved,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	configRaw, err := json.Marshal(dto.AIAgentReleaseConfigSnapshot{
		SchemaVersion: 1,
		DraftRevision: agent.DraftRevision,
		ServiceMode:   agent.ServiceMode,
	})
	if err != nil {
		t.Fatalf("marshal release snapshot: %v", err)
	}
	knowledgeRaw := `{"schemaVersion":1,"tenantId":1,"productId":10,"bindings":[],"revisions":[]}`
	legacyRelease := &models.AIAgentRelease{
		TenantID:               agent.TenantID,
		ProductID:              agent.ProductID,
		AgentID:                agent.ID,
		ReleaseNo:              1,
		WorkflowID:             fork.ID,
		WorkflowVersionID:      forkVersion.ID,
		WorkflowDefinitionHash: forkVersion.DefinitionHash,
		AgentConfigSnapshot:    string(configRaw),
		AgentConfigHash:        migrationTestHash(string(configRaw)),
		KnowledgeScopeSnapshot: knowledgeRaw,
		KnowledgeScopeHash:     migrationTestHash(knowledgeRaw),
		ReviewStatus:           enums.AIAgentReviewStatusApproved,
		DeploymentStatus:       models.AIAgentReleaseDeploymentActive,
		Status:                 enums.StatusOk,
		AuditFields:            models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacyRelease).Error; err != nil {
		t.Fatalf("create active legacy release: %v", err)
	}
	if err := db.Model(agent).Update("active_release_id", legacyRelease.ID).Error; err != nil {
		t.Fatalf("bind active legacy release: %v", err)
	}
	profile := &models.ProductServiceProfile{
		TenantID:              agent.TenantID,
		ProductID:             agent.ProductID,
		DefaultFlowTemplateID: fork.ID,
		Status:                enums.StatusOk,
		AuditFields:           models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(profile).Error; err != nil {
		t.Fatalf("create product service profile: %v", err)
	}

	if err := removeLegacyWorkflowRouting(db); err != nil {
		t.Fatalf("removeLegacyWorkflowRouting() error = %v", err)
	}
	var currentPlatform models.AIWorkflow
	if err := db.First(&currentPlatform, platform.ID).Error; err != nil {
		t.Fatalf("reload platform workflow: %v", err)
	}
	if currentPlatform.CurrentStableVersionID == legacyPlatformVersion.ID {
		t.Fatalf("platform workflow still points to the legacy version: %+v", currentPlatform)
	}
	var currentAgent models.AIAgent
	if err := db.First(&currentAgent, agent.ID).Error; err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if currentAgent.WorkflowID != platform.ID || currentAgent.WorkflowVersionID != currentPlatform.CurrentStableVersionID || currentAgent.ActiveReleaseID != 0 || currentAgent.Status != enums.StatusDisabled || currentAgent.ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("agent still uses legacy routing: %+v", currentAgent)
	}
	var retiredRelease models.AIAgentRelease
	if err := db.First(&retiredRelease, legacyRelease.ID).Error; err != nil {
		t.Fatalf("reload legacy release: %v", err)
	}
	if retiredRelease.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("legacy release was not retained as retired audit history: %+v", retiredRelease)
	}
	var retiredFork models.AIWorkflow
	if err := db.First(&retiredFork, fork.ID).Error; err != nil {
		t.Fatalf("reload enterprise fork: %v", err)
	}
	if retiredFork.Status != enums.StatusDeleted {
		t.Fatalf("enterprise fork was not retired: %+v", retiredFork)
	}
	var retiredForkVersion models.AIWorkflowVersion
	if err := db.First(&retiredForkVersion, forkVersion.ID).Error; err != nil {
		t.Fatalf("reload enterprise fork version: %v", err)
	}
	if retiredForkVersion.ReleaseChannel != models.AIWorkflowReleaseChannelDeprecated {
		t.Fatalf("enterprise fork version was not deprecated: %+v", retiredForkVersion)
	}
	var retiredPlatformVersion models.AIWorkflowVersion
	if err := db.First(&retiredPlatformVersion, legacyPlatformVersion.ID).Error; err != nil {
		t.Fatalf("reload legacy platform version: %v", err)
	}
	if retiredPlatformVersion.ReleaseChannel != models.AIWorkflowReleaseChannelDeprecated {
		t.Fatalf("legacy platform version was not deprecated: %+v", retiredPlatformVersion)
	}
	var currentProfile models.ProductServiceProfile
	if err := db.First(&currentProfile, profile.ID).Error; err != nil {
		t.Fatalf("reload product profile: %v", err)
	}
	if currentProfile.DefaultFlowTemplateID != platform.ID {
		t.Fatalf("product profile still points to the enterprise fork: %+v", currentProfile)
	}

	if err := removeLegacyWorkflowRouting(db); err != nil {
		t.Fatalf("second removeLegacyWorkflowRouting() error = %v", err)
	}
	var releaseCount int64
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&releaseCount).Error; err != nil {
		t.Fatalf("count releases: %v", err)
	}
	if releaseCount != 1 {
		t.Fatalf("migration is not idempotent, release count = %d", releaseCount)
	}

	currentVersion := models.AIWorkflowVersion{}
	if err := db.First(&currentVersion, currentPlatform.CurrentStableVersionID).Error; err != nil {
		t.Fatalf("reload current platform version: %v", err)
	}
	autoRelease := &models.AIAgentRelease{
		TenantID:               agent.TenantID,
		ProductID:              agent.ProductID,
		AgentID:                agent.ID,
		ReleaseNo:              2,
		WorkflowID:             platform.ID,
		WorkflowVersionID:      currentVersion.ID,
		WorkflowDefinitionHash: currentVersion.DefinitionHash,
		AgentConfigSnapshot:    string(configRaw),
		AgentConfigHash:        migrationTestHash(string(configRaw)),
		KnowledgeScopeSnapshot: knowledgeRaw,
		KnowledgeScopeHash:     migrationTestHash(knowledgeRaw),
		ReviewStatus:           enums.AIAgentReviewStatusApproved,
		ReviewComment:          platformWorkflowAutomaticUpgradeComment,
		ReviewedByName:         "system",
		DeploymentStatus:       models.AIAgentReleaseDeploymentActive,
		DeployedByName:         "system",
		Status:                 enums.StatusOk,
		AuditFields:            models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(autoRelease).Error; err != nil {
		t.Fatalf("create automatically activated release: %v", err)
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(map[string]any{
		"active_release_id": autoRelease.ID,
		"status":            enums.StatusOk,
		"review_status":     enums.AIAgentReviewStatusApproved,
	}).Error; err != nil {
		t.Fatalf("activate automatic release: %v", err)
	}
	if err := enforceManualWorkflowReleaseDeployment(db); err != nil {
		t.Fatalf("enforceManualWorkflowReleaseDeployment() error = %v", err)
	}
	if err := db.First(&currentAgent, agent.ID).Error; err != nil {
		t.Fatalf("reload agent after deployment enforcement: %v", err)
	}
	if currentAgent.ActiveReleaseID != 0 || currentAgent.Status != enums.StatusDisabled || currentAgent.ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("automatically activated release still controls the agent: %+v", currentAgent)
	}
	if err := db.First(autoRelease, autoRelease.ID).Error; err != nil {
		t.Fatalf("reload automatically activated release: %v", err)
	}
	if autoRelease.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("automatically activated release was not retired: %+v", autoRelease)
	}
	if err := enforceManualWorkflowReleaseDeployment(db); err != nil {
		t.Fatalf("second enforceManualWorkflowReleaseDeployment() error = %v", err)
	}
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&releaseCount).Error; err != nil {
		t.Fatalf("count releases after enforcement: %v", err)
	}
	if releaseCount != 2 {
		t.Fatalf("deployment enforcement is not idempotent, release count = %d", releaseCount)
	}
}
