package ai

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

const (
	embeddingSourceEnv  = "EMBEDDING_SOURCE"
	embeddingAPIKeyEnv  = "EMBEDDING_API_KEY"
	embeddingBaseURLEnv = "EMBEDDING_BASE_URL"
	openAIAPIKeyEnv     = "OPENAI_API_KEY"
	openAIBaseURLEnv    = "OPENAI_BASE_URL"
	llmModelEnv         = "LLM_MODEL"
	embeddingModelEnv   = "EMBEDDING_MODEL"
	embeddingDimEnv     = "EMBEDDING_DIM"
	llmFallbackModelEnv = "LLM_FALLBACK_MODEL"
	llmFallbackTimeout  = "LLM_FALLBACK_TIMEOUT"
	llmAttemptTimeout   = "LLM_FAILOVER_ATTEMPT_TIMEOUT"
	llmCircuitCooldown  = "LLM_FAILOVER_CIRCUIT_COOLDOWN"

	embeddingSourceAuto    = "auto"
	embeddingSourceAliyun  = "aliyun"
	embeddingSourceSub2API = "sub2api"
)

const (
	defaultLLMFallbackTimeout = 30 * time.Second
	defaultLLMAttemptTimeout  = 15 * time.Second
	defaultLLMCircuitCooldown = 60 * time.Second
)

type LLMFailoverPolicy struct {
	AttemptTimeout  time.Duration
	CircuitCooldown time.Duration
}

func embeddingSource() (string, error) {
	source := strings.ToLower(strings.TrimSpace(os.Getenv(embeddingSourceEnv)))
	if source == "" {
		return embeddingSourceAuto, nil
	}
	switch source {
	case embeddingSourceAuto, embeddingSourceAliyun, embeddingSourceSub2API:
		return source, nil
	default:
		return "", fmt.Errorf("%s must be one of %s, %s, %s", embeddingSourceEnv, embeddingSourceAuto, embeddingSourceAliyun, embeddingSourceSub2API)
	}
}

func environmentAIConfig(modelType enums.AIModelType) (*models.AIConfig, error) {
	switch modelType {
	case enums.AIModelTypeLLM:
		return environmentOpenAICompatibleConfig(modelType, llmModelEnv, "", 0)
	case enums.AIModelTypeEmbedding:
		return environmentOpenAICompatibleConfig(modelType, embeddingModelEnv, embeddingDimEnv, 0)
	default:
		return nil, nil
	}
}

func environmentOpenAICompatibleConfig(modelType enums.AIModelType, modelEnv string, dimensionEnv string, fallbackDimension int) (*models.AIConfig, error) {
	modelName := strings.TrimSpace(os.Getenv(modelEnv))
	dimensionText := ""
	if dimensionEnv != "" {
		dimensionText = strings.TrimSpace(os.Getenv(dimensionEnv))
	}
	if modelName == "" && dimensionText == "" {
		return nil, nil
	}

	apiKeyEnv := openAIAPIKeyEnv
	baseURLEnv := openAIBaseURLEnv
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(baseURLEnv)), "/")
	if modelType == enums.AIModelTypeEmbedding {
		apiKeyEnv = embeddingAPIKeyEnv
		baseURLEnv = embeddingBaseURLEnv
		apiKey = firstEnvironmentValue(embeddingAPIKeyEnv, openAIAPIKeyEnv)
		baseURL = strings.TrimRight(firstEnvironmentValue(embeddingBaseURLEnv, openAIBaseURLEnv), "/")
	}
	missing := make([]string, 0, 4)
	if apiKey == "" {
		missing = append(missing, apiKeyEnv)
	}
	if baseURL == "" {
		missing = append(missing, baseURLEnv)
	}
	if modelName == "" {
		missing = append(missing, modelEnv)
	}
	if dimensionEnv != "" && dimensionText == "" && fallbackDimension <= 0 {
		missing = append(missing, dimensionEnv)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s environment config is incomplete: missing %s", modelType, strings.Join(missing, ", "))
	}

	dimension := fallbackDimension
	if dimensionText != "" {
		parsed, err := strconv.Atoi(dimensionText)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must be a positive integer", dimensionEnv)
		}
		dimension = parsed
	}

	return &models.AIConfig{
		Name:          environmentOpenAICompatibleConfigName(modelType),
		Provider:      enums.AIProviderOpenAI,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		ModelType:     modelType,
		ModelName:     modelName,
		Dimension:     dimension,
		TimeoutMS:     60000,
		MaxRetryCount: 2,
		Status:        enums.StatusOk,
		Remark:        "OpenAI-compatible model configuration loaded from environment variables.",
	}, nil
}

