package services

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var KnowledgeRetrievalService = newKnowledgeRetrievalService()

func newKnowledgeRetrievalService() *knowledgeRetrievalService {
	ret := &knowledgeRetrievalService{
		resolveScope: func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
			return ProductKnowledgeResolver.ResolveScope(scope)
		},
		resolveTenantScope: func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
			return ProductKnowledgeResolver.ResolveTenantScope(scope)
		},
		generateEmbedding: ai.Embedding.GenerateEmbedding,
		getProvider:       vectordb.GetProvider,
		getRAGConfig: func() config.RAGConfig {
			return config.CurrentOrDefault().RAG.Normalized()
		},
		rerankAvailable: func(ctx context.Context) bool {
			route, err := ai.ResolveCapabilityRoute(ctx, enums.AIModelTypeRerank)
			return err == nil && route != nil
		},
		rerankResults: rag.Rerank.RerankResults,
	}
	ret.lexicalSearch = func(ctx context.Context, req knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error) {
		return ret.searchLexicalResults(ctx, req)
	}
	return ret
}

type knowledgeRetrievalService struct {
	resolveScope       func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error)
	resolveTenantScope func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error)
	generateEmbedding  func(ctx context.Context, text string) (*ai.EmbeddingResult, error)
	getProvider        func() vectordb.Provider
	getRAGConfig       func() config.RAGConfig
	rerankAvailable    func(ctx context.Context) bool
	rerankResults      func(ctx context.Context, query string, results []rag.RetrieveResult, topN int) ([]rag.RetrieveResult, error)
	lexicalSearch      func(ctx context.Context, req knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error)
}

type KnowledgeRetrievalRequest struct {
	Scope                   dto.KnowledgeScopeContext
	AllowedKnowledgeBaseIDs []int64
	IncludeTenantShared     bool
	TenantKnowledgeOnly     bool
	Query                   string
	TopK                    int
	ScoreThreshold          float64
	RerankTopN              int
	ContextMaxTokens        int
	Channel                 string
	Scene                   string
	SessionID               string
	ConversationID          int64
	WriteRetrieveLog        bool
}

type KnowledgeRetrievalTimingTrace struct {
	ScopeResolveMs int64 `json:"scopeResolveMs"`
	EmbeddingMs    int64 `json:"embeddingMs"`
	VectorSearchMs int64 `json:"vectorSearchMs"`
	HydrateMs      int64 `json:"hydrateMs"`
	LexicalMs      int64 `json:"lexicalMs"`
	RerankMs       int64 `json:"rerankMs"`
}

type KnowledgeRetrievalStrategyTrace struct {
	RequestedMode        string   `json:"requestedMode"`
	AppliedMode          string   `json:"appliedMode"`
	HybridEnabled        bool     `json:"hybridEnabled"`
	DenseHitCount        int      `json:"denseHitCount"`
	DenseTopScore        float64  `json:"denseTopScore"`
	LexicalTriggered     bool     `json:"lexicalTriggered"`
	LexicalReason        string   `json:"lexicalReason,omitempty"`
	LexicalTerms         []string `json:"lexicalTerms,omitempty"`
	ExactMatchTerms      []string `json:"exactMatchTerms,omitempty"`
	LexicalHitCount      int      `json:"lexicalHitCount"`
	FusionHitCount       int      `json:"fusionHitCount"`
	RerankRequested      bool     `json:"rerankRequested"`
	RerankApplied        bool     `json:"rerankApplied"`
	RerankReason         string   `json:"rerankReason,omitempty"`
	RerankCandidateCount int      `json:"rerankCandidateCount"`
}

type KnowledgeRetrievalResult struct {
	ResolvedScope     dto.ResolvedKnowledgeScope      `json:"resolvedScope"`
	RawHits           []rag.RetrieveResult            `json:"rawHits"`
	RerankedHits      []rag.RetrieveResult            `json:"rerankedHits"`
	ContextHits       []rag.RetrieveResult            `json:"contextHits"`
	ContextText       string                          `json:"contextText"`
	Citations         []dto.KnowledgeCitation         `json:"citations"`
	TopScore          float64                         `json:"topScore"`
	NoAnswerReason    string                          `json:"noAnswerReason"`
	TimingTrace       KnowledgeRetrievalTimingTrace   `json:"timingTrace"`
	Strategy          KnowledgeRetrievalStrategyTrace `json:"strategy"`
	CollectionName    string                          `json:"collectionName"`
	IndexGenerationID int64                           `json:"indexGenerationId"`
	EmbeddingModel    string                          `json:"embeddingModel"`
	RetrieveLogID     int64                           `json:"retrieveLogId"`
}

type knowledgeLexicalSearchRequest struct {
	Scope               dto.ResolvedKnowledgeScope
	KnowledgeBaseIDs    []int64
	Query               string
	Terms               []string
	ExactTerms          []string
	IndexGenerationID   int64
	TenantKnowledgeOnly bool
	Limit               int
}

type knowledgeRetrieveLogExtras struct {
	Answer           string
	AnswerStatus     int
	GenerateMs       int64
	PromptTokens     int
	CompletionTokens int
	ModelName        string
	TotalLatencyMs   int64
}

func firstInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

const (
	knowledgeRetrievalModeAuto            = "auto"
	knowledgeRetrievalModeDenseOnly       = "dense_only"
	knowledgeRetrievalModeLexicalOnly     = "lexical_only"
	knowledgeRetrievalModeHybridRRF       = "hybrid_rrf"
	knowledgeRetrievalModeHybridRRFRerank = "hybrid_rrf_rerank"
)

var knowledgeLexicalTokenPattern = regexp.MustCompile(`[[:alnum:]][[:alnum:]._/-]{1,63}`)

