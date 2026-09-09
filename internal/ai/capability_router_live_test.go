package ai_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
)

func TestLiveUnifiedLLMRoute(t *testing.T) {
	if os.Getenv("RUN_AI_CAPABILITY_LIVE_TEST") != "1" {
		t.Skip("set RUN_AI_CAPABILITY_LIVE_TEST=1 to call the configured LLM capability")
	}
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	scope := ai.CapabilityScope{TenantID: 1}
	ctx = ai.WithCapabilityScope(ctx, scope)
	route, err := ai.ResolveCapabilityRouteForScope(ctx, "llm", scope)
	if err != nil {
		t.Fatal(err)
	}
	models, err := ai.ListCapabilityModels(ctx, route)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) < 2 {
		t.Fatalf("unified LLM model catalog returned %d model(s): %v", len(models), models)
	}
	t.Logf("unified LLM model catalog (%d): %s", len(models), strings.Join(models, ", "))
	result, err := ai.LLM.Chat(ctx, "Return a concise plain-text response.", "Reply with OK.")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		t.Fatal("LLM capability returned no content")
	}
	t.Logf("unified LLM route model=%s response=%q", result.ModelName, result.Content)
}
