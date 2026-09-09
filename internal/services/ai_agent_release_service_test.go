package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func TestAIAgentReleaseLifecycleKeepsActiveReleaseWhileEditingDraft(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID, Code: "RELEASE-1", Name: "Release Product", DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent() error = %v", err)
	}
	first := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if first == nil || first.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("product creation should deploy the initial release: %+v", first)
	}
	originalPrompt := agent.SystemPrompt

	deployed := AIAgentService.Get(agent.ID)
	if deployed == nil || deployed.ActiveReleaseID != first.ID || deployed.Status != enums.StatusOk {
		t.Fatalf("unexpected deployed agent: %+v", deployed)
	}
	firstRevision := deployed.DraftRevision
	if err := AIAgentService.UpdateAIAgent(request.UpdateAIAgentRequest{
		ID: deployed.ID,
		CreateAIAgentRequest: request.CreateAIAgentRequest{
			Name:                deployed.Name,
			Description:         deployed.Description,
			AIConfigID:          deployed.AIConfigID,
			LLMModelName:        deployed.LLMModelName,
			ServiceMode:         deployed.ServiceMode,
			SystemPrompt:        originalPrompt + "\n草稿新增诊断约束。",
			WelcomeMessage:      deployed.WelcomeMessage,
			ReplyTimeoutSeconds: deployed.ReplyTimeoutSeconds,
			TeamIDs:             utils.SplitInt64s(deployed.TeamIDs),
			HandoffMode:         deployed.HandoffMode,
			FallbackMode:        deployed.FallbackMode,
			FallbackMessage:     deployed.FallbackMessage,
			KnowledgeIDs:        utils.SplitInt64s(deployed.KnowledgeIDs),
			SkillIDs:            utils.SplitInt64s(deployed.SkillIDs),
		},
	}, fixture.Operator); err != nil {
		t.Fatalf("UpdateAIAgent() error = %v", err)
	}
	edited := AIAgentService.Get(agent.ID)
	if edited.ActiveReleaseID != first.ID || edited.Status != enums.StatusOk || edited.DraftRevision != firstRevision+1 {
		t.Fatalf("editing draft must not stop or switch production: %+v", edited)
	}
	runtimeAgent, runtimeRelease, err := AIAgentReleaseService.MaterializeRuntimeAgent(db, *edited)
	if err != nil {
		t.Fatalf("MaterializeRuntimeAgent() error = %v", err)
	}
	if runtimeRelease == nil || runtimeRelease.ID != first.ID || runtimeAgent.SystemPrompt != originalPrompt {
		t.Fatalf("runtime must keep using first release snapshot: release=%+v prompt=%q", runtimeRelease, runtimeAgent.SystemPrompt)
	}

	second := createApprovedAgentRelease(t, agent.ID, fixture.Operator)
	if second.AgentConfigHash == first.AgentConfigHash {
		t.Fatalf("prompt change must alter the release config hash")
	}
	if err := AIAgentReleaseService.Deploy(second.ID, fixture.Operator); err != nil {
		t.Fatalf("Deploy(second) error = %v", err)
	}
	firstAfter := repositories.AIAgentReleaseRepository.Get(db, first.ID)
	secondAfter := repositories.AIAgentReleaseRepository.Get(db, second.ID)
	if firstAfter.DeploymentStatus != models.AIAgentReleaseDeploymentRetired || secondAfter.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("deploy should atomically retire the previous release: first=%+v second=%+v", firstAfter, secondAfter)
	}
	if err := AIAgentReleaseService.Rollback(first.ID, fixture.Operator); err != nil {
		t.Fatalf("Rollback(first) error = %v", err)
	}
	firstAfter = repositories.AIAgentReleaseRepository.Get(db, first.ID)
	secondAfter = repositories.AIAgentReleaseRepository.Get(db, second.ID)
	if firstAfter.DeploymentStatus != models.AIAgentReleaseDeploymentActive || secondAfter.DeploymentStatus != models.AIAgentReleaseDeploymentRolledBack {
		t.Fatalf("rollback should restore the approved historical release: first=%+v second=%+v", firstAfter, secondAfter)
	}

	otherTenant := &dto.AuthPrincipal{UserID: 777, Username: "other-tenant", TenantID: fixture.Tenant.ID + 1, DomainType: "enterprise", Domain: "enterprise"}
	if item := AIAgentReleaseService.Get(first.ID, otherTenant); item != nil {
		t.Fatalf("cross-tenant release read should be rejected: %+v", item)
	}
	if err := AIAgentReleaseService.Deploy(first.ID, otherTenant); err == nil {
		t.Fatalf("cross-tenant release deployment should fail")
	}

	active := repositories.AIAgentReleaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentActive))
	if len(active) != 1 || active[0].ID != first.ID {
		t.Fatalf("expected exactly one active release, got %+v", active)
	}
}

