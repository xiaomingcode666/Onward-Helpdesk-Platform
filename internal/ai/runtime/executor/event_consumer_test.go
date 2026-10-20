package executor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/ai/runtime/internal/impl/factory"
	"remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/pkg/aistream"
	"remotehelpdesk/internal/pkg/toolx"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestConsumeAgentEventsStreamsBeforeModelCompletes(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{IsStreaming: true, Role: schema.Assistant, MessageStream: reader}}})
	gen.Close()
	observed := make(chan string, 4)
	ctx := aistream.WithHandler(context.Background(), func(text string) { observed <- text })
	summary := &RunResult{Status: "started"}
	done := make(chan struct{})
	go func() {
		consumeAgentEvents(events, summary, nil, nil, func(text string) { aistream.Emit(ctx, text) })
		close(done)
	}()
	writer.Send(schema.AssistantMessage("Hello", nil), nil)
	select {
	case got := <-observed:
		if got != "Hello" {
			t.Fatalf("first chunk=%q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first chunk was buffered until completion")
	}
	writer.Send(schema.AssistantMessage(" world", nil), nil)
	writer.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not finish")
	}
	if summary.ReplyText != "Hello world" || summary.Status != "completed" {
		t.Fatalf("lost final reply: %+v", summary)
	}
}

func TestConsumeMessageStreamPreservesUsageAndHidesReasoningAndTools(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](8)
	writer.Send(&schema.Message{Role: schema.Assistant, ReasoningContent: "private reasoning"}, nil)
	writer.Send(schema.AssistantMessage("Public", nil), nil)
	writer.Send(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "tool-1", Type: "function", Function: schema.FunctionCall{Name: "search", Arguments: `{}`}}}}, nil)
	writer.Send(schema.AssistantMessage("private tool preparation", nil), nil)
	writer.Send(&schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 12, CompletionTokens: 4, TotalTokens: 16}}}, nil)
	writer.Close()
	var snapshots []string
	message, err := consumeMessageStream(reader, func(text string) { snapshots = append(snapshots, text) })
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 || snapshots[0] != "Public" || snapshots[1] != "" {
		t.Fatalf("private output leaked: %#v", snapshots)
	}
	if message.ResponseMeta.Usage.TotalTokens != 16 || len(message.ToolCalls) != 1 {
		t.Fatalf("stream metadata lost: %+v", message)
	}
}

func TestConsumeAgentEventsStreamErrorDoesNotCompletePartialReply(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](4)
	writer.Send(schema.AssistantMessage("Partial", nil), nil)
	writer.Send(nil, errors.New("model stream failed"))
	writer.Close()
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{IsStreaming: true, Role: schema.Assistant, MessageStream: reader}}})
	gen.Close()
	summary := &RunResult{Status: "started"}
	consumeAgentEvents(events, summary, nil, nil)
	if summary.Status != "error" || summary.ReplyText != "" || summary.ErrorMessage != "model stream failed" {
		t.Fatalf("partial reply completed: %+v", summary)
	}
}

func TestConsumeAgentEventsIgnoresPlainGraphToolText(t *testing.T) {
	summary := &RunResult{
		Status:           "started",
		InvokedToolCodes: make([]string, 0),
	}
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role:     schema.Tool,
				ToolName: toolx.GraphHandoffConversation.Name,
				Message: &schema.Message{
					Content: "已为你转接人工客服，请稍候。，请稍候。",
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, nil, map[string]string{
		toolx.GraphHandoffConversation.Name: toolx.GraphHandoffConversation.Code,
	})

	if summary.ReplyText != "" {
		t.Fatalf("unexpected reply text: %q", summary.ReplyText)
	}
	if summary.Status != "completed" {
		t.Fatalf("unexpected summary status: %q", summary.Status)
	}
}

