package services

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type knowledgeRetrievalTestProvider struct {
	lastSearchRequest *vectordb.SearchRequest
	results           []vectordb.SearchResult
	emptyResults      bool
	searchErr         error
}

func (p *knowledgeRetrievalTestProvider) CreateCollection(context.Context, string, int) error {
	return nil
}

func (p *knowledgeRetrievalTestProvider) DeleteCollection(context.Context, string) error {
	return nil
}

func (p *knowledgeRetrievalTestProvider) GetCollection(context.Context, string) (*vectordb.CollectionInfo, error) {
	return nil, nil
}

func (p *knowledgeRetrievalTestProvider) ListCollections(context.Context) ([]string, error) {
	return nil, nil
}

func (p *knowledgeRetrievalTestProvider) UpsertVectors(context.Context, string, []vectordb.Vector) error {
	return nil
}

func (p *knowledgeRetrievalTestProvider) DeleteVectors(context.Context, string, []string) error {
	return nil
}

func (p *knowledgeRetrievalTestProvider) Search(_ context.Context, req *vectordb.SearchRequest) ([]vectordb.SearchResult, error) {
	cloned := *req
	if req.Filter != nil {
		filter := *req.Filter
		cloned.Filter = &filter
	}
	p.lastSearchRequest = &cloned
	if p.searchErr != nil {
		return nil, p.searchErr
	}
	if p.emptyResults {
		return nil, nil
	}
	if len(p.results) > 0 {
		return append([]vectordb.SearchResult(nil), p.results...), nil
	}
	return []vectordb.SearchResult{{ID: "vec-doc-1", Score: 0.92}}, nil
}

func (p *knowledgeRetrievalTestProvider) Close() error {
	return nil
}

