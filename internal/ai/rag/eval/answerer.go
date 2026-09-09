package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/services"
)

type AnswerResult struct {
	Generated        bool
	LatencyMs        int64
	ModelName        string
	PromptTokens     int
	CompletionTokens int
	Content          string
}

type GenerateAnswerFunc func(ctx context.Context, item Case, retrieval *services.KnowledgeRetrievalResult) (*AnswerResult, error)

func DefaultGenerateAnswer(ctx context.Context, item Case, retrieval *services.KnowledgeRetrievalResult) (*AnswerResult, error) {
	if retrieval == nil {
		return &AnswerResult{}, nil
	}
	if strings.TrimSpace(retrieval.ContextText) == "" {
		return &AnswerResult{}, nil
	}

	systemPrompt := strings.TrimSpace(`
You are evaluating a customer-support RAG system for industrial equipment.
Answer only from the provided knowledge context.
If the context is insufficient, the request is unsafe, or the situation needs human diagnosis, say so plainly and suggest contacting support.
Do not invent procedures, part numbers, or fault-code meanings that are not present in the context.
Keep the answer concise and operational.`)

	userPrompt := fmt.Sprintf("Question:\n%s\n\nKnowledge context:\n%s", item.Input.Query, retrieval.ContextText)
	startedAt := time.Now()
	resp, err := ai.LLM.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return &AnswerResult{
		Generated:        true,
		LatencyMs:        time.Since(startedAt).Milliseconds(),
		ModelName:        resp.ModelName,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		Content:          strings.TrimSpace(resp.Content),
	}, nil
}