func (s *knowledgeRetrievalService) Retrieve(ctx context.Context, req KnowledgeRetrievalRequest) (*KnowledgeRetrievalResult, error) {
	finish := func(result *KnowledgeRetrievalResult) (*KnowledgeRetrievalResult, error) {
		s.maybeWriteRetrieveLog(ctx, req, result)
		return result, nil
	}

	query := strings.TrimSpace(req.Query)
	if req.Scope.TenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if query == "" {
		return finish(&KnowledgeRetrievalResult{
			ResolvedScope:  dto.ResolvedKnowledgeScope{Context: req.Scope},
			NoAnswerReason: "empty-query",
		})
	}

	result := &KnowledgeRetrievalResult{}
	scopeStartedAt := time.Now()
	resolveScope := s.resolveScope
	if req.TenantKnowledgeOnly {
		resolveScope = s.resolveTenantScope
	}
	if resolveScope == nil {
		return nil, fmt.Errorf("knowledge scope resolver is not configured")
	}
	resolvedScope, err := resolveScope(req.Scope)
	result.TimingTrace.ScopeResolveMs = time.Since(scopeStartedAt).Milliseconds()
	if err != nil {
		return nil, err
	}
	result.ResolvedScope = *resolvedScope
	if len(resolvedScope.EntryKeys) == 0 || len(resolvedScope.RevisionIDs) == 0 {
		result.NoAnswerReason = "no-scope"
		return finish(result)
	}

	effectiveKnowledgeBaseIDs := append([]int64(nil), resolvedScope.KnowledgeBaseIDs...)
	productStrictScope := !req.IncludeTenantShared && !req.TenantKnowledgeOnly && resolvedScope.Context.ProductID > 0
	if productStrictScope {
		switch {
		case len(resolvedScope.ProductKnowledgeBaseIDs) > 0:
			effectiveKnowledgeBaseIDs = append([]int64(nil), resolvedScope.ProductKnowledgeBaseIDs...)
		case len(resolvedScope.TenantSharedKnowledgeBaseIDs) > 0:
			effectiveKnowledgeBaseIDs = nil
		}
	}
	if len(req.AllowedKnowledgeBaseIDs) > 0 {
		if req.IncludeTenantShared {
			selectedProductKnowledgeBaseIDs := intersectInt64s(
				resolvedScope.ProductKnowledgeBaseIDs,
				req.AllowedKnowledgeBaseIDs,
			)
			effectiveKnowledgeBaseIDs = unionInt64s(
				resolvedScope.TenantSharedKnowledgeBaseIDs,
				selectedProductKnowledgeBaseIDs,
			)
			result.ResolvedScope.ProductKnowledgeBaseIDs = append([]int64(nil), selectedProductKnowledgeBaseIDs...)
		} else {
			effectiveKnowledgeBaseIDs = intersectInt64s(effectiveKnowledgeBaseIDs, req.AllowedKnowledgeBaseIDs)
		}
	}
	if len(effectiveKnowledgeBaseIDs) == 0 {
		result.NoAnswerReason = "no-scope"
		return finish(result)
	}
	result.ResolvedScope.KnowledgeBaseIDs = append([]int64(nil), effectiveKnowledgeBaseIDs...)
	if productStrictScope {
		result.ResolvedScope.TenantSharedKnowledgeBaseIDs = nil
	}

	ragCfg := s.currentRAGConfig()
	result.Strategy.RequestedMode = normalizeKnowledgeRetrievalMode(ragCfg.RetrievalMode)
	result.Strategy.HybridEnabled = ragCfg.HybridEnabled
	lexicalTerms, exactTerms := extractKnowledgeLexicalTerms(query)
	result.Strategy.LexicalTerms = append([]string(nil), lexicalTerms...)
	result.Strategy.ExactMatchTerms = append([]string(nil), exactTerms...)
	topK := req.TopK
	if topK <= 0 {
		topK = ragCfg.DenseTopK
	}
	scoreThreshold := float32(req.ScoreThreshold)
	if scoreThreshold <= 0 {
		scoreThreshold = 0.3
	}
	result.CollectionName = ragCfg.ActiveCollectionAlias
	activeGeneration := KnowledgeIndexGenerationService.GetActiveGeneration()
	if activeGeneration != nil {
		result.IndexGenerationID = activeGeneration.ID
		if strings.TrimSpace(activeGeneration.CollectionName) != "" {
			result.CollectionName = activeGeneration.CollectionName
		}
	}

	var (
		searchResults       []vectordb.SearchResult
		denseErr            error
		denseErrDegradation = true
	)
	provider := s.getProvider()
	if provider == nil {
		denseErr = fmt.Errorf("vectordb provider not initialized")
	} else {
		embeddingStartedAt := time.Now()
		embeddingCtx := ai.WithCapabilityScope(ctx, ai.CapabilityScope{
			TenantID:        resolvedScope.Context.TenantID,
			ProductID:       resolvedScope.Context.ProductID,
			KnowledgeBaseID: firstInt64(effectiveKnowledgeBaseIDs),
		})
		embeddingResult, embeddingErr := s.generateEmbedding(embeddingCtx, query)
		result.TimingTrace.EmbeddingMs = time.Since(embeddingStartedAt).Milliseconds()
		if embeddingErr != nil {
			denseErr = fmt.Errorf("generate query embedding failed: %w", embeddingErr)
		} else if activeGeneration != nil && activeGeneration.Dimension > 0 && len(embeddingResult.Vector) != activeGeneration.Dimension {
			denseErr = fmt.Errorf("query embedding dimension %d does not match active knowledge index generation %d dimension %d", len(embeddingResult.Vector), activeGeneration.ID, activeGeneration.Dimension)
			denseErrDegradation = false
		} else {
			result.EmbeddingModel = embeddingResult.ModelName
			searchStartedAt := time.Now()
			searchResults, denseErr = provider.Search(ctx, &vectordb.SearchRequest{
				CollectionName: result.CollectionName,
				Vector:         embeddingResult.Vector,
				TopK:           topK,
				ScoreThreshold: scoreThreshold,
				Filter: &vectordb.SearchFilter{
					TenantID:          resolvedScope.Context.TenantID,
					EntryKeys:         append([]string(nil), resolvedScope.EntryKeys...),
					KnowledgeBaseIDs:  append([]int64(nil), effectiveKnowledgeBaseIDs...),
					Languages:         resolvedKnowledgeScopeLanguages(*resolvedScope),
					ReviewStatus:      "published",
					RevisionIDs:       append([]int64(nil), resolvedScope.RevisionIDs...),
					ScopeKeys:         rag.BuildKnowledgeRuntimeScopeKeys(resolvedScope.Context.TenantID, resolvedScope.Context.ProductID),
					AllowLegacyScope:  true,
					IndexGenerationID: result.IndexGenerationID,
				},
			})
			result.TimingTrace.VectorSearchMs = time.Since(searchStartedAt).Milliseconds()
		}
	}
	if denseErr != nil {
		slog.Warn("knowledge dense retrieval unavailable; attempting lexical fallback",
			"tenant_id", req.Scope.TenantID,
			"product_id", req.Scope.ProductID,
			"query", truncateForLog(query, 80),
			"error", denseErr)
	} else {
		hydrateStartedAt := time.Now()
		result.RawHits = s.hydrateResults(searchResults, *resolvedScope)
		result.TimingTrace.HydrateMs = time.Since(hydrateStartedAt).Milliseconds()
	}
	result.Strategy.DenseHitCount = len(result.RawHits)
	result.Strategy.DenseTopScore = topRetrieveScore(result.RawHits)

	candidates := append([]rag.RetrieveResult(nil), result.RawHits...)
	runLexical, lexicalReason := shouldRunKnowledgeLexicalRescue(result.Strategy.RequestedMode, ragCfg, result.RawHits, exactTerms)
	if denseErr != nil {
		if ragCfg.LexicalTopK <= 0 || len(lexicalTerms) == 0 || (!req.TenantKnowledgeOnly && !ragCfg.HybridEnabled) {
			if denseErrDegradation || req.TenantKnowledgeOnly {
				result.NoAnswerReason = "dense-unavailable"
				result.Strategy.AppliedMode = knowledgeRetrievalModeDenseOnly
				return finish(result)
			}
			return nil, denseErr
		}
		runLexical = true
		lexicalReason = "dense-unavailable"
		if req.TenantKnowledgeOnly {
			lexicalReason = "tenant-general-dense-unavailable"
		}
	} else if req.TenantKnowledgeOnly && len(result.RawHits) == 0 && ragCfg.LexicalTopK > 0 && len(lexicalTerms) > 0 {
		runLexical = true
		lexicalReason = "tenant-general-dense-empty"
	}
	result.Strategy.LexicalTriggered = runLexical
	result.Strategy.LexicalReason = lexicalReason
	if runLexical {
		lexicalStartedAt := time.Now()
		lexicalHits, lexicalErr := s.lexicalSearch(ctx, knowledgeLexicalSearchRequest{
			Scope:               *resolvedScope,
			KnowledgeBaseIDs:    effectiveKnowledgeBaseIDs,
			Query:               query,
			Terms:               lexicalTerms,
			ExactTerms:          exactTerms,
			IndexGenerationID:   result.IndexGenerationID,
			TenantKnowledgeOnly: req.TenantKnowledgeOnly,
			Limit:               ragCfg.LexicalTopK,
		})
		result.TimingTrace.LexicalMs = time.Since(lexicalStartedAt).Milliseconds()
		if lexicalErr != nil {
			slog.Warn("knowledge lexical rescue failed", "tenant_id", req.Scope.TenantID, "query", truncateForLog(query, 80), "error", lexicalErr)
		} else {
			result.Strategy.LexicalHitCount = len(lexicalHits)
			if len(lexicalHits) > 0 {
				candidates = fuseKnowledgeRetrievalResults(result.RawHits, lexicalHits, ragCfg.RRFK)
				result.Strategy.FusionHitCount = len(candidates)
			}
		}
	}
	if len(candidates) == 0 {
		if denseErr != nil {
			if denseErrDegradation {
				result.NoAnswerReason = "dense-unavailable"
				if runLexical {
					result.Strategy.AppliedMode = knowledgeRetrievalModeLexicalOnly
				} else {
					result.Strategy.AppliedMode = knowledgeRetrievalModeDenseOnly
				}
				return finish(result)
			}
			return nil, denseErr
		}
		switch {
		case len(searchResults) == 0:
			result.NoAnswerReason = "no-hit"
		case len(result.RawHits) == 0:
			result.NoAnswerReason = "no-hydrated-hit"
		default:
			result.NoAnswerReason = "no-candidate"
		}
		result.Strategy.AppliedMode = knowledgeRetrievalModeDenseOnly
		return finish(result)
	}
	if result.Strategy.FusionHitCount == 0 {
		result.Strategy.FusionHitCount = len(candidates)
	}
	if result.Strategy.LexicalHitCount > 0 && result.Strategy.DenseHitCount == 0 {
		result.Strategy.AppliedMode = knowledgeRetrievalModeLexicalOnly
	} else if result.Strategy.LexicalHitCount > 0 {
		result.Strategy.AppliedMode = knowledgeRetrievalModeHybridRRF
	} else {
		result.Strategy.AppliedMode = knowledgeRetrievalModeDenseOnly
	}

	result.RerankedHits = append([]rag.RetrieveResult(nil), candidates...)
	rerankTopN := resolveKnowledgeRerankTopN(req.RerankTopN, ragCfg.RerankTopN, len(result.RerankedHits), ragCfg.RerankMaxCandidates)
	result.Strategy.RerankCandidateCount = minInt(len(result.RerankedHits), ragCfg.RerankMaxCandidates)
	runRerank, rerankReason := shouldRunKnowledgeRerank(result.Strategy.RequestedMode, ragCfg, result.Strategy, result.RerankedHits)
	rerankCtx := ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:        resolvedScope.Context.TenantID,
		ProductID:       resolvedScope.Context.ProductID,
		KnowledgeBaseID: firstInt64(effectiveKnowledgeBaseIDs),
	})
	if runRerank && (s.rerankAvailable == nil || !s.rerankAvailable(rerankCtx)) {
		runRerank = false
		rerankReason = "rerank-unavailable"
	}
	result.Strategy.RerankRequested = runRerank
	result.Strategy.RerankReason = rerankReason
	if runRerank && rerankTopN > 0 && result.Strategy.RerankCandidateCount > 1 {
		rerankStartedAt := time.Now()
		rerankInput := append([]rag.RetrieveResult(nil), result.RerankedHits[:result.Strategy.RerankCandidateCount]...)
		reranked, rerankErr := s.rerankResults(rerankCtx, query, rerankInput, rerankTopN)
		result.TimingTrace.RerankMs = time.Since(rerankStartedAt).Milliseconds()
		if rerankErr != nil {
			slog.Warn("knowledge rerank failed", "tenant_id", req.Scope.TenantID, "query", truncateForLog(query, 80), "error", rerankErr)
		} else if len(reranked) > 0 {
			result.RerankedHits = reranked
			result.Strategy.RerankApplied = true
			if result.Strategy.LexicalHitCount > 0 {
				result.Strategy.AppliedMode = knowledgeRetrievalModeHybridRRFRerank
			}
		}
	}
	result.RerankedHits = filterKnowledgeResultsByExplicitTerms(result.RerankedHits, query)

	contextMaxTokens := req.ContextMaxTokens
	if contextMaxTokens <= 0 {
		contextMaxTokens = ragCfg.ContextMaxTokens
	}
	result.ContextHits = rag.Retrieve.SelectContextResults(result.RerankedHits, contextMaxTokens)
	if len(result.ContextHits) > ragCfg.MaxContextItems && ragCfg.MaxContextItems > 0 {
		result.ContextHits = append([]rag.RetrieveResult(nil), result.ContextHits[:ragCfg.MaxContextItems]...)
	}
	result.ContextText = strings.TrimSpace(rag.Retrieve.BuildContext(ctx, result.ContextHits, contextMaxTokens))
	result.Citations = buildKnowledgeCitations(result.ContextHits)
	if len(result.RerankedHits) > 0 {
		result.TopScore = float64(result.RerankedHits[0].Score)
	}
	if len(result.ContextHits) == 0 || result.ContextText == "" {
		result.NoAnswerReason = "no-context"
	}
	return finish(result)
}

