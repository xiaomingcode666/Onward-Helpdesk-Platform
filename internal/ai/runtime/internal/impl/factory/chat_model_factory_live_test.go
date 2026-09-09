package factory

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/cloudwego/eino/schema"
)

func TestChatModelFactoryLiveFailover(t *testing.T) {
	if os.Getenv("RUN_LIVE_AI_TESTS") != "1" {
		t.Skip("set RUN_LIVE_AI_TESTS=1 to call the configured fallback model")
	}
	if strings.TrimSpace(os.Getenv("LLM_FALLBACK_MODEL")) == "" {
		t.Fatal("LLM_FALLBACK_MODEL is required")
	}
	t.Setenv("LLM_FAILOVER_ATTEMPT_TIMEOUT", "2s")
	t.Setenv("LLM_FAILOVER_CIRCUIT_COOLDOWN", "10s")

	chatModel, err := NewChatModelFactory().Build(context.Background(), models.AIConfig{
		Name:          "Unavailable primary",
		Provider:      enums.AIProviderOpenAI,
		BaseURL:       "http://127.0.0.1:1/v1",
		APIKey:        "unavailable-primary",
		ModelType:     enums.AIModelTypeLLM,
		ModelName:     "unavailable-primary",
		TimeoutMS:     500,
		MaxRetryCount: 0,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	message, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage("只回复 OK")})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if strings.TrimSpace(message.Content) == "" {
		t.Fatal("fallback returned an empty response")
	}
	if message.Extra[ActualModelExtraKey] != strings.TrimSpace(os.Getenv("LLM_FALLBACK_MODEL")) {
		t.Fatalf("actual model metadata = %#v", message.Extra)
	}
}