func TestPlatformWorkflowManifestUpgradeKeepsReviewedReleaseRunningUntilManualUpgrade(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID, Code: "RELEASE-PINNED", Name: "Pinned Release Product", DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent() error = %v", err)
	}
	active := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if active == nil || active.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("product creation should deploy the initial release: %+v", active)
	}
	workflow := AIWorkflowService.Get(agent.WorkflowID)
	definition := AIWorkflowService.DefaultAgentWorkflowDefinition()
	for i := range definition.Nodes {
		if definition.Nodes[i].ID == "retrieve_1" {
			definition.Nodes[i].Config = json.RawMessage(`{"scope":"public_product","topK":12,"scoreThreshold":0.35}`)
		}
	}
	spec, ok, err := platformBuiltInWorkflowSpec(workflow.Code)
	if err != nil || !ok {
		t.Fatalf("platformBuiltInWorkflowSpec() = ok %v error %v", ok, err)
	}
	spec.definition = definition
	_, latest, err := AIWorkflowService.ensurePlatformBuiltInWorkflowDB(db, spec)
	if err != nil {
		t.Fatalf("ensurePlatformBuiltInWorkflowDB() error = %v", err)
	}
	if latest.ID == active.WorkflowVersionID {
		t.Fatal("platform manifest upgrade did not create a new stable version")
	}
	stored := AIAgentService.Get(agent.ID)
	if stored == nil || stored.ActiveReleaseID != active.ID {
		t.Fatalf("platform manifest upgrade changed the active release: %+v", stored)
	}
	if _, runtimeRelease, err := AIAgentReleaseService.MaterializeRuntimeAgent(db, *stored); err != nil || runtimeRelease == nil || runtimeRelease.ID != active.ID {
		t.Fatalf("reviewed prior stable release stopped before manual upgrade: release=%+v err=%v", runtimeRelease, err)
	}
	if err := AIAgentReleaseService.Deploy(active.ID, fixture.Operator); err == nil || !strings.Contains(err.Error(), "历史版本不能再次部署") {
		t.Fatalf("Deploy(historical platform release) error = %v", err)
	}
	if err := db.Model(&models.AIWorkflowVersion{}).Where("id = ?", active.WorkflowVersionID).Update("release_channel", models.AIWorkflowReleaseChannelDeprecated).Error; err != nil {
		t.Fatalf("deprecate prior workflow version: %v", err)
	}
	if _, _, err := AIAgentReleaseService.MaterializeRuntimeAgent(db, *stored); err == nil || !strings.Contains(err.Error(), "已经退役") {
		t.Fatalf("MaterializeRuntimeAgent(deprecated release) error = %v", err)
	}
}