func setupKnowledgeRetrievalServiceTestDB(t *testing.T) int64 {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeChunk{},
		&models.KnowledgeIndexGeneration{},
		&models.KnowledgeRetrieveLog{},
		&models.KnowledgeRetrieveHit{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	tenantID := int64(4201)
	if err := db.Create(&models.KnowledgeBase{
		ID:                 11,
		TenantID:           tenantID,
		Name:               "Hydraulic KB",
		KnowledgeType:      string(enums.KnowledgeBaseTypeDocument),
		Status:             enums.StatusOk,
		DefaultRerankLimit: 4,
		ChunkProvider:      "structured",
		ChunkTargetTokens:  320,
		ChunkMaxTokens:     420,
		ChunkOverlapTokens: 32,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge base error = %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		ID:                  101,
		TenantID:            tenantID,
		KnowledgeBaseID:     11,
		Title:               "Pump Startup Manual",
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		Content:             "Check the pressure valve before startup.",
		ReviewStatus:        "published",
		Language:            "en-US",
		Status:              enums.StatusOk,
		IndexStatus:         enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:         "doc-hash-1",
		CurrentRevisionID:   501,
		PublishedRevisionID: 501,
		AuditFields:         models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge document error = %v", err)
	}
	if err := db.Create(&models.KnowledgeChunk{
		ID:                201,
		TenantID:          tenantID,
		KnowledgeBaseID:   11,
		DocumentID:        101,
		ChunkNo:           1,
		Title:             "Startup",
		Content:           "Check the pressure valve before startup.",
		ContentHash:       "chunk-hash-1",
		ChunkType:         string(enums.KnowledgeChunkTypeText),
		Provider:          "structured",
		RevisionID:        501,
		Language:          "en-US",
		ReviewStatus:      "published",
		IndexGenerationID: 9,
		Status:            enums.StatusOk,
		VectorID:          "vec-doc-1",
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create knowledge chunk error = %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		ID:                  102,
		TenantID:            tenantID,
		KnowledgeBaseID:     11,
		Title:               "P-101 Alarm Manual",
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		Content:             "Reset the P-101 alarm by checking the valve lock and restarting the unit.",
		ReviewStatus:        "published",
		Language:            "en-US",
		Status:              enums.StatusOk,
		IndexStatus:         enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:         "doc-hash-2",
		CurrentRevisionID:   502,
		PublishedRevisionID: 502,
		AuditFields:         models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge document 102 error = %v", err)
	}
	if err := db.Create(&models.KnowledgeChunk{
		ID:                202,
		TenantID:          tenantID,
		KnowledgeBaseID:   11,
		DocumentID:        102,
		ChunkNo:           1,
		Title:             "P-101 Alarm",
		Content:           "P-101 reset steps: check the valve lock and restart the unit.",
		ContentHash:       "chunk-hash-2",
		ChunkType:         string(enums.KnowledgeChunkTypeText),
		Provider:          "structured",
		RevisionID:        502,
		Language:          "en-US",
		ReviewStatus:      "published",
		IndexGenerationID: 9,
		Status:            enums.StatusOk,
		VectorID:          "vec-doc-2",
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create knowledge chunk 202 error = %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		ID:                  103,
		TenantID:            tenantID,
		KnowledgeBaseID:     11,
		Title:               "Valve Lock Checklist",
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		Content:             "Inspect the lock pin and pressure valve before restart.",
		ReviewStatus:        "published",
		Language:            "en-US",
		Status:              enums.StatusOk,
		IndexStatus:         enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:         "doc-hash-3",
		CurrentRevisionID:   503,
		PublishedRevisionID: 503,
		AuditFields:         models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge document 103 error = %v", err)
	}
	if err := db.Create(&models.KnowledgeChunk{
		ID:                203,
		TenantID:          tenantID,
		KnowledgeBaseID:   11,
		DocumentID:        103,
		ChunkNo:           1,
		Title:             "Valve Lock",
		Content:           "Inspect the lock pin and pressure valve before restart.",
		ContentHash:       "chunk-hash-3",
		ChunkType:         string(enums.KnowledgeChunkTypeText),
		Provider:          "structured",
		RevisionID:        503,
		Language:          "en-US",
		ReviewStatus:      "published",
		IndexGenerationID: 9,
		Status:            enums.StatusOk,
		VectorID:          "vec-doc-3",
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create knowledge chunk 203 error = %v", err)
	}
	if err := db.Create(&models.KnowledgeIndexGeneration{
		ID:               9,
		CollectionName:   "knowledge_chunks_v2_embed_v1_20260724",
		CollectionAlias:  "knowledge_chunks_active",
		SchemaVersion:    2,
		EmbeddingModel:   "embed-v1",
		EmbeddingVersion: "embed-v1",
		Dimension:        2,
		Status:           "active",
		PointCount:       1,
		ActivatedAt:      &now,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge index generation error = %v", err)
	}
	return tenantID
}

func TestKnowledgeRetrievalUsesActiveGenerationAndWritesRetrieveLog(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                 scope,
			KnowledgeBaseIDs:        []int64{11},
			ProductKnowledgeBaseIDs: []int64{11},
			EntryKeys:               []string{"document:101"},
			RevisionIDs:             []int64{501},
			ResolutionTrace: []dto.KnowledgeScopeTraceItem{
				{Step: "product_binding", Reason: "published link matched", Applied: true},
			},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{
			Vector:    []float32{0.1, 0.2},
			ModelName: "embed-v1",
			Dimension: 2,
		}, nil
	}
	service.getProvider = func() vectordb.Provider {
		return provider
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID:  tenantID,
			ProductID: 88,
			Locale:    "en-US",
			Audience:  "customer",
		},
		Query:            "startup pressure issue",
		Channel:          string(enums.KnowledgeRetrieveChannelIM),
		Scene:            string(enums.KnowledgeRetrieveSceneRuntime),
		ContextMaxTokens: 1024,
		WriteRetrieveLog: true,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result == nil {
		t.Fatalf("Retrieve() result is nil")
	}
	if result.IndexGenerationID != 9 {
		t.Fatalf("IndexGenerationID = %d, want 9", result.IndexGenerationID)
	}
	if result.RetrieveLogID <= 0 {
		t.Fatalf("RetrieveLogID = %d, want > 0", result.RetrieveLogID)
	}
	if provider.lastSearchRequest == nil || provider.lastSearchRequest.Filter == nil {
		t.Fatalf("search request filter not captured")
	}
	if provider.lastSearchRequest.CollectionName != "knowledge_chunks_v2_embed_v1_20260724" {
		t.Fatalf("CollectionName = %q, want active generation collection", provider.lastSearchRequest.CollectionName)
	}
	if provider.lastSearchRequest.Filter.IndexGenerationID != 9 {
		t.Fatalf("SearchFilter.IndexGenerationID = %d, want 9", provider.lastSearchRequest.Filter.IndexGenerationID)
	}

	logItem := KnowledgeRetrieveLogService.Get(result.RetrieveLogID)
	if logItem == nil {
		t.Fatalf("retrieve log not found")
	}
	if logItem.TenantID != tenantID {
		t.Fatalf("log tenant_id = %d, want %d", logItem.TenantID, tenantID)
	}
	if logItem.CollectionName != "knowledge_chunks_v2_embed_v1_20260724" {
		t.Fatalf("log collection_name = %q", logItem.CollectionName)
	}
	if logItem.IndexGenerationID != 9 {
		t.Fatalf("log index_generation_id = %d, want 9", logItem.IndexGenerationID)
	}
	if logItem.EmbeddingModel != "embed-v1" {
		t.Fatalf("log embedding_model = %q", logItem.EmbeddingModel)
	}
	if logItem.NoAnswerReason != "" {
		t.Fatalf("log no_answer_reason = %q, want empty", logItem.NoAnswerReason)
	}
	if !strings.Contains(logItem.TraceData, "\"collectionName\":\"knowledge_chunks_v2_embed_v1_20260724\"") {
		t.Fatalf("trace data missing collection name: %s", logItem.TraceData)
	}
	if !strings.Contains(logItem.TraceData, "\"indexGenerationId\":9") {
		t.Fatalf("trace data missing generation id: %s", logItem.TraceData)
	}

	hits := KnowledgeRetrieveLogService.FindHitsByRetrieveLogID(result.RetrieveLogID)
	if len(hits) != 1 {
		t.Fatalf("hit count = %d, want 1", len(hits))
	}
	if hits[0].TenantID != tenantID {
		t.Fatalf("hit tenant_id = %d, want %d", hits[0].TenantID, tenantID)
	}
}

func TestKnowledgeRetrievalUsesTenantResolverWithoutProductContext(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	service := newKnowledgeRetrievalService()
	productResolverCalled := false
	tenantResolverCalled := false
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		productResolverCalled = true
		return nil, errors.New("product resolver must not be used")
	}
	service.resolveTenantScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		tenantResolverCalled = true
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{15},
			TenantSharedKnowledgeBaseIDs: []int64{15},
		}, nil
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope:               dto.KnowledgeScopeContext{TenantID: tenantID, Audience: "customer"},
		TenantKnowledgeOnly: true,
		Query:               "what support is available",
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if productResolverCalled || !tenantResolverCalled {
		t.Fatalf("resolver selection product=%v tenant=%v", productResolverCalled, tenantResolverCalled)
	}
	if result == nil || result.NoAnswerReason != "no-scope" || !slices.Equal(result.ResolvedScope.KnowledgeBaseIDs, []int64{15}) {
		t.Fatalf("unexpected tenant-only result: %+v", result)
	}
}

func TestKnowledgeRetrievalRejectsEmbeddingDimensionBeforeVectorSearch(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                 scope,
			KnowledgeBaseIDs:        []int64{11},
			ProductKnowledgeBaseIDs: []int64{11},
			EntryKeys:               []string{"document:101"},
			RevisionIDs:             []int64{501},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{
			Vector:    []float32{0.1, 0.2, 0.3},
			ModelName: "wrong-dimension-model",
			Dimension: 3,
		}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return config.RAGConfig{HybridEnabled: false}
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID:  tenantID,
			ProductID: 88,
			Locale:    "en-US",
			Audience:  "customer",
		},
		Query: "startup pressure issue",
	})
	if result != nil || err == nil || !strings.Contains(err.Error(), "does not match active knowledge index generation") {
		t.Fatalf("Retrieve() = (%#v, %v), want generation dimension error", result, err)
	}
	if provider.lastSearchRequest != nil {
		t.Fatalf("vector search should not run for a mismatched query vector: %#v", provider.lastSearchRequest)
	}
}

