package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
)

var KnowledgeDebugService = newKnowledgeDebugService()

type knowledgeDebugService struct {
	chat func(context.Context, string, string) (*ai.ChatCompletionResult, error)
}

func newKnowledgeDebugService() *knowledgeDebugService {
	return &knowledgeDebugService{chat: ai.LLM.Chat}
}

func (s *knowledgeDebugService) DebugSearch(ctx context.Context, req request.KnowledgeSearchRequest, operator *dto.AuthPrincipal) (*response.KnowledgeSearchResponse, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, errorsx.InvalidParamI18n("error.e0340")
	}
	startedAt := time.Now()
	result, err := KnowledgeRetrievalService.Retrieve(ctx, buildDebugRetrievalRequest(req, operator, true))
	if err != nil {
		return nil, err
	}
	hits := buildKnowledgeSearchResponses(result.RerankedHits)
	return &response.KnowledgeSearchResponse{
		Question:  strings.TrimSpace(req.Question),
		Results:   hits,
		HitCount:  len(hits),
		LatencyMs: time.Since(startedAt).Milliseconds(),
	}, nil
}

func (s *knowledgeDebugService) DebugAnswer(ctx context.Context, req request.KnowledgeAnswerRequest, operator *dto.AuthPrincipal) (*response.KnowledgeAnswerResponse, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, errorsx.InvalidParamI18n("error.e0340")
	}
	startedAt := time.Now()
	retrieveStartedAt := time.Now()
	retrievalReq := buildDebugAnswerRetrievalRequest(req, operator)
	result, err := KnowledgeRetrievalService.Retrieve(ctx, retrievalReq)
	if err != nil {
		return nil, err
	}
	retrieveMs := time.Since(retrieveStartedAt).Milliseconds()

	status := enums.KnowledgeAnswerStatusNormal
	answer := ""
	modelName := ""
	promptTokens := 0
	completionTokens := 0
	generateStartedAt := time.Now()
	if strings.TrimSpace(result.ContextText) == "" {
		status = enums.KnowledgeAnswerStatusNoAnswer
		answer = debugNoAnswerText(req.Locale)
	} else {
		answerMode := resolveDebugAnswerMode(req.AnswerMode, result.ResolvedScope.KnowledgeBaseIDs)
		llmCtx := ai.WithCapabilityScope(ctx, ai.CapabilityScope{
			TenantID:        result.ResolvedScope.Context.TenantID,
			ProductID:       result.ResolvedScope.Context.ProductID,
			KnowledgeBaseID: firstInt64(result.ResolvedScope.KnowledgeBaseIDs),
		})
		chat := s.chat
		if chat == nil {
			chat = ai.LLM.Chat
		}
		llmResult, llmErr := chat(
			llmCtx,
			debugAnswerSystemPrompt(answerMode, req.Locale),
			debugAnswerUserPrompt(req.Locale, req.Question, result.ContextText),
		)
		if llmErr != nil || llmResult == nil || strings.TrimSpace(llmResult.Content) == "" {
			status = enums.KnowledgeAnswerStatusFallback
			answer = debugEvidenceFallbackText(req.Locale, result)
		} else {
			answer = strings.TrimSpace(llmResult.Content)
			modelName = llmResult.ModelName
			promptTokens = llmResult.PromptTokens
			completionTokens = llmResult.CompletionTokens
		}
	}
	generateMs := time.Since(generateStartedAt).Milliseconds()

	KnowledgeRetrievalService.writeRetrieveLog(ctx, retrievalReq, result, knowledgeRetrieveLogExtras{
		Answer:           answer,
		AnswerStatus:     int(status),
		GenerateMs:       generateMs,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        modelName,
		TotalLatencyMs:   time.Since(startedAt).Milliseconds(),
	})

	hits := buildKnowledgeSearchResponses(result.RerankedHits)
	return &response.KnowledgeAnswerResponse{
		Question:         strings.TrimSpace(req.Question),
		Answer:           answer,
		AnswerStatus:     int(status),
		AnswerStatusName: enums.GetKnowledgeAnswerStatusLabel(status),
		Citations:        buildKnowledgeCitationResponses(result.Citations),
		Hits:             hits,
		HitCount:         len(hits),
		TopScore:         result.TopScore,
		LatencyMs:        time.Since(startedAt).Milliseconds(),
		RetrieveMs:       retrieveMs,
		GenerateMs:       generateMs,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        modelName,
		RetrieveLogID:    result.RetrieveLogID,
	}, nil
}