func TestAIAgentReleaseCandidateIsIdempotentAndSnapshotHashIsCanonical(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID, Code: "RELEASE-IDEMPOTENT", Name: "Idempotent Release", DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent() error = %v", err)
	}
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"welcome_message": agent.WelcomeMessage + " candidate draft",
	}); err != nil {
		t.Fatalf("seed candidate draft: %v", err)
	}
	first, err := AIAgentReleaseService.CreateCandidate(agent.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateCandidate(first) error = %v", err)
	}
	second, err := AIAgentReleaseService.CreateCandidate(agent.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateCandidate(second) error = %v", err)
	}
	if second.ID != first.ID || second.ReleaseNo != first.ReleaseNo {
		t.Fatalf("identical candidate creation must be idempotent: first=%+v second=%+v", first, second)
	}
	var count int64
	if err := db.Model(&models.AIAgentRelease{}).Where("agent_id = ?", agent.ID).Count(&count).Error; err != nil {
		t.Fatalf("count releases: %v", err)
	}
	if count != 2 {
		t.Fatalf("release count = %d, want initial release plus one candidate", count)
	}

	left := *agent
	left.TeamIDs = "3,1,2,1"
	left.KnowledgeIDs = "9,7,9"
	left.SkillIDs = "5,4"
	left.AllowedGraphTools = `["tool.z","tool.a","tool.a"]`
	left.AllowedMCPTools = `[{"toolCode":"mcp.z"},{"toolCode":"mcp.a"}]`
	right := left
	right.TeamIDs = "2,1,3"
	right.KnowledgeIDs = "7,9"
	right.SkillIDs = "4,5"
	right.AllowedGraphTools = `["tool.a","tool.z"]`
	right.AllowedMCPTools = `[{"toolCode":"mcp.a"},{"toolCode":"mcp.z"}]`
	_, leftHash, err := buildAgentReleaseConfigSnapshot(&left)
	if err != nil {
		t.Fatalf("build left snapshot: %v", err)
	}
	_, rightHash, err := buildAgentReleaseConfigSnapshot(&right)
	if err != nil {
		t.Fatalf("build right snapshot: %v", err)
	}
	if leftHash != rightHash {
		t.Fatalf("set ordering must not change snapshot hash: left=%s right=%s", leftHash, rightHash)
	}
}

func TestAIAgentReleaseCandidateEnforcesWorkflowCapabilityBoundary(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID, Code: "RELEASE-AI-ONLY", Name: "AI-only Release Product", DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent() error = %v", err)
	}
	_, version, err := AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB(db)
	if err != nil {
		t.Fatalf("EnsurePlatformDeviceAIOnlyWorkflowDB() error = %v", err)
	}
	if err := AIWorkflowService.BindAgentWorkflowVersion(agent.ID, version.ID, fixture.Operator); err != nil {
		t.Fatalf("BindAgentWorkflowVersion() error = %v", err)
	}
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"service_mode":     enums.IMConversationServiceModeAIFirst,
		"team_ids":         "8,9",
		"fallback_message": "当前知识不足，我会协助转接人工工程师。",
	}); err != nil {
		t.Fatalf("seed inconsistent AI-only draft: %v", err)
	}

	release, err := AIAgentReleaseService.CreateCandidate(agent.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateCandidate() error = %v", err)
	}
	var snapshot dto.AIAgentReleaseConfigSnapshot
	if err := json.Unmarshal([]byte(release.AgentConfigSnapshot), &snapshot); err != nil {
		t.Fatalf("unmarshal release snapshot: %v", err)
	}
	if snapshot.ServiceMode != enums.IMConversationServiceModeAIOnly || len(snapshot.TeamIDs) != 0 {
		t.Fatalf("AI-only release retained handoff configuration: %+v", snapshot)
	}
	if snapshot.FallbackMessage != workflowcapability.SafeKnowledgeFallbackMessage {
		t.Fatalf("AI-only release fallback = %q, want %q", snapshot.FallbackMessage, workflowcapability.SafeKnowledgeFallbackMessage)
	}

	snapshot.ServiceMode = enums.IMConversationServiceModeAIFirst
	snapshot.TeamIDs = []int64{8, 9}
	snapshot.FallbackMessage = "当前知识不足，我会协助转接人工工程师。"
	unsafeSnapshot, unsafeHash, err := marshalSnapshot(snapshot)
	if err != nil {
		t.Fatalf("marshal unsafe legacy snapshot: %v", err)
	}
	if err := repositories.AIAgentReleaseRepository.Updates(db, release.ID, map[string]any{
		"agent_config_snapshot": unsafeSnapshot,
		"agent_config_hash":     unsafeHash,
		"review_status":         enums.AIAgentReviewStatusApproved,
	}); err != nil {
		t.Fatalf("seed unsafe legacy release: %v", err)
	}
	if err := AIAgentReleaseService.Deploy(release.ID, fixture.Operator); err == nil || !strings.Contains(err.Error(), "能力边界") {
		t.Fatalf("Deploy(unsafe AI-only release) error = %v, want capability boundary rejection", err)
	}
}

