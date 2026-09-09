package services

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	workflowvalidator "remotehelpdesk/internal/ai/workflow/validator"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestAIAgentServiceCreatesDefaultWorkflow(t *testing.T) {
	setupAIAgentWorkflowTestDB(t)
	operator := aiAgentWorkflowTestOperator()
	aiConfigID := createAIAgentWorkflowTestConfig(t)
	knowledgeID := createAIAgentWorkflowTestKnowledgeBase(t)

	item, err := AIAgentService.CreateAIAgent(request.CreateAIAgentRequest{
		Name:         "workflow agent",
		AIConfigID:   aiConfigID,
		ServiceMode:  enums.IMConversationServiceModeAIOnly,
		HandoffMode:  enums.AIAgentHandoffModeWaitPool,
		FallbackMode: enums.AIAgentFallbackModeNoAnswer,
		KnowledgeIDs: []int64{knowledgeID},
	}, operator)
	if err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}

	workflow, err := AIWorkflowService.GetOrCreateAgentWorkflow(item.ID, operator)
	if err != nil {
		t.Fatalf("GetOrCreateAgentWorkflow() error = %v", err)
	}
	if workflow.AgentID != 0 || workflow.Scope != models.AIWorkflowScopePlatform || !workflow.Locked {
		t.Fatalf("expected shared locked platform workflow, got %+v", workflow)
	}
	if workflow.Name != "平台 AI 优先人工协同售后流程" {
		t.Fatalf("unexpected workflow name: %s", workflow.Name)
	}
	storedAgent := AIAgentService.Get(item.ID)
	if storedAgent == nil || storedAgent.WorkflowID != workflow.ID || storedAgent.WorkflowVersionID <= 0 {
		t.Fatalf("expected agent to pin the shared stable workflow: %+v", storedAgent)
	}
	var stored dsl.Definition
	if err := json.Unmarshal([]byte(workflow.DraftDefinition), &stored); err != nil {
		t.Fatalf("unmarshal draft definition: %v", err)
	}
	if stored.EntryNodeID == "" {
		t.Fatalf("expected default draft definition")
	}
	validation := workflowvalidator.ValidateDefinition(stored, workflowregistry.DefaultRegistry())
	if !validation.Valid {
		t.Fatalf("expected default workflow to be valid, got %#v", validation.Errors)
	}
	if nodeTypeByID(stored, "understanding_1") != workflowregistry.NodeTypeConversationUnderstanding {
		t.Fatalf("expected default workflow to include conversation understanding, got nodes: %#v", stored.Nodes)
	}
	if nodeTypeByID(stored, "policy_1") != workflowregistry.NodeTypeReplyPolicy {
		t.Fatalf("expected default workflow to include reply policy, got nodes: %#v", stored.Nodes)
	}
	for _, edge := range [][2]string{
		{"start_1", "entry_context_1"},
		{"entry_context_1", "service_access_1"},
		{"service_access_1", "service_access_route_1"},
		{"service_access_route_1", "understanding_1"},
		{"understanding_1", "policy_1"},
	} {
		if !workflowEdgeExists(stored, edge[0], edge[1]) {
			t.Fatalf("expected default workflow entry chain to include %s -> %s, got edges: %#v", edge[0], edge[1], stored.Edges)
		}
	}
	for _, nodeType := range []string{
		workflowregistry.NodeTypeConversationUnderstanding,
		workflowregistry.NodeTypeReplyPolicy,
		workflowregistry.NodeTypeHandoffToHuman,
		workflowregistry.NodeTypePrepareTicketDraft,
		workflowregistry.NodeTypeHumanConfirm,
		workflowregistry.NodeTypeCreateTicket,
		workflowregistry.NodeTypeKnowledgeRetrieve,
		workflowregistry.NodeTypeAnswerabilityGate,
		workflowregistry.NodeTypeLLMReply,
		workflowregistry.NodeTypeSendReply,
	} {
		if !workflowHasNodeType(stored, nodeType) {
			t.Fatalf("expected default workflow to include %s node: %#v", nodeType, stored.Nodes)
		}
	}
	assertConditionBranchToNodeType(t, stored, "policy_route_1", workflowregistry.NodeTypeSendReply, "eq", "direct_reply")
	assertConditionBranchToNodeType(t, stored, "policy_route_1", workflowregistry.NodeTypeHandoffToHuman, "eq", "handoff_to_human")
	assertConditionBranchToNodeType(t, stored, "policy_route_1", workflowregistry.NodeTypePrepareTicketDraft, "eq", "prepare_ticket")
	assertConditionBranchToNodeID(t, stored, "answerability_route_1", "reply_1", "eq", "answerable")
	assertConditionRouteExcludesNodeID(t, stored, "answerability_route_1", "handoff_1")
	assertDefaultBranchToNodeID(t, stored, "answerability_route_1", "diagnostic_safe_reply_1")
	if !workflowEdgeExists(stored, "create_ticket_1", "ticket_result_reply_1") {
		t.Fatalf("expected create_ticket to flow into a customer-visible result reply")
	}
}

