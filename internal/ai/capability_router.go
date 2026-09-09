package ai

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

const (
	llmSourceEnv = "LLM_SOURCE"

	llmSourceAuto     = "auto"
	llmSourceSub2API  = "sub2api"
	llmSourceDatabase = "database"

	sub2APIBaseURLEnv        = "SUB2API_BASE_URL"
	sub2APIDefaultKeyEnv     = "SUB2API_DEFAULT_KEY"
	sub2APILLMModelEnv       = "SUB2API_LLM_MODEL"
	sub2APIEmbeddingModelEnv = "SUB2API_EMBEDDING_MODEL"
	sub2APIEmbeddingDimEnv   = "SUB2API_EMBEDDING_DIM"

	platformSub2APIHostConfigKey     = "platform.sub2api.host"
	platformSub2APILLMModelConfigKey = "platform.sub2api.default_llm_model"
)

type CapabilitySource string

const (
	CapabilitySourceDatabase    CapabilitySource = "database"
	CapabilitySourceEnvironment CapabilitySource = "environment"
	CapabilitySourceSub2API     CapabilitySource = "sub2api"
)

type CapabilityScope struct {
	TenantID        int64
	ProductID       int64
	KnowledgeBaseID int64
	CredentialChain []string
}

type CapabilityRoute struct {
	Config models.AIConfig
	Source CapabilitySource
	Scope  CapabilityScope
}

type capabilityScopeContextKey struct{}

func WithCapabilityScope(ctx context.Context, scope CapabilityScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(scope.CredentialChain) == 0 {
		scope.CredentialChain = CapabilityScopeFromContext(ctx).CredentialChain
	} else {
		scope.CredentialChain = append([]string(nil), scope.CredentialChain...)
	}
	return context.WithValue(ctx, capabilityScopeContextKey{}, scope)
}

func CapabilityScopeFromContext(ctx context.Context) CapabilityScope {
	if ctx == nil {
		return CapabilityScope{}
	}
	scope, _ := ctx.Value(capabilityScopeContextKey{}).(CapabilityScope)
	scope.CredentialChain = append([]string(nil), scope.CredentialChain...)
	return scope
}

func ResolveCapabilityRoute(ctx context.Context, modelType enums.AIModelType) (*CapabilityRoute, error) {
	return ResolveCapabilityRouteForScope(ctx, modelType, CapabilityScopeFromContext(ctx))
}

func ResolveCapabilityRouteForScope(ctx context.Context, modelType enums.AIModelType, scope CapabilityScope) (*CapabilityRoute, error) {
	databaseItem := repositories.AIConfigRepository.GetEnabled(sqls.DB(), modelType)
	switch modelType {
	case enums.AIModelTypeLLM:
		return resolveLLMRoute(scope, databaseItem)
	case enums.AIModelTypeEmbedding:
		return resolveEmbeddingRoute(scope, databaseItem)
	default:
		if databaseItem == nil {
			return nil, nil
		}
		return routeFromConfig(*databaseItem, CapabilitySourceDatabase, scope), nil
	}
}

func resolveLLMRoute(scope CapabilityScope, databaseItem *models.AIConfig) (*CapabilityRoute, error) {
	source, err := configuredLLMSource()
	if err != nil {
		return nil, err
	}
	if scopeUsesPlatformTranslationCredential(scope) {
		return sub2APICapabilityRoute(enums.AIModelTypeLLM, scope)
	}
	switch source {
	case llmSourceDatabase:
		if len(scope.CredentialChain) > 0 {
			return nil, fmt.Errorf("workflow model credential policy conflicts with %s=%s", llmSourceEnv, llmSourceDatabase)
		}
		if databaseItem == nil {
			return nil, nil
		}
		return routeFromConfig(*databaseItem, CapabilitySourceDatabase, scope), nil
	case llmSourceSub2API:
		return sub2APICapabilityRoute(enums.AIModelTypeLLM, scope)
	default:
		route, routeErr := sub2APICapabilityRoute(enums.AIModelTypeLLM, scope)
		if route != nil {
			return route, nil
		}
		if len(scope.CredentialChain) > 0 {
			if routeErr != nil {
				return nil, routeErr
			}
			return nil, fmt.Errorf("workflow model credential policy has no available credential")
		}
		if databaseItem != nil {
			return routeFromConfig(*databaseItem, CapabilitySourceDatabase, scope), nil
		}
		item, configErr := environmentAIConfig(enums.AIModelTypeLLM)
		if configErr != nil {
			return nil, configErr
		}
		if item != nil {
			return routeFromConfig(*item, CapabilitySourceEnvironment, scope), nil
		}
		return nil, routeErr
	}
}

func resolveEmbeddingRoute(scope CapabilityScope, databaseItem *models.AIConfig) (*CapabilityRoute, error) {
	source, err := embeddingSource()
	if err != nil {
		return nil, err
	}
	switch source {
	case embeddingSourceAliyun:
		item, configErr := environmentAIConfig(enums.AIModelTypeEmbedding)
		if configErr != nil {
			return nil, configErr
		}
		if item == nil {
			return nil, fmt.Errorf("%s=%s requires environment embedding configuration", embeddingSourceEnv, embeddingSourceAliyun)
		}
		return routeFromConfig(*item, CapabilitySourceEnvironment, scope), nil
	case embeddingSourceSub2API:
		return sub2APICapabilityRoute(enums.AIModelTypeEmbedding, scope)
	default:
		if databaseItem != nil {
			return routeFromConfig(*databaseItem, CapabilitySourceDatabase, scope), nil
		}
		item, configErr := environmentAIConfig(enums.AIModelTypeEmbedding)
		if configErr != nil {
			return nil, configErr
		}
		if item != nil {
			return routeFromConfig(*item, CapabilitySourceEnvironment, scope), nil
		}
		return nil, nil
	}
}