func TestTenantDefaultAIAgentRejectsManualWorkflowAndReleaseOperationsButAllowsModelDraft(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	agent, err := TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB() error = %v", err)
	}
	if agent == nil || agent.ActiveReleaseID <= 0 {
		t.Fatalf("tenant default agent missing active release: %+v", agent)
	}

	if _, err := AIAgentReleaseService.CreateCandidate(agent.ID, fixture.Operator); err == nil || !strings.Contains(err.Error(), "managed by the platform") {
		t.Fatalf("CreateCandidate(default agent) error = %v", err)
	}
	if err := AIWorkflowService.BindAgentWorkflowVersion(agent.ID, agent.WorkflowVersionID, fixture.Operator); err == nil || !strings.Contains(err.Error(), "managed by the platform") {
		t.Fatalf("BindAgentWorkflowVersion(default agent) error = %v", err)
	}
	originalActiveReleaseID := agent.ActiveReleaseID
	if err := AIAgentService.UpdateAIAgent(request.UpdateAIAgentRequest{
		ID: agent.ID,
		CreateAIAgentRequest: request.CreateAIAgentRequest{
			AIConfigID:   agent.AIConfigID,
			LLMModelName: "gpt-5.4",
		},
	}, fixture.Operator); err != nil {
		t.Fatalf("UpdateAIAgent(default agent) error = %v", err)
	}
	updated := repositories.AIAgentRepository.Get(db, agent.ID)
	if updated == nil || updated.LLMModelName != "gpt-5.4" {
		t.Fatalf("tenant default agent model draft was not saved: %+v", updated)
	}
	if updated.ActiveReleaseID != originalActiveReleaseID {
		t.Fatalf("managed draft update must not switch production release: before=%d after=%d", originalActiveReleaseID, updated.ActiveReleaseID)
	}
	candidates := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
		Desc("release_no"))
	if len(candidates) != 1 || candidates[0].ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("managed model draft candidate = %+v", candidates)
	}
}

func TestTenantDefaultManagedUpdateRequiresExplicitReviewAndDeploy(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	agent, err := TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(initial) error = %v", err)
	}
	if agent == nil || agent.ActiveReleaseID <= 0 {
		t.Fatalf("initial tenant default release missing: %+v", agent)
	}
	initialReleaseID := agent.ActiveReleaseID
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"welcome_message": "outdated managed draft",
	}); err != nil {
		t.Fatalf("seed outdated managed draft: %v", err)
	}

	agent, err = TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(update) error = %v", err)
	}
	if agent.ActiveReleaseID != initialReleaseID {
		t.Fatalf("managed update deployed without review: before=%d after=%d", initialReleaseID, agent.ActiveReleaseID)
	}
	candidates := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
		Desc("release_no"))
	if len(candidates) != 1 || candidates[0].ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("managed update candidate = %+v", candidates)
	}
	candidate := candidates[0]
	if err := AIAgentReleaseService.SubmitReview(candidate.ID, fixture.Operator); err != nil {
		t.Fatalf("SubmitReview(default candidate) error = %v", err)
	}
	if err := AIAgentReleaseService.Review(candidate.ID, true, "tenant administrator approved", fixture.Operator); err != nil {
		t.Fatalf("Review(default candidate) error = %v", err)
	}
	if err := AIAgentReleaseService.Deploy(candidate.ID, fixture.Operator); err != nil {
		t.Fatalf("Deploy(default candidate) error = %v", err)
	}
	deployed := repositories.AIAgentRepository.Get(db, agent.ID)
	if deployed == nil || deployed.ActiveReleaseID != candidate.ID {
		t.Fatalf("explicitly approved candidate was not deployed: %+v", deployed)
	}
}

