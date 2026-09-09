package migration

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureTenantDefaultAIAgentsCreatesRunnableTenantEntry(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "tenant-default-agent.db")), &gorm.Config{
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
		&models.Department{},
		&models.AgentTeam{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tenant := &models.Tenant{Name: "Default Agent Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	legacyKnowledgeBase := &models.KnowledgeBase{
		TenantID:      tenant.ID,
		Name:          "企业通用知识库",
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		AccessScope:   string(enums.KnowledgeBaseAccessScopeTenant),
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacyKnowledgeBase).Error; err != nil {
		t.Fatal(err)
	}

	if err := ensureTenantDefaultAIAgents(db); err != nil {
		t.Fatalf("ensureTenantDefaultAIAgents() error = %v", err)
	}
	agent := repositories.AIAgentRepository.Take(db,
		"tenant_id = ? AND product_id = 0 AND source = ? AND status <> ?",
		tenant.ID, services.TenantDefaultAIAgentSource, enums.StatusDeleted)
	if agent == nil || agent.Status != enums.StatusOk || agent.ServiceMode != enums.IMConversationServiceModeAIOnly || agent.TeamIDs != "" {
		t.Fatalf("tenant default agent = %+v", agent)
	}
	workflow := repositories.AIWorkflowRepository.Get(db, agent.WorkflowID)
	if workflow == nil || workflow.Code != services.PlatformDeviceAIOnlyWorkflowCode || agent.WorkflowVersionID != workflow.CurrentStableVersionID {
		t.Fatalf("tenant default workflow binding = agent %+v workflow %+v", agent, workflow)
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if release == nil || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive || release.ReviewStatus != enums.AIAgentReviewStatusApproved {
		t.Fatalf("tenant default release = %+v", release)
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, firstTenantDefaultKnowledgeID(agent.KnowledgeIDs))
	if knowledgeBase == nil || knowledgeBase.ID != legacyKnowledgeBase.ID || knowledgeBase.AccessScope != string(enums.KnowledgeBaseAccessScopeTenant) || knowledgeBase.Remark != "builtin-tenant-default-service" {
		t.Fatalf("tenant default knowledge base = %+v", knowledgeBase)
	}

	firstReleaseID := agent.ActiveReleaseID
	firstUpdatedAt := agent.UpdatedAt
	if err := ensureTenantDefaultAIAgents(db); err != nil {
		t.Fatalf("idempotent ensure error = %v", err)
	}
	again := repositories.AIAgentRepository.Get(db, agent.ID)
	if again == nil || again.ActiveReleaseID != firstReleaseID {
		t.Fatalf("idempotent ensure changed default release: before=%d after=%+v", firstReleaseID, again)
	}
	if !again.UpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("idempotent ensure changed audit timestamp: before=%v after=%v", firstUpdatedAt, again.UpdatedAt)
	}
	var releaseCount int64
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&releaseCount).Error; err != nil {
		t.Fatalf("count tenant default releases: %v", err)
	}
	if releaseCount != 1 {
		t.Fatalf("idempotent ensure created %d releases, want 1", releaseCount)
	}

	if err := db.Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(map[string]any{
		"service_mode":        enums.IMConversationServiceModeHumanOnly,
		"team_ids":            "99",
		"knowledge_ids":       "",
		"skill_ids":           "88",
		"allowed_mcp_tools":   `[{"name":"handoff"}]`,
		"allowed_graph_tools": `["create_ticket"]`,
	}).Error; err != nil {
		t.Fatalf("corrupt tenant default agent: %v", err)
	}
	if err := ensureTenantDefaultAIAgents(db); err != nil {
		t.Fatalf("repair tenant default agent: %v", err)
	}
	repaired := repositories.AIAgentRepository.Get(db, agent.ID)
	if repaired == nil || repaired.ServiceMode != enums.IMConversationServiceModeAIOnly || repaired.TeamIDs != "" || repaired.KnowledgeIDs == "" || repaired.SkillIDs != "" || repaired.AllowedMCPTools != "" || repaired.AllowedGraphTools != "" {
		t.Fatalf("tenant default agent was not repaired: %+v", repaired)
	}
	if repaired.ActiveReleaseID != firstReleaseID {
		t.Fatalf("agent repair changed production release without review: before=%d after=%d", firstReleaseID, repaired.ActiveReleaseID)
	}
	candidates := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
		Desc("release_no"))
	if len(candidates) != 1 || candidates[0].ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("agent repair should create one unreviewed candidate: %+v", candidates)
	}

	if err := db.Model(&models.AIWorkflowVersion{}).Where("id = ?", workflow.CurrentStableVersionID).
		Update("definition_hash", "obsolete-platform-definition").Error; err != nil {
		t.Fatalf("mark platform stable version obsolete: %v", err)
	}
	if err := ensureTenantDefaultAIAgents(db); err != nil {
		t.Fatalf("upgrade platform-managed tenant default agent: %v", err)
	}
	upgraded := repositories.AIAgentRepository.Get(db, agent.ID)
	if upgraded == nil || upgraded.WorkflowVersionID == repaired.WorkflowVersionID || upgraded.ActiveReleaseID != firstReleaseID {
		t.Fatalf("platform-managed upgrade should update only the draft: repaired=%+v upgraded=%+v", repaired, upgraded)
	}
	latestCandidates := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
		Desc("release_no"))
	if len(latestCandidates) != 2 || latestCandidates[0].WorkflowVersionID != upgraded.WorkflowVersionID || latestCandidates[0].ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("platform upgrade should create a second unreviewed candidate: %+v", latestCandidates)
	}
}

func firstTenantDefaultKnowledgeID(raw string) int64 {
	ids := utils.SplitInt64s(raw)
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}