func TestKnowledgeRetrievalAgentSelectionKeepsTenantSharedKnowledge(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{11, 12, 13},
			ProductKnowledgeBaseIDs:      []int64{11, 12},
			TenantSharedKnowledgeBaseIDs: []int64{13},
			EntryKeys:                    []string{"document:101"},
			RevisionIDs:                  []int64{501},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope:                   dto.KnowledgeScopeContext{TenantID: tenantID, ProductID: 88, Locale: "en-US", Audience: "customer"},
		AllowedKnowledgeBaseIDs: []int64{11},
		IncludeTenantShared:     true,
		Query:                   "startup pressure issue",
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result == nil || provider.lastSearchRequest == nil || provider.lastSearchRequest.Filter == nil {
		t.Fatal("expected a vector search request")
	}
	want := []int64{11, 13}
	if !slices.Equal(provider.lastSearchRequest.Filter.KnowledgeBaseIDs, want) {
		t.Fatalf("filter knowledge base ids = %v, want %v", provider.lastSearchRequest.Filter.KnowledgeBaseIDs, want)
	}
	if !slices.Equal(result.ResolvedScope.KnowledgeBaseIDs, want) {
		t.Fatalf("resolved knowledge base ids = %v, want %v", result.ResolvedScope.KnowledgeBaseIDs, want)
	}
}

func TestKnowledgeRetrievalProductStrictScopeExcludesTenantSharedKnowledge(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{11, 13},
			ProductKnowledgeBaseIDs:      []int64{11},
			TenantSharedKnowledgeBaseIDs: []int64{13},
			EntryKeys:                    []string{"document:101"},
			RevisionIDs:                  []int64{501},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope:                   dto.KnowledgeScopeContext{TenantID: tenantID, ProductID: 88, Locale: "en-US", Audience: "customer"},
		AllowedKnowledgeBaseIDs: []int64{11, 13},
		IncludeTenantShared:     false,
		Query:                   "startup pressure issue",
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result == nil || provider.lastSearchRequest == nil || provider.lastSearchRequest.Filter == nil {
		t.Fatal("expected a vector search request")
	}
	want := []int64{11}
	if !slices.Equal(provider.lastSearchRequest.Filter.KnowledgeBaseIDs, want) {
		t.Fatalf("filter knowledge base ids = %v, want %v", provider.lastSearchRequest.Filter.KnowledgeBaseIDs, want)
	}
	if !slices.Equal(result.ResolvedScope.KnowledgeBaseIDs, want) {
		t.Fatalf("resolved knowledge base ids = %v, want %v", result.ResolvedScope.KnowledgeBaseIDs, want)
	}
	if len(result.ResolvedScope.TenantSharedKnowledgeBaseIDs) != 0 {
		t.Fatalf("tenant shared knowledge should be removed from strict product scope, got %v", result.ResolvedScope.TenantSharedKnowledgeBaseIDs)
	}
}

