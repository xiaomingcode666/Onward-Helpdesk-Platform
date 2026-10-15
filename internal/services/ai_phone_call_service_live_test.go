package services

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// TestLiveAIPhoneDecision sends the production system prompt through the
// tenant's configured LLM route and checks the reply parses as a decision.
func TestLiveAIPhoneDecision(t *testing.T) {
	if os.Getenv("RUN_AI_PHONE_LIVE_TEST") != "1" {
		t.Skip("set RUN_AI_PHONE_LIVE_TEST=1 (and AI_PHONE_LIVE_TENANT_ID) to call the configured LLM")
	}
	tenantID, err := strconv.ParseInt(os.Getenv("AI_PHONE_LIVE_TENANT_ID"), 10, 64)
	if err != nil || tenantID <= 0 {
		t.Fatal("AI_PHONE_LIVE_TENANT_ID must be a tenant id")
	}
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET default_transaction_read_only=on").Error; err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)

	svc := newAIPhoneCallService()
	if effort := os.Getenv("AI_PHONE_LIVE_REASONING_EFFORT"); effort != "" {
		svc.reasoningEffort = effort
	}
	if model := os.Getenv("AI_PHONE_LIVE_MODEL"); model != "" {
		// Compare models without changing the tenant's stored default.
		svc.chat = func(ctx context.Context, system, user string) (*ai.ChatCompletionResult, error) {
			route, err := ai.ResolveCapabilityRoute(ctx, "llm")
			if err != nil || route == nil {
				t.Fatalf("resolve route: %v", err)
			}
			cfg := route.Config
			cfg.ModelName = model
			if baseURL := os.Getenv("AI_PHONE_LIVE_BASE_URL"); baseURL != "" {
				cfg = models.AIConfig{Provider: enums.AIProviderOpenAI, ModelType: enums.AIModelTypeLLM, ModelName: model, BaseURL: baseURL, APIKey: os.Getenv(os.Getenv("AI_PHONE_LIVE_API_KEY_ENV")), MaxOutputTokens: 512}
			}
			return ai.LLM.ChatWithConfig(ctx, cfg, system, user)
		}
	}
	call := &aiPhoneCall{
		callID: "live-test",
		line:   aiPhoneLine{LineID: "line-1", ProjectKey: "ai-phone", Language: "zh-CN", TenantID: tenantID},
		fields: map[string]string{},
		history: []aiPhoneTurn{
			{Role: "agent", Text: aiPhoneLanguages["zh-CN"].greeting},
		},
	}
	for _, turn := range []string{"作为20名初中的感觉设备受冷遇", "我的打印机开不了机，设备编号是 P1001", "对，帮我创建工单"} {
		call.history = append(call.history, aiPhoneTurn{Role: "customer", Text: turn})
		started := time.Now()
		decision, err := svc.decide(context.Background(), call, turn)
		if err != nil {
			t.Fatalf("turn %q: %v", turn, err)
		}
		t.Logf("turn %q -> action=%s reply=%q ticket=%+v elapsed=%s", turn, decision.Action, decision.ReplyText, decision.Ticket, time.Since(started).Round(time.Millisecond))
		if decision.Action == aiPhoneActionPrepareTicket {
			call.pendingTicket = decision.Ticket
		}
		call.history = append(call.history, aiPhoneTurn{Role: "agent", Text: decision.ReplyText})
	}
}
