package builders

import (
	"encoding/json"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
)

func TestBuildAIWorkflowNodeSpecsIncludesVariableContracts(t *testing.T) {
	specs := BuildAIWorkflowNodeSpecs(workflowregistry.DefaultRegistry().List())

	var startFound bool
	var sendReplyFound bool
	for _, spec := range specs {
		switch spec.Type {
		case workflowregistry.NodeTypeStart:
			startFound = true
			if !hasResponseVariable(spec.OutputSchema, "userMessage") {
				t.Fatalf("expected start output userMessage, got %#v", spec.OutputSchema)
			}
		case workflowregistry.NodeTypeSendReply:
			sendReplyFound = true
			if !hasResponseVariable(spec.InputSchema, "replyText") {
				t.Fatalf("expected send_reply input replyText, got %#v", spec.InputSchema)
			}
		}
	}
	if !startFound || !sendReplyFound {
		t.Fatalf("expected start and send_reply specs in response")
	}
}

func TestBuildAIWorkflowDerivesHumanHandoffCapabilityFromDefinition(t *testing.T) {
	withHandoff, err := json.Marshal(dsl.Definition{Nodes: []dsl.Node{{ID: "handoff_1", Type: workflowregistry.NodeTypeHandoffToHuman}}})
	if err != nil {
		t.Fatalf("marshal handoff definition: %v", err)
	}
	withoutHandoff, err := json.Marshal(dsl.Definition{Nodes: []dsl.Node{{ID: "reply_1", Type: workflowregistry.NodeTypeSendReply}}})
	if err != nil {
		t.Fatalf("marshal AI-only definition: %v", err)
	}

	if response := BuildAIWorkflow(&models.AIWorkflow{DraftDefinition: string(withHandoff)}); !response.HumanHandoffEnabled {
		t.Fatal("expected handoff capability from handoff_to_human node")
	}
	if response := BuildAIWorkflow(&models.AIWorkflow{DraftDefinition: string(withoutHandoff)}); response.HumanHandoffEnabled {
		t.Fatal("expected AI-only capability without handoff_to_human node")
	}
}

func TestBuildAIWorkflowRunIncludesAuditDisplayFields(t *testing.T) {
	startedAt := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(1500 * time.Millisecond)

	resp := BuildAIWorkflowRunWithContext(
		&models.AIWorkflowRun{
			ID:                9,
			WorkflowID:        11,
			WorkflowVersionID: 22,
			AIAgentID:         33,
			StartedAt:         startedAt,
			EndedAt:           &endedAt,
			Status:            1,
		},
		&models.AIWorkflow{Name: "售后会话流程"},
		&models.AIWorkflowVersion{Version: 3},
		&models.AIAgent{Name: "售后 Agent"},
		&dto.AIWorkflowSkillAudit{
			State:                    dto.AIWorkflowSkillStateSelected,
			MiddlewareEnabled:        true,
			CandidateSkills:          []dto.AIWorkflowSkillCandidate{{ID: 3, Name: "操作与维修指导"}},
			SelectedSkillID:          3,
			SelectedSkillName:        "操作与维修指导",
			SelectedSkillDescription: "依据知识给出操作步骤",
			MatchReason:              "eino_skill_tool",
			ExposedToolCodes:         []string{"builtin/skill"},
			InvokedToolCodes:         []string{"builtin/skill"},
			SourceMessageID:          44,
		},
	)

	if resp.WorkflowName != "售后会话流程" {
		t.Fatalf("expected workflow name, got %q", resp.WorkflowName)
	}
	if resp.WorkflowVersion != 3 {
		t.Fatalf("expected workflow version 3, got %d", resp.WorkflowVersion)
	}
	if resp.AIAgentName != "售后 Agent" {
		t.Fatalf("expected agent name, got %q", resp.AIAgentName)
	}
	if resp.DurationMS != 1500 {
		t.Fatalf("expected duration 1500ms, got %d", resp.DurationMS)
	}
	if resp.SkillAudit.SelectedSkillID != 3 || resp.SkillAudit.SelectedSkillName != "操作与维修指导" {
		t.Fatalf("unexpected skill audit: %#v", resp.SkillAudit)
	}
	if len(resp.SkillAudit.CandidateSkills) != 1 || len(resp.SkillAudit.InvokedToolCodes) != 1 {
		t.Fatalf("expected candidate and invoked tool audit data: %#v", resp.SkillAudit)
	}
}

func TestBuildAIWorkflowHumanHandlingAudit(t *testing.T) {
	handoffAt := time.Date(2026, 7, 30, 21, 4, 10, 0, time.UTC)
	firstReplyAt := handoffAt.Add(4 * time.Second)

	resp := BuildAIWorkflowHumanHandlingAudit(&dto.AIWorkflowHumanHandlingAudit{
		HandoffOccurred:          true,
		HandoffAt:                handoffAt,
		HandoffReason:            "客户请求人工",
		HandledByHuman:           true,
		HandlerUserID:            25,
		HandlerName:              "产品测试1值班工程师",
		FirstHumanReplyMessageID: 2470,
		FirstHumanReplyAt:        firstReplyAt,
		ConversationStatus:       enums.IMConversationStatusClosed,
		ConversationStatusName:   "已关闭",
	})

	if resp == nil {
		t.Fatal("expected human handling response")
	}
	if !resp.HandoffOccurred || !resp.HandledByHuman || resp.HandlerUserID != 25 {
		t.Fatalf("unexpected human handling flags: %#v", resp)
	}
	if resp.HandlerName != "产品测试1值班工程师" || resp.FirstHumanReplyMessageID != 2470 {
		t.Fatalf("unexpected human handling identity: %#v", resp)
	}
	if resp.HandoffAt != "2026-07-30 21:04:10" || resp.FirstHumanReplyAt != "2026-07-30 21:04:14" {
		t.Fatalf("unexpected human handling timestamps: %#v", resp)
	}
	if resp.ConversationStatus != int(enums.IMConversationStatusClosed) || resp.ConversationStatusName != "已关闭" {
		t.Fatalf("unexpected conversation status: %#v", resp)
	}
}

func TestBuildAIWorkflowRunDetailIncludesPublishedDefinitionSnapshot(t *testing.T) {
	definition := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "开始"},
			{ID: "reply_1", Type: workflowregistry.NodeTypeLLMReply, Name: "运行时回复"},
		},
		Edges: []dsl.Edge{{ID: "edge_start_reply", Source: "start_1", Target: "reply_1"}},
	}
	buf, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}

	resp := BuildAIWorkflowRunDetailWithContext(
		&models.AIWorkflowRun{ID: 9, WorkflowVersionID: 22, Status: 1, StartedAt: time.Now()},
		nil,
		&models.AIWorkflow{Name: "当前 Workflow 草稿不应参与审计图"},
		&models.AIWorkflowVersion{Version: 3, Definition: string(buf)},
		&models.AIAgent{Name: "售后 Agent"},
		nil,
	)

	if resp.Definition.EntryNodeID != "start_1" {
		t.Fatalf("expected run detail definition from published version, got %#v", resp.Definition)
	}
	if len(resp.Definition.Nodes) != 2 || resp.Definition.Nodes[1].Name != "运行时回复" {
		t.Fatalf("expected published definition nodes, got %#v", resp.Definition.Nodes)
	}
	if len(resp.Definition.Edges) != 1 || resp.Definition.Edges[0].ID != "edge_start_reply" {
		t.Fatalf("expected published definition edges, got %#v", resp.Definition.Edges)
	}
}

func hasResponseVariable(items []workflowregistry.VariableSpec, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}