func configuredLLMSource() (string, error) {
	source := strings.ToLower(strings.TrimSpace(os.Getenv(llmSourceEnv)))
	if source == "" {
		return llmSourceAuto, nil
	}
	switch source {
	case llmSourceAuto, llmSourceSub2API, llmSourceDatabase:
		return source, nil
	default:
		return "", fmt.Errorf("%s must be one of %s, %s, %s", llmSourceEnv, llmSourceAuto, llmSourceSub2API, llmSourceDatabase)
	}
}

func sub2APICapabilityRoute(modelType enums.AIModelType, scope CapabilityScope) (*CapabilityRoute, error) {
	baseURL := resolveSub2APIBaseURL()
	credential, err := resolveSub2APICredential(scope)
	if err != nil {
		return nil, err
	}
	if baseURL == "" || credential.APIKey == "" {
		return nil, nil
	}

	modelName, dimension, err := resolveSub2APIModel(modelType, scope)
	if err != nil {
		return nil, err
	}
	cfg := config.CurrentOrDefault().Sub2API
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}
	item := models.AIConfig{
		Name:                      "Platform AI Capability",
		Provider:                  enums.AIProviderSub2API,
		BaseURL:                   openAICompatibleBaseURL(baseURL),
		APIKey:                    credential.APIKey,
		ModelType:                 modelType,
		ModelName:                 modelName,
		Dimension:                 dimension,
		TimeoutMS:                 int(timeout.Milliseconds()),
		MaxRetryCount:             maxRetries,
		Status:                    enums.StatusOk,
		Remark:                    "Resolved internally from the tenant Sub2API account.",
		RuntimeCredentialScope:    string(credential.Scope),
		RuntimeAPIKeyID:           credential.APIKeyID,
		RuntimeCredentialFallback: credential.Fallback,
	}
	return routeFromConfig(item, CapabilitySourceSub2API, scope), nil
}

func resolveSub2APIBaseURL() string {
	if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APIHostConfigKey); item != nil {
		if value := strings.TrimSpace(item.ConfigValue); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(config.CurrentOrDefault().Sub2API.BaseURL); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(sub2APIBaseURLEnv))
}

func resolveSub2APIModel(modelType enums.AIModelType, scope CapabilityScope) (string, int, error) {
	switch modelType {
	case enums.AIModelTypeLLM:
		modelName := ""
		if !scopeUsesPlatformTranslationCredential(scope) {
			modelName = resolveTenantSub2APIDefaultLLMModel(scope.TenantID)
		}
		if modelName == "" {
			modelName = strings.TrimSpace(os.Getenv(sub2APILLMModelEnv))
		}
		if modelName == "" {
			if item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APILLMModelConfigKey); item != nil {
				modelName = strings.TrimSpace(item.ConfigValue)
			}
		}
		if modelName == "" {
			return "", 0, fmt.Errorf("Sub2API LLM capability requires %s or %s", sub2APILLMModelEnv, platformSub2APILLMModelConfigKey)
		}
		return modelName, 0, nil
	case enums.AIModelTypeEmbedding:
		modelName := firstNonBlank(os.Getenv(sub2APIEmbeddingModelEnv), os.Getenv(embeddingModelEnv))
		dimensionText := firstNonBlank(os.Getenv(sub2APIEmbeddingDimEnv), os.Getenv(embeddingDimEnv))
		if modelName == "" || dimensionText == "" {
			return "", 0, fmt.Errorf("Sub2API embedding capability requires %s/%s and %s/%s", sub2APIEmbeddingModelEnv, embeddingModelEnv, sub2APIEmbeddingDimEnv, embeddingDimEnv)
		}
		dimension, err := strconv.Atoi(dimensionText)
		if err != nil || dimension <= 0 {
			return "", 0, fmt.Errorf("%s/%s must be a positive integer", sub2APIEmbeddingDimEnv, embeddingDimEnv)
		}
		return modelName, dimension, nil
	default:
		return "", 0, fmt.Errorf("unsupported Sub2API capability type: %s", modelType)
	}
}

func scopeUsesPlatformTranslationCredential(scope CapabilityScope) bool {
	if len(scope.CredentialChain) != 1 {
		return false
	}
	return strings.TrimSpace(scope.CredentialChain[0]) == ModelCredentialScopePlatformTranslation
}

func resolveTenantSub2APIDefaultLLMModel(tenantID int64) string {
	if tenantID <= 0 {
		return ""
	}
	if account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID); account != nil {
		return strings.TrimSpace(account.DefaultLLMModel)
	}
	return ""
}

func openAICompatibleBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		return baseURL
	}
	return baseURL + "/v1"
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func routeFromConfig(item models.AIConfig, source CapabilitySource, scope CapabilityScope) *CapabilityRoute {
	return &CapabilityRoute{Config: item, Source: source, Scope: scope}
}
