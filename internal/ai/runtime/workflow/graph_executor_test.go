package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	runtimeexecutor "remotehelpdesk/internal/ai/runtime/executor"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
)

func TestGraphExecutorRoutesConditionWithExistingDSL(t *testing.T) {
	result, err := NewGraphExecutor().Execute(context.Background(), Input{
		Definition:  conditionalReplyDefinition(),
		UserMessage: models.Message{Content: "vip"},
	})
	if err != nil {
		t.Fatalf("execute Eino Graph workflow: %v", err)
	}
	if result.ReplyText != "VIP reply" {
		t.Fatalf("unexpected reply: %q", result.ReplyText)
	}
	assertPath(t, result.NodePath, []string{"start_1", "condition_1", "vip_reply", "send_vip", "end_1"})
	trace := findNodeTrace(result.NodeTraces, "condition_1")
	if trace == nil || !containsAll(trace.OutputPreview, `"selectedBranchId":"vip"`, `"selectedTargetNodeId":"vip_reply"`) {
		t.Fatalf("condition branch decision is missing from trace: %#v", trace)
	}
}

func TestGraphExecutorRoutesFailureWithoutRunningNormalPath(t *testing.T) {
	result, err := NewEngine().Execute(context.Background(), Input{
		Definition:   graphFailureFallbackDefinition(),
		UserMessage:  models.Message{Content: "设备无法启动"},
		AgentRuntime: &fakeWorkflowAgentRuntime{err: errors.New("model timeout")},
	})
	if err != nil {
		t.Fatalf("execute Eino Graph workflow with failure fallback: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "reply_1", "fallback_1", "fallback_send_1", "end_1"})
	if result.ReplyText != "服务暂时不可用，已进入兜底流程。" {
		t.Fatalf("unexpected fallback reply: %q", result.ReplyText)
	}
	if findNodeTrace(result.NodeTraces, "send_1") != nil {
		t.Fatalf("normal success path ran after model failure: %#v", result.NodePath)
	}
	trace := findNodeTrace(result.NodeTraces, "reply_1")
	if trace == nil || trace.Status != "recovered" || !strings.Contains(trace.ErrorMessage, "model timeout") {
		t.Fatalf("expected recovered Eino node trace, got %#v", trace)
	}
}

func TestGraphExecutorKeepsFailureTargetOffNormalPath(t *testing.T) {
	result, err := NewEngine().Execute(context.Background(), Input{
		Definition:  graphFailureFallbackDefinition(),
		UserMessage: models.Message{Content: "设备如何启动"},
		AgentRuntime: &fakeWorkflowAgentRuntime{result: &runtimeexecutor.RunResult{
			ReplyText: "正常回答",
		}},
	})
	if err != nil {
		t.Fatalf("execute normal Eino Graph workflow: %v", err)
	}
	assertPath(t, result.NodePath, []string{"start_1", "reply_1", "send_1", "end_1"})
	if result.ReplyText != "正常回答" || findNodeTrace(result.NodeTraces, "fallback_1") != nil {
		t.Fatalf("failure fallback ran on normal path: result=%q path=%#v", result.ReplyText, result.NodePath)
	}
}

func graphFailureFallbackDefinition() dsl.Definition {
	return dsl.Definition{
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
}

func TestGraphExecutorRunsFanOutNodesConcurrentlyAndJoins(t *testing.T) {
	runtime := &concurrentWorkflowAgentRuntime{delay: 120 * time.Millisecond}
	startedAt := time.Now()
	result, err := NewGraphExecutor().Execute(context.Background(), Input{
		Definition:   parallelReplyDefinition(),
		UserMessage:  models.Message{Content: "diagnose"},
		AgentRuntime: runtime,
	})
	if err != nil {
		t.Fatalf("execute parallel Eino Graph workflow: %v", err)
	}
	if runtime.maxConcurrent.Load() < 2 {
		t.Fatalf("expected Eino Graph fan-out concurrency, max=%d", runtime.maxConcurrent.Load())
	}
	if elapsed := time.Since(startedAt); elapsed >= 220*time.Millisecond {
		t.Fatalf("parallel branches took too long: %s", elapsed)
	}
	for _, nodeID := range []string{"start_1", "diagnose_a", "diagnose_b", "end_1"} {
		if findNodeTrace(result.NodeTraces, nodeID) == nil {
			t.Fatalf("expected node %s in graph trace: %#v", nodeID, result.NodePath)
		}
	}
}

func TestGraphExecutorPreservesBusinessCheckpointResume(t *testing.T) {
	executor := NewGraphExecutor()
	input := Input{
		Definition:   humanConfirmWorkflowDefinition(),
		Conversation: models.Conversation{ID: 11},
		UserMessage:  models.Message{ID: 22, Content: "创建工单"},
		AIAgent:      models.AIAgent{ID: 33},
	}
	interrupted, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !interrupted.Interrupted || interrupted.CheckPointData == "" {
		t.Fatalf("expected graph checkpoint interruption: %#v", interrupted)
	}
	resumed, err := executor.Resume(context.Background(), input, interrupted.CheckPointData, "确认")
	if err != nil {
		t.Fatalf("resume graph workflow: %v", err)
	}
	if resumed.Interrupted || resumed.Status != "completed" {
		t.Fatalf("expected completed resumed graph: %#v", resumed)
	}
	assertPath(t, resumed.NodePath, []string{"confirm_route_1", "end_1"})
}

func TestGraphExecutorRunsFixedVersionSubflow(t *testing.T) {
	resolver := &fakeWorkflowVersionResolver{definitions: map[int64]dsl.Definition{
		2: staticReplySubflowDefinition("subflow reply"),
	}}
	result, err := NewGraphExecutor().Execute(context.Background(), Input{
		Definition:           parentSubflowDefinition(workflowregistry.NodeTypeSubflow, map[string]any{"workflowVersionId": 2}),
		UserMessage:          models.Message{Content: "diagnose"},
		WorkflowResolver:     resolver,
		WorkflowVersionStack: []int64{1},
	})
	if err != nil {
		t.Fatalf("execute subflow: %v", err)
	}
	if result.ReplyText != "subflow reply" || resolver.calls.Load() != 1 {
		t.Fatalf("unexpected subflow result=%#v calls=%d", result, resolver.calls.Load())
	}
}

func TestGraphExecutorRunsBoundedTroubleshootingLoop(t *testing.T) {
	resolver := &fakeWorkflowVersionResolver{definitions: map[int64]dsl.Definition{
		2: staticReplySubflowDefinition("retry result"),
	}}
	result, err := NewGraphExecutor().Execute(context.Background(), Input{
		Definition: parentSubflowDefinition(workflowregistry.NodeTypeLoop, map[string]any{
			"workflowVersionId": 2,
			"maxIterations":     4,
			"until": map[string]any{
				"left":     map[string]any{"nodeId": "nested_1", "field": "iteration"},
				"operator": "gte",
				"right":    2,
			},
		}),
		UserMessage:          models.Message{Content: "diagnose"},
		WorkflowResolver:     resolver,
		WorkflowVersionStack: []int64{1},
	})
	if err != nil {
		t.Fatalf("execute troubleshooting loop: %v", err)
	}
	if result.ReplyText != "retry result" || resolver.calls.Load() != 2 {
		t.Fatalf("unexpected loop result=%#v calls=%d", result, resolver.calls.Load())
	}
	trace := findNodeTrace(result.NodeTraces, "nested_1")
	if trace == nil || !containsAll(trace.OutputPreview, `"iteration":2`, `"completed":true`) {
		t.Fatalf("expected bounded loop output, got %#v", trace)
	}
}

func TestKnowledgeMergeDeduplicatesParallelRetrievalItems(t *testing.T) {
	state := newRunState(Input{})
	state.vars["kb_a"] = map[string]any{"items": []map[string]any{
		{"knowledgeBaseId": int64(1), "documentId": int64(10), "chunkId": int64(100), "content": "A"},
	}}
	state.vars["kb_b"] = map[string]any{"items": []map[string]any{
		{"knowledgeBaseId": int64(1), "documentId": int64(10), "chunkId": int64(100), "content": "A"},
		{"knowledgeBaseId": int64(2), "documentId": int64(20), "chunkId": int64(200), "content": "B"},
	}}
	node := dsl.Node{
		ID:   "merge_1",
		Type: workflowregistry.NodeTypeKnowledgeMerge,
		Config: mustMarshalWorkflowTestConfig(map[string]any{"sources": []map[string]any{
			{"nodeId": "kb_a", "field": "items"},
			{"nodeId": "kb_b", "field": "items"},
		}}),
	}
	if err := NewExecutor().executeKnowledgeMerge(state, node); err != nil {
		t.Fatalf("merge knowledge: %v", err)
	}
	items := toWorkflowObjectItems(state.vars[node.ID]["items"])
	if len(items) != 2 || state.vars[node.ID]["summary"] != "A\n\nB" {
		t.Fatalf("unexpected merged knowledge: %#v", state.vars[node.ID])
	}
}

func TestWorkflowEffectKeyIncludesSubflowCallPath(t *testing.T) {
	input := Input{
		Conversation:         models.Conversation{TenantID: 1, ID: 2},
		UserMessage:          models.Message{ID: 3},
		WorkflowVersionStack: []int64{10, 20},
		WorkflowNodeStack:    []string{"subflow_a"},
	}
	node := dsl.Node{ID: "create_1", Type: workflowregistry.NodeTypeCreateTicket}
	first, ok := buildWorkflowEffectRequest(input, node, "{}")
	if !ok {
		t.Fatal("expected create_ticket effect request")
	}
	input.WorkflowNodeStack = []string{"subflow_b"}
	second, _ := buildWorkflowEffectRequest(input, node, "{}")
	if first.IdempotencyKey == second.IdempotencyKey {
		t.Fatalf("different subflow call sites shared effect key %q", first.IdempotencyKey)
	}
}

func TestWorkflowEffectRequestCoversAfterSalesSideEffects(t *testing.T) {
	input := Input{
		Conversation:         models.Conversation{TenantID: 1, ProductID: 2, ID: 3},
		UserMessage:          models.Message{ID: 4},
		AIAgent:              models.AIAgent{ActiveReleaseID: 5},
		WorkflowVersionStack: []int64{6},
	}
	for _, tt := range []struct {
		nodeType   string
		effectType string
	}{
		{nodeType: workflowregistry.NodeTypeCreateTicket, effectType: workflowregistry.NodeTypeCreateTicket},
		{nodeType: workflowregistry.NodeTypeHandoffToHuman, effectType: workflowregistry.NodeTypeHandoffToHuman},
		{nodeType: workflowregistry.NodeTypeCreateVideoMeeting, effectType: workflowregistry.NodeTypeCreateVideoMeeting},
		{nodeType: workflowregistry.NodeTypeCreateKnowledgeCandidate, effectType: workflowregistry.NodeTypeCreateKnowledgeCandidate},
	} {
		request, ok := buildWorkflowEffectRequest(input, dsl.Node{ID: tt.nodeType + "_1", Type: tt.nodeType}, "{}")
		if !ok {
			t.Fatalf("expected %s to be tracked as workflow effect", tt.nodeType)
		}
		if request.EffectType != tt.effectType || request.IdempotencyKey == "" {
			t.Fatalf("unexpected effect request for %s: %#v", tt.nodeType, request)
		}
	}
}

type concurrentWorkflowAgentRuntime struct {
	delay         time.Duration
	current       atomic.Int32
	maxConcurrent atomic.Int32
}

type fakeWorkflowVersionResolver struct {
	definitions map[int64]dsl.Definition
	calls       atomic.Int32
}

func (r *fakeWorkflowVersionResolver) ResolveWorkflowVersion(_ context.Context, ref WorkflowVersionReference) (ResolvedWorkflowVersion, error) {
	r.calls.Add(1)
	definition, ok := r.definitions[ref.WorkflowVersionID]
	if !ok {
		return ResolvedWorkflowVersion{}, errors.New("workflow version not found")
	}
	return ResolvedWorkflowVersion{
		WorkflowID:     ref.WorkflowVersionID,
		VersionID:      ref.WorkflowVersionID,
		DefinitionHash: "hash",
		Definition:     definition,
	}, nil
}

func (r *concurrentWorkflowAgentRuntime) ExecuteWorkflowNode(_ context.Context, _ runtimeexecutor.WorkflowNodeInput) (*runtimeexecutor.RunResult, error) {
	current := r.current.Add(1)
	for {
		maximum := r.maxConcurrent.Load()
		if current <= maximum || r.maxConcurrent.CompareAndSwap(maximum, current) {
			break
		}
	}
	time.Sleep(r.delay)
	r.current.Add(-1)
	return &runtimeexecutor.RunResult{Status: "completed", ReplyText: "diagnosed"}, nil
}

func parallelReplyDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart},
			{
				ID:   "diagnose_a",
				Type: workflowregistry.NodeTypeLLMReply,
				Inputs: map[string]dsl.VariableSelector{
					"userMessage": {NodeID: "start_1", Field: "userMessage"},
				},
			},
			{
				ID:   "diagnose_b",
				Type: workflowregistry.NodeTypeLLMReply,
				Inputs: map[string]dsl.VariableSelector{
					"userMessage": {NodeID: "start_1", Field: "userMessage"},
				},
			},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd},
		},
		Edges: []dsl.Edge{
			{ID: "start_a", Source: "start_1", Target: "diagnose_a"},
			{ID: "start_b", Source: "start_1", Target: "diagnose_b"},
			{ID: "a_end", Source: "diagnose_a", Target: "end_1"},
			{ID: "b_end", Source: "diagnose_b", Target: "end_1"},
		},
	}
}

