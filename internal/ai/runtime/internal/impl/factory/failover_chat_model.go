package factory

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	ActualModelExtraKey    = "remotehelpdesk.actual_model"
	ActualProviderExtraKey = "remotehelpdesk.actual_provider"
)

type chatModelCandidate struct {
	model      model.ToolCallingChatModel
	provider   string
	modelName  string
	baseURL    string
	circuitKey string
}

func newChatModelCandidate(config models.AIConfig, chatModel model.ToolCallingChatModel) chatModelCandidate {
	keyFingerprint := sha256.Sum256([]byte(strings.TrimSpace(config.APIKey)))
	return chatModelCandidate{
		model:      chatModel,
		provider:   string(config.Provider),
		modelName:  strings.TrimSpace(config.ModelName),
		baseURL:    strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"),
		circuitKey: fmt.Sprintf("%s|%s|%x", config.Provider, strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"), keyFingerprint[:8]),
	}
}

type llmCircuitBreaker struct {
	mu          sync.Mutex
	openedUntil map[string]time.Time
}

var sharedLLMCircuitBreaker = &llmCircuitBreaker{openedUntil: make(map[string]time.Time)}

func (b *llmCircuitBreaker) isOpen(key string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	until := b.openedUntil[key]
	if until.IsZero() || !now.Before(until) {
		delete(b.openedUntil, key)
		return false
	}
	return true
}

func (b *llmCircuitBreaker) open(key string, until time.Time) {
	b.mu.Lock()
	b.openedUntil[key] = until
	b.mu.Unlock()
}

func (b *llmCircuitBreaker) close(key string) {
	b.mu.Lock()
	delete(b.openedUntil, key)
	b.mu.Unlock()
}

type failoverChatModel struct {
	candidates      []chatModelCandidate
	attemptTimeout  time.Duration
	circuitCooldown time.Duration
	breaker         *llmCircuitBreaker
}

func newFailoverChatModel(candidates []chatModelCandidate, attemptTimeout time.Duration, circuitCooldown time.Duration) model.ToolCallingChatModel {
	return &failoverChatModel{
		candidates:      candidates,
		attemptTimeout:  attemptTimeout,
		circuitCooldown: circuitCooldown,
		breaker:         sharedLLMCircuitBreaker,
	}
}

func (m *failoverChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	var callErrors []error
	for index, candidate := range m.candidates {
		if m.skipOpenCircuit(index, candidate) {
			continue
		}
		attemptCtx, cancel := m.attemptContext(ctx)
		message, err := candidate.model.Generate(attemptCtx, input, opts...)
		cancel()
		if err == nil {
			m.breaker.close(candidate.circuitKey)
			annotateActualModel(message, candidate)
			if index > 0 {
				logLLMFailoverSuccess(candidate)
			}
			return message, nil
		}
		callErrors = append(callErrors, candidateError(candidate, err))
		if ctx.Err() != nil {
			return nil, errors.Join(callErrors...)
		}
		m.breaker.open(candidate.circuitKey, time.Now().Add(m.circuitCooldown))
		logLLMProviderFailure(candidate, err, index+1 < len(m.candidates))
	}
	if len(callErrors) == 0 {
		return nil, fmt.Errorf("all LLM providers are temporarily unavailable")
	}
	return nil, fmt.Errorf("all LLM providers failed: %w", errors.Join(callErrors...))
}

func (m *failoverChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	var callErrors []error
	for index, candidate := range m.candidates {
		if m.skipOpenCircuit(index, candidate) {
			continue
		}
		attemptCtx, cancel := m.attemptContext(ctx)
		stream, err := candidate.model.Stream(attemptCtx, input, opts...)
		if err == nil {
			m.breaker.close(candidate.circuitKey)
			if index > 0 {
				logLLMFailoverSuccess(candidate)
			}
			// The stream owns the request lifetime after creation. Cancelling here
			// would terminate a healthy response before the caller can consume it.
			_ = cancel
			return stream, nil
		}
		cancel()
		callErrors = append(callErrors, candidateError(candidate, err))
		if ctx.Err() != nil {
			return nil, errors.Join(callErrors...)
		}
		m.breaker.open(candidate.circuitKey, time.Now().Add(m.circuitCooldown))
		logLLMProviderFailure(candidate, err, index+1 < len(m.candidates))
	}
	if len(callErrors) == 0 {
		return nil, fmt.Errorf("all LLM providers are temporarily unavailable")
	}
	return nil, fmt.Errorf("all LLM providers failed: %w", errors.Join(callErrors...))
}

func (m *failoverChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound := make([]chatModelCandidate, 0, len(m.candidates))
	for _, candidate := range m.candidates {
		chatModel, err := candidate.model.WithTools(tools)
		if err != nil {
			return nil, fmt.Errorf("bind tools to provider=%s model=%s: %w", candidate.provider, candidate.modelName, err)
		}
		candidate.model = chatModel
		bound = append(bound, candidate)
	}
	return &failoverChatModel{
		candidates:      bound,
		attemptTimeout:  m.attemptTimeout,
		circuitCooldown: m.circuitCooldown,
		breaker:         m.breaker,
	}, nil
}

func (m *failoverChatModel) skipOpenCircuit(index int, candidate chatModelCandidate) bool {
	// Always allow the last provider. Otherwise two open circuits could make a
	// request fail without checking whether the fallback has recovered.
	return index < len(m.candidates)-1 && m.breaker.isOpen(candidate.circuitKey, time.Now())
}

func (m *failoverChatModel) attemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if m.attemptTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, m.attemptTimeout)
}

func annotateActualModel(message *schema.Message, candidate chatModelCandidate) {
	if message == nil {
		return
	}
	if message.Extra == nil {
		message.Extra = make(map[string]any, 2)
	}
	message.Extra[ActualModelExtraKey] = candidate.modelName
	message.Extra[ActualProviderExtraKey] = candidate.provider
}

func candidateError(candidate chatModelCandidate, err error) error {
	return fmt.Errorf("provider=%s model=%s: %w", candidate.provider, candidate.modelName, err)
}

func logLLMProviderFailure(candidate chatModelCandidate, err error, hasFallback bool) {
	slog.Warn("LLM provider call failed",
		"provider", candidate.provider,
		"model", candidate.modelName,
		"base_url", candidate.baseURL,
		"fallback_available", hasFallback,
		"error", err,
	)
}

func logLLMFailoverSuccess(candidate chatModelCandidate) {
	slog.Warn("LLM request completed through fallback provider",
		"provider", candidate.provider,
		"model", candidate.modelName,
		"base_url", candidate.baseURL,
	)
}