func TestAIAgentServicePersistsUnifiedLLMModelSelection(t *testing.T) {
	setupAIAgentWorkflowTestDB(t)
	operator := aiAgentWorkflowTestOperator()
	knowledgeID := createAIAgentWorkflowTestKnowledgeBase(t)

	item, err := AIAgentService.CreateAIAgent(request.CreateAIAgentRequest{
		Name:         "model selection agent",
		LLMModelName: "  gpt-5.6  ",
		ServiceMode:  enums.IMConversationServiceModeAIOnly,
		HandoffMode:  enums.AIAgentHandoffModeWaitPool,
		FallbackMode: enums.AIAgentFallbackModeNoAnswer,
		KnowledgeIDs: []int64{knowledgeID},
	}, operator)
	if err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}
	if item.LLMModelName != "gpt-5.6" {
		t.Fatalf("created LLM model = %q", item.LLMModelName)
	}
	stored := AIAgentService.Get(item.ID)
	if stored == nil || stored.LLMModelName != "gpt-5.6" {
		t.Fatalf("stored agent = %#v", stored)
	}

	aiConfigID := createAIAgentWorkflowTestConfig(t)
	err = AIAgentService.UpdateAIAgent(request.UpdateAIAgentRequest{
		ID: item.ID,
		CreateAIAgentRequest: request.CreateAIAgentRequest{
			Name:         item.Name,
			AIConfigID:   aiConfigID,
			LLMModelName: "gpt-5.5",
			ServiceMode:  enums.IMConversationServiceModeAIOnly,
			HandoffMode:  enums.AIAgentHandoffModeWaitPool,
			FallbackMode: enums.AIAgentFallbackModeNoAnswer,
			KnowledgeIDs: []int64{knowledgeID},
		},
	}, operator)
	if err != nil {
		t.Fatalf("UpdateAIAgent() error = %v", err)
	}
	stored = AIAgentService.Get(item.ID)
	if stored == nil || stored.LLMModelName != "" {
		t.Fatalf("explicit AI config must clear the unified model override: %#v", stored)
	}
}

func TestAIWorkflowServiceDefaultAgentWorkflowDefinitionIsValid(t *testing.T) {
	definition := AIWorkflowService.DefaultAgentWorkflowDefinition()
	if definition.EntryNodeID == "" {
		t.Fatalf("expected default workflow definition")
	}
	validation := workflowvalidator.ValidateDefinition(definition, workflowregistry.DefaultRegistry())
	if !validation.Valid {
		t.Fatalf("expected default workflow definition to be valid, got %#v", validation.Errors)
	}
	if nodeTypeByID(definition, "understanding_1") != workflowregistry.NodeTypeConversationUnderstanding {
		t.Fatalf("expected default workflow to include conversation understanding, got nodes: %#v", definition.Nodes)
	}
	if nodeTypeByID(definition, "policy_1") != workflowregistry.NodeTypeReplyPolicy {
		t.Fatalf("expected default workflow to include reply policy, got nodes: %#v", definition.Nodes)
	}
	if !workflowHasNodeType(definition, workflowregistry.NodeTypeHandoffToHuman) {
		t.Fatalf("expected default workflow to include human handoff node")
	}
	if !workflowHasNodeType(definition, workflowregistry.NodeTypeCreateTicket) {
		t.Fatalf("expected default workflow to include ticket creation node")
	}
	assertDefaultBranchToNodeID(t, definition, "answerability_route_1", "diagnostic_safe_reply_1")
	assertConditionBranchToNodeID(t, definition, "policy_route_1", "handoff_1", "eq", "handoff_to_human")
	for nodeID, targetID := range map[string]string{
		"entry_context_1":  "quick_fallback_reply_1",
		"service_access_1": "quick_fallback_reply_1",
		"quick_retrieve_1": "quick_fallback_reply_1",
		"quick_reply_1":    "quick_fallback_reply_1",
		"understanding_1":  "diagnosis_failure_route_1",
		"policy_1":         "diagnosis_failure_route_1",
		"retrieve_1":       "diagnosis_failure_route_1",
		"answerability_1":  "diagnosis_failure_route_1",
		"reply_1":          "diagnosis_failure_route_1",
		"handoff_1":        "service_failure_reply_1",
		"draft_ticket_1":   "service_failure_reply_1",
		"create_ticket_1":  "service_failure_reply_1",
	} {
		assertWorkflowFailureTarget(t, definition, nodeID, targetID)
	}
}

