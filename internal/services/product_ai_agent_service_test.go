package services

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	workflowbuiltin "remotehelpdesk/internal/ai/workflow/builtin"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func TestDefaultProductAgentPromptFollowsLatestCustomerLanguage(t *testing.T) {
	prompt := defaultProductAgentSystemPrompt(&models.Product{Name: "Test Product", Code: "TEST-1"})
	for _, required := range []string{"客户原始最新消息", "客户使用英文时必须完整使用英文", "知识片段是中文"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("default product Agent prompt is missing %q: %s", required, prompt)
		}
	}
}

func TestDefaultProductAgentWelcomeMessageIsEnglish(t *testing.T) {
	got := defaultProductAgentWelcomeMessage(&models.Product{Name: "Northstar Cooling Pump"})
	want := "Hello, I'm the after-sales support assistant for Northstar Cooling Pump. Please describe the device issue, model, and troubleshooting steps you've already tried."
	if got != want {
		t.Fatalf("default product Agent welcome message = %q, want %q", got, want)
	}
}

func TestProductAgentsSharePinnedPlatformWorkflowVersion(t *testing.T) {
	_, fixture := setupProductCenterTestDB(t)
	createProductAgent := func(code string) *models.AIAgent {
		product, err := ProductService.CreateProduct(request.CreateProductRequest{
			TenantID: fixture.Tenant.ID, Code: code, Name: "Product " + code, DefaultLocale: "en-US",
		}, fixture.Operator)
		if err != nil {
			t.Fatalf("CreateProduct(%s) error = %v", code, err)
		}
		agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
		if err != nil {
			t.Fatalf("EnsureProductCustomerAgent(%s) error = %v", code, err)
		}
		return agent
	}

	first := createProductAgent("A")
	second := createProductAgent("B")
	if first.WorkflowID <= 0 || first.WorkflowID != second.WorkflowID || first.WorkflowVersionID != second.WorkflowVersionID {
		t.Fatalf("new product agents should share one pinned platform version: first=%+v second=%+v", first, second)
	}
	workflow := AIWorkflowService.Get(first.WorkflowID)
	if workflow == nil || workflow.Scope != models.AIWorkflowScopePlatform || !workflow.Locked {
		t.Fatalf("shared workflow should be a locked platform template: %+v", workflow)
	}

	definition := AIWorkflowService.DefaultAgentWorkflowDefinition()
	for i := range definition.Nodes {
		if definition.Nodes[i].ID == "retrieve_1" {
			definition.Nodes[i].Config = json.RawMessage(`{"scope":"public_product","topK":10,"scoreThreshold":0.35}`)
		}
	}
	manifests, err := loadPlatformBuiltInWorkflowManifests()
	if err != nil {
		t.Fatalf("load platform manifests: %v", err)
	}
	manifestFound := false
	for index := range manifests {
		if manifests[index].Code == workflow.Code {
			manifests[index].Definition = definition
			manifestFound = true
		}
	}
	if !manifestFound {
		t.Fatalf("platform manifest %s is missing", workflow.Code)
	}
	originalLoader := loadPlatformBuiltInWorkflowManifests
	loadPlatformBuiltInWorkflowManifests = func() ([]workflowbuiltin.Manifest, error) {
		return manifests, nil
	}
	t.Cleanup(func() { loadPlatformBuiltInWorkflowManifests = originalLoader })
	_, version2, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("ensurePlatformBuiltInWorkflowDB(v2) error = %v", err)
	}
	if first.WorkflowVersionID == version2.ID || second.WorkflowVersionID == version2.ID {
		t.Fatalf("materializing a new platform manifest version must not mutate existing agents")
	}
	third := createProductAgent("C")
	if third.WorkflowVersionID != version2.ID {
		t.Fatalf("new product agent should bind latest stable version %d, got %d", version2.ID, third.WorkflowVersionID)
	}
}

