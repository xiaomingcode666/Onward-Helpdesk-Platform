package executor

import (
	"encoding/json"
	"testing"

	"remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/ai/runtime/internal/impl/factory"
	"remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/pkg/toolx"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

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