func TestAIWorkflowServiceDeviceAIOnlyDefinitionKeepsFullDiagnosisWithoutHumanActions(t *testing.T) {
	definition := AIWorkflowService.DeviceAIOnlyWorkflowDefinition()
	validation := workflowvalidator.ValidateDefinition(definition, workflowregistry.DefaultRegistry())
	if !validation.Valid {
		t.Fatalf("expected device AI-only workflow definition to be valid, got %#v", validation.Errors)
	}
	for _, nodeType := range []string{
		workflowregistry.NodeTypeEntryContext,
		workflowregistry.NodeTypeCondition,
		workflowregistry.NodeTypeKnowledgeRetrieve,
		workflowregistry.NodeTypeAnswerabilityGate,
		workflowregistry.NodeTypeLLMReply,
		workflowregistry.NodeTypeSendReply,
	} {
		if !workflowHasNodeType(definition, nodeType) {
			t.Fatalf("device AI-only workflow must contain %s", nodeType)
		}
	}
	for _, nodeType := range []string{
		workflowregistry.NodeTypeHandoffToHuman,
		workflowregistry.NodeTypePrepareTicketDraft,
		workflowregistry.NodeTypeCreateTicket,
		workflowregistry.NodeTypeCreateVideoMeeting,
	} {
		if workflowHasNodeType(definition, nodeType) {
			t.Fatalf("device AI-only workflow must not contain %s", nodeType)
		}
	}
	assertConditionBranchToNodeID(t, definition, "context_route_1", "device_retrieve_1", "is_true", nil)
	assertDefaultBranchToNodeID(t, definition, "context_route_1", "quick_retrieve_1")
	assertDefaultBranchToNodeID(t, definition, "answerability_route_1", "diagnostic_safe_reply_1")
	for nodeID, targetID := range map[string]string{
		"entry_context_1":   "quick_safe_reply_1",
		"quick_retrieve_1":  "quick_safe_reply_1",
		"quick_reply_1":     "quick_safe_reply_1",
		"device_retrieve_1": "diagnostic_safe_reply_1",
		"answerability_1":   "diagnostic_safe_reply_1",
		"device_reply_1":    "diagnostic_safe_reply_1",
	} {
		assertWorkflowFailureTarget(t, definition, nodeID, targetID)
	}
	assertWorkflowLayoutDoesNotOverlap(t, definition)
}

