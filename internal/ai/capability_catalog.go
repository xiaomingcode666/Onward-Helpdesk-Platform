package ai

import (
	"context"
	"sort"
	"strings"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/providers"
)

// ListCapabilityModels returns the model names exposed by a resolved capability
// without leaking provider credentials to callers.
func ListCapabilityModels(ctx context.Context, route *CapabilityRoute) ([]string, error) {
	if route == nil {
		return []string{}, nil
	}
	if route.Source != CapabilitySourceSub2API {
		return normalizeCapabilityModelNames(route.Config.ModelName, nil), nil
	}

	cfg := config.CurrentOrDefault().Sub2API
	cfg.BaseURL = sub2APIProviderBaseURL(route.Config.BaseURL)
	cfg.DefaultKey = route.Config.APIKey
	provider := providers.NewSub2APIProvider(&cfg)
	response, err := provider.ListModels(ctx, route.Config.APIKey)
	if err != nil {
		return normalizeCapabilityModelNames(route.Config.ModelName, nil), err
	}
	return normalizeCapabilityModelNames(route.Config.ModelName, response.Data), nil
}

func normalizeCapabilityModelNames(preferred string, items []providers.Sub2APIModel) []string {
	preferred = strings.TrimSpace(preferred)
	seen := make(map[string]struct{}, len(items)+1)
	models := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ID)
		if name == "" || name == preferred {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}
	sort.Strings(models)
	if preferred == "" {
		return models
	}
	return append([]string{preferred}, models...)
}

func sub2APIProviderBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		return baseURL[:len(baseURL)-len("/v1")]
	}
	return baseURL
}
