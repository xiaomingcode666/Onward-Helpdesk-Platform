package ai

import (
	"context"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
)

func newOpenAIClient(config models.AIConfig) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
	}
	if config.TimeoutMS > 0 {
		opts = append(opts, option.WithRequestTimeout(time.Duration(config.TimeoutMS)*time.Millisecond))
	}
	if config.MaxRetryCount >= 0 {
		opts = append(opts, option.WithMaxRetries(config.MaxRetryCount))
	}
	if config.Provider == enums.AIProviderSub2API {
		timeout := time.Duration(config.TimeoutMS) * time.Millisecond
		opts = append(opts, option.WithHTTPClient(providers.NewSub2APIHTTPClient(config.APIKey, timeout)))
	}

	return openai.NewClient(opts...)
}

func GetEnabledAIConfig(modelType enums.AIModelType) (*models.AIConfig, error) {
	return GetEnabledAIConfigWithContext(context.Background(), modelType)
}

func GetEnabledAIConfigWithContext(ctx context.Context, modelType enums.AIModelType) (*models.AIConfig, error) {
	route, err := ResolveCapabilityRoute(ctx, modelType)
	if err != nil {
		return nil, err
	}
	if route != nil {
		return &route.Config, nil
	}
	return nil, errorsx.BusinessErrorI18n(2005, "error.aiConfig.noneEnabled")
}