func TestKnowledgeDebugSearchUsesOperatorTenantAndUnifiedProductScope(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	service := newKnowledgeRetrievalService()
	var capturedScope dto.KnowledgeScopeContext
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		capturedScope = scope
		return &dto.ResolvedKnowledgeScope{
			Context:                 scope,
			KnowledgeBaseIDs:        []int64{11},
			ProductKnowledgeBaseIDs: []int64{11},
			EntryKeys:               []string{"document:101"},
			RevisionIDs:             []int64{501},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	previous := KnowledgeRetrievalService
	KnowledgeRetrievalService = service
	t.Cleanup(func() { KnowledgeRetrievalService = previous })

	result, err := KnowledgeDebugService.DebugSearch(context.Background(), request.KnowledgeSearchRequest{
		TenantID:         tenantID + 999,
		ProductID:        88,
		KnowledgeBaseIDs: []int64{11},
		Locale:           "en-US",
		Audience:         "customer",
		Question:         "startup pressure issue",
	}, &dto.AuthPrincipal{TenantID: tenantID, UserID: 1001})
	if err != nil {
		t.Fatalf("DebugSearch() error = %v", err)
	}
	if capturedScope.TenantID != tenantID {
		t.Fatalf("resolved tenant = %d, want operator tenant %d", capturedScope.TenantID, tenantID)
	}
	if capturedScope.ProductID != 88 || capturedScope.Audience != "customer" {
		t.Fatalf("resolved scope = %+v", capturedScope)
	}
	if result == nil || result.HitCount != 1 {
		t.Fatalf("DebugSearch() result = %+v, want one hit", result)
	}
	if provider.lastSearchRequest == nil || provider.lastSearchRequest.Filter == nil {
		t.Fatal("unified Qdrant search filter was not used")
	}
	if provider.lastSearchRequest.Filter.TenantID != tenantID {
		t.Fatalf("search filter tenant = %d, want %d", provider.lastSearchRequest.Filter.TenantID, tenantID)
	}
	if len(provider.lastSearchRequest.Filter.EntryKeys) != 1 || provider.lastSearchRequest.Filter.EntryKeys[0] != "document:101" {
		t.Fatalf("search filter entry keys = %#v", provider.lastSearchRequest.Filter.EntryKeys)
	}
	if got := provider.lastSearchRequest.Filter.ScopeKeys; len(got) != 2 || got[0] != "tenant:"+strconv.FormatInt(tenantID, 10) || got[1] != "product:88" {
		t.Fatalf("search filter scope keys = %#v", got)
	}
}

func TestKnowledgeDebugAnswerFallsBackToRetrievedEvidenceWhenLLMUnavailable(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{}
	retrievalService := newKnowledgeRetrievalService()
	retrievalService.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:          scope,
			KnowledgeBaseIDs: []int64{11},
			EntryKeys:        []string{"document:101"},
			RevisionIDs:      []int64{501},
		}, nil
	}
	retrievalService.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	retrievalService.getProvider = func() vectordb.Provider { return provider }
	previous := KnowledgeRetrievalService
	KnowledgeRetrievalService = retrievalService
	t.Cleanup(func() { KnowledgeRetrievalService = previous })

	debugService := newKnowledgeDebugService()
	debugService.chat = func(context.Context, string, string) (*ai.ChatCompletionResult, error) {
		return nil, errors.New("provider rate limited")
	}
	result, err := debugService.DebugAnswer(context.Background(), request.KnowledgeAnswerRequest{
		ProductID:        88,
		KnowledgeBaseIDs: []int64{11},
		Locale:           "zh-CN",
		Audience:         "customer",
		Question:         "启动前要检查什么？",
		AnswerMode:       int(enums.KnowledgeAnswerModeStrict),
	}, &dto.AuthPrincipal{TenantID: tenantID, UserID: 1001})
	if err != nil {
		t.Fatalf("DebugAnswer() error = %v", err)
	}
	if result.AnswerStatus != int(enums.KnowledgeAnswerStatusFallback) {
		t.Fatalf("AnswerStatus = %d, want fallback", result.AnswerStatus)
	}
	if result.HitCount != 1 || len(result.Citations) != 1 {
		t.Fatalf("fallback lost retrieval evidence: hits=%d citations=%d", result.HitCount, len(result.Citations))
	}
	if !strings.Contains(result.Answer, "AI 生成服务暂时不可用") ||
		!strings.Contains(result.Answer, "Check the pressure valve before startup") ||
		strings.Contains(result.Answer, "当前知识库暂无明确信息") {
		t.Fatalf("fallback answer = %q", result.Answer)
	}
}