func TestTenantDefaultEnsureReplacesUnsafeActiveWorkflowImmediately(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	agent, err := TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(initial) error = %v", err)
	}
	if agent == nil || agent.ActiveReleaseID <= 0 {
		t.Fatalf("initial tenant default release missing: %+v", agent)
	}
	unsafeWorkflow, unsafeVersion, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	if err != nil {
		t.Fatalf("EnsurePlatformDefaultWorkflowDB() error = %v", err)
	}
	originalReleaseID := agent.ActiveReleaseID
	if err := repositories.AIAgentReleaseRepository.Updates(db, originalReleaseID, map[string]any{
		"workflow_id":              unsafeWorkflow.ID,
		"workflow_version_id":      unsafeVersion.ID,
		"workflow_definition_hash": unsafeVersion.DefinitionHash,
		"deployment_status":        models.AIAgentReleaseDeploymentActive,
		"review_status":            enums.AIAgentReviewStatusApproved,
		"update_user_id":           fixture.Operator.UserID,
		"update_user_name":         fixture.Operator.Username,
	}); err != nil {
		t.Fatalf("seed unsafe active tenant default release: %v", err)
	}

	agent, err = TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(repair unsafe active) error = %v", err)
	}
	if agent.ActiveReleaseID == originalReleaseID {
		t.Fatalf("unsafe tenant default active release was not replaced: %+v", agent)
	}
	repairedRelease := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if repairedRelease == nil || repairedRelease.ReviewStatus != enums.AIAgentReviewStatusApproved || repairedRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("repaired tenant default release is not active: %+v", repairedRelease)
	}
	repairedWorkflow := repositories.AIWorkflowRepository.Get(db, repairedRelease.WorkflowID)
	if repairedWorkflow == nil || repairedWorkflow.Code != PlatformDeviceAIOnlyWorkflowCode {
		t.Fatalf("tenant default release was not repaired to AI-only workflow: release=%+v workflow=%+v", repairedRelease, repairedWorkflow)
	}
	originalRelease := repositories.AIAgentReleaseRepository.Get(db, originalReleaseID)
	if originalRelease == nil || originalRelease.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("unsafe release was not retired: %+v", originalRelease)
	}
	if AIWorkflowService.AgentAllowsHumanHandoff(agent) || AIWorkflowService.AgentAllowsTicketCreation(agent) {
		t.Fatal("repaired tenant default production release must not expose handoff or ticket creation")
	}
}