func TestAIWorkflowServiceMaterializesFourPlatformWorkflows(t *testing.T) {
	setupAIAgentWorkflowTestDB(t)
	defaultWorkflow, defaultVersion, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDefaultWorkflowDB() error = %v", err)
	}
	deviceWorkflow, deviceVersion, err := AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDeviceAIOnlyWorkflowDB() error = %v", err)
	}
	knowledgeWorkflow, knowledgeVersion, err := AIWorkflowService.EnsurePlatformKnowledgeSupportWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformKnowledgeSupportWorkflowDB() error = %v", err)
	}
	dispatchWorkflow, dispatchVersion, err := AIWorkflowService.EnsurePlatformDispatchOnlyWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDispatchOnlyWorkflowDB() error = %v", err)
	}
	workflowIDs := map[int64]struct{}{defaultWorkflow.ID: {}, deviceWorkflow.ID: {}, knowledgeWorkflow.ID: {}, dispatchWorkflow.ID: {}}
	if len(workflowIDs) != 4 {
		t.Fatalf("expected four distinct platform workflows, got default=%+v device=%+v knowledge=%+v dispatch=%+v", defaultWorkflow, deviceWorkflow, knowledgeWorkflow, dispatchWorkflow)
	}
	if defaultWorkflow.Code != PlatformDefaultAfterSalesWorkflowCode || deviceWorkflow.Code != PlatformDeviceAIOnlyWorkflowCode || knowledgeWorkflow.Code != PlatformKnowledgeSupportWorkflowCode || dispatchWorkflow.Code != PlatformDispatchOnlyWorkflowCode {
		t.Fatalf("unexpected platform workflow codes: default=%q device=%q knowledge=%q dispatch=%q", defaultWorkflow.Code, deviceWorkflow.Code, knowledgeWorkflow.Code, dispatchWorkflow.Code)
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&models.AIAgent{WorkflowID: defaultWorkflow.ID, WorkflowVersionID: defaultVersion.ID}) {
		t.Fatal("collaboration workflow must allow human handoff")
	}
	if !AIWorkflowService.AgentAllowsTicketCreation(&models.AIAgent{WorkflowID: defaultWorkflow.ID, WorkflowVersionID: defaultVersion.ID}) {
		t.Fatal("collaboration workflow must allow customer ticket creation")
	}
	if AIWorkflowService.AgentAllowsHumanHandoff(&models.AIAgent{WorkflowID: deviceWorkflow.ID, WorkflowVersionID: deviceVersion.ID}) {
		t.Fatal("AI diagnosis workflow must disable human handoff")
	}
	if AIWorkflowService.AgentAllowsTicketCreation(&models.AIAgent{WorkflowID: deviceWorkflow.ID, WorkflowVersionID: deviceVersion.ID}) {
		t.Fatal("AI diagnosis workflow must disable customer ticket creation")
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&models.AIAgent{ServiceMode: enums.IMConversationServiceModeAIFirst, WorkflowID: knowledgeWorkflow.ID, WorkflowVersionID: knowledgeVersion.ID}) {
		t.Fatal("knowledge support workflow must allow human handoff")
	}
	if AIWorkflowService.AgentAllowsTicketCreation(&models.AIAgent{WorkflowID: knowledgeWorkflow.ID, WorkflowVersionID: knowledgeVersion.ID}) {
		t.Fatal("knowledge support workflow must disable customer ticket creation")
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&models.AIAgent{WorkflowID: dispatchWorkflow.ID, WorkflowVersionID: dispatchVersion.ID}) {
		t.Fatal("dispatch workflow must allow human handoff")
	}
	if !AIWorkflowService.AgentAllowsTicketCreation(&models.AIAgent{WorkflowID: dispatchWorkflow.ID, WorkflowVersionID: dispatchVersion.ID}) {
		t.Fatal("dispatch workflow must allow customer ticket creation")
	}
}

func TestAIWorkflowServiceDispatchOnlyWorkflowDefinitionIsRuleDriven(t *testing.T) {
	definition := AIWorkflowService.DispatchOnlyWorkflowDefinition()
	validation := workflowvalidator.ValidateDefinition(definition, workflowregistry.DefaultRegistry())
	if !validation.Valid {
		t.Fatalf("expected dispatch-only workflow definition to be valid, got %#v", validation.Errors)
	}
	for _, nodeType := range []string{
		workflowregistry.NodeTypeAnalyzeConversation,
		workflowregistry.NodeTypePrepareTicketDraft,
		workflowregistry.NodeTypeHumanConfirm,
		workflowregistry.NodeTypeCreateTicket,
		workflowregistry.NodeTypeHandoffToHuman,
		workflowregistry.NodeTypeSendReply,
	} {
		if !workflowHasNodeType(definition, nodeType) {
			t.Fatalf("dispatch-only workflow must contain %s", nodeType)
		}
	}
	for _, nodeType := range []string{
		workflowregistry.NodeTypeKnowledgeRetrieve,
		workflowregistry.NodeTypeAnswerabilityGate,
	} {
		if workflowHasNodeType(definition, nodeType) {
			t.Fatalf("dispatch-only workflow must not contain %s", nodeType)
		}
	}
	for _, node := range definition.Nodes {
		if node.Type != workflowregistry.NodeTypeLLMReply {
			continue
		}
		if !strings.Contains(string(node.Config), "staticReply") {
			t.Fatalf("dispatch-only workflow must use static replies, got %s: %s", node.ID, string(node.Config))
		}
		if strings.Contains(string(node.Config), "\"prompt\"") {
			t.Fatalf("dispatch-only workflow reply node %s should not use a model prompt", node.ID)
		}
	}
}