func TestProductAIAgentServiceEnsuresDeployedProductAgent(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	for _, skill := range []models.SkillDefinition{
		{TenantID: 0, Name: "设备故障诊断", Remark: defaultProductAgentSkillRemarks[0], Status: enums.StatusOk},
		{TenantID: 0, Name: "安全停机与人工升级", Remark: defaultProductAgentSkillRemarks[1], Status: enums.StatusOk},
		{TenantID: 0, Name: "操作与维修指导", Remark: defaultProductAgentSkillRemarks[2], Status: enums.StatusOk},
	} {
		if err := db.Create(&skill).Error; err != nil {
			t.Fatalf("create default skill: %v", err)
		}
	}

	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "HP-AI",
		Name:          "Hydraulic Pump AI",
		Category:      "hydraulic",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent() error = %v", err)
	}
	if agent.TenantID != fixture.Tenant.ID || agent.ProductID != product.ID {
		t.Fatalf("agent product scope mismatch: %+v", agent)
	}
	if agent.Source != "product_auto" || agent.Status != enums.StatusOk || agent.ReviewStatus != enums.AIAgentReviewStatusApproved || agent.ActiveReleaseID <= 0 {
		t.Fatalf("product agent should be deployed by default: %+v", agent)
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if release == nil || release.ReviewStatus != enums.AIAgentReviewStatusApproved || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("product agent should have an approved active release: %+v", release)
	}
	workflow := AIWorkflowService.GetByAgentID(agent.ID)
	if workflow == nil {
		t.Fatalf("expected default workflow for product agent")
	}
	if workflow.Scope != models.AIWorkflowScopePlatform || !workflow.Locked || agent.WorkflowVersionID <= 0 {
		t.Fatalf("product agent should bind the shared platform stable workflow: agent=%+v workflow=%+v", agent, workflow)
	}

	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
	if profile == nil || profile.DefaultKnowledgeBaseID <= 0 || profile.DefaultAIAgentID != agent.ID {
		t.Fatalf("product profile should bind kb and agent: %+v", profile)
	}
	if !slices.Contains(utils.SplitInt64s(agent.KnowledgeIDs), profile.DefaultKnowledgeBaseID) {
		t.Fatalf("agent knowledge ids %q should contain product default kb %d", agent.KnowledgeIDs, profile.DefaultKnowledgeBaseID)
	}
	if skillIDs := utils.SplitInt64s(agent.SkillIDs); len(skillIDs) != 3 {
		t.Fatalf("product agent should bind the three workflow-safe skills, got %q", agent.SkillIDs)
	}
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"active_release_id": 0,
		"status":            enums.StatusDisabled,
		"review_status":     enums.AIAgentReviewStatusUnreviewed,
	}); err != nil {
		t.Fatalf("reset product agent to legacy state: %v", err)
	}
	repaired, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent(repair legacy product agent) error = %v", err)
	}
	if repaired.ActiveReleaseID != release.ID || repaired.Status != enums.StatusOk || repaired.ReviewStatus != enums.AIAgentReviewStatusApproved {
		t.Fatalf("legacy product agent should be repaired using the initial release: %+v", repaired)
	}
	result, err := ProductAIAgentService.ProvisionTenantProductAgents(fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("ProvisionTenantProductAgents() error = %v", err)
	}
	if result.Total != 1 || result.Created != 0 || result.Existing != 1 || result.Failed != 0 {
		t.Fatalf("unexpected provision result: %+v", result)
	}
}