func firstEnvironmentValue(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func environmentOpenAICompatibleConfigName(modelType enums.AIModelType) string {
	switch modelType {
	case enums.AIModelTypeLLM:
		return "Environment OpenAI-compatible LLM"
	case enums.AIModelTypeEmbedding:
		return "Environment OpenAI-compatible Embedding"
	default:
		return "Environment OpenAI-compatible Model"
	}
}

// ResolveEnvironmentLLMFallbackConfig returns the optional OpenAI-compatible
// fallback used after the primary LLM provider fails. It deliberately uses a
// separate model variable from Embedding so callers cannot send chat requests
// to an embedding-only model by accident.
func ResolveEnvironmentLLMFallbackConfig() (*models.AIConfig, error) {
	modelName := strings.TrimSpace(os.Getenv(llmFallbackModelEnv))
	if modelName == "" {
		return nil, nil
	}

	apiKey := strings.TrimSpace(os.Getenv(openAIAPIKeyEnv))
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(openAIBaseURLEnv)), "/")
	missing := make([]string, 0, 2)
	if apiKey == "" {
		missing = append(missing, openAIAPIKeyEnv)
	}
	if baseURL == "" {
		missing = append(missing, openAIBaseURLEnv)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("LLM fallback environment config is incomplete: missing %s", strings.Join(missing, ", "))
	}

	timeout, err := environmentDuration(llmFallbackTimeout, defaultLLMFallbackTimeout)
	if err != nil {
		return nil, err
	}
	return &models.AIConfig{
		Name:          "Environment LLM Fallback",
		Provider:      enums.AIProviderOpenAI,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		ModelType:     enums.AIModelTypeLLM,
		ModelName:     modelName,
		TimeoutMS:     int(timeout.Milliseconds()),
		MaxRetryCount: 1,
		Status:        enums.StatusOk,
		Remark:        "Secondary OpenAI-compatible LLM used only after the primary provider fails.",
	}, nil
}

func ResolveLLMCandidateConfigs(primary models.AIConfig) ([]models.AIConfig, error) {
	ret := []models.AIConfig{primary}
	fallback, err := ResolveEnvironmentLLMFallbackConfig()
	if err != nil {
		return nil, err
	}
	if fallback == nil || sameLLMEndpoint(primary, *fallback) {
		return ret, nil
	}
	return append(ret, *fallback), nil
}

func ResolveLLMFailoverPolicy() (LLMFailoverPolicy, error) {
	attemptTimeout, err := environmentDuration(llmAttemptTimeout, defaultLLMAttemptTimeout)
	if err != nil {
		return LLMFailoverPolicy{}, err
	}
	circuitCooldown, err := environmentDuration(llmCircuitCooldown, defaultLLMCircuitCooldown)
	if err != nil {
		return LLMFailoverPolicy{}, err
	}
	return LLMFailoverPolicy{
		AttemptTimeout:  attemptTimeout,
		CircuitCooldown: circuitCooldown,
	}, nil
}

func sameLLMEndpoint(left models.AIConfig, right models.AIConfig) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(left.BaseURL), "/"), strings.TrimRight(strings.TrimSpace(right.BaseURL), "/")) &&
		strings.TrimSpace(left.APIKey) == strings.TrimSpace(right.APIKey)
}

func environmentDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}
