package ai

import (
	"context"
	"encoding/json"
	"testing"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestApplyProviderSpecificChatParamsIncludesDashScopeThinkingFlag(t *testing.T) {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String("hello"),
					},
				},
			},
		},
		Model: shared.ChatModel("qwen3.5-plus"),
	}

	applyProviderSpecificChatParams(&params, models.AIConfig{
		BaseURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
		ModelName: "qwen3.5-plus",
	})

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got, ok := body["enable_thinking"].(bool); !ok || got {
		t.Fatalf("expected enable_thinking=false in request body, got body=%s", raw)
	}
}

func TestChatParamsUseBoundedDefaultForSub2API(t *testing.T) {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{OfString: openai.String("hello")},
			},
		}},
		Model: shared.ChatModel("gpt-5.5"),
	}
	config := models.AIConfig{Provider: enums.AIProviderSub2API, ModelName: "gpt-5.5"}
	applyChatOutputLimit(&params, config)
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got, ok := body["max_completion_tokens"].(float64); !ok || got != 2048 {
		t.Fatalf("expected max_completion_tokens=2048, got body=%s", raw)
	}
}

func TestApplyReasoningEffortOnlyWhenRequested(t *testing.T) {
	params := openai.ChatCompletionNewParams{}
	applyReasoningEffort(context.Background(), &params)
	if params.ReasoningEffort != "" {
		t.Fatalf("default reasoning effort = %q, want provider default", params.ReasoningEffort)
	}
	applyReasoningEffort(WithReasoningEffort(context.Background(), "low"), &params)
	if params.ReasoningEffort != "low" {
		t.Fatalf("reasoning effort = %q, want low", params.ReasoningEffort)
	}
}
