package ai

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestEnvironmentAIConfigEmbedding(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "test-key")
	t.Setenv(embeddingBaseURLEnv, "https://example.test/v1/")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "1024")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		t.Fatalf("environmentAIConfig() error = %v", err)
	}
	if item == nil {
		t.Fatal("environmentAIConfig() returned nil")
	}
	if item.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q", item.BaseURL)
	}
	if item.ModelName != "text-embedding-v3" || item.Dimension != 1024 {
		t.Fatalf("embedding config = model %q dimension %d", item.ModelName, item.Dimension)
	}
	if item.APIKey != "test-key" || item.Status != enums.StatusOk {
		t.Fatal("embedding credentials or status were not populated")
	}
}

func TestEnvironmentAIConfigEmbeddingPrefersDedicatedCredentials(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "embedding-key")
	t.Setenv(embeddingBaseURLEnv, "https://embedding.test/v1/")
	t.Setenv(openAIAPIKeyEnv, "chat-key")
	t.Setenv(openAIBaseURLEnv, "https://chat.test/v1")
	t.Setenv(embeddingModelEnv, "text-embedding-v4")
	t.Setenv(embeddingDimEnv, "1024")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		t.Fatal(err)
	}
	if item.APIKey != "embedding-key" || item.BaseURL != "https://embedding.test/v1" {
		t.Fatalf("dedicated embedding credentials were not selected: %#v", item)
	}
}

func TestEnvironmentAIConfigEmbeddingSupportsLegacyOpenAICredentials(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "")
	t.Setenv(embeddingBaseURLEnv, "")
	t.Setenv(openAIAPIKeyEnv, "legacy-key")
	t.Setenv(openAIBaseURLEnv, "https://legacy.test/v1/")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "1024")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		t.Fatal(err)
	}
	if item.APIKey != "legacy-key" || item.BaseURL != "https://legacy.test/v1" {
		t.Fatalf("legacy embedding credentials were not selected: %#v", item)
	}
}

func TestEnvironmentAIConfigInactiveWithoutEmbeddingVariables(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "unrelated-openai-key")
	t.Setenv(embeddingBaseURLEnv, "https://example.test/v1")
	t.Setenv(embeddingModelEnv, "")
	t.Setenv(embeddingDimEnv, "")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if err != nil || item != nil {
		t.Fatalf("environmentAIConfig() = (%v, %v), want (nil, nil)", item, err)
	}
}

func TestEnvironmentAIConfigRejectsIncompleteConfiguration(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "")
	t.Setenv(embeddingBaseURLEnv, "https://example.test/v1")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "1024")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if item != nil || err == nil || !strings.Contains(err.Error(), embeddingAPIKeyEnv) {
		t.Fatalf("environmentAIConfig() = (%v, %v), want missing-key error", item, err)
	}
}

func TestEnvironmentAIConfigRejectsInvalidDimension(t *testing.T) {
	t.Setenv(embeddingAPIKeyEnv, "test-key")
	t.Setenv(embeddingBaseURLEnv, "https://example.test/v1")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "0")

	item, err := environmentAIConfig(enums.AIModelTypeEmbedding)
	if item != nil || err == nil || !strings.Contains(err.Error(), embeddingDimEnv) {
		t.Fatalf("environmentAIConfig() = (%v, %v), want invalid-dimension error", item, err)
	}
}

func setValidEmbeddingEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(embeddingAPIKeyEnv, "test-key")
	t.Setenv(embeddingBaseURLEnv, "https://example.test/v1")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "1024")
}

func TestResolveEnvironmentLLMFallbackConfig(t *testing.T) {
	t.Setenv(llmFallbackModelEnv, "qwen3.7-flash")
	t.Setenv(openAIAPIKeyEnv, "test-key")
	t.Setenv(openAIBaseURLEnv, "https://aliyun.test/v1/")
	t.Setenv(llmFallbackTimeout, "25s")

	config, err := ResolveEnvironmentLLMFallbackConfig()
	if err != nil {
		t.Fatalf("ResolveEnvironmentLLMFallbackConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("fallback config is nil")
	}
	if config.Provider != enums.AIProviderOpenAI || config.ModelType != enums.AIModelTypeLLM {
		t.Fatalf("provider/type = %s/%s", config.Provider, config.ModelType)
	}
	if config.ModelName != "qwen3.7-flash" || config.BaseURL != "https://aliyun.test/v1" || config.TimeoutMS != 25000 {
		t.Fatalf("fallback config = %#v", config)
	}
}

func TestResolveEnvironmentLLMFallbackConfigDisabledWithoutModel(t *testing.T) {
	t.Setenv(llmFallbackModelEnv, "")
	t.Setenv(openAIAPIKeyEnv, "test-key")
	t.Setenv(openAIBaseURLEnv, "https://aliyun.test/v1")

	config, err := ResolveEnvironmentLLMFallbackConfig()
	if err != nil || config != nil {
		t.Fatalf("ResolveEnvironmentLLMFallbackConfig() = (%#v, %v), want disabled", config, err)
	}
}

func TestResolveLLMCandidateConfigsAvoidsSameEndpoint(t *testing.T) {
	t.Setenv(llmFallbackModelEnv, "fallback-model")
	t.Setenv(openAIAPIKeyEnv, "same-key")
	t.Setenv(openAIBaseURLEnv, "https://same.test/v1/")

	configs, err := ResolveLLMCandidateConfigs(models.AIConfig{
		Provider:  enums.AIProviderOpenAI,
		BaseURL:   "https://same.test/v1",
		APIKey:    "same-key",
		ModelName: "primary-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(configs))
	}
}

func TestResolveLLMFailoverPolicy(t *testing.T) {
	t.Setenv(llmAttemptTimeout, "7s")
	t.Setenv(llmCircuitCooldown, "45s")
	policy, err := ResolveLLMFailoverPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.AttemptTimeout != 7*time.Second || policy.CircuitCooldown != 45*time.Second {
		t.Fatalf("policy = %#v", policy)
	}
}

func TestResolveLLMFailoverPolicyRejectsInvalidDuration(t *testing.T) {
	t.Setenv(llmAttemptTimeout, "0s")
	if _, err := ResolveLLMFailoverPolicy(); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}
