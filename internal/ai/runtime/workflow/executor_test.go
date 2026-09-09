package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"remotehelpdesk/internal/ai/rag"
	runtimeexecutor "remotehelpdesk/internal/ai/runtime/executor"
	"remotehelpdesk/internal/ai/runtime/internal/impl/retrievers"
	"remotehelpdesk/internal/ai/runtime/registry"
	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/toolx"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeWorkflowAgentRuntime struct {
	input  runtimeexecutor.WorkflowNodeInput
	result *runtimeexecutor.RunResult
	err    error
}

func (f *fakeWorkflowAgentRuntime) ExecuteWorkflowNode(_ context.Context, input runtimeexecutor.WorkflowNodeInput) (*runtimeexecutor.RunResult, error) {
	f.input = input
	return f.result, f.err
}

func mustMarshalWorkflowTestConfig(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func TestWorkflowPreviewJSONPreservesUTF8WhenTruncated(t *testing.T) {
	preview := workflowPreviewJSON(map[string]string{
		"content": strings.Repeat("中文排障步骤", 500),
	})
	if !utf8.ValidString(preview) {
		t.Fatalf("workflow preview contains invalid UTF-8: %q", preview[len(preview)-8:])
	}
	if len([]byte(preview)) > 2000 {
		t.Fatalf("workflow preview is %d bytes, want at most 2000", len([]byte(preview)))
	}
}

func TestSendReplyPreservesKnowledgeCitations(t *testing.T) {
	definition := dsl.Definition{
		EntryNodeID: "send_1",
		Nodes: []dsl.Node{{
			ID:   "send_1",
			Type: workflowregistry.NodeTypeSendReply,
			Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "answer_1", Field: "replyText"},
				"citations": {NodeID: "retrieve_1", Field: "citations"},
			},
		}},
	}
	state := newRunState(Input{Definition: definition})
	state.setNodeVars("answer_1", map[string]any{"replyText": "Calibrate the SI-42 every 90 days."})
	state.setNodeVars("retrieve_1", map[string]any{"citations": []dto.KnowledgeCitation{{
		DocumentID:    106,
		DocumentTitle: "odt-uat-knowledge-20260906.md",
		ChunkNo:       0,
		Snippet:       "The SI-42 calibration interval is 90 days.",
	}}})

	if err := NewExecutor().executeNode(context.Background(), state, definition.Nodes[0]); err != nil {
		t.Fatalf("execute send reply: %v", err)
	}
	if state.result.ReplyText != "Calibrate the SI-42 every 90 days." {
		t.Fatalf("reply text = %q", state.result.ReplyText)
	}
	if len(state.result.ReplyCitations) != 1 || state.result.ReplyCitations[0].DocumentID != 106 {
		t.Fatalf("reply citations = %#v", state.result.ReplyCitations)
	}
}

func TestEntryContextAndServiceAccessPolicyKeepUnboundSessionsAIOnly(t *testing.T) {
	definition := serviceAccessPolicyTestDefinition()
	state := newRunState(Input{
		Definition: definition,
		Conversation: models.Conversation{
			ProductID:   11,
			ServiceMode: enums.IMConversationServiceModeAIFirst,
		},
	})
	executor := NewExecutor()
	for _, nodeID := range []string{"start_1", "entry_1", "access_1"} {
		if err := executor.executeNode(context.Background(), state, state.nodesByID[nodeID]); err != nil {
			t.Fatalf("execute %s: %v", nodeID, err)
		}
	}
	if got := state.vars["entry_1"]["entryMode"]; got != "quick_ai" {
		t.Fatalf("entryMode = %v, want quick_ai", got)
	}
	if truthy(state.vars["access_1"]["allowHumanHandoff"]) || truthy(state.vars["access_1"]["allowTicketCreation"]) {
		t.Fatalf("unbound session unexpectedly received service actions: %#v", state.vars["access_1"])
	}
	if got := state.vars["access_1"]["conversationTag"]; got != "AI问答" {
		t.Fatalf("conversationTag = %v, want AI问答", got)
	}
}

func TestEntryContextAndServiceAccessPolicyEnableBoundDeviceService(t *testing.T) {
	definition := serviceAccessPolicyTestDefinition()
	state := newRunState(Input{
		Definition: definition,
		Conversation: models.Conversation{
			ProductID:      11,
			ProductModelID: 12,
			DeviceID:       13,
			ServiceCodeID:  14,
			ServiceMode:    enums.IMConversationServiceModeAIFirst,
		},
	})
	executor := NewExecutor()
	for _, nodeID := range []string{"start_1", "entry_1", "access_1"} {
		if err := executor.executeNode(context.Background(), state, state.nodesByID[nodeID]); err != nil {
			t.Fatalf("execute %s: %v", nodeID, err)
		}
	}
	if got := state.vars["entry_1"]["entryMode"]; got != "device_service_code" {
		t.Fatalf("entryMode = %v, want device_service_code", got)
	}
	if !truthy(state.vars["access_1"]["allowHumanHandoff"]) || !truthy(state.vars["access_1"]["allowTicketCreation"]) {
		t.Fatalf("bound device did not receive configured service actions: %#v", state.vars["access_1"])
	}
	if got := state.vars["access_1"]["serviceMode"]; got != "ai_first" {
		t.Fatalf("serviceMode = %v, want ai_first", got)
	}
}

func TestEntryContextSupportsSimulatedDeviceBindingWithoutDatabaseIDs(t *testing.T) {
	definition := serviceAccessPolicyTestDefinition()
	state := newRunState(Input{
		Definition:          definition,
		SimulateDeviceBound: true,
		Conversation: models.Conversation{
			ProductID:   11,
			ServiceMode: enums.IMConversationServiceModeAIFirst,
		},
	})
	executor := NewExecutor()
	for _, nodeID := range []string{"start_1", "entry_1", "access_1"} {
		if err := executor.executeNode(context.Background(), state, state.nodesByID[nodeID]); err != nil {
			t.Fatalf("execute %s: %v", nodeID, err)
		}
	}
	if got := state.vars["entry_1"]["entryMode"]; got != "bound_device" {
		t.Fatalf("entryMode = %v, want bound_device", got)
	}
	if !truthy(state.vars["access_1"]["allowHumanHandoff"]) || !truthy(state.vars["access_1"]["allowTicketCreation"]) {
		t.Fatalf("simulated bound device did not receive configured service actions: %#v", state.vars["access_1"])
	}
	if state.input.Conversation.DeviceID != 0 || state.input.Conversation.ServiceCodeID != 0 {
		t.Fatalf("simulation must not fabricate database IDs: %#v", state.input.Conversation)
	}
}

func TestServiceAccessPolicyHonorsAIOnlyBoundaryForBoundDevice(t *testing.T) {
	definition := serviceAccessPolicyTestDefinition()
	state := newRunState(Input{
		Definition: definition,
		Conversation: models.Conversation{
			ProductID:   11,
			DeviceID:    13,
			ServiceMode: enums.IMConversationServiceModeAIOnly,
		},
	})
	executor := NewExecutor()
	for _, nodeID := range []string{"start_1", "entry_1", "access_1"} {
		if err := executor.executeNode(context.Background(), state, state.nodesByID[nodeID]); err != nil {
			t.Fatalf("execute %s: %v", nodeID, err)
		}
	}
	if truthy(state.vars["access_1"]["allowHumanHandoff"]) || truthy(state.vars["access_1"]["allowTicketCreation"]) {
		t.Fatalf("AI-only session crossed service boundary: %#v", state.vars["access_1"])
	}
}

func serviceAccessPolicyTestDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart},
			{ID: "entry_1", Type: workflowregistry.NodeTypeEntryContext, Config: mustMarshalWorkflowTestConfig(dsl.EntryContextConfig{UnboundMode: "quick_ai"}), Inputs: map[string]dsl.VariableSelector{
				"productId":      {NodeID: "start_1", Field: "productId"},
				"productModelId": {NodeID: "start_1", Field: "productModelId"},
				"deviceId":       {NodeID: "start_1", Field: "deviceId"},
				"serviceCodeId":  {NodeID: "start_1", Field: "serviceCodeId"},
			}},
			{ID: "access_1", Type: workflowregistry.NodeTypeServiceAccessPolicy, Config: mustMarshalWorkflowTestConfig(dsl.ServiceAccessPolicyConfig{HumanHandoffMode: "device_only", TicketAccessMode: "device_only"}), Inputs: map[string]dsl.VariableSelector{
				"contextLevel":            {NodeID: "entry_1", Field: "contextLevel"},
				"deviceBound":             {NodeID: "entry_1", Field: "deviceBound"},
				"conversationServiceMode": {NodeID: "start_1", Field: "conversationServiceMode"},
			}},
			{ID: "handoff_1", Type: workflowregistry.NodeTypeHandoffToHuman},
			{ID: "ticket_1", Type: workflowregistry.NodeTypeCreateTicket},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd},
		},
	}
}