func unionInt64s(groups ...[]int64) []int64 {
	result := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, group := range groups {
		for _, value := range group {
			if value <= 0 {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (s *knowledgeRetrievalService) hydrateResults(searchResults []vectordb.SearchResult, scope dto.ResolvedKnowledgeScope) []rag.RetrieveResult {
	if len(searchResults) == 0 {
		return nil
	}

	vectorIDs := make([]string, 0, len(searchResults))
	for _, item := range searchResults {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		vectorIDs = append(vectorIDs, item.ID)
	}
	chunks := repositories.KnowledgeChunkRepository.FindByVectorIDs(sqls.DB(), vectorIDs)
	chunkByVectorID := make(map[string]*models.KnowledgeChunk, len(chunks))
	documentIDs := make([]int64, 0)
	faqIDs := make([]int64, 0)
	documentSeen := make(map[int64]struct{})
	faqSeen := make(map[int64]struct{})
	for i := range chunks {
		chunk := &chunks[i]
		chunkByVectorID[chunk.VectorID] = chunk
		if chunk.DocumentID > 0 {
			if _, ok := documentSeen[chunk.DocumentID]; !ok {
				documentSeen[chunk.DocumentID] = struct{}{}
				documentIDs = append(documentIDs, chunk.DocumentID)
			}
		}
		if chunk.FaqID > 0 {
			if _, ok := faqSeen[chunk.FaqID]; !ok {
				faqSeen[chunk.FaqID] = struct{}{}
				faqIDs = append(faqIDs, chunk.FaqID)
			}
		}
	}
	documents := repositories.KnowledgeDocumentRepository.FindByIDs(sqls.DB(), documentIDs)
	documentByID := make(map[int64]*models.KnowledgeDocument, len(documents))
	for i := range documents {
		documentByID[documents[i].ID] = &documents[i]
	}
	faqs := repositories.KnowledgeFAQRepository.FindByIDs(sqls.DB(), faqIDs)
	faqByID := make(map[int64]*models.KnowledgeFAQ, len(faqs))
	for i := range faqs {
		faqByID[faqs[i].ID] = &faqs[i]
	}

	allowedRevisions := make(map[int64]struct{}, len(scope.RevisionIDs))
	for _, id := range scope.RevisionIDs {
		allowedRevisions[id] = struct{}{}
	}
	allowedEntries := make(map[string]struct{}, len(scope.EntryKeys))
	for _, item := range scope.EntryKeys {
		allowedEntries[item] = struct{}{}
	}

	results := make([]rag.RetrieveResult, 0, len(searchResults))
	for _, searchResult := range searchResults {
		chunk := chunkByVectorID[searchResult.ID]
		if chunk == nil || chunk.TenantID != scope.Context.TenantID || chunk.Status != enums.StatusOk {
			continue
		}
		entryKey := resolveChunkEntryKey(*chunk)
		if _, ok := allowedEntries[entryKey]; !ok {
			continue
		}
		revisionID := resolveChunkRevisionID(*chunk)
		if _, ok := allowedRevisions[revisionID]; !ok {
			continue
		}
		if chunk.ReviewStatus != "" && chunk.ReviewStatus != "published" {
			continue
		}

		item := rag.RetrieveResult{
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			ChunkID:         chunk.ID,
			DocumentID:      chunk.DocumentID,
			FaqID:           chunk.FaqID,
			ChunkNo:         chunk.ChunkNo,
			Title:           chunk.Title,
			SectionPath:     chunk.SectionPath,
			Content:         chunk.Content,
			Score:           searchResult.Score,
			DenseScore:      searchResult.Score,
			RetrievalSource: "dense",
			ChunkType:       chunk.ChunkType,
		}
		if chunk.DocumentID > 0 {
			doc := documentByID[chunk.DocumentID]
			if doc == nil || doc.TenantID != scope.Context.TenantID || doc.Status != enums.StatusOk || doc.ReviewStatus != "published" {
				continue
			}
			item.DocumentTitle = doc.Title
		}
		if chunk.FaqID > 0 {
			faq := faqByID[chunk.FaqID]
			if faq == nil || faq.TenantID != scope.Context.TenantID || faq.Status != enums.StatusOk || faq.ReviewStatus != "published" {
				continue
			}
			item.FaqQuestion = faq.Question
		}
		results = append(results, item)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ChunkID < results[j].ChunkID
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func buildRuntimeLanguageFilter(locale string) []string {
	return buildKnowledgeScopeLanguageFilter(locale)
}

func resolvedKnowledgeScopeLanguages(scope dto.ResolvedKnowledgeScope) []string {
	if len(scope.Languages) > 0 {
		return append([]string(nil), scope.Languages...)
	}
	return buildRuntimeLanguageFilter(scope.Context.Locale)
}

func buildKnowledgeCitations(items []rag.RetrieveResult) []dto.KnowledgeCitation {
	if len(items) == 0 {
		return nil
	}
	citations := make([]dto.KnowledgeCitation, 0, len(items))
	for _, item := range items {
		citations = append(citations, dto.KnowledgeCitation{
			DocumentID:    item.DocumentID,
			DocumentTitle: item.DocumentTitle,
			FaqID:         item.FaqID,
			FaqQuestion:   item.FaqQuestion,
			ChunkNo:       item.ChunkNo,
			Title:         item.Title,
			SectionPath:   item.SectionPath,
			Snippet:       item.Content,
			Score:         float64(item.Score),
		})
	}
	return citations
}

func (s *knowledgeRetrievalService) currentRAGConfig() config.RAGConfig {
	if s.getRAGConfig != nil {
		return s.getRAGConfig().Normalized()
	}
	return config.CurrentOrDefault().RAG.Normalized()
}

func (s *knowledgeRetrievalService) searchLexicalResults(_ context.Context, req knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error) {
	documentIDs, faqIDs := splitKnowledgeScopeEntryIDs(req.Scope.EntryKeys)
	lexicalQuery := repositories.KnowledgeChunkLexicalQuery{
		TenantID:          req.Scope.Context.TenantID,
		KnowledgeBaseIDs:  req.KnowledgeBaseIDs,
		DocumentIDs:       documentIDs,
		FAQIDs:            faqIDs,
		RevisionIDs:       append([]int64(nil), req.Scope.RevisionIDs...),
		Languages:         resolvedKnowledgeScopeLanguages(req.Scope),
		ReviewStatus:      "published",
		IndexGenerationID: req.IndexGenerationID,
		Terms:             append([]string(nil), req.Terms...),
		Limit:             maxKnowledgeRetrievalInt(req.Limit*4, req.Limit),
	}
	chunks := repositories.KnowledgeChunkRepository.FindLexicalCandidates(sqls.DB(), lexicalQuery)
	if len(chunks) == 0 && req.TenantKnowledgeOnly && req.IndexGenerationID > 0 {
		lexicalQuery.IndexGenerationID = 0
		chunks = repositories.KnowledgeChunkRepository.FindLexicalCandidates(sqls.DB(), lexicalQuery)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	documentByID, faqByID := loadKnowledgeChunkEntryMaps(chunks)
	results := make([]rag.RetrieveResult, 0, len(chunks))
	queryLower := strings.ToLower(strings.TrimSpace(req.Query))
	for _, chunk := range chunks {
		score := scoreKnowledgeLexicalChunk(chunk, queryLower, req.Terms, req.ExactTerms)
		if score <= 0 {
			continue
		}
		item := rag.RetrieveResult{
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			ChunkID:         chunk.ID,
			DocumentID:      chunk.DocumentID,
			FaqID:           chunk.FaqID,
			ChunkNo:         chunk.ChunkNo,
			Title:           chunk.Title,
			SectionPath:     chunk.SectionPath,
			Content:         chunk.Content,
			Score:           score,
			LexicalScore:    score,
			RetrievalSource: "lexical",
			ChunkType:       chunk.ChunkType,
		}
		if chunk.DocumentID > 0 {
			if doc := documentByID[chunk.DocumentID]; doc != nil {
				item.DocumentTitle = doc.Title
			}
		}
		if chunk.FaqID > 0 {
			if faq := faqByID[chunk.FaqID]; faq != nil {
				item.FaqQuestion = faq.Question
			}
		}
		results = append(results, item)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ChunkID < results[j].ChunkID
		}
		return results[i].Score > results[j].Score
	})
	if req.Limit > 0 && len(results) > req.Limit {
		return append([]rag.RetrieveResult(nil), results[:req.Limit]...), nil
	}
	return results, nil
}

func normalizeKnowledgeRetrievalMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case knowledgeRetrievalModeDenseOnly:
		return knowledgeRetrievalModeDenseOnly
	case knowledgeRetrievalModeHybridRRF:
		return knowledgeRetrievalModeHybridRRF
	case knowledgeRetrievalModeHybridRRFRerank:
		return knowledgeRetrievalModeHybridRRFRerank
	default:
		return knowledgeRetrievalModeAuto
	}
}

func shouldRunKnowledgeLexicalRescue(mode string, ragCfg config.RAGConfig, denseHits []rag.RetrieveResult, exactTerms []string) (bool, string) {
	mode = normalizeKnowledgeRetrievalMode(mode)
	if !ragCfg.HybridEnabled {
		return false, ""
	}
	if ragCfg.LexicalTopK <= 0 {
		return false, ""
	}
	if mode == knowledgeRetrievalModeDenseOnly {
		return false, ""
	}
	if mode == knowledgeRetrievalModeHybridRRF || mode == knowledgeRetrievalModeHybridRRFRerank {
		if len(exactTerms) > 0 {
			return true, "forced-hybrid-exact-term"
		}
		return true, "forced-hybrid-mode"
	}
	if len(denseHits) == 0 {
		return true, "dense-empty"
	}
	if len(exactTerms) > 0 {
		return true, "exact-match-term"
	}
	if len(denseHits) < ragCfg.HybridDenseMinHits {
		return true, "dense-hit-count-low"
	}
	if float64(denseHits[0].Score) < ragCfg.HybridDenseMinScore {
		return true, "dense-top-score-low"
	}
	return false, ""
}

func filterKnowledgeResultsByExplicitTerms(results []rag.RetrieveResult, query string) []rag.RetrieveResult {
	if len(results) == 0 {
		return nil
	}
	terms := extractKnowledgeExplicitIdentifierTerms(query)
	if len(terms) == 0 {
		return append([]rag.RetrieveResult(nil), results...)
	}
	filtered := make([]rag.RetrieveResult, 0, len(results))
	for _, item := range results {
		if knowledgeResultContainsAnyTerm(item, terms) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return append([]rag.RetrieveResult(nil), results...)
	}
	return filtered
}

func extractKnowledgeExplicitIdentifierTerms(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	terms := make([]string, 0)
	seen := make(map[string]struct{})
	for _, token := range knowledgeLexicalTokenPattern.FindAllString(query, -1) {
		normalized := normalizeKnowledgeIdentifierTerm(token)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		terms = append(terms, normalized)
		if len(terms) >= 4 {
			break
		}
	}
	return terms
}

func normalizeKnowledgeIdentifierTerm(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.Trim(token, "._/-")
	if utf8.RuneCountInString(token) < 2 {
		return ""
	}
	hasLetter := false
	hasDigit := false
	for _, r := range token {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ""
	}
	return token
}

func knowledgeResultContainsAnyTerm(item rag.RetrieveResult, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		item.DocumentTitle,
		item.FaqQuestion,
		item.Title,
		item.SectionPath,
		item.Content,
	}, "\n"))
	for _, term := range terms {
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func shouldRunKnowledgeRerank(mode string, ragCfg config.RAGConfig, strategy KnowledgeRetrievalStrategyTrace, candidates []rag.RetrieveResult) (bool, string) {
	mode = normalizeKnowledgeRetrievalMode(mode)
	if ragCfg.RerankTopN <= 0 || len(candidates) <= 1 {
		return false, ""
	}
	switch mode {
	case knowledgeRetrievalModeDenseOnly:
		return false, ""
	case knowledgeRetrievalModeHybridRRF:
		return false, ""
	case knowledgeRetrievalModeHybridRRFRerank:
		return true, "forced-rerank-mode"
	default:
		if strategy.LexicalHitCount > 0 {
			return true, "hybrid-candidates"
		}
		if strategy.DenseTopScore >= ragCfg.RerankSkipTopScore {
			return false, "dense-top-score-strong"
		}
		return true, "dense-top-score-weak"
	}
}

func resolveKnowledgeRerankTopN(requestTopN, defaultTopN, candidateCount, maxCandidates int) int {
	topN := requestTopN
	if topN <= 0 {
		topN = defaultTopN
	}
	if maxCandidates > 0 && topN > maxCandidates {
		topN = maxCandidates
	}
	if candidateCount > 0 && topN > candidateCount {
		topN = candidateCount
	}
	if topN < 0 {
		return 0
	}
	return topN
}

func extractKnowledgeLexicalTerms(query string) ([]string, []string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	terms := make([]string, 0)
	exactTerms := make([]string, 0)
	seenTerms := make(map[string]struct{})
	seenExact := make(map[string]struct{})
	addTerm := func(term string) {
		normalized := strings.ToLower(strings.TrimSpace(term))
		normalized = strings.Trim(normalized, "._/-:：,，.。?？!！;；()（）[]【】")
		if utf8.RuneCountInString(normalized) < 2 {
			return
		}
		if _, ok := seenTerms[normalized]; ok {
			return
		}
		seenTerms[normalized] = struct{}{}
		terms = append(terms, normalized)
	}
	for _, token := range knowledgeLexicalTokenPattern.FindAllString(query, -1) {
		normalized := strings.ToLower(strings.Trim(token, "._/-"))
		if utf8.RuneCountInString(normalized) < 2 {
			continue
		}
		if isKnowledgeExactMatchToken(token) {
			if _, ok := seenExact[normalized]; !ok {
				seenExact[normalized] = struct{}{}
				exactTerms = append(exactTerms, normalized)
			}
		}
		if utf8.RuneCountInString(normalized) >= 3 {
			addTerm(normalized)
		}
	}
	extractKnowledgeCJKLexicalTerms(query, addTerm)
	queryLower := strings.ToLower(query)
	if utf8.RuneCountInString(queryLower) >= 2 && utf8.RuneCountInString(queryLower) <= 48 {
		addTerm(queryLower)
	}
	if len(terms) > 12 {
		terms = append([]string(nil), terms[:12]...)
	}
	return terms, exactTerms
}

func extractKnowledgeCJKLexicalTerms(query string, addTerm func(string)) {
	if addTerm == nil {
		return
	}
	segment := make([]rune, 0, 16)
	flush := func() {
		if len(segment) == 0 {
			return
		}
		addKnowledgeCJKSegmentTerms(segment, addTerm)
		segment = segment[:0]
	}
	for _, r := range query {
		if isKnowledgeCJKRune(r) {
			segment = append(segment, r)
			continue
		}
		flush()
	}
	flush()
}

func addKnowledgeCJKSegmentTerms(segment []rune, addTerm func(string)) {
	if len(segment) < 2 {
		return
	}
	if len(segment) <= 8 {
		addTerm(string(segment))
	}
	for _, window := range []int{6, 5, 4, 3, 2} {
		if len(segment) < window {
			continue
		}
		for i := 0; i+window <= len(segment); i++ {
			addTerm(string(segment[i : i+window]))
		}
	}
}

func isKnowledgeCJKRune(r rune) bool {
	return unicode.In(r, unicode.Han)
}

func isKnowledgeExactMatchToken(token string) bool {
	hasDigit := false
	hasLetter := false
	hasLower := false
	for _, r := range token {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsLetter(r):
			hasLetter = true
			if unicode.IsLower(r) {
				hasLower = true
			}
		}
	}
	if hasDigit {
		return true
	}
	if strings.ContainsAny(token, "-_/.") {
		return true
	}
	return hasLetter && !hasLower
}

func splitKnowledgeScopeEntryIDs(entryKeys []string) ([]int64, []int64) {
	documentIDs := make([]int64, 0)
	faqIDs := make([]int64, 0)
	for _, entryKey := range entryKeys {
		entryType, rawID, ok := strings.Cut(strings.TrimSpace(entryKey), ":")
		if !ok {
			continue
		}
		entryID, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
		if err != nil || entryID <= 0 {
			continue
		}
		switch strings.TrimSpace(entryType) {
		case "document":
			documentIDs = append(documentIDs, entryID)
		case "faq":
			faqIDs = append(faqIDs, entryID)
		}
	}
	return documentIDs, faqIDs
}

func loadKnowledgeChunkEntryMaps(chunks []models.KnowledgeChunk) (map[int64]*models.KnowledgeDocument, map[int64]*models.KnowledgeFAQ) {
	documentIDs := make([]int64, 0)
	faqIDs := make([]int64, 0)
	documentSeen := make(map[int64]struct{})
	faqSeen := make(map[int64]struct{})
	for i := range chunks {
		chunk := chunks[i]
		if chunk.DocumentID > 0 {
			if _, ok := documentSeen[chunk.DocumentID]; !ok {
				documentSeen[chunk.DocumentID] = struct{}{}
				documentIDs = append(documentIDs, chunk.DocumentID)
			}
		}
		if chunk.FaqID > 0 {
			if _, ok := faqSeen[chunk.FaqID]; !ok {
				faqSeen[chunk.FaqID] = struct{}{}
				faqIDs = append(faqIDs, chunk.FaqID)
			}
		}
	}
	documents := repositories.KnowledgeDocumentRepository.FindByIDs(sqls.DB(), documentIDs)
	documentByID := make(map[int64]*models.KnowledgeDocument, len(documents))
	for i := range documents {
		documentByID[documents[i].ID] = &documents[i]
	}
	faqs := repositories.KnowledgeFAQRepository.FindByIDs(sqls.DB(), faqIDs)
	faqByID := make(map[int64]*models.KnowledgeFAQ, len(faqs))
	for i := range faqs {
		faqByID[faqs[i].ID] = &faqs[i]
	}
	return documentByID, faqByID
}

func scoreKnowledgeLexicalChunk(chunk models.KnowledgeChunk, query string, terms []string, exactTerms []string) float32 {
	title := strings.ToLower(strings.TrimSpace(chunk.Title))
	section := strings.ToLower(strings.TrimSpace(chunk.SectionPath))
	content := strings.ToLower(strings.TrimSpace(chunk.Content))
	score := 0.0
	if query != "" {
		if strings.Contains(title, query) {
			score += 6
		}
		if strings.Contains(section, query) {
			score += 5
		}
		if strings.Contains(content, query) {
			score += 3
		}
	}
	for _, term := range exactTerms {
		if term == "" {
			continue
		}
		if strings.Contains(title, term) {
			score += 5
		}
		if strings.Contains(section, term) {
			score += 4
		}
		if strings.Contains(content, term) {
			score += 2
		}
	}
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(title, term) {
			score += 2.5
		}
		if strings.Contains(section, term) {
			score += 2
		}
		if strings.Contains(content, term) {
			score += 1.2
		}
	}
	if score <= 0 {
		return 0
	}
	return float32(score)
}

