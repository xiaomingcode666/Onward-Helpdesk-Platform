package workflow

import (
	"context"
	runtimeexecutor "remotehelpdesk/internal/ai/runtime/executor"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/aistream"
)

func TestReplyStreamContextOnlyExposesDirectPublicReply(t *testing.T) {
	var chunks []string
	ctx := aistream.WithHandler(context.Background(), func(text string) { chunks = append(chunks, text) })
	input := Input{Definition: dsl.Definition{Nodes: []dsl.Node{
		{ID: "answer", Type: workflowregistry.NodeTypeLLMReply},
		{ID: "send", Type: workflowregistry.NodeTypeSendReply, Inputs: map[string]dsl.VariableSelector{"replyText": {NodeID: "answer", Field: "replyText"}}},
	}, Edges: []dsl.Edge{{Source: "answer", Target: "send"}}}}
	state := newRunState(input)
	aistream.Emit(workflowReplyStreamContext(ctx, state, "internal"), "private intermediate result")
	if len(chunks) != 0 {
		t.Fatal("internal result was exposed")
	}
	aistream.Emit(workflowReplyStreamContext(ctx, state, "answer"), "public answer")
	if len(chunks) != 1 || chunks[0] != "public answer" {
		t.Fatalf("public answer not streamed: %#v", chunks)
	}
	state.input.DryRun = true
	aistream.Emit(workflowReplyStreamContext(ctx, state, "answer"), "preview only")
	if len(chunks) != 1 {
		t.Fatal("dry run exposed a public stream")
	}
}

type streamingWorkflowRuntime struct {
	release <-chan struct{}
}

func (r *streamingWorkflowRuntime) ExecuteWorkflowNode(ctx context.Context, _ runtimeexecutor.WorkflowNodeInput) (*runtimeexecutor.RunResult, error) {
	aistream.Emit(ctx, "Hello")
	select {
	case <-r.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	aistream.Emit(ctx, "Hello world")
	return &runtimeexecutor.RunResult{Status: "completed", ReplyText: "Hello world"}, nil
}

func TestGraphWorkflowForwardsPublicSnapshotsBeforeCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	release := make(chan struct{})
	observed := make(chan string, 4)
	ctx = aistream.WithHandler(ctx, func(text string) { observed <- text })
	type outcome struct {
		result *Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := NewEngine().Execute(ctx, Input{Definition: dynamicLLMReplyDefinition(), UserMessage: models.Message{Content: "Question"}, AgentRuntime: &streamingWorkflowRuntime{release: release}})
		done <- outcome{result, err}
	}()
	select {
	case text := <-observed:
		if text != "Hello" {
			t.Fatalf("first snapshot = %q", text)
		}
	case result := <-done:
		t.Fatalf("workflow finished before streaming: %+v, %v", result.result, result.err)
	case <-ctx.Done():
		t.Fatal("workflow buffered public snapshots")
	}
	close(release)
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.result.ReplyText != "Hello world" {
			t.Fatalf("final reply lost: %+v", result.result)
		}
	case <-ctx.Done():
		t.Fatal("workflow did not finish")
	}
}