func TestUnderstandConversationMessageHonorsActionNegation(t *testing.T) {
	tests := []struct {
		message    string
		wantIntent string
		wantScope  string
	}{
		{
			message:    "设备显示故障码 RHD-FLOW-ALPHA-7742，请告诉我标准复位步骤，暂时不要转人工。",
			wantIntent: "business_question",
			wantScope:  "needs_knowledge",
		},
		{
			message:    "请先按产品知识给出下一步安全排查，不要自动转人工。",
			wantIntent: "business_question",
			wantScope:  "needs_knowledge",
		},
		{
			message:    "先不要创建工单，我想继续排查设备。",
			wantIntent: "business_question",
			wantScope:  "needs_knowledge",
		},
		{
			message:    "请转人工工程师处理",
			wantIntent: "handoff_request",
			wantScope:  "needs_handoff",
		},
		{
			message:    "请帮我创建工单",
			wantIntent: "ticket_request",
			wantScope:  "needs_ticket",
		},
	}
	for _, tt := range tests {
		got := understandConversationMessage(tt.message)
		if got.MessageIntent != tt.wantIntent || got.AnswerScope != tt.wantScope {
			t.Errorf("understandConversationMessage(%q) = intent %q scope %q, want %q/%q", tt.message, got.MessageIntent, got.AnswerScope, tt.wantIntent, tt.wantScope)
		}
	}
}

func TestSanitizeUnverifiedActionClaims(t *testing.T) {
	got := sanitizeUnverifiedActionClaims(
		"我已为你转人工工程师处理，并已为你创建工单；我已邀请供应商。",
		workflowcapability.Set{HumanHandoff: true, TicketCreation: true},
	)
	for _, forbidden := range []string{"已为你转人工", "已为你创建工单", "已邀请供应商"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("unverified action claim %q remained in %q", forbidden, got)
		}
	}
	if !strings.Contains(got, "建议转人工工程师继续处理") || !strings.Contains(got, "确认后由系统执行") {
		t.Fatalf("unexpected grounded reply: %q", got)
	}
}

func TestSanitizeUnverifiedActionClaimsRespectsAIOnlyCapabilities(t *testing.T) {
	capabilities := workflowcapability.Set{}
	got := sanitizeUnverifiedActionClaims("我已为你转人工工程师处理，并已为你创建工单。", capabilities)
	if strings.Contains(got, "建议转人工") || strings.Contains(got, "请通过页面确认") || strings.Contains(got, "确认后由系统执行") {
		t.Fatalf("AI-only reply still offers an unavailable action: %q", got)
	}
	if !strings.Contains(got, "不提供转人工") || !strings.Contains(got, "不创建工单") {
		t.Fatalf("AI-only reply does not state its capability boundary: %q", got)
	}

	agent := models.AIAgent{FallbackMessage: "当前知识不足，我会协助转接人工工程师。"}
	if fallback := workflowKnowledgeFallbackReply(agent, capabilities); fallback != workflowcapability.SafeKnowledgeFallbackMessage {
		t.Fatalf("AI-only fallback = %q, want %q", fallback, workflowcapability.SafeKnowledgeFallbackMessage)
	}
}

func TestDecideWorkflowReplyPolicyRespectsCapabilityBoundary(t *testing.T) {
	agent := models.AIAgent{}

	handoff := decideWorkflowReplyPolicy(agent, workflowcapability.Set{}, workflowReplyPolicyInput{
		MessageIntent: "handoff_request",
		AnswerScope:   "needs_handoff",
	})
	if handoff.Action != "direct_reply" || handoff.RequiresFlow || handoff.TargetFlow != "" || !strings.Contains(handoff.ReplyText, "不提供转人工") {
		t.Fatalf("AI-only handoff decision = %+v", handoff)
	}

	ticket := decideWorkflowReplyPolicy(agent, workflowcapability.Set{}, workflowReplyPolicyInput{
		MessageIntent: "ticket_request",
		AnswerScope:   "needs_ticket",
	})
	if ticket.Action != "direct_reply" || ticket.RequiresFlow || ticket.TargetFlow != "" || !strings.Contains(ticket.ReplyText, "不创建工单") {
		t.Fatalf("AI-only ticket decision = %+v", ticket)
	}

	capable := decideWorkflowReplyPolicy(agent, workflowcapability.Set{HumanHandoff: true}, workflowReplyPolicyInput{
		MessageIntent: "handoff_request",
		AnswerScope:   "needs_handoff",
	})
	if capable.Action != "handoff_to_human" || !capable.RequiresFlow || capable.TargetFlow != "handoff_to_human" {
		t.Fatalf("handoff-capable decision = %+v", capable)
	}
}

func TestUnderstandConversationMessageOnlyClarifiesActuallyAmbiguousQuestions(t *testing.T) {
	tests := []struct {
		message    string
		wantIntent string
		wantScope  string
	}{
		{
			message:    "现场控制参数再次漂移时，视频复核后应该怎么处理，后续权限有什么建议？",
			wantIntent: "business_question",
			wantScope:  "needs_knowledge",
		},
		{
			message:    "控制器报 E42 怎么处理？",
			wantIntent: "business_question",
			wantScope:  "needs_knowledge",
		},
		{
			message:    "这个怎么处理？",
			wantIntent: "ambiguous_question",
			wantScope:  "needs_clarification",
		},
		{
			message:    "怎么办",
			wantIntent: "ambiguous_question",
			wantScope:  "needs_clarification",
		},
	}
	for _, tt := range tests {
		got := understandConversationMessage(tt.message)
		if got.MessageIntent != tt.wantIntent || got.AnswerScope != tt.wantScope {
			t.Errorf("understandConversationMessage(%q) = intent %q scope %q, want %q/%q", tt.message, got.MessageIntent, got.AnswerScope, tt.wantIntent, tt.wantScope)
		}
	}
}

func TestMergeWorkflowRetrieverTracePreservesAgentTrace(t *testing.T) {
	retrieval := &retrievers.KnowledgeRetrieveResult{
		Hits:           []rag.RetrieveResult{{KnowledgeBaseID: 1, DocumentID: 2, Score: 0.91}},
		ContextResults: []rag.RetrieveResult{{KnowledgeBaseID: 1, DocumentID: 2, Score: 0.91}},
	}
	retrieval.TraceSummary.TopK = 8
	retrieval.TraceSummary.ContextMaxTokens = 4000
	trace := mergeWorkflowRetrieverTrace(`{"agentNodes":{"reply_1":{"status":"completed"}}}`, retrieval)
	for _, want := range []string{`"agentNodes"`, `"retriever"`, `"count":1`, `"topK":8`, `"contextMaxTokens":4000`} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace %q does not contain %q", trace, want)
		}
	}
}

func TestExecutorRoutesByConditionNodeBranch(t *testing.T) {
	executor := NewExecutor()
	result, err := executor.Execute(context.Background(), Input{
		Definition: conditionalReplyDefinition(),
		UserMessage: models.Message{
			Content: "vip",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "VIP reply" {
		t.Fatalf("unexpected reply: %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "condition_1", "vip_reply", "send_vip", "end_1"})
}

func TestExecutorDryRunCanOverrideConditionBranch(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: conditionalReplyDefinition(),
		UserMessage: models.Message{
			Content: "vip",
		},
		DryRun: true,
		BranchOverrides: map[string]string{
			"condition_1": "default",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow with branch override: %v", err)
	}
	if result.ReplyText != "Normal reply" {
		t.Fatalf("branch override reply = %q, want Normal reply", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "condition_1", "normal_reply", "send_normal", "end_1"})
	trace := findNodeTrace(result.NodeTraces, "condition_1")
	if trace == nil || !strings.Contains(trace.OutputPreview, `"selectedBranchId":"default"`) || !strings.Contains(trace.OutputPreview, `"reason":"test run branch override"`) {
		t.Fatalf("expected overridden branch trace, got %#v", trace)
	}
}

func TestExecutorRoutesNodeFailureToDeclaredFallback(t *testing.T) {
	definition := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "reply_1", Type: workflowregistry.NodeTypeLLMReply, Name: "AI reply", ErrorTargetNodeID: "fallback_1", Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "send_1", Type: workflowregistry.NodeTypeSendReply, Name: "Send", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "reply_1", Field: "replyText"},
			}},
			{ID: "fallback_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Fallback", Config: json.RawMessage(`{"staticReply":"服务暂时不可用，已进入兜底流程。"}`), Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "fallback_send_1", Type: workflowregistry.NodeTypeSendReply, Name: "Send fallback", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "fallback_1", Field: "replyText"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_reply", Source: "start_1", Target: "reply_1"},
			{ID: "edge_reply_send", Source: "reply_1", Target: "send_1"},
			{ID: "edge_reply_fallback", Source: "reply_1", Target: "fallback_1"},
			{ID: "edge_send_end", Source: "send_1", Target: "end_1"},
			{ID: "edge_fallback_send", Source: "fallback_1", Target: "fallback_send_1"},
			{ID: "edge_fallback_end", Source: "fallback_send_1", Target: "end_1"},
		},
	}
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   definition,
		UserMessage:  models.Message{Content: "设备无法启动"},
		AgentRuntime: &fakeWorkflowAgentRuntime{err: errors.New("model timeout")},
	})
	if err != nil {
		t.Fatalf("execute workflow with fallback: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "reply_1", "fallback_1", "fallback_send_1", "end_1"})
	if result.ReplyText != "服务暂时不可用，已进入兜底流程。" {
		t.Fatalf("unexpected fallback reply: %q", result.ReplyText)
	}
	trace := findNodeTrace(result.NodeTraces, "reply_1")
	if trace == nil || trace.Status != "recovered" || !strings.Contains(trace.ErrorMessage, "model timeout") || !strings.Contains(trace.OutputPreview, `"errorTargetNodeId":"fallback_1"`) {
		t.Fatalf("expected recovered node trace, got %#v", trace)
	}
}

func TestAnswerabilityGateRejectsWeakUnrelatedEvidence(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		content    string
		score      float64
		wantStatus string
	}{
		{name: "unrelated medium score", query: "控制器报 E42 怎么复位", content: "账户发票开具与邮寄说明", score: 0.48, wantStatus: "unanswerable"},
		{name: "matched evidence", query: "控制器报 E42 怎么复位", content: "E42 故障码表示过压，断电后按复位键八秒", score: 0.48, wantStatus: "answerable"},
		{name: "strong semantic score", query: "控制器无法启动", content: "上电异常的安全检查顺序", score: 0.91, wantStatus: "answerable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := dsl.Definition{Nodes: []dsl.Node{
				{ID: "start_1", Type: workflowregistry.NodeTypeStart},
				{ID: "retrieve_1", Type: workflowregistry.NodeTypeKnowledgeRetrieve},
				{ID: "gate_1", Type: workflowregistry.NodeTypeAnswerabilityGate, Config: mustMarshalWorkflowTestConfig(dsl.AnswerabilityGateConfig{
					MinScore: 0.35, StrongScore: 0.72, MinMatchedTerms: 1,
				}), Inputs: map[string]dsl.VariableSelector{
					"userMessage":    {NodeID: "start_1", Field: "userMessage"},
					"knowledgeItems": {NodeID: "retrieve_1", Field: "items"},
				}},
			}}
			state := newRunState(Input{Definition: definition})
			state.setNodeVars("start_1", map[string]any{"userMessage": tt.query})
			state.setNodeVars("retrieve_1", map[string]any{"items": []map[string]any{{"content": tt.content, "score": tt.score}}})
			if err := NewExecutor().executeAnswerabilityGate(state, definition.Nodes[2]); err != nil {
				t.Fatalf("execute answerability gate: %v", err)
			}
			if got := toString(state.vars["gate_1"]["answerability"]); got != tt.wantStatus {
				t.Fatalf("answerability = %q, want %q; output=%#v", got, tt.wantStatus, state.vars["gate_1"])
			}
		})
	}
}

