package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestResolveCapabilityRouteUsesAliyunEmbeddingEnvironment(t *testing.T) {
	setupCapabilityRouterTestDB(t)
	setValidEmbeddingEnvironment(t)
	t.Setenv(embeddingSourceEnv, embeddingSourceAliyun)

	ctx := WithCapabilityScope(context.Background(), CapabilityScope{TenantID: 9, ProductID: 12})
	route, err := ResolveCapabilityRoute(ctx, enums.AIModelTypeEmbedding)
	if err != nil {
		t.Fatalf("ResolveCapabilityRoute() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceEnvironment {
		t.Fatalf("route = %#v, want environment source", route)
	}
	if route.Config.ModelName != "text-embedding-v3" || route.Config.Dimension != 1024 {
		t.Fatalf("embedding config = %#v", route.Config)
	}
	if route.Scope.TenantID != 9 || route.Scope.ProductID != 12 {
		t.Fatalf("scope = %#v", route.Scope)
	}
}

func TestResolveCapabilityRouteUsesTenantSub2APILLM(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{TenantID: 7})
	if err != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceSub2API {
		t.Fatalf("route = %#v, want Sub2API source", route)
	}
	if route.Config.Provider != enums.AIProviderSub2API {
		t.Fatalf("provider = %q", route.Config.Provider)
	}
	if route.Config.BaseURL != "https://sub2api.test/v1" || route.Config.APIKey != "tenant-key" {
		t.Fatalf("Sub2API endpoint or tenant key not resolved: %#v", route.Config)
	}
	if route.Config.ModelName != "qwen-test" {
		t.Fatalf("model = %q", route.Config.ModelName)
	}
}

func TestResolveCapabilityRouteDatabaseLLMCompatibility(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	databaseItem := models.AIConfig{
		Name:      "Legacy LLM",
		Provider:  enums.AIProviderOpenAI,
		ModelType: enums.AIModelTypeLLM,
		ModelName: "legacy-model",
		Status:    enums.StatusOk,
	}
	if err := db.Create(&databaseItem).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRoute(context.Background(), enums.AIModelTypeLLM)
	if err != nil {
		t.Fatalf("ResolveCapabilityRoute() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceDatabase || route.Config.ID != databaseItem.ID {
		t.Fatalf("route = %#v, want legacy database config", route)
	}
}

func TestResolveCapabilityRouteAutoUsesSub2APILLMBeforeEnvironment(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceAuto)
	t.Setenv(llmModelEnv, "qwen3.5-plus")
	t.Setenv(embeddingAPIKeyEnv, "dashscope-key")
	t.Setenv(embeddingBaseURLEnv, "https://dashscope.aliyuncs.com/compatible-mode/v1")
	t.Setenv(sub2APILLMModelEnv, "sub2api-qwen")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{TenantID: 7})
	if err != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceSub2API {
		t.Fatalf("route = %#v, want Sub2API source", route)
	}
	if route.Config.Provider != enums.AIProviderSub2API || route.Config.ModelName != "sub2api-qwen" || route.Config.BaseURL != "https://sub2api.test/v1" {
		t.Fatalf("Sub2API LLM route = %#v", route.Config)
	}
}

func TestResolveCapabilityRouteSub2APIEmbeddingSwitch(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(embeddingSourceEnv, embeddingSourceSub2API)
	t.Setenv(sub2APIEmbeddingModelEnv, "future-embedding-model")
	t.Setenv(sub2APIEmbeddingDimEnv, "1536")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeEmbedding, CapabilityScope{TenantID: 7})
	if err != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() error = %v", err)
	}
	if route == nil || route.Config.ModelName != "future-embedding-model" || route.Config.Dimension != 1536 {
		t.Fatalf("route = %#v", route)
	}
}

func TestResolveCapabilityRouteAutoUsesDatabaseEmbeddingBeforeSub2API(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(embeddingSourceEnv, embeddingSourceAuto)
	t.Setenv(sub2APIBaseURLEnv, "https://sub2api.test")
	t.Setenv(sub2APIDefaultKeyEnv, "platform-default-key")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, "1024")
	if err := db.Create(&models.AIConfig{
		Name:      "P0 deterministic embedding",
		Provider:  enums.AIProviderOpenAI,
		BaseURL:   "http://127.0.0.1:18099/v1",
		APIKey:    "local-key",
		ModelType: enums.AIModelTypeEmbedding,
		ModelName: "e2e-embedding",
		Dimension: 8,
		Status:    enums.StatusOk,
		SortNo:    10000,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeEmbedding, CapabilityScope{})
	if err != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceDatabase {
		t.Fatalf("route = %#v, want database source", route)
	}
	if route.Config.BaseURL != "http://127.0.0.1:18099/v1" || route.Config.ModelName != "e2e-embedding" || route.Config.Dimension != 8 {
		t.Fatalf("database embedding route = %#v", route.Config)
	}
}

func TestResolveCapabilityRouteAutoUsesEnvironmentEmbeddingBeforeSub2API(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(embeddingSourceEnv, embeddingSourceAuto)
	setValidEmbeddingEnvironment(t)
	t.Setenv(sub2APIEmbeddingModelEnv, "sub2api-embedding")
	t.Setenv(sub2APIEmbeddingDimEnv, "1536")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeEmbedding, CapabilityScope{})
	if err != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() error = %v", err)
	}
	if route == nil || route.Source != CapabilitySourceEnvironment {
		t.Fatalf("route = %#v, want environment source", route)
	}
	if route.Config.Provider != enums.AIProviderOpenAI || route.Config.ModelName != "text-embedding-v3" || route.Config.BaseURL != "https://example.test/v1" {
		t.Fatalf("environment embedding route = %#v", route.Config)
	}
}

