package factory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

type ChatModelFactory struct{}

func NewChatModelFactory() *ChatModelFactory {
	return &ChatModelFactory{}
}

func (f *ChatModelFactory) Build(ctx context.Context, aiConfig models.AIConfig) (model.ToolCallingChatModel, error) {
	configs, err := ai.ResolveLLMCandidateConfigs(aiConfig)
	if err != nil {
		return nil, fmt.Errorf("resolve LLM failover candidates: %w", err)
	}
	policy, err := ai.ResolveLLMFailoverPolicy()
	if err != nil {
		return nil, fmt.Errorf("resolve LLM failover policy: %w", err)
	}
	candidates := make([]chatModelCandidate, 0, len(configs))
	for _, item := range configs {
		chatModel, buildErr := f.buildSingle(ctx, item)
		if buildErr != nil {
			return nil, buildErr
		}
		candidates = append(candidates, newChatModelCandidate(item, chatModel))
	}
	if len(candidates) == 1 {
		return candidates[0].model, nil
	}
	return newFailoverChatModel(candidates, policy.AttemptTimeout, policy.CircuitCooldown), nil
}

func (f *ChatModelFactory) buildSingle(ctx context.Context, aiConfig models.AIConfig) (model.ToolCallingChatModel, error) {
	conf := &openai.ChatModelConfig{
		APIKey:  strings.TrimSpace(aiConfig.APIKey),
		BaseURL: strings.TrimSpace(aiConfig.BaseURL),
		Model:   strings.TrimSpace(aiConfig.ModelName),
	}
	if aiConfig.TimeoutMS > 0 {
		conf.Timeout = time.Duration(aiConfig.TimeoutMS) * time.Millisecond
	}
	if aiConfig.MaxOutputTokens > 0 {
		maxCompletionTokens := aiConfig.MaxOutputTokens
		conf.MaxCompletionTokens = &maxCompletionTokens
	}
	if aiConfig.Provider == enums.AIProviderSub2API {
		conf.HTTPClient = providers.NewSub2APIHTTPClient(aiConfig.APIKey, conf.Timeout)
	}
	if aiConfig.Provider == enums.AIProviderOpenAI && isAzureOpenAIBaseURL(aiConfig.BaseURL) {
		conf.ByAzure = true
		conf.APIVersion = "2024-06-01"
	}
	if extraFields := providerExtraFields(aiConfig); len(extraFields) > 0 {
		conf.ExtraFields = extraFields
	}
	chatModel, err := openai.NewChatModel(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("build chat model provider=%s model=%s: %w", aiConfig.Provider, aiConfig.ModelName, err)
	}
	return chatModel, nil
}

func isAzureOpenAIBaseURL(baseURL string) bool {
	baseURL = strings.ToLower(strings.TrimSpace(baseURL))
	return strings.Contains(baseURL, ".openai.azure.com")
}

func providerExtraFields(aiConfig models.AIConfig) map[string]any {
	baseURL := strings.ToLower(strings.TrimSpace(aiConfig.BaseURL))
	modelName := strings.ToLower(strings.TrimSpace(aiConfig.ModelName))
	if strings.Contains(baseURL, "dashscope.aliyuncs.com") && strings.HasPrefix(modelName, "qwen3") {
		return map[string]any{
			"enable_thinking": false,
		}
	}
	return nil
}