func TestAnswerabilityGateUsesDenseScoreAfterHybridFusion(t *testing.T) {
	definition := dsl.Definition{Nodes: []dsl.Node{
		{ID: "start_1", Type: workflowregistry.NodeTypeStart},
		{ID: "retrieve_1", Type: workflowregistry.NodeTypeKnowledgeRetrieve},
		{ID: "gate_1", Type: workflowregistry.NodeTypeAnswerabilityGate, Config: mustMarshalWorkflowTestConfig(dsl.AnswerabilityGateConfig{
			MinScore: 0.35, StrongScore: 0.72, MinMatchedTerms: 1,
		}), Inputs: map[string]dsl.VariableSelector{
			"userMessage":    {NodeID: "start_1", Field: "userMessage"},
			"knowledgeItems": {NodeID: "retrieve_1", Field: "items"},
		}},
	}}
	state := newRunState(Input{Definition: definition})
	state.setNodeVars("start_1", map[string]any{"userMessage": "故障码 RHD-FLOW-ALPHA-7742 怎么复位"})
	state.setNodeVars("retrieve_1", map[string]any{"items": []map[string]any{{
		"content": "故障码 RHD-FLOW-ALPHA-7742 需要断开主电源 30 秒",
		"score":   0.03278688, "denseScore": 0.95, "fusionScore": 0.03278688,
	}}})
	if err := NewExecutor().executeAnswerabilityGate(state, definition.Nodes[2]); err != nil {
		t.Fatalf("execute answerability gate: %v", err)
	}
	if got := toString(state.vars["gate_1"]["answerability"]); got != "answerable" {
		t.Fatalf("hybrid evidence answerability = %q, want answerable; output=%#v", got, state.vars["gate_1"])
	}
	if got := toFloat(state.vars["gate_1"]["bestScore"]); got != 0.95 {
		t.Fatalf("bestScore = %v, want original dense score", got)
	}
}

func TestExecutorConditionNodeTraceExplainsMatchedEdge(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: conditionalReplyDefinition(),
		UserMessage: models.Message{
			Content: "vip",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}

	trace := findNodeTrace(result.NodeTraces, "condition_1")
	if trace == nil {
		t.Fatalf("expected condition node trace, got %#v", result.NodeTraces)
	}
	for _, want := range []string{
		`"selectedEdgeId":"edge_condition_vip"`,
		`"selectedBranchId":"vip"`,
		`"selectedTargetNodeId":"vip_reply"`,
		`"operator":"eq"`,
		`"leftValue":"vip"`,
		`"matched":true`,
	} {
		if !strings.Contains(trace.OutputPreview, want) {
			t.Fatalf("expected condition trace output to contain %s, got %s", want, trace.OutputPreview)
		}
	}
}

func TestExecutorConditionNodeTraceExplainsDefaultEdge(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: conditionalReplyDefinition(),
		UserMessage: models.Message{
			Content: "normal",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}

	trace := findNodeTrace(result.NodeTraces, "condition_1")
	if trace == nil {
		t.Fatalf("expected condition node trace, got %#v", result.NodeTraces)
	}
	for _, want := range []string{
		`"selectedEdgeId":"edge_condition_default"`,
		`"selectedBranchId":"default"`,
		`"selectedTargetNodeId":"normal_reply"`,
		`"reason":"no condition branch matched; selected default branch"`,
		`"leftValue":"normal"`,
		`"matched":false`,
	} {
		if !strings.Contains(trace.OutputPreview, want) {
			t.Fatalf("expected condition trace output to contain %s, got %s", want, trace.OutputPreview)
		}
	}
}

func TestExecutorUsesDefaultEdgeWhenConditionDoesNotMatch(t *testing.T) {
	executor := NewExecutor()
	result, err := executor.Execute(context.Background(), Input{
		Definition: conditionalReplyDefinition(),
		UserMessage: models.Message{
			Content: "normal",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "Normal reply" {
		t.Fatalf("unexpected reply: %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "condition_1", "normal_reply", "send_normal", "end_1"})
}

func TestExecutorHandoffToHumanRunsRealDispatchAction(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	createWorkflowExecutorHandoffTeam(t, db, 1, "售后支持组")
	createWorkflowExecutorHandoffActiveSchedule(t, db, 1)
	createWorkflowExecutorHandoffAgentProfile(t, db, 101, 1)
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "需要人工处理")

	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   handoffWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if strings.TrimSpace(result.ReplyText) != "" {
		t.Fatalf("expected workflow handoff node to avoid duplicate reply text, got %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "handoff_1", "handoff_route_1", "assigned_end"})

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected assigned conversation to wait for acceptance, got status=%d", current.Status)
	}
	if current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 {
		t.Fatalf("unexpected assignment: assignee=%d team=%d", current.CurrentAssigneeID, current.CurrentTeamID)
	}
	if current.HandoffAt == nil || current.HandoffReason != "需要人工处理" {
		t.Fatalf("expected handoff metadata, got at=%v reason=%q", current.HandoffAt, current.HandoffReason)
	}

	notice := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Eq("sender_type", enums.IMSenderTypeAI).Desc("id"))
	if notice == nil || strings.TrimSpace(notice.Content) == "" {
		t.Fatalf("expected handoff service to send ai notice, got %+v", notice)
	}
}

func TestExecutorResumeSkipsHandoffWhenConfirmationCancelled(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	createWorkflowExecutorHandoffTeam(t, db, 1, "售后支持组")
	createWorkflowExecutorHandoffActiveSchedule(t, db, 1)
	createWorkflowExecutorHandoffAgentProfile(t, db, 101, 1)
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "需要人工处理")
	input := Input{
		Definition:   handoffAfterConfirmationWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	}

	interrupted, err := NewExecutor().Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !interrupted.Interrupted {
		t.Fatalf("expected workflow to interrupt before handoff")
	}

	result, err := NewExecutor().Resume(context.Background(), input, interrupted.CheckPointData, "取消")
	if err != nil {
		t.Fatalf("resume workflow: %v", err)
	}
	if result.Interrupted {
		t.Fatalf("expected cancelled resume to complete")
	}
	assertPath(t, result.NodePath, []string{"handoff_1", "end_1"})

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("expected conversation to remain ai serving, got status=%d", current.Status)
	}
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.HandoffAt != nil {
		t.Fatalf("expected no handoff side effect, got assignee=%d team=%d handoffAt=%v", current.CurrentAssigneeID, current.CurrentTeamID, current.HandoffAt)
	}
	if count := services.MessageService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID).Eq("sender_type", enums.IMSenderTypeAI)); count != 0 {
		t.Fatalf("expected no handoff notice message, got %d", count)
	}
}