func fuseKnowledgeRetrievalResults(denseHits, lexicalHits []rag.RetrieveResult, rrfK int) []rag.RetrieveResult {
	if len(denseHits) == 0 && len(lexicalHits) == 0 {
		return nil
	}
	if rrfK <= 0 {
		rrfK = 60
	}
	merged := make(map[string]*rag.RetrieveResult, len(denseHits)+len(lexicalHits))
	apply := func(items []rag.RetrieveResult, source string) {
		for index := range items {
			item := items[index]
			key := knowledgeRetrievalResultKey(item)
			current, ok := merged[key]
			if !ok {
				cloned := item
				current = &cloned
				merged[key] = current
			}
			switch source {
			case "dense":
				if current.DenseScore <= 0 {
					current.DenseScore = item.Score
				}
			case "lexical":
				if current.LexicalScore <= 0 {
					current.LexicalScore = item.Score
				}
			}
			current.FusionScore += 1 / float32(rrfK+index+1)
			current.Score = current.FusionScore
			current.RetrievalSource = "hybrid"
		}
	}
	apply(denseHits, "dense")
	apply(lexicalHits, "lexical")
	results := make([]rag.RetrieveResult, 0, len(merged))
	for _, item := range merged {
		results = append(results, *item)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ChunkID < results[j].ChunkID
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func knowledgeRetrievalResultKey(item rag.RetrieveResult) string {
	if item.ChunkID > 0 {
		return "chunk:" + strconv.FormatInt(item.ChunkID, 10)
	}
	if item.DocumentID > 0 {
		return "document:" + strconv.FormatInt(item.DocumentID, 10) + ":" + strconv.Itoa(item.ChunkNo)
	}
	if item.FaqID > 0 {
		return "faq:" + strconv.FormatInt(item.FaqID, 10) + ":" + strconv.Itoa(item.ChunkNo)
	}
	return fmt.Sprintf("kb:%d:%s", item.KnowledgeBaseID, item.Title)
}

func topRetrieveScore(items []rag.RetrieveResult) float64 {
	if len(items) == 0 {
		return 0
	}
	return float64(items[0].Score)
}

func truncateForLog(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes]) + "..."
}

func maxKnowledgeRetrievalInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func (s *knowledgeRetrievalService) maybeWriteRetrieveLog(ctx context.Context, req KnowledgeRetrievalRequest, result *KnowledgeRetrievalResult) {
	if !req.WriteRetrieveLog || result == nil || req.Scope.TenantID <= 0 {
		return
	}
	s.writeRetrieveLog(ctx, req, result, knowledgeRetrieveLogExtras{})
}

func (s *knowledgeRetrievalService) writeRetrieveLog(ctx context.Context, req KnowledgeRetrievalRequest, result *KnowledgeRetrievalResult, extras knowledgeRetrieveLogExtras) {
	if result == nil || req.Scope.TenantID <= 0 {
		return
	}

	knowledgeBase := resolvePrimaryKnowledgeBase(result.ResolvedScope.KnowledgeBaseIDs)
	ragCfg := s.currentRAGConfig()
	rerankLimit := resolveKnowledgeRerankTopN(req.RerankTopN, ragCfg.RerankTopN, len(result.RerankedHits), ragCfg.RerankMaxCandidates)
	chunkProvider := ""
	chunkTargetTokens := 0
	chunkMaxTokens := 0
	chunkOverlapTokens := 0
	if knowledgeBase != nil {
		chunkProvider = knowledgeBase.ChunkProvider
		chunkTargetTokens = knowledgeBase.ChunkTargetTokens
		chunkMaxTokens = knowledgeBase.ChunkMaxTokens
		chunkOverlapTokens = knowledgeBase.ChunkOverlapTokens
	}

	answerStatus := extras.AnswerStatus
	if answerStatus == 0 {
		answerStatus = int(enums.KnowledgeAnswerStatusNormal)
	}
	if extras.AnswerStatus == 0 && strings.TrimSpace(result.NoAnswerReason) != "" {
		answerStatus = int(enums.KnowledgeAnswerStatusNoAnswer)
	}
	retrieveMs := result.TimingTrace.EmbeddingMs + result.TimingTrace.VectorSearchMs + result.TimingTrace.HydrateMs + result.TimingTrace.LexicalMs + result.TimingTrace.RerankMs
	latencyMs := extras.TotalLatencyMs
	if latencyMs <= 0 {
		latencyMs = result.TimingTrace.ScopeResolveMs + retrieveMs + extras.GenerateMs
	}
	logItem, err := rag.RetrieveLog.CreateRetrieveLog(&rag.CreateRetrieveLogRequest{
		TenantID:                 req.Scope.TenantID,
		KnowledgeBaseID:          firstKnowledgeBaseID(result.ResolvedScope.KnowledgeBaseIDs),
		CollectionName:           result.CollectionName,
		IndexGenerationID:        result.IndexGenerationID,
		Channel:                  defaultKnowledgeRetrieveChannel(req.Channel),
		Scene:                    defaultKnowledgeRetrieveScene(req.Scene),
		SessionID:                req.SessionID,
		ConversationID:           req.ConversationID,
		Question:                 strings.TrimSpace(req.Query),
		Answer:                   strings.TrimSpace(extras.Answer),
		AnswerStatus:             answerStatus,
		ChunkProvider:            chunkProvider,
		ChunkTargetTokens:        chunkTargetTokens,
		ChunkMaxTokens:           chunkMaxTokens,
		ChunkOverlapTokens:       chunkOverlapTokens,
		RerankEnabled:            result.Strategy.RerankApplied,
		RerankLimit:              rerankLimit,
		Hits:                     buildKnowledgeSearchResponses(result.RerankedHits),
		UsedHits:                 buildKnowledgeSearchResponses(result.ContextHits),
		Citations:                buildKnowledgeCitationResponses(result.Citations),
		LatencyMs:                latencyMs,
		RetrieveMs:               retrieveMs,
		GenerateMs:               extras.GenerateMs,
		PromptTokens:             extras.PromptTokens,
		CompletionTokens:         extras.CompletionTokens,
		EmbeddingModel:           result.EmbeddingModel,
		ModelName:                extras.ModelName,
		NoAnswerReason:           result.NoAnswerReason,
		RequestedRetrievalMode:   result.Strategy.RequestedMode,
		AppliedRetrievalMode:     result.Strategy.AppliedMode,
		LexicalTriggered:         result.Strategy.LexicalTriggered,
		LexicalReason:            result.Strategy.LexicalReason,
		LexicalTerms:             append([]string(nil), result.Strategy.LexicalTerms...),
		ExactMatchTerms:          append([]string(nil), result.Strategy.ExactMatchTerms...),
		DenseHitCount:            result.Strategy.DenseHitCount,
		LexicalHitCount:          result.Strategy.LexicalHitCount,
		FusionHitCount:           result.Strategy.FusionHitCount,
		RerankApplied:            result.Strategy.RerankApplied,
		RerankReason:             result.Strategy.RerankReason,
		RerankCandidateCount:     result.Strategy.RerankCandidateCount,
		ScopeContext:             req.Scope,
		ResolvedKnowledgeBaseIDs: append([]int64(nil), result.ResolvedScope.KnowledgeBaseIDs...),
		ResolvedEntryKeys:        append([]string(nil), result.ResolvedScope.EntryKeys...),
		ResolvedRevisionIDs:      append([]int64(nil), result.ResolvedScope.RevisionIDs...),
		ResolutionTrace:          append([]dto.KnowledgeScopeTraceItem(nil), result.ResolvedScope.ResolutionTrace...),
		ScopeResolveMs:           result.TimingTrace.ScopeResolveMs,
		EmbeddingMs:              result.TimingTrace.EmbeddingMs,
		VectorSearchMs:           result.TimingTrace.VectorSearchMs,
		HydrateMs:                result.TimingTrace.HydrateMs,
		LexicalMs:                result.TimingTrace.LexicalMs,
		RerankMs:                 result.TimingTrace.RerankMs,
	}, nil)
	if err != nil {
		slog.Warn("write knowledge retrieve log failed", "tenant_id", req.Scope.TenantID, "scene", req.Scene, "error", err)
		return
	}
	result.RetrieveLogID = logItem.ID
}

