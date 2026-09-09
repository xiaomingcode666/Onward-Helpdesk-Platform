package retrievers

import (
	"context"
	"strings"

	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

const defaultRuntimeKnowledgeContextTokens = 4000
const defaultRuntimeKnowledgeTopK = 8
const defaultRuntimeKnowledgeScoreThreshold = 0.3
const defaultRuntimeKnowledgeMaxContextItems = 5

type KnowledgeRetriever struct {
	AIAgent        models.AIAgent
	Scope          dto.KnowledgeScopeContext
	ConversationID int64
	TenantOnly     bool
}

type KnowledgeRetrieveOptions struct {
	ContextMaxTokens int
	MaxContextItems  int
	TopK             int
	ScoreThreshold   float64
	QueryPreview     string
}

type KnowledgeBaseRetrievePolicy struct {
	KnowledgeBaseID int64
	TopK            int
	ScoreThreshold  float64
}

type KnowledgeRetrieveResult struct {
	KnowledgeBaseIDs []int64
	Query            string
	Options          KnowledgeRetrieveOptions
	Hits             []rag.RetrieveResult
	ContextResults   []rag.RetrieveResult
	Citations        []dto.KnowledgeCitation
	ContextText      string
	TopScore         float64
	AnswerMode       enums.KnowledgeAnswerMode
	Trace            *rag.RetrieveTrace
	TraceItems       []callbacks.RetrieverTraceItem
	TraceSummary     callbacks.RetrieverTraceSummary
	Policies         []KnowledgeBaseRetrievePolicy
}

func NewKnowledgeRetriever(aiAgent models.AIAgent, scope ...dto.KnowledgeScopeContext) *KnowledgeRetriever {
	ret := &KnowledgeRetriever{AIAgent: aiAgent}
	if len(scope) > 0 {
		ret.Scope = scope[0]
	}
	return ret
}

func (r *KnowledgeRetriever) WithConversationID(conversationID int64) *KnowledgeRetriever {
	r.ConversationID = conversationID
	return r
}

func (r *KnowledgeRetriever) WithTenantKnowledgeOnly() *KnowledgeRetriever {
	r.TenantOnly = true
	return r
}

func DefaultKnowledgeRetrieveOptions() KnowledgeRetrieveOptions {
	return KnowledgeRetrieveOptions{
		ContextMaxTokens: defaultRuntimeKnowledgeContextTokens,
		MaxContextItems:  defaultRuntimeKnowledgeMaxContextItems,
	}
}

func (r *KnowledgeRetriever) KnowledgeBaseIDs() []int64 {
	return utils.SplitInt64s(r.AIAgent.KnowledgeIDs)
}

func (r *KnowledgeRetriever) Retrieve(ctx context.Context, query string) ([]rag.RetrieveResult, *rag.RetrieveTrace, error) {
	return r.RetrieveByOptions(ctx, DefaultKnowledgeRetrieveOptions(), query)
}

func (r *KnowledgeRetriever) RetrieveByOptions(ctx context.Context, opts KnowledgeRetrieveOptions, query string) ([]rag.RetrieveResult, *rag.RetrieveTrace, error) {
	allowedKnowledgeBaseIDs := r.KnowledgeBaseIDs()
	includeTenantShared := r.includeTenantSharedKnowledge()
	result, err := services.KnowledgeRetrievalService.Retrieve(ctx, services.KnowledgeRetrievalRequest{
		Scope:                   r.Scope,
		AllowedKnowledgeBaseIDs: allowedKnowledgeBaseIDs,
		IncludeTenantShared:     includeTenantShared,
		TenantKnowledgeOnly:     r.TenantOnly,
		Query:                   query,
		TopK:                    opts.TopK,
		ScoreThreshold:          opts.ScoreThreshold,
		RerankTopN:              0,
		ContextMaxTokens:        opts.ContextMaxTokens,
		Scene:                   "runtime",
		ConversationID:          r.ConversationID,
		WriteRetrieveLog:        r.ConversationID > 0,
	})
	if err != nil {
		return nil, nil, err
	}
	if result == nil {
		return nil, nil, nil
	}
	return append([]rag.RetrieveResult(nil), result.RerankedHits...), &rag.RetrieveTrace{
		EmbeddingMs:    result.TimingTrace.EmbeddingMs,
		VectorSearchMs: result.TimingTrace.VectorSearchMs,
		HydrateMs:      result.TimingTrace.HydrateMs,
	}, nil
}

func (r *KnowledgeRetriever) RetrieveContext(ctx context.Context, query string) (*KnowledgeRetrieveResult, error) {
	return r.RetrieveContextByOptions(ctx, DefaultKnowledgeRetrieveOptions(), query)
}

func (r *KnowledgeRetriever) RetrieveContextByOptions(ctx context.Context, opts KnowledgeRetrieveOptions, query string) (*KnowledgeRetrieveResult, error) {
	query = strings.TrimSpace(query)
	knowledgeBaseIDs := r.KnowledgeBaseIDs()
	policies := r.resolvePolicies(knowledgeBaseIDs, opts)
	contextMaxTokens := opts.ContextMaxTokens
	if contextMaxTokens <= 0 {
		contextMaxTokens = defaultRuntimeKnowledgeContextTokens
	}
	maxContextItems := opts.MaxContextItems
	if maxContextItems <= 0 {
		maxContextItems = defaultRuntimeKnowledgeMaxContextItems
	}
	queryPreview := strings.TrimSpace(opts.QueryPreview)
	if queryPreview == "" {
		queryPreview = query
	}
	ret := &KnowledgeRetrieveResult{
		KnowledgeBaseIDs: append([]int64(nil), knowledgeBaseIDs...),
		Query:            query,
		Options: KnowledgeRetrieveOptions{
			ContextMaxTokens: contextMaxTokens,
			MaxContextItems:  maxContextItems,
			TopK:             opts.TopK,
			ScoreThreshold:   opts.ScoreThreshold,
			QueryPreview:     queryPreview,
		},
		Policies: append([]KnowledgeBaseRetrievePolicy(nil), policies...),
	}
	if query == "" || (len(knowledgeBaseIDs) == 0 && r.Scope.ProductID <= 0 && !r.TenantOnly) {
		return ret, nil
	}
	allowedKnowledgeBaseIDs := knowledgeBaseIDs
	includeTenantShared := r.includeTenantSharedKnowledge()
	retrievalResult, err := services.KnowledgeRetrievalService.Retrieve(ctx, services.KnowledgeRetrievalRequest{
		Scope:                   r.Scope,
		AllowedKnowledgeBaseIDs: allowedKnowledgeBaseIDs,
		IncludeTenantShared:     includeTenantShared,
		TenantKnowledgeOnly:     r.TenantOnly,
		Query:                   query,
		TopK:                    opts.TopK,
		ScoreThreshold:          opts.ScoreThreshold,
		RerankTopN:              0,
		ContextMaxTokens:        contextMaxTokens,
		Scene:                   "runtime",
		ConversationID:          r.ConversationID,
		WriteRetrieveLog:        r.ConversationID > 0,
	})
	if err != nil {
		return nil, err
	}
	if retrievalResult == nil {
		return ret, nil
	}
	ret.KnowledgeBaseIDs = append([]int64(nil), retrievalResult.ResolvedScope.KnowledgeBaseIDs...)
	ret.Policies = r.resolvePolicies(ret.KnowledgeBaseIDs, opts)
	trace := &rag.RetrieveTrace{
		EmbeddingMs:    retrievalResult.TimingTrace.EmbeddingMs,
		VectorSearchMs: retrievalResult.TimingTrace.VectorSearchMs,
		HydrateMs:      retrievalResult.TimingTrace.HydrateMs,
	}
	ret.Hits = append([]rag.RetrieveResult(nil), retrievalResult.RerankedHits...)
	ret.Trace = trace
	ret.ContextResults = limitContextResults(append([]rag.RetrieveResult(nil), retrievalResult.ContextHits...), maxContextItems)
	ret.Citations = append([]dto.KnowledgeCitation(nil), retrievalResult.Citations...)
	ret.ContextText = strings.TrimSpace(retrievalResult.ContextText)
	ret.TopScore = retrievalResult.TopScore
	ret.AnswerMode = resolveRuntimeAnswerMode(retrievalResult.ResolvedScope.KnowledgeBaseIDs, retrievalResult.RerankedHits)
	ret.TraceItems = buildRetrieverTraceItems(queryPreview, retrievalResult.RerankedHits, trace)
	ret.TraceSummary = buildRetrieverTraceSummary(ret.Options, ret.Policies, ret.ContextResults, retrievalResult.RerankedHits, trace)
	return ret, nil
}

func (r *KnowledgeRetriever) includeTenantSharedKnowledge() bool {
	if r == nil || r.TenantOnly {
		return true
	}
	return r.Scope.ProductID <= 0 &&
		r.Scope.ProductModelID <= 0 &&
		r.Scope.DeviceID <= 0 &&
		r.Scope.ServiceCodeID <= 0
}

func ScopeFromConversation(conversation models.Conversation) dto.KnowledgeScopeContext {
	locale := "default"
	if conversation.CustomerEntrySessionID > 0 {
		entrySession := repositories.CustomerEntrySessionRepository.Get(sqls.DB(), conversation.CustomerEntrySessionID)
		if entrySession != nil && entrySession.TenantID == conversation.TenantID && (conversation.ProductID <= 0 || entrySession.ProductID == conversation.ProductID) {
			if sessionLocale := strings.TrimSpace(entrySession.Locale); sessionLocale != "" {
				locale = sessionLocale
			}
		}
	}
	return dto.KnowledgeScopeContext{
		TenantID:       conversation.TenantID,
		ProductID:      conversation.ProductID,
		ProductModelID: conversation.ProductModelID,
		DeviceID:       conversation.DeviceID,
		ServiceCodeID:  conversation.ServiceCodeID,
		Locale:         locale,
		RegionCode:     "",
		Audience:       "customer",
	}
}

func limitContextResults(results []rag.RetrieveResult, maxItems int) []rag.RetrieveResult {
	if len(results) == 0 {
		return nil
	}
	if maxItems <= 0 || len(results) <= maxItems {
		return append([]rag.RetrieveResult(nil), results...)
	}
	return append([]rag.RetrieveResult(nil), results[:maxItems]...)
}

func buildContextText(results []rag.RetrieveResult) string {
	if len(results) == 0 {
		return ""
	}
	return strings.TrimSpace(rag.Retrieve.BuildContext(context.Background(), results, 1<<30))
}

func resolveTopScore(results []rag.RetrieveResult) float64 {
	if len(results) == 0 {
		return 0
	}
	return float64(results[0].Score)
}

func (r *KnowledgeRetriever) resolvePolicies(knowledgeBaseIDs []int64, opts KnowledgeRetrieveOptions) []KnowledgeBaseRetrievePolicy {
	if len(knowledgeBaseIDs) == 0 {
		return nil
	}
	knowledgeBases := loadRuntimeKnowledgeBases(knowledgeBaseIDs)
	ret := make([]KnowledgeBaseRetrievePolicy, 0, len(knowledgeBaseIDs))
	for _, knowledgeBaseID := range knowledgeBaseIDs {
		policy := KnowledgeBaseRetrievePolicy{
			KnowledgeBaseID: knowledgeBaseID,
			TopK:            defaultRuntimeKnowledgeTopK,
			ScoreThreshold:  defaultRuntimeKnowledgeScoreThreshold,
		}
		if knowledgeBase, ok := knowledgeBases[knowledgeBaseID]; ok {
			if knowledgeBase.DefaultTopK > 0 {
				policy.TopK = knowledgeBase.DefaultTopK
			}
			if knowledgeBase.DefaultScoreThreshold > 0 {
				policy.ScoreThreshold = knowledgeBase.DefaultScoreThreshold
			}
		}
		if opts.TopK > 0 {
			policy.TopK = opts.TopK
		}
		if opts.ScoreThreshold > 0 {
			policy.ScoreThreshold = opts.ScoreThreshold
		}
		ret = append(ret, policy)
	}
	return ret
}

func resolveRuntimeAnswerMode(knowledgeBaseIDs []int64, results []rag.RetrieveResult) enums.KnowledgeAnswerMode {
	knowledgeBases := loadRuntimeKnowledgeBases(knowledgeBaseIDs)
	if len(knowledgeBases) == 0 {
		return enums.KnowledgeAnswerModeStrict
	}
	if len(results) > 0 {
		if knowledgeBase, ok := knowledgeBases[results[0].KnowledgeBaseID]; ok {
			return normalizeRuntimeAnswerMode(knowledgeBase)
		}
	}
	for _, knowledgeBaseID := range knowledgeBaseIDs {
		if knowledgeBase, ok := knowledgeBases[knowledgeBaseID]; ok {
			return normalizeRuntimeAnswerMode(knowledgeBase)
		}
	}
	return enums.KnowledgeAnswerModeStrict
}

func normalizeRuntimeAnswerMode(knowledgeBase models.KnowledgeBase) enums.KnowledgeAnswerMode {
	answerMode := enums.KnowledgeAnswerMode(knowledgeBase.AnswerMode)
	if answerMode == 0 {
		answerMode = enums.KnowledgeAnswerModeStrict
	}
	return answerMode
}

func loadRuntimeKnowledgeBases(ids []int64) map[int64]models.KnowledgeBase {
	if len(ids) == 0 {
		return nil
	}
	items := repositories.KnowledgeBaseRepository.Find(sqls.DB(), sqls.NewCnd().In("id", ids))
	if len(items) == 0 {
		return nil
	}
	ret := make(map[int64]models.KnowledgeBase, len(items))
	for _, item := range items {
		if item.Status != enums.StatusOk {
			continue
		}
		ret[item.ID] = item
	}
	return ret
}

func buildRetrieverTraceItems(queryPreview string, results []rag.RetrieveResult, trace *rag.RetrieveTrace) []callbacks.RetrieverTraceItem {
	if len(results) == 0 {
		return nil
	}
	latencyMs := int64(0)
	if trace != nil {
		latencyMs = trace.EmbeddingMs + trace.VectorSearchMs + trace.HydrateMs
	}
	ret := make([]callbacks.RetrieverTraceItem, 0, len(results))
	for _, item := range results {
		ret = append(ret, callbacks.RetrieverTraceItem{
			Query:           queryPreview,
			KnowledgeBaseID: item.KnowledgeBaseID,
			DocumentID:      item.DocumentID,
			DocumentTitle:   item.DocumentTitle,
			Score:           float64(item.Score),
			LatencyMs:       latencyMs,
		})
	}
	return ret
}

func buildRetrieverTraceSummary(opts KnowledgeRetrieveOptions, policies []KnowledgeBaseRetrievePolicy, contextResults []rag.RetrieveResult, results []rag.RetrieveResult, trace *rag.RetrieveTrace) callbacks.RetrieverTraceSummary {
	ret := callbacks.RetrieverTraceSummary{
		TopK:             opts.TopK,
		ScoreThreshold:   opts.ScoreThreshold,
		ContextMaxTokens: opts.ContextMaxTokens,
		MaxContextItems:  opts.MaxContextItems,
		HitCount:         len(results),
		ContextCount:     len(contextResults),
		Policies:         buildRetrieverPolicyTraceItems(policies),
	}
	if ret.TopK <= 0 && len(policies) == 1 {
		ret.TopK = policies[0].TopK
	}
	if ret.ScoreThreshold <= 0 && len(policies) == 1 {
		ret.ScoreThreshold = policies[0].ScoreThreshold
	}
	if trace != nil {
		ret.EmbeddingMs = trace.EmbeddingMs
		ret.VectorSearchMs = trace.VectorSearchMs
		ret.HydrateMs = trace.HydrateMs
	}
	return ret
}

func buildRetrieverPolicyTraceItems(policies []KnowledgeBaseRetrievePolicy) []callbacks.RetrieverPolicyTraceItem {
	if len(policies) == 0 {
		return nil
	}
	ret := make([]callbacks.RetrieverPolicyTraceItem, 0, len(policies))
	for _, item := range policies {
		ret = append(ret, callbacks.RetrieverPolicyTraceItem{
			KnowledgeBaseID: item.KnowledgeBaseID,
			TopK:            item.TopK,
			ScoreThreshold:  item.ScoreThreshold,
		})
	}
	return ret
}