func TestKnowledgeDebugAnswerPromptsFollowRequestedLocale(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		wantSystem string
		wantUser   string
	}{
		{name: "english", locale: "en", wantSystem: "Respond in English", wantUser: "User question:"},
		{name: "english region", locale: "en-US", wantSystem: "Respond in English", wantUser: "Knowledge excerpts:"},
		{name: "chinese", locale: "zh-CN", wantSystem: "请使用中文回答", wantUser: "用户问题："},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			systemPrompt := debugAnswerSystemPrompt(enums.KnowledgeAnswerModeStrict, tt.locale)
			if !strings.Contains(systemPrompt, tt.wantSystem) {
				t.Fatalf("system prompt %q does not contain %q", systemPrompt, tt.wantSystem)
			}
			userPrompt := debugAnswerUserPrompt(tt.locale, "How do I reset E17?", "Hold STOP + RESET.")
			if !strings.Contains(userPrompt, tt.wantUser) {
				t.Fatalf("user prompt %q does not contain %q", userPrompt, tt.wantUser)
			}
		})
	}
}

func TestKnowledgeRetrievalAutoSkipsLexicalAndRerankForStrongDenseResults(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{
		results: []vectordb.SearchResult{
			{ID: "vec-doc-1", Score: 0.93},
			{ID: "vec-doc-2", Score: 0.88},
			{ID: "vec-doc-3", Score: 0.84},
		},
	}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:          scope,
			KnowledgeBaseIDs: []int64{11},
			EntryKeys:        []string{"document:101", "document:102", "document:103"},
			RevisionIDs:      []int64{501, 502, 503},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:       true,
			RetrievalMode:       "auto",
			DenseTopK:           10,
			LexicalTopK:         12,
			HybridDenseMinHits:  2,
			HybridDenseMinScore: 0.55,
			RerankTopN:          5,
			RerankMaxCandidates: 10,
			RerankSkipTopScore:  0.82,
			ContextMaxTokens:    1024,
			MaxContextItems:     5,
		}).Normalized()
	}
	lexicalCalled := false
	rerankCalled := false
	service.lexicalSearch = func(context.Context, knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error) {
		lexicalCalled = true
		return nil, nil
	}
	service.rerankResults = func(_ context.Context, _ string, results []rag.RetrieveResult, _ int) ([]rag.RetrieveResult, error) {
		rerankCalled = true
		return results, nil
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "en-US",
			Audience: "customer",
		},
		Query:            "pressure valve startup",
		ContextMaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if lexicalCalled {
		t.Fatalf("lexical rescue should not run for strong dense results")
	}
	if rerankCalled {
		t.Fatalf("rerank should not run for strong dense results")
	}
	if result.Strategy.AppliedMode != knowledgeRetrievalModeDenseOnly {
		t.Fatalf("AppliedMode = %q, want %q", result.Strategy.AppliedMode, knowledgeRetrievalModeDenseOnly)
	}
	if result.Strategy.RerankApplied {
		t.Fatalf("RerankApplied = true, want false")
	}
	if len(result.RerankedHits) != 3 {
		t.Fatalf("RerankedHits = %d, want 3", len(result.RerankedHits))
	}
}

func TestKnowledgeRetrievalAutoUsesLexicalRescueAndRerankForWeakDenseExactTerm(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{
		results: []vectordb.SearchResult{
			{ID: "vec-doc-1", Score: 0.31},
		},
	}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:          scope,
			KnowledgeBaseIDs: []int64{11},
			EntryKeys:        []string{"document:101", "document:102", "document:103"},
			RevisionIDs:      []int64{501, 502, 503},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:       true,
			RetrievalMode:       "auto",
			DenseTopK:           10,
			LexicalTopK:         5,
			HybridDenseMinHits:  3,
			HybridDenseMinScore: 0.55,
			RRFK:                60,
			RerankTopN:          2,
			RerankMaxCandidates: 4,
			RerankSkipTopScore:  0.82,
			ContextMaxTokens:    1024,
			MaxContextItems:     5,
		}).Normalized()
	}
	lexicalCalled := false
	service.lexicalSearch = func(ctx context.Context, req knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error) {
		lexicalCalled = true
		return service.searchLexicalResults(ctx, req)
	}
	rerankCalled := false
	service.rerankAvailable = func(context.Context) bool { return true }
	service.rerankResults = func(_ context.Context, _ string, results []rag.RetrieveResult, topN int) ([]rag.RetrieveResult, error) {
		rerankCalled = true
		cloned := append([]rag.RetrieveResult(nil), results...)
		sort.SliceStable(cloned, func(i, j int) bool {
			return cloned[i].DocumentID > cloned[j].DocumentID
		})
		if topN > 0 && len(cloned) > topN {
			cloned = cloned[:topN]
		}
		for i := range cloned {
			cloned[i].RerankScore = 0.99 - float32(i)*0.01
			cloned[i].Score = cloned[i].RerankScore
		}
		return cloned, nil
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "en-US",
			Audience: "customer",
		},
		Query:            "P-101 reset",
		ContextMaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if !lexicalCalled {
		t.Fatalf("lexical rescue should run for weak dense exact-term query")
	}
	if !rerankCalled {
		t.Fatalf("rerank should run after hybrid rescue")
	}
	if !result.Strategy.LexicalTriggered {
		t.Fatalf("LexicalTriggered = false, want true")
	}
	if result.Strategy.LexicalReason != "exact-match-term" {
		t.Fatalf("LexicalReason = %q, want exact-match-term", result.Strategy.LexicalReason)
	}
	if !result.Strategy.RerankApplied {
		t.Fatalf("RerankApplied = false, want true")
	}
	if result.Strategy.AppliedMode != knowledgeRetrievalModeHybridRRFRerank {
		t.Fatalf("AppliedMode = %q, want %q", result.Strategy.AppliedMode, knowledgeRetrievalModeHybridRRFRerank)
	}
	if len(result.RerankedHits) == 0 || result.RerankedHits[0].DocumentID != 102 {
		t.Fatalf("top reranked document = %d, want 102", firstResultDocumentID(result.RerankedHits))
	}
}

