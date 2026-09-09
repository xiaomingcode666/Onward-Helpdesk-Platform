package factory

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeToolCallingChatModel struct {
	generateCalls  atomic.Int32
	withToolsCalls atomic.Int32
	generate       func(context.Context) (*schema.Message, error)
}

func (m *fakeToolCallingChatModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.generateCalls.Add(1)
	return m.generate(ctx)
}

func (m *fakeToolCallingChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream unavailable")
}

func (m *fakeToolCallingChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.withToolsCalls.Add(1)
	return m, nil
}

func TestFailoverChatModelGenerateUsesFallbackAndOpensCircuit(t *testing.T) {
	primary := &fakeToolCallingChatModel{generate: func(context.Context) (*schema.Message, error) {
		return nil, errors.New("primary unavailable")
	}}
	fallback := &fakeToolCallingChatModel{generate: func(context.Context) (*schema.Message, error) {
		return &schema.Message{Role: schema.Assistant, Content: "fallback answer"}, nil
	}}
	breaker := &llmCircuitBreaker{openedUntil: make(map[string]time.Time)}
	chatModel := &failoverChatModel{
		candidates: []chatModelCandidate{
			{model: primary, provider: "sub2api", modelName: "primary", circuitKey: "primary"},
			{model: fallback, provider: "openai", modelName: "fallback", circuitKey: "fallback"},
		},
		attemptTimeout:  time.Second,
		circuitCooldown: time.Minute,
		breaker:         breaker,
	}

	for attempt := 0; attempt < 2; attempt++ {
		message, err := chatModel.Generate(context.Background(), nil)
		if err != nil {
			t.Fatalf("Generate() attempt %d error = %v", attempt, err)
		}
		if message.Content != "fallback answer" || message.Extra[ActualModelExtraKey] != "fallback" {
			t.Fatalf("message = %#v", message)
		}
	}
	if primary.generateCalls.Load() != 1 {
		t.Fatalf("primary calls = %d, want circuit to skip second call", primary.generateCalls.Load())
	}
	if fallback.generateCalls.Load() != 2 {
		t.Fatalf("fallback calls = %d, want 2", fallback.generateCalls.Load())
	}
}

func TestFailoverChatModelDoesNotFallbackAfterCallerCancellation(t *testing.T) {
	primary := &fakeToolCallingChatModel{generate: func(ctx context.Context) (*schema.Message, error) {
		return nil, ctx.Err()
	}}
	fallback := &fakeToolCallingChatModel{generate: func(context.Context) (*schema.Message, error) {
		return &schema.Message{Content: "must not run"}, nil
	}}
	chatModel := &failoverChatModel{
		candidates: []chatModelCandidate{
			{model: primary, provider: "primary", modelName: "one", circuitKey: "cancel-primary"},
			{model: fallback, provider: "fallback", modelName: "two", circuitKey: "cancel-fallback"},
		},
		attemptTimeout:  time.Second,
		circuitCooldown: time.Minute,
		breaker:         &llmCircuitBreaker{openedUntil: make(map[string]time.Time)},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := chatModel.Generate(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context canceled", err)
	}
	if fallback.generateCalls.Load() != 0 {
		t.Fatalf("fallback calls = %d, want 0", fallback.generateCalls.Load())
	}
}

func TestFailoverChatModelWithToolsBindsEveryCandidate(t *testing.T) {
	primary := &fakeToolCallingChatModel{generate: func(context.Context) (*schema.Message, error) { return nil, nil }}
	fallback := &fakeToolCallingChatModel{generate: func(context.Context) (*schema.Message, error) { return nil, nil }}
	chatModel := &failoverChatModel{
		candidates: []chatModelCandidate{
			{model: primary, circuitKey: "tools-primary"},
			{model: fallback, circuitKey: "tools-fallback"},
		},
		breaker: &llmCircuitBreaker{openedUntil: make(map[string]time.Time)},
	}
	if _, err := chatModel.WithTools([]*schema.ToolInfo{{Name: "handoff_to_human"}}); err != nil {
		t.Fatal(err)
	}
	if primary.withToolsCalls.Load() != 1 || fallback.withToolsCalls.Load() != 1 {
		t.Fatalf("tool binding calls = primary:%d fallback:%d", primary.withToolsCalls.Load(), fallback.withToolsCalls.Load())
	}
}