func TestAIAgentRejectsDisabledTeamPool(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	team := &models.AgentTeam{
		TenantID:       fixture.Tenant.ID,
		Name:           "Disabled escalation pool",
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         enums.StatusDisabled,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create disabled team: %v", err)
	}
	kb := &models.KnowledgeBase{
		TenantID:      fixture.Tenant.ID,
		Name:          "Dispatch Guardrail KB",
		KnowledgeType: string(enums.KnowledgeBaseTypeFAQ),
		Status:        enums.StatusOk,
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	if _, err := AIAgentService.CreateAIAgent(request.CreateAIAgentRequest{
		Name:                "Disabled Team Agent",
		ServiceMode:         enums.IMConversationServiceModeAIFirst,
		TeamIDs:             []int64{team.ID},
		HandoffMode:         enums.AIAgentHandoffModeDefaultTeamPool,
		ReplyTimeoutSeconds: 180,
		KnowledgeIDs:        []int64{kb.ID},
	}, fixture.Operator); err == nil {
		t.Fatalf("CreateAIAgent() should reject disabled team pools")
	}
	if count := repositories.AIAgentRepository.Count(db, sqls.NewCnd().
		Eq("tenant_id", fixture.Tenant.ID).
		Eq("name", "Disabled Team Agent")); count != 0 {
		t.Fatalf("disabled team pool should not create agent, got count=%d", count)
	}
}

func TestProductStatusUpdateDisablesRepairTeamAndProvisionSkipsProduct(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	if err := db.AutoMigrate(
		&models.TicketProgress{},
		&models.TicketDispatchAttempt{},
		&models.Conversation{},
		&models.Notification{},
	); err != nil {
		t.Fatalf("AutoMigrate dispatch recovery tables: %v", err)
	}
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "DISABLED-PRODUCT",
		Name:          "Disabled Product",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	team := ProductSupportOrganizationService.FindProductRepairTeam(db, fixture.Tenant.ID, product.ID)
	if team == nil || team.Status != enums.StatusOk {
		t.Fatalf("product should start with active repair team: %+v", team)
	}
	assignedAt := time.Now().Add(-2 * time.Minute)
	deadline := assignedAt.Add(30 * time.Minute)
	assignedTicket := &models.Ticket{
		TicketNo:          "PRODUCT-DISABLE-ASSIGNED",
		TenantID:          fixture.Tenant.ID,
		ProductID:         product.ID,
		CurrentTeamID:     team.ID,
		CurrentAssigneeID: 101,
		Status:            enums.TicketStatusPendingAssigneeAccept,
		AssignedAt:        &assignedAt,
		AcceptDeadlineAt:  &deadline,
		AuditFields:       models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(assignedTicket).Error; err != nil {
		t.Fatalf("create assigned ticket: %v", err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: fixture.Tenant.ID, TicketID: assignedTicket.ID, TeamID: team.ID, AssigneeID: 101,
		AttemptNo: 1, Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatalf("create dispatch attempt: %v", err)
	}
	pooledTicket := &models.Ticket{
		TicketNo:      "PRODUCT-DISABLE-POOL",
		TenantID:      fixture.Tenant.ID,
		ProductID:     product.ID,
		CurrentTeamID: team.ID,
		Status:        enums.TicketStatusPendingDispatch,
		AuditFields:   models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(pooledTicket).Error; err != nil {
		t.Fatalf("create pooled ticket: %v", err)
	}

	if err := ProductService.UpdateStatus(product.ID, int(enums.StatusDisabled), fixture.Operator); err != nil {
		t.Fatalf("UpdateStatus(disabled) error = %v", err)
	}
	refreshedTeam := repositories.AgentTeamRepository.Get(db, team.ID)
	if refreshedTeam == nil || refreshedTeam.Status != enums.StatusDisabled {
		t.Fatalf("product status disable should disable repair team: %+v", refreshedTeam)
	}
	var recoveredAssigned models.Ticket
	if err := db.First(&recoveredAssigned, assignedTicket.ID).Error; err != nil {
		t.Fatalf("reload assigned ticket: %v", err)
	}
	if recoveredAssigned.CurrentAssigneeID != 0 || recoveredAssigned.CurrentTeamID != 0 ||
		recoveredAssigned.Status != enums.TicketStatusPendingDispatch || recoveredAssigned.AcceptDeadlineAt != nil {
		t.Fatalf("product disable should recover pending assigned ticket: %+v", recoveredAssigned)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.First(&attempt, "ticket_id = ?", assignedTicket.ID).Error; err != nil {
		t.Fatalf("reload dispatch attempt: %v", err)
	}
	if attempt.Outcome != ticketDispatchOutcomeSuperseded || attempt.EndedAt == nil {
		t.Fatalf("product disable should settle pending dispatch attempt: %+v", attempt)
	}
	var releasedPool models.Ticket
	if err := db.First(&releasedPool, pooledTicket.ID).Error; err != nil {
		t.Fatalf("reload pooled ticket: %v", err)
	}
	if releasedPool.CurrentTeamID != 0 || releasedPool.LastDispatchFailureReason != "team_disabled_pool_released" {
		t.Fatalf("product disable should release team pool ticket: %+v", releasedPool)
	}
	if _, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator); err == nil {
		t.Fatalf("EnsureProductCustomerAgent() should reject disabled products")
	}
	result, err := ProductAIAgentService.ProvisionTenantProductAgents(fixture.Tenant.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("ProvisionTenantProductAgents() error = %v", err)
	}
	if result.Total != 0 || result.Created != 0 || result.Existing != 0 || result.Failed != 0 {
		t.Fatalf("disabled products should be skipped by provisioning: %+v", result)
	}
}

func TestProductAgentTeamBindingRejectsDisabledOrWrongScopeTeam(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "BIND-SCOPE", "Binding Scope")
	agent := &models.AIAgent{
		TenantID:  fixture.Tenant.ID,
		ProductID: product.ID,
		Source:    "product_auto",
		Name:      "Binding Scope Agent",
		Status:    enums.StatusOk,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	disabledTeam := &models.AgentTeam{
		TenantID:       fixture.Tenant.ID,
		ProductID:      product.ID,
		TeamType:       AgentTeamTypeProductRepair,
		Name:           "Disabled Binding Team",
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         enums.StatusDisabled,
	}
	if err := db.Create(disabledTeam).Error; err != nil {
		t.Fatalf("create disabled team: %v", err)
	}
	if err := ProductSupportOrganizationService.EnsureProductAgentTeamBindingDB(db, agent, disabledTeam.ID); err == nil {
		t.Fatalf("EnsureProductAgentTeamBindingDB() should reject disabled team")
	}

	otherProduct := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "OTHER-SCOPE", "Other Scope")
	wrongScopeTeam := &models.AgentTeam{
		TenantID:       fixture.Tenant.ID,
		ProductID:      otherProduct.ID,
		TeamType:       AgentTeamTypeProductRepair,
		Name:           "Wrong Scope Team",
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         enums.StatusOk,
	}
	if err := db.Create(wrongScopeTeam).Error; err != nil {
		t.Fatalf("create wrong scope team: %v", err)
	}
	if err := ProductSupportOrganizationService.EnsureProductAgentTeamBindingDB(db, agent, wrongScopeTeam.ID); err == nil {
		t.Fatalf("EnsureProductAgentTeamBindingDB() should reject product scope mismatch")
	}
	refreshed := repositories.AIAgentRepository.Get(db, agent.ID)
	if refreshed == nil || refreshed.TeamIDs != "" {
		t.Fatalf("invalid team binding should not mutate team_ids: %+v", refreshed)
	}
}

func TestProductAIAgentEnsureReplacesUnsafePlatformWorkflowRelease(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "SAFE-UP",
		Name:          "Safety Upgrade Controller",
		Category:      "power",
		DefaultLocale: "zh-CN",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	agent, err := ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent(initial) error = %v", err)
	}
	workflow := AIWorkflowService.Get(agent.WorkflowID)
	currentVersion := AIWorkflowService.GetVersion(agent.WorkflowVersionID)
	if workflow == nil || currentVersion == nil {
		t.Fatalf("product agent workflow missing: agent=%+v", agent)
	}
	unsafeDefinition := unsafeAfterSalesWorkflowDefinitionForTest(t)
	unsafeRaw, err := json.Marshal(unsafeDefinition)
	if err != nil {
		t.Fatalf("marshal unsafe workflow: %v", err)
	}
	now := time.Now()
	unsafeVersion := &models.AIWorkflowVersion{
		WorkflowID:          workflow.ID,
		Version:             currentVersion.Version + 100,
		Status:              enums.StatusOk,
		Definition:          string(unsafeRaw),
		DefinitionHash:      hashString(string(unsafeRaw)),
		ModelPolicySnapshot: "{}",
		ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
		SchemaVersion:       1,
		ChangeSummary:       "historical unsafe handoff workflow",
		SourceVersionID:     currentVersion.ID,
		PublishedAt:         &now,
		PublishedByID:       fixture.Operator.UserID,
		PublishedByName:     fixture.Operator.Username,
		AuditFields:         utils.BuildAuditFields(fixture.Operator),
	}
	if err := db.Create(unsafeVersion).Error; err != nil {
		t.Fatalf("create unsafe workflow version: %v", err)
	}
	agent.WorkflowID = workflow.ID
	agent.WorkflowVersionID = unsafeVersion.ID
	agent.Status = enums.StatusOk
	agent.ReviewStatus = enums.AIAgentReviewStatusApproved
	if err := repositories.AIAgentRepository.Update(db, agent); err != nil {
		t.Fatalf("update agent to unsafe version: %v", err)
	}
	unsafeSnapshot, unsafeAgentHash, err := buildAgentReleaseConfigSnapshotForDefinition(agent, &unsafeDefinition)
	if err != nil {
		t.Fatalf("build unsafe release snapshot: %v", err)
	}
	knowledgeSnapshot, knowledgeHash, err := AIAgentReleaseService.buildKnowledgeScopeSnapshot(db, agent)
	if err != nil {
		t.Fatalf("build knowledge snapshot: %v", err)
	}
	unsafeRelease := &models.AIAgentRelease{
		TenantID:               agent.TenantID,
		ProductID:              agent.ProductID,
		AgentID:                agent.ID,
		ReleaseNo:              repositories.AIAgentReleaseRepository.MaxReleaseNo(db, agent.ID) + 1,
		WorkflowID:             workflow.ID,
		WorkflowVersionID:      unsafeVersion.ID,
		WorkflowDefinitionHash: unsafeVersion.DefinitionHash,
		AgentConfigSnapshot:    unsafeSnapshot,
		AgentConfigHash:        unsafeAgentHash,
		KnowledgeScopeSnapshot: knowledgeSnapshot,
		KnowledgeScopeHash:     knowledgeHash,
		ReviewStatus:           enums.AIAgentReviewStatusApproved,
		ReviewComment:          "historical unsafe platform release",
		ReviewedAt:             &now,
		ReviewedByID:           fixture.Operator.UserID,
		ReviewedByName:         fixture.Operator.Username,
		DeploymentStatus:       models.AIAgentReleaseDeploymentActive,
		DeployedAt:             &now,
		DeployedByID:           fixture.Operator.UserID,
		DeployedByName:         fixture.Operator.Username,
		Status:                 enums.StatusOk,
		AuditFields:            utils.BuildAuditFields(fixture.Operator),
	}
	if agent.ActiveReleaseID > 0 {
		if err := repositories.AIAgentReleaseRepository.Updates(db, agent.ActiveReleaseID, map[string]any{
			"deployment_status": models.AIAgentReleaseDeploymentRetired,
		}); err != nil {
			t.Fatalf("retire current release before seeding historical unsafe release: %v", err)
		}
	}
	if err := db.Create(unsafeRelease).Error; err != nil {
		t.Fatalf("create unsafe active release: %v", err)
	}
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"active_release_id": unsafeRelease.ID,
	}); err != nil {
		t.Fatalf("bind unsafe active release: %v", err)
	}

	agent, err = ProductAIAgentService.EnsureProductCustomerAgent(fixture.Tenant.ID, product.ID, fixture.Operator)
	if err != nil {
		t.Fatalf("EnsureProductCustomerAgent(repair) error = %v", err)
	}
	if agent.ActiveReleaseID == unsafeRelease.ID {
		t.Fatalf("unsafe release remained active: %+v", agent)
	}
	repaired := AIAgentReleaseService.Get(agent.ActiveReleaseID, fixture.Operator)
	if repaired == nil || repaired.WorkflowVersionID != currentVersion.ID || repaired.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		t.Fatalf("repaired release = %+v, want current safe version %d", repaired, currentVersion.ID)
	}
	old := repositories.AIAgentReleaseRepository.Get(db, unsafeRelease.ID)
	if old == nil || old.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("unsafe release was not retired: %+v", old)
	}
	if defaultAfterSalesWorkflowAutoHandoffOnKnowledgeFailure(currentVersion) {
		t.Fatal("current platform default workflow still auto-handoffs on knowledge failure")
	}
}

func unsafeAfterSalesWorkflowDefinitionForTest(t *testing.T) dsl.Definition {
	t.Helper()
	definition := AIWorkflowService.DefaultAgentWorkflowDefinition()
	for i := range definition.Nodes {
		if definition.Nodes[i].ID != "answerability_route_1" {
			continue
		}
		config := dsl.ConditionConfig{}
		if err := json.Unmarshal(definition.Nodes[i].Config, &config); err != nil {
			t.Fatalf("unmarshal answerability route: %v", err)
		}
		config.Branches = append(config.Branches, dsl.ConditionBranch{
			ID:           "handoff",
			Name:         "转人工处理",
			TargetNodeID: "handoff_1",
			Condition: &dsl.Condition{
				Left:     &dsl.VariableSelector{NodeID: "service_access_1", Field: "allowHumanHandoff"},
				Operator: "is_true",
			},
		})
		raw, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("marshal answerability route: %v", err)
		}
		definition.Nodes[i].Config = raw
	}
	definition.Edges = append(definition.Edges, dsl.Edge{
		ID:     "edge_answerability_handoff_unsafe",
		Source: "answerability_route_1",
		Target: "handoff_1",
	})
	return definition
}