func TestKnowledgeRetrievalFallsBackToScopedLexicalSearchWhenVectorStoreIsUnavailable(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	provider := &knowledgeRetrievalTestProvider{searchErr: errors.New("qdrant restarting")}
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:          scope,
			KnowledgeBaseIDs: []int64{11},
			EntryKeys:        []string{"document:101", "document:102"},
			RevisionIDs:      []int64{501, 502},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:    true,
			RetrievalMode:    "auto",
			DenseTopK:        10,
			LexicalTopK:      5,
			RerankTopN:       0,
			ContextMaxTokens: 1024,
			MaxContextItems:  5,
		}).Normalized()
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "en-US",
			Audience: "customer",
		},
		Query:            "P-101 reset",
		ContextMaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Strategy.DenseHitCount != 0 || result.Strategy.LexicalHitCount == 0 {
		t.Fatalf("strategy = %+v, want lexical-only fallback", result.Strategy)
	}
	if result.Strategy.LexicalReason != "dense-unavailable" {
		t.Fatalf("LexicalReason = %q, want dense-unavailable", result.Strategy.LexicalReason)
	}
	if result.Strategy.AppliedMode != knowledgeRetrievalModeLexicalOnly {
		t.Fatalf("AppliedMode = %q, want %q", result.Strategy.AppliedMode, knowledgeRetrievalModeLexicalOnly)
	}
	if len(result.ContextHits) == 0 || result.ContextHits[0].DocumentID != 102 {
		t.Fatalf("ContextHits = %+v, want scoped P-101 document", result.ContextHits)
	}
	if !strings.Contains(result.ContextText, "P-101 reset steps") {
		t.Fatalf("ContextText = %q, want lexical repair procedure", result.ContextText)
	}
}

func TestKnowledgeRetrievalTenantLexicalFallbackMatchesChineseGeneralQuery(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	now := time.Now()
	if err := sqls.DB().Create(&models.KnowledgeDocument{
		ID:                  104,
		TenantID:            tenantID,
		KnowledgeBaseID:     11,
		Title:               "通用售后安全启动指南",
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		Content:             "设备长期不用重新开机前检查外观、管路和急停状态，确认无漏液、异味和松动后再低负载启动。",
		ReviewStatus:        "published",
		Language:            "default",
		Status:              enums.StatusOk,
		IndexStatus:         enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:         "doc-hash-zh-general",
		CurrentRevisionID:   504,
		PublishedRevisionID: 504,
		AuditFields:         models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create chinese general document error = %v", err)
	}
	if err := sqls.DB().Create(&models.KnowledgeChunk{
		ID:                204,
		TenantID:          tenantID,
		KnowledgeBaseID:   11,
		DocumentID:        104,
		ChunkNo:           1,
		Title:             "设备长期不用重新开机前检查",
		Content:           "设备长期不用重新开机前检查外观、管路和急停状态，确认无漏液、异味和松动后再低负载启动。",
		ContentHash:       "chunk-hash-zh-general",
		ChunkType:         string(enums.KnowledgeChunkTypeText),
		Provider:          "structured",
		RevisionID:        504,
		Language:          "default",
		ReviewStatus:      "published",
		IndexGenerationID: 9,
		Status:            enums.StatusOk,
		VectorID:          "vec-doc-4",
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create chinese general chunk error = %v", err)
	}

	provider := &knowledgeRetrievalTestProvider{searchErr: errors.New("qdrant restarting")}
	service := newKnowledgeRetrievalService()
	service.resolveTenantScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{11},
			TenantSharedKnowledgeBaseIDs: []int64{11},
			EntryKeys:                    []string{"document:104"},
			RevisionIDs:                  []int64{504},
			Languages:                    []string{"default", "en"},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:    true,
			RetrievalMode:    "auto",
			DenseTopK:        10,
			LexicalTopK:      8,
			RerankTopN:       0,
			ContextMaxTokens: 1024,
			MaxContextItems:  5,
		}).Normalized()
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "default",
			Audience: "customer",
		},
		TenantKnowledgeOnly: true,
		Query:               "GENERAL-A-1786284141592：设备长期不用，重新开机前要注意什么？",
		ContextMaxTokens:    1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Strategy.LexicalHitCount == 0 || result.Strategy.AppliedMode != knowledgeRetrievalModeLexicalOnly {
		t.Fatalf("strategy = %+v, want lexical-only chinese fallback", result.Strategy)
	}
	if len(result.ContextHits) == 0 || result.ContextHits[0].DocumentID != 104 {
		t.Fatalf("ContextHits = %+v, want chinese general document", result.ContextHits)
	}
	if !strings.Contains(result.ContextText, "设备长期不用重新开机前检查") {
		t.Fatalf("ContextText = %q, want chinese restart checklist", result.ContextText)
	}
}