func buildDebugRetrievalRequest(req request.KnowledgeSearchRequest, operator *dto.AuthPrincipal, writeLog bool) KnowledgeRetrievalRequest {
	return KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID:       resolveDebugTenantID(req.TenantID, operator),
			ProductID:      req.ProductID,
			ProductModelID: req.ProductModelID,
			Locale:         req.Locale,
			RegionCode:     req.RegionCode,
			Audience:       normalizeDebugAudience(req.Audience),
		},
		AllowedKnowledgeBaseIDs: append([]int64(nil), req.KnowledgeBaseIDs...),
		Query:                   req.Question,
		TopK:                    req.TopK,
		ScoreThreshold:          req.ScoreThreshold,
		RerankTopN:              req.RerankLimit,
		Channel:                 req.Channel,
		Scene:                   req.Scene,
		SessionID:               req.SessionID,
		ConversationID:          req.ConversationID,
		WriteRetrieveLog:        writeLog,
	}
}

func buildDebugAnswerRetrievalRequest(req request.KnowledgeAnswerRequest, operator *dto.AuthPrincipal) KnowledgeRetrievalRequest {
	return buildDebugRetrievalRequest(request.KnowledgeSearchRequest{
		TenantID:         req.TenantID,
		ProductID:        req.ProductID,
		ProductModelID:   req.ProductModelID,
		KnowledgeBaseIDs: append([]int64(nil), req.KnowledgeBaseIDs...),
		Locale:           req.Locale,
		RegionCode:       req.RegionCode,
		Audience:         req.Audience,
		Question:         req.Question,
		TopK:             req.TopK,
		ScoreThreshold:   req.ScoreThreshold,
		RerankLimit:      req.RerankLimit,
		Channel:          req.Channel,
		Scene:            req.Scene,
		SessionID:        req.SessionID,
		ConversationID:   req.ConversationID,
	}, operator, false)
}

func resolveDebugTenantID(requestTenantID int64, operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return 0
	}
	if operator.TenantID > 0 {
		return operator.TenantID
	}
	if operator.TargetTenantID > 0 {
		return operator.TargetTenantID
	}
	return requestTenantID
}

func normalizeDebugAudience(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "agent", "engineer":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "customer"
	}
}

func resolveDebugAnswerMode(requested int, knowledgeBaseIDs []int64) enums.KnowledgeAnswerMode {
	mode := enums.KnowledgeAnswerMode(requested)
	if mode == enums.KnowledgeAnswerModeStrict || mode == enums.KnowledgeAnswerModeAssist {
		return mode
	}
	if knowledgeBase := resolvePrimaryKnowledgeBase(knowledgeBaseIDs); knowledgeBase != nil {
		mode = enums.KnowledgeAnswerMode(knowledgeBase.AnswerMode)
		if mode == enums.KnowledgeAnswerModeStrict || mode == enums.KnowledgeAnswerModeAssist {
			return mode
		}
	}
	return enums.KnowledgeAnswerModeStrict
}

func debugAnswerSystemPrompt(mode enums.KnowledgeAnswerMode, locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		if mode == enums.KnowledgeAnswerModeAssist {
			return "你是客服知识库助手。请优先依据提供的知识片段回答，可以做轻度归纳，但不要编造未提供的事实。请使用中文回答。"
		}
		return "你是严格的客服知识库助手。只能依据提供的知识片段回答；如果资料不足，请明确说明知识库暂无明确信息。请使用中文回答。"
	}
	if mode == enums.KnowledgeAnswerModeAssist {
		return "You are a customer service knowledge assistant. Base the answer primarily on the supplied knowledge excerpts. You may summarize them lightly, but do not invent facts. Respond in English."
	}
	return "You are a strict customer service knowledge assistant. Answer only from the supplied knowledge excerpts. If the information is insufficient, say clearly that the knowledge base has no definitive answer. Respond in English."
}

func debugAnswerUserPrompt(locale, question, contextText string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return fmt.Sprintf("用户问题：%s\n\n参考资料：\n%s", strings.TrimSpace(question), contextText)
	}
	return fmt.Sprintf("User question: %s\n\nKnowledge excerpts:\n%s", strings.TrimSpace(question), contextText)
}

func debugNoAnswerText(locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return "当前知识库暂无明确信息。"
	}
	return "The current knowledge base does not contain enough information to answer this question."
}

func debugEvidenceFallbackText(locale string, result *KnowledgeRetrievalResult) string {
	if result == nil || len(result.ContextHits) == 0 {
		return debugNoAnswerText(locale)
	}
	hit := result.ContextHits[0]
	title := firstNonEmptyString(
		strings.TrimSpace(hit.DocumentTitle),
		strings.TrimSpace(hit.Title),
		strings.TrimSpace(hit.SectionPath),
	)
	content := strings.TrimSpace(hit.Content)
	if content == "" {
		return debugNoAnswerText(locale)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return fmt.Sprintf("AI 生成服务暂时不可用。根据知识库《%s》：%s", title, content)
	}
	return fmt.Sprintf("AI generation is temporarily unavailable. According to the knowledge base entry %q: %s", title, content)
}