func TestAIWorkflowServiceHumanHandoffUsesActiveReleaseVersion(t *testing.T) {
	setupAIAgentWorkflowTestDB(t)
	if err := sqls.DB().AutoMigrate(&models.AIAgentRelease{}); err != nil {
		t.Fatalf("auto migrate agent releases: %v", err)
	}
	defaultWorkflow, defaultVersion, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDefaultWorkflowDB() error = %v", err)
	}
	aiOnlyWorkflow, aiOnlyVersion, err := AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDeviceAIOnlyWorkflowDB() error = %v", err)
	}

	createAgentWithRelease := func(name string, productID int64, draftWorkflow *models.AIWorkflow, draftVersion *models.AIWorkflowVersion, releaseWorkflow *models.AIWorkflow, releaseVersion *models.AIWorkflowVersion) *models.AIAgent {
		t.Helper()
		agent := &models.AIAgent{
			TenantID:          1,
			ProductID:         productID,
			Name:              name,
			WorkflowID:        draftWorkflow.ID,
			WorkflowVersionID: draftVersion.ID,
			Status:            enums.StatusOk,
		}
		if err := sqls.DB().Create(agent).Error; err != nil {
			t.Fatalf("create agent: %v", err)
		}
		release := &models.AIAgentRelease{
			TenantID:          agent.TenantID,
			ProductID:         agent.ProductID,
			AgentID:           agent.ID,
			ReleaseNo:         1,
			WorkflowID:        releaseWorkflow.ID,
			WorkflowVersionID: releaseVersion.ID,
			ReviewStatus:      enums.AIAgentReviewStatusApproved,
			DeploymentStatus:  models.AIAgentReleaseDeploymentActive,
			Status:            enums.StatusOk,
		}
		if err := sqls.DB().Create(release).Error; err != nil {
			t.Fatalf("create active release: %v", err)
		}
		if err := sqls.DB().Model(agent).Update("active_release_id", release.ID).Error; err != nil {
			t.Fatalf("bind active release: %v", err)
		}
		agent.ActiveReleaseID = release.ID
		return agent
	}

	aiOnlyProduction := createAgentWithRelease(
		"AI-only production",
		1,
		defaultWorkflow,
		defaultVersion,
		aiOnlyWorkflow,
		aiOnlyVersion,
	)
	if AIWorkflowService.AgentAllowsHumanHandoff(aiOnlyProduction) {
		t.Fatal("AI-only production release must disable handoff even when the draft uses the default workflow")
	}
	if AIWorkflowService.AgentAllowsTicketCreation(aiOnlyProduction) {
		t.Fatal("AI-only production release must disable ticket creation even when the draft uses the default workflow")
	}

	defaultProduction := createAgentWithRelease(
		"default production",
		2,
		aiOnlyWorkflow,
		aiOnlyVersion,
		defaultWorkflow,
		defaultVersion,
	)
	if !AIWorkflowService.AgentAllowsHumanHandoff(defaultProduction) {
		t.Fatal("default production release must allow handoff even when the draft uses the AI-only workflow")
	}
	if !AIWorkflowService.AgentAllowsTicketCreation(defaultProduction) {
		t.Fatal("default production release must allow ticket creation even when the draft uses the AI-only workflow")
	}

	mismatchedProduction := createAgentWithRelease(
		"mismatched production",
		3,
		aiOnlyWorkflow,
		aiOnlyVersion,
		aiOnlyWorkflow,
		defaultVersion,
	)
	if AIWorkflowService.AgentAllowsHumanHandoff(mismatchedProduction) {
		t.Fatal("a workflow version from a different workflow must fail closed")
	}
}

func TestAIWorkflowServiceDefaultAgentWorkflowLayoutDoesNotOverlap(t *testing.T) {
	definition := AIWorkflowService.DefaultAgentWorkflowDefinition()
	assertWorkflowLayoutDoesNotOverlap(t, definition)
}

