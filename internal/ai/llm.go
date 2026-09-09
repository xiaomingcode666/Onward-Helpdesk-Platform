package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/mlogclub/simple/common/strs"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

type ChatCompletionResult struct {
	Content                string
	ModelName              string
	Provider               enums.AIProvider
	RuntimeCredentialScope string
	RuntimeAPIKeyID        string
	PromptTokens           int
	CompletionTokens       int
}

type llm struct{}

var LLM = &llm{}

func (s *llm) Chat(ctx context.Context, systemPrompt string, userPrompt string) (*ChatCompletionResult, error) {
	config, err := GetEnabledAIConfigWithContext(ctx, enums.AIModelTypeLLM)
	if err != nil {
		return nil, err
	}
	candidates, err := ResolveLLMCandidateConfigs(*config)
	if err != nil {
		return nil, fmt.Errorf("resolve LLM failover candidates: %w", err)
	}
	policy, err := ResolveLLMFailoverPolicy()
	if err != nil {
		return nil, fmt.Errorf("resolve LLM failover policy: %w", err)
	}

	callErrors := make([]error, 0, len(candidates))
	for index, candidate := range candidates {
		attemptCtx, cancel := context.WithTimeout(ctx, policy.AttemptTimeout)
		result, callErr := s.ChatWithConfig(attemptCtx, candidate, systemPrompt, userPrompt)
		cancel()
		if callErr == nil {
			if index > 0 {
				slog.Warn("LLM request completed through fallback provider", "provider", candidate.Provider, "model", candidate.ModelName)
			}
			return result, nil
		}
		callErrors = append(callErrors, callErr)
		if ctx.Err() != nil {
			return nil, errors.Join(callErrors...)
		}
		slog.Warn("LLM provider call failed", "provider", candidate.Provider, "model", candidate.ModelName, "fallback_available", index+1 < len(candidates), "error", callErr)
	}
	return nil, fmt.Errorf("all LLM providers failed: %w", errors.Join(callErrors...))
}

func (s *llm) ChatWithConfig(ctx context.Context, config models.AIConfig, systemPrompt string, userPrompt string) (*ChatCompletionResult, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if strs.IsNotBlank(systemPrompt) {
		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(systemPrompt),
				},
			},
		})
	}
	messages = append(messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(userPrompt),
			},
		},
	})

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    shared.ChatModel(config.ModelName),
	}
	if config.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(config.MaxOutputTokens))
	}
	applyProviderSpecificChatParams(&params, config)

	client := newOpenAIClient(config)
	chatResp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to call llm api (model=%s provider=%s system_chars=%d user_chars=%d max_output_tokens=%d): %w",
			config.ModelName, config.Provider, utf8.RuneCountInString(systemPrompt), utf8.RuneCountInString(userPrompt), config.MaxOutputTokens, err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no llm choices in response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return &ChatCompletionResult{
		Content:                content,
		ModelName:              config.ModelName,
		Provider:               config.Provider,
		RuntimeCredentialScope: config.RuntimeCredentialScope,
		RuntimeAPIKeyID:        config.RuntimeAPIKeyID,
		PromptTokens:           int(chatResp.Usage.PromptTokens),
		CompletionTokens:       int(chatResp.Usage.CompletionTokens),
	}, nil
}

func applyProviderSpecificChatParams(params *openai.ChatCompletionNewParams, config models.AIConfig) {
	if params == nil {
		return
	}
	if isDashScopeQwenThinkingModel(config) {
		params.SetExtraFields(map[string]any{
			"enable_thinking": false,
		})
	}
}

func isDashScopeQwenThinkingModel(config models.AIConfig) bool {
	baseURL := strings.ToLower(strings.TrimSpace(config.BaseURL))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	return strings.Contains(baseURL, "dashscope.aliyuncs.com") && strings.HasPrefix(modelName, "qwen3")
}