func TestTenantDefaultEnsureCreatesReviewCandidateForStalePlatformAIOnlyRelease(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	agent, err := TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(initial) error = %v", err)
	}
	if agent == nil || agent.ActiveReleaseID <= 0 {
		t.Fatalf("initial tenant default release missing: %+v", agent)
	}
	currentWorkflow := repositories.AIWorkflowRepository.Get(db, agent.WorkflowID)
	currentVersion := repositories.AIWorkflowVersionRepository.Get(db, agent.WorkflowVersionID)
	if currentWorkflow == nil || currentVersion == nil || currentWorkflow.Code != PlatformDeviceAIOnlyWorkflowCode {
		t.Fatalf("tenant default workflow not initialized to AI-only: workflow=%+v version=%+v", currentWorkflow, currentVersion)
	}

	var obsoleteDefinition map[string]any
	if err := json.Unmarshal([]byte(currentVersion.Definition), &obsoleteDefinition); err != nil {
		t.Fatalf("unmarshal current workflow definition: %v", err)
	}
	nodes, ok := obsoleteDefinition["nodes"].([]any)
	if !ok {
		t.Fatalf("workflow definition nodes malformed: %#v", obsoleteDefinition["nodes"])
	}
	for _, rawNode := range nodes {
		node, ok := rawNode.(map[string]any)
		if !ok || node["id"] != "quick_reply_1" {
			continue
		}
		config, ok := node["config"].(map[string]any)
		if !ok {
			t.Fatalf("quick_reply_1 config malformed: %#v", node["config"])
		}
		delete(config, "allowEmptyKnowledge")
	}
	obsoleteDefinitionJSON, err := json.Marshal(obsoleteDefinition)
	if err != nil {
		t.Fatalf("marshal obsolete workflow definition: %v", err)
	}
	now := time.Now()
	obsoleteVersion := &models.AIWorkflowVersion{
		WorkflowID:          currentWorkflow.ID,
		Version:             repositories.AIWorkflowVersionRepository.MaxVersionByWorkflowID(db, currentWorkflow.ID) + 1,
		Status:              enums.StatusOk,
		Definition:          string(obsoleteDefinitionJSON),
		DefinitionHash:      hashDefinition(string(obsoleteDefinitionJSON)),
		ModelPolicySnapshot: currentVersion.ModelPolicySnapshot,
		ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
		SchemaVersion:       currentVersion.SchemaVersion,
		ChangeSummary:       "legacy AI-only version before platform safety upgrade",
		PublishedAt:         &now,
		PublishedByName:     "system",
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserName: "system",
			UpdatedAt:      now,
			UpdateUserName: "system",
		},
	}
	if err := repositories.AIWorkflowVersionRepository.Create(db, obsoleteVersion); err != nil {
		t.Fatalf("create obsolete AI-only version: %v", err)
	}
	originalReleaseID := agent.ActiveReleaseID
	if err := repositories.AIAgentReleaseRepository.Updates(db, originalReleaseID, map[string]any{
		"workflow_version_id":      obsoleteVersion.ID,
		"workflow_definition_hash": obsoleteVersion.DefinitionHash,
		"deployment_status":        models.AIAgentReleaseDeploymentActive,
		"review_status":            enums.AIAgentReviewStatusApproved,
		"update_user_id":           fixture.Operator.UserID,
		"update_user_name":         fixture.Operator.Username,
	}); err != nil {
		t.Fatalf("seed stale active tenant default release: %v", err)
	}

	agent, err = TenantDefaultAIAgentService.EnsureDB(db, fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureDB(sync stale platform AI-only release) error = %v", err)
	}
	if agent.ActiveReleaseID != originalReleaseID {
		t.Fatalf("stale AI-only release was deployed without review: before=%d after=%d", originalReleaseID, agent.ActiveReleaseID)
	}
	candidates := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentInactive).
		Desc("release_no"))
	if len(candidates) != 1 ||
		candidates[0].WorkflowID != currentWorkflow.ID ||
		candidates[0].WorkflowVersionID != currentVersion.ID ||
		candidates[0].WorkflowDefinitionHash != currentVersion.DefinitionHash ||
		candidates[0].ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("stale AI-only release should create one unreviewed candidate for the current stable version: %+v", candidates)
	}
	originalRelease := repositories.AIAgentReleaseRepository.Get(db, originalReleaseID)
	if originalRelease == nil ||
		originalRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive ||
		originalRelease.ReviewStatus != enums.AIAgentReviewStatusApproved {
		t.Fatalf("stale production release should remain active until explicit review: %+v", originalRelease)
	}
}

func createApprovedAgentRelease(t *testing.T, agentID int64, operator *dto.AuthPrincipal) *models.AIAgentRelease {
	t.Helper()
	item, err := AIAgentReleaseService.CreateCandidate(agentID, operator)
	if err != nil {
		t.Fatalf("CreateCandidate() error = %v", err)
	}
	if err := AIAgentReleaseService.SubmitReview(item.ID, operator); err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}
	if err := AIAgentReleaseService.Review(item.ID, true, "approved in test", operator); err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	return repositories.AIAgentReleaseRepository.Get(sqls.DB(), item.ID)
}
