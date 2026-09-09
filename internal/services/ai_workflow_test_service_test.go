package services

import (
	"context"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

func TestAIWorkflowServiceTestRunUsesTenantAgentAndDraftDefinition(t *testing.T) {
	setupAIWorkflowTestDB(t)
	workflow := createAIWorkflowTestTemplate(t, 1)
	previousHook := AIWorkflowTestRunHook
	t.Cleanup(func() { AIWorkflowTestRunHook = previousHook })

	called := false
	AIWorkflowTestRunHook = func(_ context.Context, input AIWorkflowTestRunInput) (*response.AIWorkflowTestRunResponse, error) {
		called = true
		if input.Workflow.ID != workflow.ID || input.Agent.ID != 12 {
			t.Fatalf("unexpected test context: workflow=%d agent=%d", input.Workflow.ID, input.Agent.ID)
		}
		if input.Request.Definition.EntryNodeID != "start_1" {
			t.Fatalf("expected current draft definition, got entry=%q", input.Request.Definition.EntryNodeID)
		}
		return &response.AIWorkflowTestRunResponse{WorkflowID: workflow.ID, AIAgentID: input.Agent.ID, Status: "completed"}, nil
	}

	result, err := AIWorkflowService.TestRun(context.Background(), workflow.ID, request.TestAIWorkflowRequest{
		AIAgentID:   12,
		Definition:  validAIWorkflowDefinition(),
		UserMessage: "设备报错 E42",
		AutoConfirm: true,
	}, aiWorkflowTestOperator())
	if err != nil {
		t.Fatalf("TestRun() error = %v", err)
	}
	if !called || result.Status != "completed" || result.AIAgentID != 12 {
		t.Fatalf("unexpected test result: called=%v result=%+v", called, result)
	}
}

func TestAIWorkflowServiceTestRunRejectsUnavailableAgents(t *testing.T) {
	setupAIWorkflowTestDB(t)
	workflow := createAIWorkflowTestTemplate(t, 1)
	foreignAgent := models.AIAgent{ID: 1001, TenantID: 2, Name: "foreign-agent", Status: enums.StatusOk}
	if err := sqls.DB().Create(&foreignAgent).Error; err != nil {
		t.Fatalf("create foreign agent: %v", err)
	}
	disabledAgent := models.AIAgent{ID: 1002, TenantID: 1, Name: "disabled-agent", Status: enums.StatusDisabled}
	if err := sqls.DB().Create(&disabledAgent).Error; err != nil {
		t.Fatalf("create disabled agent: %v", err)
	}
	previousHook := AIWorkflowTestRunHook
	t.Cleanup(func() { AIWorkflowTestRunHook = previousHook })
	AIWorkflowTestRunHook = func(_ context.Context, _ AIWorkflowTestRunInput) (*response.AIWorkflowTestRunResponse, error) {
		t.Fatal("test runner must not be called for a cross-tenant agent")
		return nil, nil
	}

	_, err := AIWorkflowService.TestRun(context.Background(), workflow.ID, request.TestAIWorkflowRequest{
		AIAgentID:   foreignAgent.ID,
		Definition:  validAIWorkflowDefinition(),
		UserMessage: "设备报错 E42",
	}, aiWorkflowTestOperator())
	if err == nil {
		t.Fatal("expected cross-tenant agent to be rejected")
	}

	_, err = AIWorkflowService.TestRun(context.Background(), workflow.ID, request.TestAIWorkflowRequest{
		AIAgentID:   disabledAgent.ID,
		Definition:  validAIWorkflowDefinition(),
		UserMessage: "设备报错 E42",
	}, aiWorkflowTestOperator())
	if err == nil {
		t.Fatal("expected disabled agent to be rejected")
	}
}

func TestAIWorkflowServiceTestRunRejectsUnknownBranchOverride(t *testing.T) {
	setupAIWorkflowTestDB(t)
	workflow := createAIWorkflowTestTemplate(t, 1)
	previousHook := AIWorkflowTestRunHook
	t.Cleanup(func() { AIWorkflowTestRunHook = previousHook })
	AIWorkflowTestRunHook = func(_ context.Context, _ AIWorkflowTestRunInput) (*response.AIWorkflowTestRunResponse, error) {
		t.Fatal("test runner must not be called for an invalid branch override")
		return nil, nil
	}

	_, err := AIWorkflowService.TestRun(context.Background(), workflow.ID, request.TestAIWorkflowRequest{
		AIAgentID:   12,
		Definition:  validAIWorkflowDefinition(),
		UserMessage: "设备报错 E42",
		BranchOverrides: map[string]string{
			"missing_condition": "fallback",
		},
	}, aiWorkflowTestOperator())
	if err == nil {
		t.Fatal("expected unknown branch override to be rejected")
	}
}

func createAIWorkflowTestTemplate(t *testing.T, tenantID int64) models.AIWorkflow {
	t.Helper()
	workflow := models.AIWorkflow{
		TenantID: tenantID,
		Scope:    models.AIWorkflowScopeTenant,
		Name:     "draft test template",
		AgentID:  0,
		Status:   enums.StatusOk,
	}
	if err := sqls.DB().Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow template: %v", err)
	}
	return workflow
}