func TestResolveCapabilityRouteRejectsSub2APIWithoutModel(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{TenantID: 7})
	if route != nil || err == nil || !strings.Contains(err.Error(), sub2APILLMModelEnv) {
		t.Fatalf("ResolveCapabilityRouteForScope() = (%#v, %v), want missing-model error", route, err)
	}
}

func TestResolveCapabilityRouteDoesNotFallBackToPlatformKeyForUnknownTenant(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	t.Setenv(sub2APIDefaultKeyEnv, "platform-default-key")
	if err := db.Create(&models.SystemConfig{
		ConfigKey:   platformSub2APIHostConfigKey,
		ConfigValue: "https://sub2api.test/",
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{TenantID: 404})
	if err != nil || route != nil {
		t.Fatalf("ResolveCapabilityRouteForScope() = (%#v, %v), want unavailable tenant route", route, err)
	}
}

func setupCapabilityRouterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIConfig{}, &models.SystemConfig{}, &models.Sub2APITenantAccount{}, &models.ProductAIUsageCredential{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if raw, closeErr := db.DB(); closeErr == nil {
			_ = raw.Close()
		}
	})
	return db
}

func TestResolveCapabilityRouteUsesProductSub2APIKey(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID:        7,
		ProductID:       42,
		Sub2APIKeyID:    "product-key-42",
		APIKeyRef:       "product-key",
		ProvisionStatus: "ready",
		Status:          enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       42,
		CredentialChain: []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route == nil || route.Config.APIKey != "product-key" || route.Config.RuntimeAPIKeyID != "product-key-42" {
		t.Fatalf("product credential was not resolved: %#v", route)
	}
	if route.Config.RuntimeCredentialScope != dsl.ModelCredentialScopeProduct || route.Config.RuntimeCredentialFallback {
		t.Fatalf("unexpected product credential metadata: %#v", route.Config)
	}
}

func TestResolveCapabilityRouteFallsBackToTenantKeyWhenProductKeyMissing(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       404,
		CredentialChain: []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route == nil || route.Config.APIKey != "tenant-key" || route.Config.RuntimeAPIKeyID != "tenant-default-key" {
		t.Fatalf("tenant fallback credential was not resolved: %#v", route)
	}
	if route.Config.RuntimeCredentialScope != dsl.ModelCredentialScopeTenantDefault || !route.Config.RuntimeCredentialFallback {
		t.Fatalf("unexpected tenant fallback metadata: %#v", route.Config)
	}
}

func TestResolveCapabilityRouteFallsBackToTenantKeyWhenProductCredentialIsIncomplete(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID:        7,
		ProductID:       42,
		APIKeyRef:       "orphaned-product-key",
		ProvisionStatus: "ready",
		Status:          enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       42,
		CredentialChain: []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route == nil || route.Config.APIKey != "tenant-key" || route.Config.RuntimeAPIKeyID != "tenant-default-key" {
		t.Fatalf("incomplete product credential should fall back to tenant key: %#v", route)
	}
	if route.Config.RuntimeCredentialScope != dsl.ModelCredentialScopeTenantDefault || !route.Config.RuntimeCredentialFallback {
		t.Fatalf("unexpected incomplete-product fallback metadata: %#v", route.Config)
	}
}

func TestResolveCapabilityRouteTenantPolicyIgnoresProductKey(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID: 7, ProductID: 42, Sub2APIKeyID: "product-key-42", APIKeyRef: "product-key", ProvisionStatus: "ready", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       42,
		CredentialChain: []string{dsl.ModelCredentialScopeTenantDefault},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route == nil || route.Config.APIKey != "tenant-key" || route.Config.RuntimeCredentialScope != dsl.ModelCredentialScopeTenantDefault || route.Config.RuntimeCredentialFallback {
		t.Fatalf("tenant policy did not use the tenant default credential: %#v", route)
	}
}

func TestResolveCapabilityRouteHonorsProductOnlyChainWithoutImplicitFallback(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceSub2API)
	t.Setenv(sub2APILLMModelEnv, "qwen-test")
	createSub2APIRouteFixtures(t, db)

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       404,
		CredentialChain: []string{dsl.ModelCredentialScopeProduct},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route != nil {
		t.Fatalf("product-only policy must not add an implicit tenant fallback: %#v", route)
	}
}

func TestResolveCapabilityRouteAutoDoesNotBypassWorkflowPolicyWithDatabaseConfig(t *testing.T) {
	db := setupCapabilityRouterTestDB(t)
	t.Setenv(llmSourceEnv, llmSourceAuto)
	if err := db.Create(&models.AIConfig{
		Name: "legacy shared model", ModelType: enums.AIModelTypeLLM, ModelName: "legacy-model", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	route, err := ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, CapabilityScope{
		TenantID:        7,
		ProductID:       404,
		CredentialChain: []string{dsl.ModelCredentialScopeProduct},
	})
	if route != nil || err == nil || !strings.Contains(err.Error(), "no available credential") {
		t.Fatalf("ResolveCapabilityRouteForScope() = (%#v, %v), want strict workflow-policy error", route, err)
	}
}

func createSub2APIRouteFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&models.SystemConfig{
		ConfigKey:   platformSub2APIHostConfigKey,
		ConfigValue: "https://sub2api.test/",
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Sub2APITenantAccount{
		TenantID:             7,
		Sub2APIAccountID:     "tenant-7",
		DefaultKeyID:         "tenant-default-key",
		DefaultKeyCiphertext: "tenant-key",
		DefaultKeyStatus:     "active",
		ProvisionStatus:      "active",
		Status:               enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
}