func TestAIWorkflowServicePublishAgentWorkflowBindsAgentVersion(t *testing.T) {
	setupAIAgentWorkflowTestDB(t)
	operator := aiAgentWorkflowTestOperator()
	aiConfigID := createAIAgentWorkflowTestConfig(t)
	knowledgeID := createAIAgentWorkflowTestKnowledgeBase(t)

	agent, err := AIAgentService.CreateAIAgent(request.CreateAIAgentRequest{
		Name:         "workflow agent without version",
		AIConfigID:   aiConfigID,
		ServiceMode:  enums.IMConversationServiceModeAIOnly,
		HandoffMode:  enums.AIAgentHandoffModeWaitPool,
		FallbackMode: enums.AIAgentFallbackModeNoAnswer,
		KnowledgeIDs: []int64{knowledgeID},
	}, operator)
	if err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}
	workflow, err := AIWorkflowService.SaveAgentWorkflow(request.SaveAIWorkflowRequest{
		AgentID:     agent.ID,
		Name:        "After sales flow",
		Description: "Support workflow",
		Definition:  validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("SaveAgentWorkflow() error = %v", err)
	}

	version, err := AIWorkflowService.PublishAgentWorkflow(request.PublishAIWorkflowRequest{
		AgentID:    agent.ID,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("PublishAgentWorkflow() error = %v", err)
	}
	if version.WorkflowID != workflow.ID {
		t.Fatalf("expected version workflow id %d, got %d", workflow.ID, version.WorkflowID)
	}
	storedAgent := AIAgentService.Get(agent.ID)
	if storedAgent == nil {
		t.Fatalf("expected stored agent")
	}
	if storedAgent.WorkflowVersionID != version.ID {
		t.Fatalf("expected agent workflow version %d, got %d", version.ID, storedAgent.WorkflowVersionID)
	}
}

func setupAIAgentWorkflowTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ai-agent-workflow-test.db")
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.AIAgent{}, &models.AIConfig{}, &models.KnowledgeBase{}, &models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
}

func createAIAgentWorkflowTestConfig(t *testing.T) int64 {
	t.Helper()
	item := &models.AIConfig{
		Name:      "workflow-test-config",
		Provider:  enums.AIProviderOpenAI,
		ModelType: enums.AIModelTypeLLM,
		ModelName: "gpt-test",
		Status:    enums.StatusOk,
	}
	if err := sqls.DB().Create(item).Error; err != nil {
		t.Fatalf("create ai config: %v", err)
	}
	return item.ID
}

func createAIAgentWorkflowTestKnowledgeBase(t *testing.T) int64 {
	t.Helper()
	item := &models.KnowledgeBase{
		Name:          "workflow-test-kb",
		KnowledgeType: string(enums.KnowledgeBaseTypeFAQ),
		Status:        enums.StatusOk,
	}
	if err := sqls.DB().Create(item).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	return item.ID
}

func createAIAgentWorkflowVersion(t *testing.T) int64 {
	t.Helper()
	workflow := &models.AIWorkflow{
		Name:    "workflow-test",
		AgentID: 1,
		Status:  enums.StatusOk,
	}
	if err := sqls.DB().Create(workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	version := &models.AIWorkflowVersion{
		WorkflowID: workflow.ID,
		Version:    1,
		Status:     enums.StatusOk,
	}
	if err := sqls.DB().Create(version).Error; err != nil {
		t.Fatalf("create workflow version: %v", err)
	}
	return version.ID
}

func aiAgentWorkflowTestOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		UserID:     1,
		Username:   "agent-workflow-tester",
		Nickname:   "agent-workflow-tester",
		DomainType: models.DomainTypePlatform,
	}
}

func workflowHasNodeType(def dsl.Definition, nodeType string) bool {
	for _, node := range def.Nodes {
		if node.Type == nodeType {
			return true
		}
	}
	return false
}

func nodeTypeByID(def dsl.Definition, nodeID string) string {
	for _, node := range def.Nodes {
		if node.ID == nodeID {
			return node.Type
		}
	}
	return ""
}

func assertConditionBranchToNodeType(t *testing.T, def dsl.Definition, sourceID string, targetType string, operator string, right any) {
	t.Helper()
	nodeTypes := workflowNodeTypeMap(def)
	for _, branch := range conditionBranches(t, def, sourceID) {
		if nodeTypes[branch.TargetNodeID] != targetType || branch.Condition == nil {
			continue
		}
		if branch.Condition.Operator == operator && branch.Condition.Right == right {
			return
		}
	}
	t.Fatalf("expected %s condition branch from %s to %s with right=%v", operator, sourceID, targetType, right)
}