func TestExecutorAnalyzeConversationOutputsBranchVariables(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "你们重复扣费了，我要投诉并转人工")

	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   analyzeConversationWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "analyze_1", "analyze_route_1", "handoff_end"})
	trace := findNodeTrace(result.NodeTraces, "analyze_1")
	if trace == nil || !strings.Contains(trace.OutputPreview, `"recommendedNextAction":"handoff_to_human"`) || !strings.Contains(trace.OutputPreview, `"summary"`) {
		t.Fatalf("expected analyze node to expose dispatch fields, got %#v", trace)
	}
}

func TestExecutorPrepareTicketDraftOutputsDraftVariable(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "订单支付失败，请帮我登记工单")

	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   prepareTicketDraftWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "draft_1", "draft_route_1", "ready_end"})
}

func TestExecutorPrepareTicketDraftStopsBeforeConfirmationWithoutIssueDetails(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "请帮我创建工单")

	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   createTicketWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.Interrupted {
		t.Fatalf("action-only request must not reach ticket confirmation")
	}
	if !strings.Contains(result.ReplyText, "故障现象") {
		t.Fatalf("expected issue clarification, got %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "draft_1"})
}

func TestExecutorCreateTicketRevalidatesLegacyReadyDraft(t *testing.T) {
	state := newRunState(Input{})
	state.vars["draft_1"] = map[string]any{
		"ticketDraft": map[string]any{
			"ready":       true,
			"title":       "请帮我创建工单",
			"description": "请帮我创建工单",
		},
	}
	state.vars["confirm_1"] = map[string]any{"confirmed": true}
	node := dsl.Node{
		ID:   "create_1",
		Type: workflowregistry.NodeTypeCreateTicket,
		Inputs: map[string]dsl.VariableSelector{
			"confirmed":   {NodeID: "confirm_1", Field: "confirmed"},
			"ticketDraft": {NodeID: "draft_1", Field: "ticketDraft"},
		},
	}

	if err := NewExecutor().executeCreateTicket(state, node); err != nil {
		t.Fatalf("executeCreateTicket() error = %v", err)
	}
	if !state.stop || !strings.Contains(state.result.ReplyText, "故障现象") {
		t.Fatalf("legacy action-only draft must be stopped: %#v", state.result)
	}
	if truthy(state.vars[node.ID]["created"]) {
		t.Fatalf("legacy action-only draft must not be created: %#v", state.vars[node.ID])
	}
}

func TestExecutorPolicyFirstWorkflowRoutesGreetingToDirectReply(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: policyFirstWorkflowDefinition(),
		UserMessage: models.Message{
			Content: "<p>你好。</p>",
		},
		AIAgent: models.AIAgent{
			KnowledgeIDs:    "1",
			FallbackMessage: "我暂时没有找到足够准确的信息。",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "您好，请问有什么可以帮您？" {
		t.Fatalf("expected greeting reply, got %q", result.ReplyText)
	}
	if result.RetrieverCount != 0 {
		t.Fatalf("expected greeting to skip retrieval, got retriever count %d", result.RetrieverCount)
	}
	assertPath(t, result.NodePath, []string{"start_1", "understanding_1", "policy_1", "policy_route_1", "send_direct_1", "end_1"})

	understandingTrace := findNodeTrace(result.NodeTraces, "understanding_1")
	if understandingTrace == nil || !strings.Contains(understandingTrace.OutputPreview, `"messageIntent":"greeting"`) || !strings.Contains(understandingTrace.OutputPreview, `"answerScope":"direct_reply"`) {
		t.Fatalf("expected understanding trace to audit greeting/direct_reply, got %#v", understandingTrace)
	}
	policyTrace := findNodeTrace(result.NodeTraces, "policy_1")
	if policyTrace == nil || !strings.Contains(policyTrace.OutputPreview, `"action":"direct_reply"`) || !strings.Contains(policyTrace.OutputPreview, `"finalReplySource":"direct_reply"`) {
		t.Fatalf("expected policy trace to audit direct reply, got %#v", policyTrace)
	}
}

func TestExecutorPolicyFirstWorkflowRoutesBusinessQuestionToKnowledge(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: policyFirstWorkflowDefinition(),
		UserMessage: models.Message{
			Content: "你们价格是多少？",
		},
		AIAgent: models.AIAgent{
			KnowledgeIDs:    "1",
			FallbackMessage: "我暂时没有找到足够准确的信息。",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "understanding_1", "policy_1", "policy_route_1", "retrieve_end"})
}

func TestExecutorPolicyFirstWorkflowKeepsBusinessQuestionAfterGreeting(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: policyFirstWorkflowDefinition(),
		UserMessage: models.Message{
			Content: "你好，我想了解售后服务范围。",
		},
		AIAgent: models.AIAgent{
			KnowledgeIDs:    "1",
			FallbackMessage: "我暂时没有找到足够准确的信息。",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "understanding_1", "policy_1", "policy_route_1", "retrieve_end"})
	understandingTrace := findNodeTrace(result.NodeTraces, "understanding_1")
	if understandingTrace == nil || !strings.Contains(understandingTrace.OutputPreview, `"messageIntent":"business_question"`) {
		t.Fatalf("expected mixed greeting to keep the business question, got %#v", understandingTrace)
	}
}

func TestExecutorPolicyFirstWorkflowDoesNotTreatDiagnosticRequestAsConfirmation(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: policyFirstWorkflowDefinition(),
		UserMessage: models.Message{
			Content: "PWR-001 又出现了，上电 12 分钟红灯，48V 母线偶发掉到 45.0V，请先给我知识库内能确认的安全处理建议。",
		},
		AIAgent: models.AIAgent{
			KnowledgeIDs:    "1",
			FallbackMessage: "我暂时没有找到足够准确的信息。",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "understanding_1", "policy_1", "policy_route_1", "retrieve_end"})
	understandingTrace := findNodeTrace(result.NodeTraces, "understanding_1")
	if understandingTrace == nil || !strings.Contains(understandingTrace.OutputPreview, `"messageIntent":"business_question"`) || !strings.Contains(understandingTrace.OutputPreview, `"answerScope":"needs_knowledge"`) {
		t.Fatalf("expected diagnostic request to route to knowledge, got %#v", understandingTrace)
	}
}