func TestConsumeAgentEventsUsesGraphToolResultReplyText(t *testing.T) {
	summary := &RunResult{
		Status:           "started",
		InvokedToolCodes: make([]string, 0),
	}
	payload, err := json.Marshal(tooling.ToolResult{
		Handled:     true,
		Terminal:    true,
		Action:      "off_hours_handoff",
		ReplyText:   "当前暂不在人工客服服务时间内，你可以先继续描述问题。",
		ShouldRetry: false,
	})
	if err != nil {
		t.Fatalf("marshal graph tool result: %v", err)
	}
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role:     schema.Tool,
				ToolName: toolx.GraphHandoffConversation.Name,
				Message: &schema.Message{
					Content: string(payload),
				},
			},
		},
	})
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role: schema.Assistant,
				Message: &schema.Message{
					Content: "我再试一次转人工。",
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, nil, map[string]string{
		toolx.GraphHandoffConversation.Name: toolx.GraphHandoffConversation.Code,
	})

	if summary.ReplyText != "当前暂不在人工客服服务时间内，你可以先继续描述问题。" {
		t.Fatalf("unexpected reply text: %q", summary.ReplyText)
	}
	if summary.Status != "completed" {
		t.Fatalf("unexpected summary status: %q", summary.Status)
	}
}

func TestConsumeAgentEventsSuppressesGraphToolResultWhenReplyAlreadySent(t *testing.T) {
	summary := &RunResult{
		Status:           "started",
		InvokedToolCodes: make([]string, 0),
	}
	payload, err := json.Marshal(tooling.ToolResult{
		Handled:     true,
		Terminal:    true,
		Action:      "off_hours_handoff",
		ReplyText:   "当前暂不在人工客服服务时间内，你可以先继续描述问题。",
		ReplySent:   true,
		ShouldRetry: false,
	})
	if err != nil {
		t.Fatalf("marshal graph tool result: %v", err)
	}
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role:     schema.Tool,
				ToolName: toolx.GraphHandoffConversation.Name,
				Message: &schema.Message{
					Content: string(payload),
				},
			},
		},
	})
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role: schema.Assistant,
				Message: &schema.Message{
					Content: "我再试一次转人工。",
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, nil, map[string]string{
		toolx.GraphHandoffConversation.Name: toolx.GraphHandoffConversation.Code,
	})

	if summary.ReplyText != "" {
		t.Fatalf("expected no committed reply because graph already sent it, got %q", summary.ReplyText)
	}
	if summary.Status != "completed" {
		t.Fatalf("unexpected summary status: %q", summary.Status)
	}
}

func TestConsumeAgentEventsCompletesGraphToolWithNoVisibleReply(t *testing.T) {
	summary := &RunResult{
		Status:           "started",
		InvokedToolCodes: make([]string, 0),
	}
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role:     schema.Tool,
				ToolName: toolx.GraphHandoffConversation.Name,
				Message: &schema.Message{
					Content: "",
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, nil, map[string]string{
		toolx.GraphHandoffConversation.Name: toolx.GraphHandoffConversation.Code,
	})

	if summary.ReplyText != "" {
		t.Fatalf("expected no reply text, got %q", summary.ReplyText)
	}
	if summary.Status != "completed" {
		t.Fatalf("unexpected summary status: %q", summary.Status)
	}
}

func TestConsumeAgentEventsCollectsModelTokenUsage(t *testing.T) {
	summary := &RunResult{Status: "started"}
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role: schema.Assistant,
				Message: &schema.Message{
					Content: "reply",
					ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
						PromptTokens:     7,
						CompletionTokens: 3,
					}},
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, nil, nil)

	if summary.PromptTokens != 7 || summary.CompletionTokens != 3 {
		t.Fatalf("unexpected token usage: prompt=%d completion=%d", summary.PromptTokens, summary.CompletionTokens)
	}
}

func TestConsumeAgentEventsRecordsActualFailoverModel(t *testing.T) {
	summary := &RunResult{
		Status:        "started",
		ModelProvider: "sub2api",
		ModelName:     "primary-model",
	}
	collector := callbacks.NewRuntimeTraceCollector()
	events, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Role: schema.Assistant,
				Message: &schema.Message{
					Content: "fallback reply",
					Extra: map[string]any{
						factory.ActualProviderExtraKey: "openai",
						factory.ActualModelExtraKey:    "fallback-model",
					},
				},
			},
		},
	})
	gen.Close()

	consumeAgentEvents(events, summary, collector, nil)

	if summary.ModelProvider != "openai" || summary.ModelName != "fallback-model" {
		t.Fatalf("actual model = %s/%s", summary.ModelProvider, summary.ModelName)
	}
	if collector.Data.Model.Provider != "openai" || collector.Data.Model.Name != "fallback-model" {
		t.Fatalf("trace model = %#v", collector.Data.Model)
	}
}