func buildKnowledgeSearchResponses(items []rag.RetrieveResult) []response.KnowledgeSearchResult {
	if len(items) == 0 {
		return nil
	}
	results := make([]response.KnowledgeSearchResult, 0, len(items))
	for _, item := range items {
		results = append(results, response.KnowledgeSearchResult{
			KnowledgeBaseID: item.KnowledgeBaseID,
			ChunkID:         item.ChunkID,
			DocumentID:      item.DocumentID,
			DocumentTitle:   item.DocumentTitle,
			FaqID:           item.FaqID,
			FaqQuestion:     item.FaqQuestion,
			ChunkNo:         item.ChunkNo,
			Title:           item.Title,
			SectionPath:     item.SectionPath,
			Content:         item.Content,
			Score:           float64(item.Score),
			RerankScore:     float64(item.RerankScore),
		})
	}
	return results
}

func buildKnowledgeCitationResponses(items []dto.KnowledgeCitation) []response.KnowledgeCitation {
	if len(items) == 0 {
		return nil
	}
	results := make([]response.KnowledgeCitation, 0, len(items))
	for _, item := range items {
		results = append(results, response.KnowledgeCitation{
			DocumentID:    item.DocumentID,
			DocumentTitle: item.DocumentTitle,
			FaqID:         item.FaqID,
			FaqQuestion:   item.FaqQuestion,
			ChunkNo:       item.ChunkNo,
			Title:         item.Title,
			SectionPath:   item.SectionPath,
			Snippet:       item.Snippet,
			Score:         item.Score,
		})
	}
	return results
}