func TestKnowledgeRetrievalTenantLexicalFallbackRunsWhenDenseOnlyConfigCannotSearch(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	createKnowledgeRetrievalChineseGeneralKnowledge(t, tenantID, 104, 204, 504, 9)

	provider := &knowledgeRetrievalTestProvider{searchErr: errors.New("qdrant unavailable")}
	service := newKnowledgeRetrievalService()
	service.resolveTenantScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{11},
			TenantSharedKnowledgeBaseIDs: []int64{11},
			EntryKeys:                    []string{"document:104"},
			RevisionIDs:                  []int64{504},
			Languages:                    []string{"default", "en"},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:    false,
			RetrievalMode:    "dense_only",
			DenseTopK:        10,
			LexicalTopK:      8,
			RerankTopN:       0,
			ContextMaxTokens: 1024,
			MaxContextItems:  5,
		}).Normalized()
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "default",
			Audience: "customer",
		},
		TenantKnowledgeOnly: true,
		Query:               "设备长期不用，重新开机前要注意什么？",
		ContextMaxTokens:    1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Strategy.LexicalHitCount == 0 || result.Strategy.AppliedMode != knowledgeRetrievalModeLexicalOnly {
		t.Fatalf("strategy = %+v, want tenant lexical fallback despite dense-only config", result.Strategy)
	}
	if result.Strategy.LexicalReason != "tenant-general-dense-unavailable" {
		t.Fatalf("LexicalReason = %q, want tenant-general-dense-unavailable", result.Strategy.LexicalReason)
	}
	if len(result.ContextHits) == 0 || result.ContextHits[0].DocumentID != 104 {
		t.Fatalf("ContextHits = %+v, want chinese general document", result.ContextHits)
	}
}

func TestKnowledgeRetrievalTenantLexicalFallbackIgnoresActiveGenerationWhenDenseEmpty(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	createKnowledgeRetrievalChineseGeneralKnowledge(t, tenantID, 104, 204, 504, 8)

	provider := &knowledgeRetrievalTestProvider{emptyResults: true}
	service := newKnowledgeRetrievalService()
	service.resolveTenantScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:                      scope,
			KnowledgeBaseIDs:             []int64{11},
			TenantSharedKnowledgeBaseIDs: []int64{11},
			EntryKeys:                    []string{"document:104"},
			RevisionIDs:                  []int64{504},
			Languages:                    []string{"default", "en"},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return &ai.EmbeddingResult{Vector: []float32{0.1, 0.2}, ModelName: "embed-v1", Dimension: 2}, nil
	}
	service.getProvider = func() vectordb.Provider { return provider }
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:    false,
			RetrievalMode:    "dense_only",
			DenseTopK:        10,
			LexicalTopK:      8,
			RerankTopN:       0,
			ContextMaxTokens: 1024,
			MaxContextItems:  5,
		}).Normalized()
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "default",
			Audience: "customer",
		},
		TenantKnowledgeOnly: true,
		Query:               "设备长期不用，重新开机前要注意什么？",
		ContextMaxTokens:    1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Strategy.LexicalHitCount == 0 || result.Strategy.AppliedMode != knowledgeRetrievalModeLexicalOnly {
		t.Fatalf("strategy = %+v, want tenant lexical fallback when dense search is empty", result.Strategy)
	}
	if result.Strategy.LexicalReason != "tenant-general-dense-empty" {
		t.Fatalf("LexicalReason = %q, want tenant-general-dense-empty", result.Strategy.LexicalReason)
	}
	if len(result.ContextHits) == 0 || result.ContextHits[0].DocumentID != 104 {
		t.Fatalf("ContextHits = %+v, want chinese general document from generation fallback", result.ContextHits)
	}
}