func TestExecutorLLMReplyUsesAgentFallbackWhenDeclaredKnowledgeIsEmpty(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: emptyKnowledgeReplyDefinition(),
		UserMessage: models.Message{
			Content: "产品功能",
		},
		AIAgent: models.AIAgent{
			KnowledgeIDs:    "1",
			FallbackMode:    enums.AIAgentFallbackModeNoAnswer,
			FallbackMessage: "我暂时没有找到足够准确的信息。你可以补充更具体的问题，我再继续帮你查。",
			SystemPrompt:    "不要编造事实。",
		},
		AIConfig: models.AIConfig{
			ModelName: "should-not-be-called",
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "我暂时没有找到足够准确的信息。你可以补充更具体的问题，我再继续帮你查。" {
		t.Fatalf("expected fallback reply, got %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "reply_1", "send_1", "end_1"})
}

func TestExecutorLLMReplyAllowsGeneralConversationWithoutKnowledgeHits(t *testing.T) {
	definition := emptyKnowledgeReplyDefinition()
	definition.Nodes[1].Config = json.RawMessage(`{"allowEmptyKnowledge":true,"prompt":"Answer general service questions naturally."}`)
	agentRuntime := &fakeWorkflowAgentRuntime{result: &runtimeexecutor.RunResult{
		Status:    "completed",
		ReplyText: "您好，请问今天需要咨询什么？",
	}}
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   definition,
		UserMessage:  models.Message{Content: "hi"},
		AIAgent:      models.AIAgent{KnowledgeIDs: "1", SystemPrompt: "企业服务助手"},
		AgentRuntime: agentRuntime,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "您好，请问今天需要咨询什么？" {
		t.Fatalf("expected general model reply, got %q", result.ReplyText)
	}
	if agentRuntime.input.UserPrompt != "hi" || !strings.Contains(agentRuntime.input.SystemPrompt, "Answer general service questions naturally.") {
		t.Fatalf("general LLM runtime did not receive expected input: %+v", agentRuntime.input)
	}
}

func TestExecutorLLMReplyRunsThroughEinoAgentRuntime(t *testing.T) {
	toolSet := &registry.ToolSet{}
	agentRuntime := &fakeWorkflowAgentRuntime{result: &runtimeexecutor.RunResult{
		Status:                "completed",
		ReplyText:             "Eino workflow reply",
		SelectedSkillID:       7,
		SelectedSkillName:     "diagnose",
		SkillRouteReason:      "matched diagnosis",
		SkillRouteTrace:       "route:diagnose",
		SkillAllowedToolCodes: []string{"mcp/manual/search"},
		PromptTokens:          13,
		CompletionTokens:      5,
		HistoryMessageCount:   3,
		ToolCallCount:         1,
		ToolCodes:             []string{"mcp/manual/search"},
		InvokedToolCodes:      []string{"mcp/manual/search"},
		TraceData:             `{"status":"completed","tools":{"count":1}}`,
	}}
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   dynamicLLMReplyDefinition(),
		Conversation: models.Conversation{ID: 11},
		UserMessage:  models.Message{ID: 22, Content: "设备无法启动"},
		AIAgent:      models.AIAgent{ID: 33, SystemPrompt: "Base prompt"},
		AIConfig:     models.AIConfig{ModelName: "test-model"},
		ToolSet:      toolSet,
		AgentRuntime: agentRuntime,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText != "Eino workflow reply" {
		t.Fatalf("unexpected reply: %q", result.ReplyText)
	}
	if agentRuntime.input.ToolSet != toolSet {
		t.Fatalf("expected workflow ToolSet to be passed to Eino runtime")
	}
	if !containsWorkflowToolCode(agentRuntime.input.BlockedToolCodes, toolx.GraphCreateTicketConfirm.Code) || !containsWorkflowToolCode(agentRuntime.input.BlockedToolCodes, toolx.GraphHandoffConversation.Code) {
		t.Fatalf("expected workflow side-effect tools to be blocked, got %#v", agentRuntime.input.BlockedToolCodes)
	}
	if agentRuntime.input.UserPrompt != "设备无法启动" {
		t.Fatalf("unexpected user prompt: %q", agentRuntime.input.UserPrompt)
	}
	if !strings.Contains(agentRuntime.input.SystemPrompt, "Base prompt") || !strings.Contains(agentRuntime.input.SystemPrompt, "Diagnose carefully") {
		t.Fatalf("unexpected system prompt: %q", agentRuntime.input.SystemPrompt)
	}
	if result.SelectedSkillID != 7 || result.ToolCallCount != 1 {
		t.Fatalf("expected Eino skill/tool summary, got %#v", result)
	}
	if result.PromptTokens != 13 || result.CompletionTokens != 5 || result.HistoryMessageCount != 3 {
		t.Fatalf("unexpected Eino usage summary: %#v", result)
	}
	if !strings.Contains(result.TraceData, `"agentNodes"`) || !strings.Contains(result.TraceData, `"reply_1"`) {
		t.Fatalf("expected node-scoped Eino trace, got %s", result.TraceData)
	}
	trace := findNodeTrace(result.NodeTraces, "reply_1")
	if trace == nil || !strings.Contains(trace.OutputPreview, `"agentStatus":"completed"`) || !strings.Contains(trace.OutputPreview, `"invokedToolCodes"`) {
		t.Fatalf("expected Eino metadata in workflow node trace, got %#v", trace)
	}
}

func TestExecutorLLMReplyPinsEnglishToOriginalCustomerMessage(t *testing.T) {
	agentRuntime := &fakeWorkflowAgentRuntime{result: &runtimeexecutor.RunResult{
		Status:    "completed",
		ReplyText: "Keep the device powered off until an engineer confirms the cause.",
	}}
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   dynamicLLMReplyDefinition(),
		Conversation: models.Conversation{ID: 41},
		UserMessage: models.Message{
			ID:      42,
			Content: "Can I safely restart the device after fault code RHD-FLOW-ALPHA-7742?",
		},
		AIAgent:      models.AIAgent{ID: 43, SystemPrompt: "你是产品售后服务助手。"},
		AIConfig:     models.AIConfig{ModelName: "test-model"},
		AgentRuntime: agentRuntime,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if result.ReplyText == "" {
		t.Fatal("workflow returned an empty reply")
	}
	for _, required := range []string{"Response language for this turn: English", "Do not switch to Chinese"} {
		if !strings.Contains(agentRuntime.input.SystemPrompt, required) {
			t.Fatalf("runtime prompt is missing %q: %s", required, agentRuntime.input.SystemPrompt)
		}
	}
}

func TestWorkflowReplyPolicyUsesEnglishForEnglishCustomerMessage(t *testing.T) {
	decision := decideWorkflowReplyPolicy(models.AIAgent{}, workflowcapability.Set{}, workflowReplyPolicyInput{
		MessageIntent: "greeting",
		AnswerScope:   "direct_reply",
		UserMessage:   "Hello, can you help me?",
	})
	if decision.ReplyText != "Hello. How can I help you?" {
		t.Fatalf("English greeting reply = %q", decision.ReplyText)
	}

	fallback := decideWorkflowReplyPolicy(models.AIAgent{}, workflowcapability.Set{}, workflowReplyPolicyInput{
		MessageIntent: "business_question",
		AnswerScope:   "needs_knowledge",
		Answerability: "insufficient",
		UserMessage:   "Can I safely restart the device?",
	})
	if fallback.ReplyText == "" || strings.Contains(fallback.ReplyText, "当前知识不足") {
		t.Fatalf("English fallback reply = %q", fallback.ReplyText)
	}
}

func TestWorkflowStaticRecoveryRepliesUseEnglishForEnglishCustomerMessage(t *testing.T) {
	englishQuestion := "Can I safely restart the device after this fault?"
	chineseReplies := []string{
		"已取消创建工单。你可以继续补充问题，我会继续帮你处理。",
		"我已整理工单草稿。请回复“确认”创建工单，或回复“取消”放弃。",
		"暂时无法读取当前可用知识或生成可靠回答。请补充产品名称、设备型号、故障现象和故障码，或稍后再试；当前不会创建工单或转接人工。",
		"我暂时无法完成这次回答。你可以换一种方式描述问题，也可以直接请求人工支持或创建工单；如果问题与设备有关，再补充产品、型号或故障现象即可。",
		"本次暂时无法完成转人工或创建工单。请核对服务码与设备信息后重试；你也可以继续描述问题，我会继续提供 AI 协助。",
		"现有产品知识不足以给出可靠的设备诊断。请补充产品型号、故障码、现场现象和已尝试步骤；当前流程仅提供 AI 服务，不会转接人工或创建工单。",
		"AI 暂时无法可靠完成本次诊断。请保持设备停机和安全隔离，并点击会话中的“转人工”按钮，由技术工程师继续处理；系统不会自动转人工或创建工单。",
	}
	for _, reply := range chineseReplies {
		localized := workflowStaticReplyForMessage(reply, englishQuestion)
		if localized == reply || strings.ContainsAny(localized, "设备工单人工") {
			t.Fatalf("static recovery reply was not localized: %q", localized)
		}
	}
}

func TestExecutorLLMReplyPreservesEinoFailureTrace(t *testing.T) {
	agentRuntime := &fakeWorkflowAgentRuntime{
		result: &runtimeexecutor.RunResult{
			Status:    "error",
			TraceData: `{"status":"error","error":{"stage":"model"}}`,
		},
		err: errors.New("model failed"),
	}
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:   dynamicLLMReplyDefinition(),
		UserMessage:  models.Message{Content: "设备无法启动"},
		AgentRuntime: agentRuntime,
	})
	if err == nil || !strings.Contains(err.Error(), "model failed") {
		t.Fatalf("expected Eino execution error, got %v", err)
	}
	if result == nil || !strings.Contains(result.TraceData, `"agentNodes"`) || !strings.Contains(result.TraceData, `"stage":"model"`) {
		t.Fatalf("expected failed Eino trace to be retained, got %#v", result)
	}
}

func containsWorkflowToolCode(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func TestExecutorHumanConfirmInterruptsWithCheckpoint(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition: humanConfirmWorkflowDefinition(),
		Conversation: models.Conversation{
			ID: 11,
		},
		UserMessage: models.Message{
			ID:      22,
			Content: "创建工单",
		},
		AIAgent: models.AIAgent{
			ID: 33,
		},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !result.Interrupted {
		t.Fatalf("expected workflow to interrupt")
	}
	if result.CheckPointID == "" {
		t.Fatalf("expected checkpoint id")
	}
	if len(result.Interrupts) != 1 {
		t.Fatalf("expected one interrupt, got %#v", result.Interrupts)
	}
	if result.Interrupts[0].Type != "human_confirm" || result.Interrupts[0].ID != "confirm_1" {
		t.Fatalf("unexpected interrupt summary: %#v", result.Interrupts[0])
	}
	if !strings.Contains(result.Interrupts[0].InfoPreview, "请确认创建工单") {
		t.Fatalf("expected confirmation prompt, got %q", result.Interrupts[0].InfoPreview)
	}
	assertPath(t, result.NodePath, []string{"start_1", "prompt_1", "confirm_1"})
}

func TestExecutorDryRunStopsAtConfirmationWhenAutoConfirmIsDisabled(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:  humanConfirmWorkflowDefinition(),
		UserMessage: models.Message{Content: "创建工单"},
		DryRun:      true,
		AutoConfirm: false,
	})
	if err != nil {
		t.Fatalf("execute dry-run workflow: %v", err)
	}
	if !result.Interrupted || result.Status != "interrupted" {
		t.Fatalf("expected dry-run to stop at confirmation, got status=%q interrupted=%v", result.Status, result.Interrupted)
	}
	assertPath(t, result.NodePath, []string{"start_1", "prompt_1", "confirm_1"})
	trace := findNodeTrace(result.NodeTraces, "confirm_1")
	if trace == nil || trace.Status != "interrupted" {
		t.Fatalf("expected interrupted confirmation trace, got %#v", trace)
	}
}