func resolvePrimaryKnowledgeBase(ids []int64) *models.KnowledgeBase {
	knowledgeBaseID := firstKnowledgeBaseID(ids)
	if knowledgeBaseID <= 0 {
		return nil
	}
	item := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if item == nil || item.Status != enums.StatusOk {
		return nil
	}
	return item
}

func firstKnowledgeBaseID(ids []int64) int64 {
	for _, item := range ids {
		if item > 0 {
			return item
		}
	}
	return 0
}

func defaultKnowledgeRetrieveChannel(channel string) string {
	if strings.TrimSpace(channel) == "" {
		return string(enums.KnowledgeRetrieveChannelAPI)
	}
	return channel
}

func defaultKnowledgeRetrieveScene(scene string) string {
	if strings.TrimSpace(scene) == "" {
		return string(enums.KnowledgeRetrieveSceneQA)
	}
	return scene
}

func resolveChunkEntryKey(chunk models.KnowledgeChunk) string {
	if chunk.DocumentID > 0 {
		return buildKnowledgeDocumentEntryKey(chunk.DocumentID)
	}
	if chunk.FaqID > 0 {
		return buildKnowledgeFAQEntryKey(chunk.FaqID)
	}
	return ""
}

func resolveChunkRevisionID(chunk models.KnowledgeChunk) int64 {
	if chunk.RevisionID > 0 {
		return chunk.RevisionID
	}
	if chunk.DocumentID > 0 {
		return chunk.DocumentID
	}
	if chunk.FaqID > 0 {
		return chunk.FaqID
	}
	return 0
}

func intersectInt64s(left, right []int64) []int64 {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	allowed := make(map[int64]struct{}, len(right))
	for _, item := range right {
		if item > 0 {
			allowed[item] = struct{}{}
		}
	}
	items := make([]int64, 0, len(left))
	for _, item := range left {
		if _, ok := allowed[item]; ok {
			items = append(items, item)
		}
	}
	return items
}