func staticReplySubflowDefinition(reply string) dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "child_start",
		Nodes: []dsl.Node{
			{ID: "child_start", Type: workflowregistry.NodeTypeStart},
			{ID: "child_reply", Type: workflowregistry.NodeTypeLLMReply, Config: mustMarshalWorkflowTestConfig(map[string]any{"staticReply": reply})},
			{ID: "child_send", Type: workflowregistry.NodeTypeSendReply, Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "child_reply", Field: "replyText"},
			}},
			{ID: "child_end", Type: workflowregistry.NodeTypeEnd},
		},
		Edges: []dsl.Edge{
			{ID: "child_1", Source: "child_start", Target: "child_reply"},
			{ID: "child_2", Source: "child_reply", Target: "child_send"},
			{ID: "child_3", Source: "child_send", Target: "child_end"},
		},
	}
}

func parentSubflowDefinition(nodeType string, config map[string]any) dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: workflowregistry.NodeTypeStart},
			{ID: "nested_1", Type: nodeType, Config: mustMarshalWorkflowTestConfig(config)},
			{ID: "send_1", Type: workflowregistry.NodeTypeSendReply, Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "nested_1", Field: "replyText"},
			}},
			{ID: "end_1", Type: workflowregistry.NodeTypeEnd},
		},
		Edges: []dsl.Edge{
			{ID: "parent_1", Source: "start_1", Target: "nested_1"},
			{ID: "parent_2", Source: "nested_1", Target: "send_1"},
			{ID: "parent_3", Source: "send_1", Target: "end_1"},
		},
	}
}

func containsAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