func assertConditionBranchToNodeID(t *testing.T, def dsl.Definition, sourceID string, targetID string, operator string, right any) {
	t.Helper()
	for _, branch := range conditionBranches(t, def, sourceID) {
		if branch.TargetNodeID != targetID || branch.Condition == nil {
			continue
		}
		if branch.Condition.Operator == operator && branch.Condition.Right == right {
			return
		}
	}
	t.Fatalf("expected %s condition branch from %s to %s with right=%v", operator, sourceID, targetID, right)
}

func assertDefaultBranchToNodeID(t *testing.T, def dsl.Definition, sourceID string, targetID string) {
	t.Helper()
	for _, branch := range conditionBranches(t, def, sourceID) {
		if branch.TargetNodeID == targetID && branch.Default {
			return
		}
	}
	t.Fatalf("expected default branch from %s to %s", sourceID, targetID)
}

func assertConditionRouteExcludesNodeID(t *testing.T, def dsl.Definition, sourceID string, targetID string) {
	t.Helper()
	for _, branch := range conditionBranches(t, def, sourceID) {
		if branch.TargetNodeID == targetID {
			t.Fatalf("condition route %s must not target %s", sourceID, targetID)
		}
	}
}

func conditionBranches(t *testing.T, def dsl.Definition, nodeID string) []dsl.ConditionBranch {
	t.Helper()
	for _, node := range def.Nodes {
		if node.ID != nodeID {
			continue
		}
		var config dsl.ConditionConfig
		if err := json.Unmarshal(node.Config, &config); err != nil {
			t.Fatalf("unmarshal condition config for %s: %v", nodeID, err)
		}
		return config.Branches
	}
	t.Fatalf("condition node not found: %s", nodeID)
	return nil
}

func workflowEdgeExists(def dsl.Definition, sourceID string, targetID string) bool {
	for _, edge := range def.Edges {
		if edge.Source == sourceID && edge.Target == targetID {
			return true
		}
	}
	return false
}

func assertWorkflowFailureTarget(t *testing.T, def dsl.Definition, nodeID string, targetID string) {
	t.Helper()
	for _, node := range def.Nodes {
		if node.ID != nodeID {
			continue
		}
		if node.ErrorTargetNodeID != targetID {
			t.Fatalf("workflow node %s failure target = %q, want %q", nodeID, node.ErrorTargetNodeID, targetID)
		}
		if !workflowEdgeExists(def, nodeID, targetID) {
			t.Fatalf("workflow failure edge is missing: %s -> %s", nodeID, targetID)
		}
		return
	}
	t.Fatalf("workflow node not found: %s", nodeID)
}

type workflowLayoutBox struct {
	NodeID string
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

func assertWorkflowLayoutDoesNotOverlap(t *testing.T, def dsl.Definition) {
	t.Helper()
	boxes := make([]workflowLayoutBox, 0, len(def.Nodes))
	for _, node := range def.Nodes {
		width, height := defaultWorkflowNodeRenderSize(node.Type)
		boxes = append(boxes, workflowLayoutBox{
			NodeID: node.ID,
			Left:   node.Position.X,
			Top:    node.Position.Y,
			Right:  node.Position.X + width,
			Bottom: node.Position.Y + height,
		})
	}
	const minGap = 32.0
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			if workflowBoxesOverlapWithGap(boxes[i], boxes[j], minGap) {
				t.Fatalf("default workflow nodes are too close or overlapping: %s=%+v %s=%+v", boxes[i].NodeID, boxes[i], boxes[j].NodeID, boxes[j])
			}
		}
	}
}

func defaultWorkflowNodeRenderSize(nodeType string) (float64, float64) {
	if nodeType == workflowregistry.NodeTypeCondition {
		return 160, 160
	}
	return 220, 128
}

func workflowBoxesOverlapWithGap(a workflowLayoutBox, b workflowLayoutBox, gap float64) bool {
	return a.Left < b.Right+gap && a.Right+gap > b.Left && a.Top < b.Bottom+gap && a.Bottom+gap > b.Top
}

func workflowNodeTypeMap(def dsl.Definition) map[string]string {
	ret := make(map[string]string, len(def.Nodes))
	for _, node := range def.Nodes {
		ret[node.ID] = node.Type
	}
	return ret
}