func createKnowledgeRetrievalChineseGeneralKnowledge(t *testing.T, tenantID, documentID, chunkID, revisionID, generationID int64) {
	t.Helper()
	now := time.Now()
	if err := sqls.DB().Create(&models.KnowledgeDocument{
		ID:                  documentID,
		TenantID:            tenantID,
		KnowledgeBaseID:     11,
		Title:               "通用售后安全启动指南",
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		Content:             "设备长期不用重新开机前检查外观、管路和急停状态，确认无漏液、异味和松动后再低负载启动。",
		ReviewStatus:        "published",
		Language:            "default",
		Status:              enums.StatusOk,
		IndexStatus:         enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:         "doc-hash-zh-general-helper",
		CurrentRevisionID:   revisionID,
		PublishedRevisionID: revisionID,
		AuditFields:         models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create chinese general document error = %v", err)
	}
	if err := sqls.DB().Create(&models.KnowledgeChunk{
		ID:                chunkID,
		TenantID:          tenantID,
		KnowledgeBaseID:   11,
		DocumentID:        documentID,
		ChunkNo:           1,
		Title:             "设备长期不用重新开机前检查",
		Content:           "设备长期不用重新开机前检查外观、管路和急停状态，确认无漏液、异味和松动后再低负载启动。",
		ContentHash:       "chunk-hash-zh-general-helper",
		ChunkType:         string(enums.KnowledgeChunkTypeText),
		Provider:          "structured",
		RevisionID:        revisionID,
		Language:          "default",
		ReviewStatus:      "published",
		IndexGenerationID: generationID,
		Status:            enums.StatusOk,
		VectorID:          "vec-doc-general-helper",
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("create chinese general chunk error = %v", err)
	}
}

func TestKnowledgeRetrievalReturnsNoAnswerWhenDenseUnavailableAndLexicalMisses(t *testing.T) {
	tenantID := setupKnowledgeRetrievalServiceTestDB(t)
	service := newKnowledgeRetrievalService()
	service.resolveScope = func(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
		return &dto.ResolvedKnowledgeScope{
			Context:          scope,
			KnowledgeBaseIDs: []int64{11},
			EntryKeys:        []string{"document:101"},
			RevisionIDs:      []int64{501},
		}, nil
	}
	service.generateEmbedding = func(context.Context, string) (*ai.EmbeddingResult, error) {
		return nil, errors.New("401 Unauthorized")
	}
	service.getProvider = func() vectordb.Provider { return &knowledgeRetrievalTestProvider{} }
	service.lexicalSearch = func(context.Context, knowledgeLexicalSearchRequest) ([]rag.RetrieveResult, error) {
		return nil, nil
	}
	service.getRAGConfig = func() config.RAGConfig {
		return (config.RAGConfig{
			HybridEnabled:    true,
			RetrievalMode:    "auto",
			DenseTopK:        10,
			LexicalTopK:      5,
			RerankTopN:       0,
			ContextMaxTokens: 1024,
			MaxContextItems:  5,
		}).Normalized()
	}

	result, err := service.Retrieve(context.Background(), KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID: tenantID,
			Locale:   "en-US",
			Audience: "customer",
		},
		Query:            "P-404 reset",
		ContextMaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.NoAnswerReason != "dense-unavailable" {
		t.Fatalf("NoAnswerReason = %q, want dense-unavailable", result.NoAnswerReason)
	}
	if !result.Strategy.LexicalTriggered || result.Strategy.LexicalReason != "dense-unavailable" {
		t.Fatalf("strategy = %+v, want lexical fallback attempt", result.Strategy)
	}
	if len(result.ContextHits) != 0 || result.ContextText != "" {
		t.Fatalf("unexpected context after lexical miss: hits=%+v context=%q", result.ContextHits, result.ContextText)
	}
}

func TestFilterKnowledgeResultsByExplicitTerms(t *testing.T) {
	results := []rag.RetrieveResult{
		{
			DocumentID:      11,
			DocumentTitle:   "E42 散热通道积尘导致风扇送风不足",
			Title:           "E42 散热通道积尘",
			Content:         "控制器降至 43℃ 后连续 30 分钟无负载复测，未再出现 E42。",
			Score:           0.9,
			RetrievalSource: "hybrid",
		},
		{
			DocumentID:      12,
			DocumentTitle:   "PWR-001 压力控制板保护输入端线束端子压接不良",
			Title:           "PWR-001 端子压接",
			Content:         "48V 母线稳定在 47.6V 到 48.8V。",
			Score:           0.8,
			RetrievalSource: "hybrid",
		},
	}

	filtered := filterKnowledgeResultsByExplicitTerms(results, "E42 又出现了，控制器 86℃，下一步怎么办？")
	if len(filtered) != 1 || filtered[0].DocumentID != 11 {
		t.Fatalf("E42 query should keep only E42 result: %+v", filtered)
	}

	filtered = filterKnowledgeResultsByExplicitTerms(results, "PWR-001 端子氧化案例怎么处理？")
	if len(filtered) != 1 || filtered[0].DocumentID != 12 {
		t.Fatalf("PWR-001 query should keep only PWR result: %+v", filtered)
	}

	unfiltered := filterKnowledgeResultsByExplicitTerms(results, "设备停机后怎么做无负载复测？")
	if len(unfiltered) != len(results) {
		t.Fatalf("query without explicit fault identifier must not be filtered: %+v", unfiltered)
	}

	fallback := filterKnowledgeResultsByExplicitTerms(results, "ZZ-999 未知故障码怎么处理？")
	if len(fallback) != len(results) {
		t.Fatalf("unknown explicit term must fall back to original results: %+v", fallback)
	}
}

func firstResultDocumentID(items []rag.RetrieveResult) int64 {
	if len(items) == 0 {
		return 0
	}
	return items[0].DocumentID
}