func TestExecutorDryRunAutoConfirmsAndSuppressesBusinessEffects(t *testing.T) {
	result, err := NewExecutor().Execute(context.Background(), Input{
		Definition:  handoffAfterConfirmationWorkflowDefinition(),
		UserMessage: models.Message{Content: "需要人工处理"},
		DryRun:      true,
		AutoConfirm: true,
	})
	if err != nil {
		t.Fatalf("execute dry-run workflow: %v", err)
	}
	if result.Interrupted || result.Status != "completed" {
		t.Fatalf("expected dry-run to complete, got status=%q interrupted=%v", result.Status, result.Interrupted)
	}
	assertPath(t, result.NodePath, []string{"start_1", "prompt_1", "confirm_1", "handoff_1", "end_1"})
	confirmTrace := findNodeTrace(result.NodeTraces, "confirm_1")
	if confirmTrace == nil || !strings.Contains(confirmTrace.OutputPreview, `"simulated":true`) {
		t.Fatalf("expected simulated confirmation trace, got %#v", confirmTrace)
	}
	handoffTrace := findNodeTrace(result.NodeTraces, "handoff_1")
	if handoffTrace == nil || !strings.Contains(handoffTrace.OutputPreview, `"simulated":true`) || !strings.Contains(handoffTrace.OutputPreview, `"created":false`) {
		t.Fatalf("expected suppressed handoff trace, got %#v", handoffTrace)
	}
}

func TestExecutorResumeHumanConfirmContinuesWithConfirmedVariable(t *testing.T) {
	executor := NewExecutor()
	input := Input{
		Definition: humanConfirmWorkflowDefinition(),
		Conversation: models.Conversation{
			ID: 11,
		},
		UserMessage: models.Message{
			ID:      22,
			Content: "创建工单",
		},
		AIAgent: models.AIAgent{
			ID: 33,
		},
	}
	interrupted, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	result, err := executor.Resume(context.Background(), input, interrupted.CheckPointData, "确认")
	if err != nil {
		t.Fatalf("resume workflow: %v", err)
	}
	if result.Interrupted {
		t.Fatalf("expected workflow resume to complete")
	}
	assertPath(t, result.NodePath, []string{"confirm_route_1", "end_1"})
}

func TestExecutorResumeCreatesTicketAfterHumanConfirmation(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	conversation := createWorkflowExecutorHandoffConversation(t, db, aiAgent.ID)
	userMessage := createWorkflowExecutorCustomerMessage(t, db, conversation.ID, "订单支付失败，请帮我登记工单")
	executor := NewExecutor()

	interrupted, err := executor.Execute(context.Background(), Input{
		Definition:   createTicketWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !interrupted.Interrupted {
		t.Fatalf("expected workflow to interrupt before creating ticket")
	}

	result, err := executor.Resume(context.Background(), Input{
		Definition:   createTicketWorkflowDefinition(),
		Conversation: conversation,
		UserMessage:  userMessage,
		AIAgent:      aiAgent,
	}, interrupted.CheckPointData, "确认")
	if err != nil {
		t.Fatalf("resume workflow: %v", err)
	}
	if result.Interrupted {
		t.Fatalf("expected workflow to complete")
	}
	assertPath(t, result.NodePath, []string{"confirm_route_1", "create_ticket_1", "end_1"})

	var ticket models.Ticket
	if err := db.First(&ticket, "conversation_id = ?", conversation.ID).Error; err != nil {
		t.Fatalf("expected created ticket: %v", err)
	}
	if ticket.Title == "" || !strings.Contains(ticket.Description, "订单支付失败") {
		t.Fatalf("unexpected ticket: %+v", ticket)
	}

	trace := findNodeTrace(result.NodeTraces, "create_ticket_1")
	if trace == nil || !strings.Contains(trace.OutputPreview, "工单已创建") {
		t.Fatalf("expected create_ticket output to include customer-visible result message, got %#v", trace)
	}
}

func TestExecutorCreateVideoMeetingNodeUsesAssignedTicket(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	createWorkflowExecutorHandoffAgentProfile(t, db, 101, 1)
	ticket := createWorkflowExecutorTicket(t, db, "TK-MEETING-1", enums.TicketStatusAccepted, 101)
	state := newRunState(Input{
		Conversation: models.Conversation{TenantID: 1},
		AIAgent:      aiAgent,
	})
	state.vars["source_1"] = map[string]any{"ticketId": ticket.ID}
	state.vars["confirm_1"] = map[string]any{"confirmed": true}
	node := dsl.Node{
		ID:   "meeting_1",
		Type: workflowregistry.NodeTypeCreateVideoMeeting,
		Inputs: map[string]dsl.VariableSelector{
			"ticketId":  {NodeID: "source_1", Field: "ticketId"},
			"confirmed": {NodeID: "confirm_1", Field: "confirmed"},
		},
	}

	if err := NewExecutor().executeCreateVideoMeeting(context.Background(), state, node); err != nil {
		t.Fatalf("create video meeting node: %v", err)
	}
	output := state.vars[node.ID]
	if !truthy(output["created"]) || strings.TrimSpace(toString(output["meetingId"])) == "" || !strings.Contains(toString(output["message"]), "视频会议已创建") {
		t.Fatalf("unexpected meeting output: %#v", output)
	}
	var count int64
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("ticket_id = ?", toString(ticket.ID)).Count(&count).Error; err != nil {
		t.Fatalf("count meetings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one meeting, got %d", count)
	}
}

func TestExecutorCreateVideoMeetingNodeSkipsWhenCancelled(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	ticket := createWorkflowExecutorTicket(t, db, "TK-MEETING-CANCEL", enums.TicketStatusAccepted, 101)
	state := newRunState(Input{
		Conversation: models.Conversation{TenantID: 1},
		AIAgent:      aiAgent,
	})
	state.vars["source_1"] = map[string]any{"ticketId": ticket.ID}
	state.vars["confirm_1"] = map[string]any{"confirmed": false}
	node := dsl.Node{
		ID:   "meeting_1",
		Type: workflowregistry.NodeTypeCreateVideoMeeting,
		Inputs: map[string]dsl.VariableSelector{
			"ticketId":  {NodeID: "source_1", Field: "ticketId"},
			"confirmed": {NodeID: "confirm_1", Field: "confirmed"},
		},
	}

	if err := NewExecutor().executeCreateVideoMeeting(context.Background(), state, node); err != nil {
		t.Fatalf("cancel video meeting node: %v", err)
	}
	if truthy(state.vars[node.ID]["created"]) {
		t.Fatalf("cancelled meeting node must not create meeting: %#v", state.vars[node.ID])
	}
	var count int64
	if err := db.Model(&models.MeetingRoomJitsi{}).Count(&count).Error; err != nil {
		t.Fatalf("count meetings: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no meetings, got %d", count)
	}
}

func TestExecutorCreateKnowledgeCandidateNodeCreatesPendingCandidate(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	aiAgent := createWorkflowExecutorHandoffAIAgent(t, db, "1")
	ticket := createWorkflowExecutorTicket(t, db, "TK-KNOWLEDGE-1", enums.TicketStatusResolved, 101)
	state := newRunState(Input{
		Conversation: models.Conversation{TenantID: 1},
		AIAgent:      aiAgent,
	})
	state.vars["source_1"] = map[string]any{
		"ticketId":        ticket.ID,
		"title":           "控制器 E42 复位后仍报警",
		"suggestion":      "检查主电源并复位控制器参数。",
		"rootCause":       "参数漂移",
		"solutionSummary": "重新校准参数并复位。",
	}
	state.vars["confirm_1"] = map[string]any{"confirmed": true}
	node := dsl.Node{
		ID:   "knowledge_1",
		Type: workflowregistry.NodeTypeCreateKnowledgeCandidate,
		Inputs: map[string]dsl.VariableSelector{
			"ticketId":         {NodeID: "source_1", Field: "ticketId"},
			"confirmed":        {NodeID: "confirm_1", Field: "confirmed"},
			"title":            {NodeID: "source_1", Field: "title"},
			"suggestion":       {NodeID: "source_1", Field: "suggestion"},
			"rootCauseSummary": {NodeID: "source_1", Field: "rootCause"},
			"solutionSummary":  {NodeID: "source_1", Field: "solutionSummary"},
		},
	}

	if err := NewExecutor().executeCreateKnowledgeCandidate(state, node); err != nil {
		t.Fatalf("create knowledge candidate node: %v", err)
	}
	output := state.vars[node.ID]
	if !truthy(output["created"]) || toInt64(output["candidateId"]) <= 0 || output["reviewStatus"] != string(enums.KnowledgeCandidateReviewStatusNeedsEnrichment) {
		t.Fatalf("unexpected knowledge candidate output: %#v", output)
	}
	var candidate models.KnowledgeCandidate
	if err := db.First(&candidate, "ticket_id = ?", ticket.ID).Error; err != nil {
		t.Fatalf("expected knowledge candidate: %v", err)
	}
	if candidate.ProductID != ticket.ProductID || candidate.Title != "控制器 E42 复位后仍报警" {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func findNodeTrace(items []NodeTrace, nodeID string) *NodeTrace {
	for i := range items {
		if items[i].NodeID == nodeID {
			return &items[i]
		}
	}
	return nil
}

func emptyKnowledgeReplyDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "reply_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Reply", Inputs: map[string]dsl.VariableSelector{
				"userMessage":    {NodeID: "start_1", Field: "userMessage"},
				"knowledgeItems": {NodeID: "missing_retrieve", Field: "items"},
			}},
			{ID: "send_1", Type: workflowregistry.NodeTypeSendReply, Name: "Send", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "reply_1", Field: "replyText"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_reply", Source: "start_1", Target: "reply_1"},
			{ID: "edge_reply_send", Source: "reply_1", Target: "send_1"},
			{ID: "edge_send_end", Source: "send_1", Target: "end_1"},
		},
	}
}

func dynamicLLMReplyDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "reply_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Reply", Config: []byte(`{"prompt":"Diagnose carefully"}`), Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "send_1", Type: workflowregistry.NodeTypeSendReply, Name: "Send", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "reply_1", Field: "replyText"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_reply", Source: "start_1", Target: "reply_1"},
			{ID: "edge_reply_send", Source: "reply_1", Target: "send_1"},
			{ID: "edge_send_end", Source: "send_1", Target: "end_1"},
		},
	}
}

func policyFirstWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "understanding_1", Type: workflowregistry.NodeTypeConversationUnderstanding, Name: "Understanding", Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "policy_1", Type: workflowregistry.NodeTypeReplyPolicy, Name: "Policy", Inputs: map[string]dsl.VariableSelector{
				"userMessage":    {NodeID: "start_1", Field: "userMessage"},
				"messageIntent":  {NodeID: "understanding_1", Field: "messageIntent"},
				"answerScope":    {NodeID: "understanding_1", Field: "answerScope"},
				"riskSignals":    {NodeID: "understanding_1", Field: "riskSignals"},
				"knowledgeItems": {NodeID: "retrieve_1", Field: "items"},
			}},
			{ID: "policy_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Policy Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "direct", Name: "Direct", TargetNodeID: "send_direct_1", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "policy_1", Field: "action"},
					Operator: "eq",
					Right:    "direct_reply",
				}},
				{ID: "knowledge", Name: "Knowledge", TargetNodeID: "retrieve_end", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "policy_1", Field: "action"},
					Operator: "eq",
					Right:    "retrieve_knowledge",
				}},
				{ID: "default", Name: "Default", TargetNodeID: "end_1", Default: true},
			}})},
			{ID: "send_direct_1", Type: workflowregistry.NodeTypeSendReply, Name: "Send Direct", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "policy_1", Field: "replyText"},
			}},
			{ID: "retrieve_end", Type: workflowregistry.NodeTypeEnd, Name: "Retrieve"},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_understanding", Source: "start_1", Target: "understanding_1"},
			{ID: "edge_understanding_policy", Source: "understanding_1", Target: "policy_1"},
			{ID: "edge_policy_route", Source: "policy_1", Target: "policy_route_1"},
			{ID: "edge_policy_direct", Source: "policy_route_1", Target: "send_direct_1"},
			{ID: "edge_policy_knowledge", Source: "policy_route_1", Target: "retrieve_end"},
			{ID: "edge_policy_default", Source: "policy_route_1", Target: "end_1"},
			{ID: "edge_send_direct_end", Source: "send_direct_1", Target: "end_1"},
		},
	}
}

func conditionalReplyDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "condition_1", Type: workflowregistry.NodeTypeCondition, Name: "Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "vip", Name: "VIP", TargetNodeID: "vip_reply", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "start_1", Field: "userMessage"},
					Operator: "eq",
					Right:    "vip",
				}},
				{ID: "default", Name: "Default", TargetNodeID: "normal_reply", Default: true},
			}})},
			{ID: "vip_reply", Type: workflowregistry.NodeTypeLLMReply, Name: "VIP", Config: []byte(`{"staticReply":"VIP reply"}`)},
			{ID: "normal_reply", Type: workflowregistry.NodeTypeLLMReply, Name: "Normal", Config: []byte(`{"staticReply":"Normal reply"}`)},
			{ID: "send_vip", Type: workflowregistry.NodeTypeSendReply, Name: "Send VIP", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "vip_reply", Field: "replyText"},
			}},
			{ID: "send_normal", Type: workflowregistry.NodeTypeSendReply, Name: "Send Normal", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "normal_reply", Field: "replyText"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_condition", Source: "start_1", Target: "condition_1"},
			{ID: "edge_condition_vip", Source: "condition_1", Target: "vip_reply"},
			{ID: "edge_condition_default", Source: "condition_1", Target: "normal_reply"},
			{ID: "edge_vip_send", Source: "vip_reply", Target: "send_vip"},
			{ID: "edge_normal_send", Source: "normal_reply", Target: "send_normal"},
			{ID: "edge_send_vip_end", Source: "send_vip", Target: "end_1"},
			{ID: "edge_send_normal_end", Source: "send_normal", Target: "end_1"},
		},
	}
}

func createTicketWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "draft_1", Type: workflowregistry.NodeTypePrepareTicketDraft, Name: "Draft", Inputs: map[string]dsl.VariableSelector{
				"issue": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "prompt_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Prompt", Config: []byte(`{"staticReply":"请确认创建工单"}`)},
			{ID: "confirm_1", Type: workflowregistry.NodeTypeHumanConfirm, Name: "Confirm", Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "prompt_1", Field: "replyText"},
			}},
			{ID: "confirm_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Confirm Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "yes", Name: "Yes", TargetNodeID: "create_ticket_1", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "confirm_1", Field: "confirmed"},
					Operator: "is_true",
				}},
				{ID: "default", Name: "Cancel", TargetNodeID: "cancel_end", Default: true},
			}})},
			{ID: "create_ticket_1", Type: workflowregistry.NodeTypeCreateTicket, Name: "Create Ticket", Inputs: map[string]dsl.VariableSelector{
				"ticketDraft": {NodeID: "draft_1", Field: "ticketDraft"},
				"confirmed":   {NodeID: "confirm_1", Field: "confirmed"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
			{ID: "cancel_end", Type: workflowregistry.NodeTypeEnd, Name: "Cancel"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_draft", Source: "start_1", Target: "draft_1"},
			{ID: "edge_draft_prompt", Source: "draft_1", Target: "prompt_1"},
			{ID: "edge_prompt_confirm", Source: "prompt_1", Target: "confirm_1"},
			{ID: "edge_confirm_route", Source: "confirm_1", Target: "confirm_route_1"},
			{ID: "edge_confirm_create", Source: "confirm_route_1", Target: "create_ticket_1"},
			{ID: "edge_confirm_cancel", Source: "confirm_route_1", Target: "cancel_end"},
			{ID: "edge_create_end", Source: "create_ticket_1", Target: "end_1"},
		},
	}
}

func humanConfirmWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "prompt_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Prompt", Config: []byte(`{"staticReply":"请确认创建工单"}`)},
			{ID: "confirm_1", Type: workflowregistry.NodeTypeHumanConfirm, Name: "Confirm", Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "prompt_1", Field: "replyText"},
			}},
			{ID: "confirm_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Confirm Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "yes", Name: "Yes", TargetNodeID: "end_1", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "confirm_1", Field: "confirmed"},
					Operator: "is_true",
				}},
				{ID: "default", Name: "Cancel", TargetNodeID: "cancel_end", Default: true},
			}})},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
			{ID: "cancel_end", Type: workflowregistry.NodeTypeEnd, Name: "Cancel"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_prompt", Source: "start_1", Target: "prompt_1"},
			{ID: "edge_prompt_confirm", Source: "prompt_1", Target: "confirm_1"},
			{ID: "edge_confirm_route", Source: "confirm_1", Target: "confirm_route_1"},
			{ID: "edge_confirm_yes", Source: "confirm_route_1", Target: "end_1"},
			{ID: "edge_confirm_cancel", Source: "confirm_route_1", Target: "cancel_end"},
		},
	}
}

func prepareTicketDraftWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "draft_1", Type: workflowregistry.NodeTypePrepareTicketDraft, Name: "Draft", Inputs: map[string]dsl.VariableSelector{
				"issue": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "draft_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Draft Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "ready", Name: "Ready", TargetNodeID: "ready_end", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "draft_1", Field: "ticketDraft"},
					Operator: "exists",
				}},
				{ID: "default", Name: "Default", TargetNodeID: "default_end", Default: true},
			}})},
			{ID: "ready_end", Type: workflowregistry.NodeTypeEnd, Name: "Ready"},
			{ID: "default_end", Type: workflowregistry.NodeTypeEnd, Name: "Default"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_draft", Source: "start_1", Target: "draft_1"},
			{ID: "edge_draft_route", Source: "draft_1", Target: "draft_route_1"},
			{ID: "edge_draft_ready", Source: "draft_route_1", Target: "ready_end"},
			{ID: "edge_draft_default", Source: "draft_route_1", Target: "default_end"},
		},
	}
}

func analyzeConversationWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "analyze_1", Type: workflowregistry.NodeTypeAnalyzeConversation, Name: "Analyze", Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "analyze_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Analyze Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "handoff", Name: "Handoff", TargetNodeID: "handoff_end", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "analyze_1", Field: "needHumanHandoff"},
					Operator: "is_true",
				}},
				{ID: "default", Name: "Default", TargetNodeID: "default_end", Default: true},
			}})},
			{ID: "handoff_end", Type: workflowregistry.NodeTypeEnd, Name: "Handoff"},
			{ID: "default_end", Type: workflowregistry.NodeTypeEnd, Name: "Default"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_analyze", Source: "start_1", Target: "analyze_1"},
			{ID: "edge_analyze_route", Source: "analyze_1", Target: "analyze_route_1"},
			{ID: "edge_analyze_handoff", Source: "analyze_route_1", Target: "handoff_end"},
			{ID: "edge_analyze_default", Source: "analyze_route_1", Target: "default_end"},
		},
	}
}

func handoffWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "handoff_1", Type: workflowregistry.NodeTypeHandoffToHuman, Name: "Handoff", Inputs: map[string]dsl.VariableSelector{
				"reason": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "handoff_route_1", Type: workflowregistry.NodeTypeCondition, Name: "Handoff Route", Config: mustMarshalWorkflowTestConfig(dsl.ConditionConfig{Branches: []dsl.ConditionBranch{
				{ID: "assigned", Name: "Assigned", TargetNodeID: "assigned_end", Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "handoff_1", Field: "decision"},
					Operator: "eq",
					Right:    string(services.HandoffDecisionAssigned),
				}},
				{ID: "default", Name: "Default", TargetNodeID: "default_end", Default: true},
			}})},
			{ID: "assigned_end", Type: workflowregistry.NodeTypeEnd, Name: "Assigned"},
			{ID: "default_end", Type: workflowregistry.NodeTypeEnd, Name: "Default"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_handoff", Source: "start_1", Target: "handoff_1"},
			{ID: "edge_handoff_route", Source: "handoff_1", Target: "handoff_route_1"},
			{ID: "edge_handoff_assigned", Source: "handoff_route_1", Target: "assigned_end"},
			{ID: "edge_handoff_default", Source: "handoff_route_1", Target: "default_end"},
		},
	}
}

func handoffAfterConfirmationWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "prompt_1", Type: workflowregistry.NodeTypeLLMReply, Name: "Prompt", Config: []byte(`{"staticReply":"请确认转人工"}`)},
			{ID: "confirm_1", Type: workflowregistry.NodeTypeHumanConfirm, Name: "Confirm", Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "prompt_1", Field: "replyText"},
			}},
			{ID: "handoff_1", Type: workflowregistry.NodeTypeHandoffToHuman, Name: "Handoff", Inputs: map[string]dsl.VariableSelector{
				"reason":    {NodeID: "start_1", Field: "userMessage"},
				"confirmed": {NodeID: "confirm_1", Field: "confirmed"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd, Name: "End"},
		},
		Edges: []dsl.Edge{
			{ID: "edge_start_prompt", Source: "start_1", Target: "prompt_1"},
			{ID: "edge_prompt_confirm", Source: "prompt_1", Target: "confirm_1"},
			{ID: "edge_confirm_handoff", Source: "confirm_1", Target: "handoff_1"},
			{ID: "edge_handoff_end", Source: "handoff_1", Target: "end_1"},
		},
	}
}

func setupWorkflowExecutorHandoffDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Product{},
		&models.AIAgent{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgentRelease{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentTeamSchedule{},
		&models.AgentTeamScheduleTemplate{},
		&models.AgentScheduleException{},
		&models.AgentProfile{},
		&models.AgentWorkStatus{},
		&models.Channel{},
		&models.Conversation{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
		&models.ChannelMessageOutbox{},
		&models.Ticket{},
		&models.TicketDispatchAttempt{},
		&models.TicketNoSequence{},
		&models.TicketTag{},
		&models.TicketProgress{},
		&models.TicketContextSnapshot{},
		&models.TicketRepairRecord{},
		&models.KnowledgeCandidate{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	if err := db.Create(&models.Tenant{ID: 1, Name: "测试租户", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	if err := db.Create(&models.Product{ID: 11, TenantID: 1, Code: "TEST-PRODUCT", Name: "测试产品", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createWorkflowExecutorHandoffAIAgent(t *testing.T, db *gorm.DB, teamIDs string) models.AIAgent {
	t.Helper()
	item := models.AIAgent{
		TenantID:    1,
		ProductID:   11,
		Name:        "测试AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool,
		TeamIDs:     teamIDs,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}
	workflow, version, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db)
	if err != nil {
		t.Fatalf("ensure handoff workflow error = %v", err)
	}
	release := models.AIAgentRelease{
		TenantID:          item.TenantID,
		ProductID:         item.ProductID,
		AgentID:           item.ID,
		ReleaseNo:         1,
		WorkflowID:        workflow.ID,
		WorkflowVersionID: version.ID,
		ReviewStatus:      enums.AIAgentReviewStatusApproved,
		DeploymentStatus:  models.AIAgentReleaseDeploymentActive,
		Status:            enums.StatusOk,
	}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("create active handoff release error = %v", err)
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", item.ID).Updates(map[string]any{
		"workflow_id":         workflow.ID,
		"workflow_version_id": version.ID,
		"active_release_id":   release.ID,
	}).Error; err != nil {
		t.Fatalf("bind active handoff release error = %v", err)
	}
	item.WorkflowID = workflow.ID
	item.WorkflowVersionID = version.ID
	item.ActiveReleaseID = release.ID
	return item
}

func createWorkflowExecutorTicket(t *testing.T, db *gorm.DB, ticketNo string, status enums.TicketStatus, assigneeID int64) models.Ticket {
	t.Helper()
	item := models.Ticket{
		TicketNo:          ticketNo,
		Title:             "控制器报警",
		Description:       "控制器 E42 报警",
		Status:            status,
		CurrentAssigneeID: assigneeID,
		TenantID:          1,
		ProductID:         11,
		ProductModelID:    12,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}
	return item
}

func createWorkflowExecutorHandoffTeam(t *testing.T, db *gorm.DB, id int64, name string) {
	t.Helper()
	if err := db.Create(&models.AgentTeam{ID: id, TenantID: 1, ProductID: 11, TeamType: services.AgentTeamTypeProductRepair, Name: name, Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team error = %v", err)
	}
}

func createWorkflowExecutorHandoffActiveSchedule(t *testing.T, db *gorm.DB, teamID int64) {
	t.Helper()
	now := time.Now()
	local := now.In(time.FixedZone("CST", 8*60*60))
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    1,
		Workdays:    fmt.Sprintf("[%d]", weekday),
		StartMinute: max(0, local.Hour()*60+local.Minute()-1),
		EndMinute:   min(24*60, local.Hour()*60+local.Minute()+30),
		Timezone:    services.EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template error = %v", err)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", teamID).Update("schedule_enforced", true).Error; err != nil {
		t.Fatalf("enable schedule enforcement error = %v", err)
	}
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1,
		TeamID:   teamID,
		StartAt:  now.Add(-time.Hour),
		EndAt:    now.Add(time.Hour),
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create schedule error = %v", err)
	}
}

func createWorkflowExecutorHandoffAgentProfile(t *testing.T, db *gorm.DB, userID int64, teamID int64) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       userID,
		Username: "agent",
		Nickname: "客服",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user error = %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           1,
		UserID:             userID,
		TeamID:             teamID,
		AgentCode:          "A001",
		DisplayName:        "客服",
		ServiceStatus:      enums.ServiceStatusIdle,
		MaxConcurrentCount: 3,
		AutoAssignEnabled:  true,
		LastOnlineAt:       &now,
		Status:             enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile error = %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        1,
		TeamID:          teamID,
		UserID:          userID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team member error = %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID:        1,
		UserID:          userID,
		Status:          services.AgentWorkStatusAvailable,
		ConfirmedAt:     now,
		StatusChangedAt: now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create work status error = %v", err)
	}
}

func createWorkflowExecutorHandoffConversation(t *testing.T, db *gorm.DB, aiAgentID int64) models.Conversation {
	t.Helper()
	now := time.Now()
	if err := db.FirstOrCreate(&models.Channel{
		ID:          1,
		Name:        "测试网页渠道",
		ChannelType: "web",
		ChannelID:   "workflow-test-web",
		AIAgentID:   aiAgentID,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create channel error = %v", err)
	}
	if err := db.FirstOrCreate(&models.Customer{
		ID:     1,
		Name:   "测试访客",
		Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer error = %v", err)
	}
	item := models.Conversation{
		TenantID:      1,
		ProductID:     11,
		AIAgentID:     aiAgentID,
		ChannelID:     1,
		CustomerID:    1,
		CustomerName:  "测试访客",
		Status:        enums.IMConversationStatusAIServing,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now,
		LastActiveAt:  now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	return item
}

func createWorkflowExecutorCustomerMessage(t *testing.T, db *gorm.DB, conversationID int64, content string) models.Message {
	t.Helper()
	now := time.Now()
	item := models.Message{
		ConversationID: conversationID,
		ClientMsgID:    "customer-message",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        content,
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create message error = %v", err)
	}
	return item
}

func assertPath(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected path length: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected path: got %#v want %#v", got, want)
		}
	}
}
