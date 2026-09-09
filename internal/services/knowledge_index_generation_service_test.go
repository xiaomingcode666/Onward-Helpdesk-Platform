package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type legacyGenerationTestProvider struct {
	aliasTarget       string
	legacyExists      bool
	targetExists      bool
	targetPointCount  int
	migrationCalls    int
	switchCalls       int
	payloadOverrides  map[string]any
	migrationLegacy   string
	migrationTarget   string
	payloadIndexCalls []string
	collectionDims    map[string]int
}

func (p *legacyGenerationTestProvider) CreateCollection(_ context.Context, name string, dimension int) error {
	if p.collectionDims == nil {
		p.collectionDims = make(map[string]int)
	}
	p.collectionDims[name] = dimension
	p.targetExists = true
	return nil
}
func (p *legacyGenerationTestProvider) DeleteCollection(context.Context, string) error { return nil }
func (p *legacyGenerationTestProvider) GetCollection(_ context.Context, name string) (*vectordb.CollectionInfo, error) {
	dimension := 1024
	if configured, ok := p.collectionDims[name]; ok {
		dimension = configured
	}
	return &vectordb.CollectionInfo{Name: name, Dimension: dimension, PointCount: p.targetPointCount}, nil
}
func (p *legacyGenerationTestProvider) ListCollections(context.Context) ([]string, error) {
	return nil, nil
}
func (p *legacyGenerationTestProvider) UpsertVectors(context.Context, string, []vectordb.Vector) error {
	return nil
}
func (p *legacyGenerationTestProvider) DeleteVectors(context.Context, string, []string) error {
	return nil
}
func (p *legacyGenerationTestProvider) Search(context.Context, *vectordb.SearchRequest) ([]vectordb.SearchResult, error) {
	return nil, nil
}
func (p *legacyGenerationTestProvider) Close() error { return nil }
func (p *legacyGenerationTestProvider) GetAliasTarget(context.Context, string) (string, error) {
	return p.aliasTarget, nil
}
func (p *legacyGenerationTestProvider) SwitchAlias(_ context.Context, _, collectionName string) error {
	p.switchCalls++
	p.aliasTarget = collectionName
	return nil
}
func (p *legacyGenerationTestProvider) CollectionExists(_ context.Context, name string) (bool, error) {
	if name == "knowledge_chunks_active" {
		return p.legacyExists, nil
	}
	return p.targetExists, nil
}
func (p *legacyGenerationTestProvider) MigrateLegacyCollectionToAlias(_ context.Context, legacyCollectionName, targetCollectionName string, payloadOverrides map[string]any) (int, error) {
	p.migrationCalls++
	p.migrationLegacy = legacyCollectionName
	p.migrationTarget = targetCollectionName
	p.payloadOverrides = payloadOverrides
	p.legacyExists = false
	p.aliasTarget = targetCollectionName
	return p.targetPointCount, nil
}
func (p *legacyGenerationTestProvider) EnsurePayloadIndexes(_ context.Context, collectionName string) error {
	p.payloadIndexCalls = append(p.payloadIndexCalls, collectionName)
	return nil
}

func TestEnsureActiveGenerationMigratesLegacyCollectionAndReusesReadyGeneration(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.KnowledgeIndexGeneration{}, &models.KnowledgeChunk{}, &models.KnowledgeRevision{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{RAG: config.RAGConfig{
		ActiveCollectionAlias: "knowledge_chunks_active",
		CollectionPrefix:      "knowledge_chunks",
		SchemaVersion:         2,
	}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })

	now := time.Now()
	generation := &models.KnowledgeIndexGeneration{
		CollectionName:   "knowledge_chunks_v2_text_embedding_v3_ready",
		CollectionAlias:  "knowledge_chunks_active",
		SchemaVersion:    2,
		EmbeddingModel:   "text-embedding-v3",
		EmbeddingVersion: "text-embedding-v3",
		Dimension:        1024,
		Status:           "ready",
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(generation).Error; err != nil {
		t.Fatalf("create generation error = %v", err)
	}
	chunk := &models.KnowledgeChunk{
		TenantID: 1, KnowledgeBaseID: 1, DocumentID: 2, VectorID: "vector-legacy",
		RevisionID: 2, IndexGenerationID: 0, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(chunk).Error; err != nil {
		t.Fatalf("create chunk error = %v", err)
	}
	revision := &models.KnowledgeRevision{
		TenantID: 1, KnowledgeBaseID: 1, EntryType: "document", EntryID: 2,
		VersionNo: 1, ReviewStatus: "published", IndexGenerationID: 0,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(revision).Error; err != nil {
		t.Fatalf("create revision error = %v", err)
	}

	provider := &legacyGenerationTestProvider{
		legacyExists:     true,
		targetExists:     true,
		targetPointCount: 18,
	}
	service := newKnowledgeIndexGenerationService()
	service.getProvider = func() vectordb.Provider { return provider }
	service.getDimension = func(context.Context) (int, error) { return 1024, nil }
	service.getEmbeddingModel = func(context.Context) (*models.AIConfig, error) {
		return &models.AIConfig{ModelName: "text-embedding-v3"}, nil
	}

	active, err := service.EnsureActiveGeneration(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureActiveGeneration() error = %v", err)
	}
	if active == nil || active.ID != generation.ID || active.Status != "active" {
		t.Fatalf("active generation = %#v, want reused generation %d", active, generation.ID)
	}
	if provider.migrationCalls != 1 || provider.switchCalls != 0 {
		t.Fatalf("migration calls = %d, switch calls = %d, want 1 and 0", provider.migrationCalls, provider.switchCalls)
	}
	if provider.migrationLegacy != "knowledge_chunks_active" || provider.migrationTarget != generation.CollectionName {
		t.Fatalf("migration = %s -> %s", provider.migrationLegacy, provider.migrationTarget)
	}
	if got := provider.payloadOverrides["index_generation_id"]; got != generation.ID {
		t.Fatalf("generation payload override = %#v, want %d", got, generation.ID)
	}

	if err := db.First(chunk, chunk.ID).Error; err != nil {
		t.Fatalf("reload chunk error = %v", err)
	}
	if chunk.IndexGenerationID != generation.ID {
		t.Fatalf("chunk generation = %d, want %d", chunk.IndexGenerationID, generation.ID)
	}
	if err := db.First(revision, revision.ID).Error; err != nil {
		t.Fatalf("reload revision error = %v", err)
	}
	if revision.IndexGenerationID != generation.ID {
		t.Fatalf("revision generation = %d, want %d", revision.IndexGenerationID, generation.ID)
	}
	if active.PointCount != 18 {
		t.Fatalf("active point count = %d, want 18", active.PointCount)
	}

	again, err := service.EnsureActiveGeneration(context.Background(), nil)
	if err != nil {
		t.Fatalf("second EnsureActiveGeneration() error = %v", err)
	}
	if again.ID != generation.ID || provider.migrationCalls != 1 {
		t.Fatalf("second ensure = %#v, migration calls = %d", again, provider.migrationCalls)
	}
	if len(provider.payloadIndexCalls) != 1 || provider.payloadIndexCalls[0] != generation.CollectionName {
		t.Fatalf("payload index calls = %#v, want active collection", provider.payloadIndexCalls)
	}

	service.getDimension = func(context.Context) (int, error) { return 1536, nil }
	service.getEmbeddingModel = func(context.Context) (*models.AIConfig, error) {
		return &models.AIConfig{ModelName: "future-embedding-model"}, nil
	}
	if mismatched, mismatchErr := service.EnsureActiveGeneration(context.Background(), nil); mismatched != nil || mismatchErr == nil || !strings.Contains(mismatchErr.Error(), "prepare, backfill, and activate a compatible generation") {
		t.Fatalf("mismatched ensure = (%#v, %v), want guarded model-switch error", mismatched, mismatchErr)
	}
	prepared, prepareErr := service.PrepareCompatibleGeneration(context.Background(), nil)
	if prepareErr != nil {
		t.Fatalf("PrepareCompatibleGeneration() error = %v", prepareErr)
	}
	if prepared == nil || prepared.ID == generation.ID || prepared.Status != "ready" || prepared.EmbeddingModel != "future-embedding-model" || prepared.Dimension != 1536 {
		t.Fatalf("prepared generation = %#v", prepared)
	}
}

func TestEnsureActiveCollectionSchemaRepairsAliasDrift(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.KnowledgeIndexGeneration{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{RAG: config.RAGConfig{ActiveCollectionAlias: "knowledge_chunks_active"}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })

	active := &models.KnowledgeIndexGeneration{
		CollectionName:  "knowledge_chunks_v2_text_embedding_v3_active",
		CollectionAlias: "knowledge_chunks_active",
		EmbeddingModel:  "text-embedding-v3",
		Dimension:       1024,
		Status:          "active",
		AuditFields:     models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create generation error = %v", err)
	}

	provider := &legacyGenerationTestProvider{
		aliasTarget:  "knowledge_chunks_v2_test_embedding",
		targetExists: true,
		collectionDims: map[string]int{
			active.CollectionName:                1024,
			"knowledge_chunks_v2_test_embedding": 8,
		},
	}
	service := newKnowledgeIndexGenerationService()
	service.getProvider = func() vectordb.Provider { return provider }

	if err := service.EnsureActiveCollectionSchema(context.Background()); err != nil {
		t.Fatalf("EnsureActiveCollectionSchema() error = %v", err)
	}
	if provider.aliasTarget != active.CollectionName || provider.switchCalls != 1 {
		t.Fatalf("alias target = %q, switch calls = %d", provider.aliasTarget, provider.switchCalls)
	}
	if len(provider.payloadIndexCalls) != 1 || provider.payloadIndexCalls[0] != active.CollectionName {
		t.Fatalf("payload index calls = %#v", provider.payloadIndexCalls)
	}
}

func TestEnsureActiveCollectionSchemaRequeuesBackfillWhenCollectionLost(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.KnowledgeIndexGeneration{},
		&models.KnowledgeChunk{},
		&models.KnowledgeRevision{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeIndexSyncTask{},
		&models.ProductKnowledgeLink{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{RAG: config.RAGConfig{
		Enabled:               true,
		IngestionEnabled:      true,
		ActiveCollectionAlias: "knowledge_chunks_active",
		TaskBatchSize:         20,
	}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })

	now := time.Now()
	active := &models.KnowledgeIndexGeneration{
		CollectionName:   "knowledge_chunks_v2_text_embedding_v3_active",
		CollectionAlias:  "knowledge_chunks_active",
		SchemaVersion:    2,
		EmbeddingModel:   "text-embedding-v3",
		EmbeddingVersion: "text-embedding-v3",
		Dimension:        1024,
		Status:           "active",
		PointCount:       1,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create generation error = %v", err)
	}
	knowledgeBase := &models.KnowledgeBase{
		TenantID:      1,
		Name:          "产品知识库",
		KnowledgeType: "document",
		AccessScope:   "tenant",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(knowledgeBase).Error; err != nil {
		t.Fatalf("create knowledge base error = %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID:        1,
		KnowledgeBaseID: knowledgeBase.ID,
		Title:           "启动失败排查",
		Content:         "检查安全互锁和上电顺序。",
		ContentHash:     knowledgeContentHash("检查安全互锁和上电顺序。"),
		ReviewStatus:    "published",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document error = %v", err)
	}
	revision := &models.KnowledgeRevision{
		TenantID:        1,
		KnowledgeBaseID: knowledgeBase.ID,
		EntryType:       "document",
		EntryID:         document.ID,
		VersionNo:       1,
		Title:           document.Title,
		Content:         document.Content,
		ContentHash:     document.ContentHash,
		ReviewStatus:    "published",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(revision).Error; err != nil {
		t.Fatalf("create revision error = %v", err)
	}
	if err := db.Model(document).Updates(map[string]any{
		"current_revision_id":   revision.ID,
		"published_revision_id": revision.ID,
	}).Error; err != nil {
		t.Fatalf("update document revision error = %v", err)
	}
	if err := db.Create(&models.KnowledgeChunk{
		TenantID: 1, KnowledgeBaseID: knowledgeBase.ID, DocumentID: document.ID, RevisionID: revision.ID,
		IndexGenerationID: active.ID, VectorID: "lost-vector", Status: enums.StatusOk,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create chunk error = %v", err)
	}
	task, err := KnowledgeIndexSyncService.EnqueueEntrySyncToGeneration(context.Background(), 1, knowledgeBase.ID, "knowledge_document", document.ID, revision.ID, document.ContentHash, "upsert", active, nil)
	if err != nil {
		t.Fatalf("enqueue initial task error = %v", err)
	}
	oldTaskUpdatedAt := now.Add(-time.Hour)
	if err := db.Model(task).Updates(map[string]any{
		"status":     "succeeded",
		"updated_at": oldTaskUpdatedAt,
	}).Error; err != nil {
		t.Fatalf("mark task succeeded error = %v", err)
	}

	provider := &legacyGenerationTestProvider{
		aliasTarget:       active.CollectionName,
		targetExists:      true,
		targetPointCount:  0,
		collectionDims:    map[string]int{active.CollectionName: 1024},
		payloadIndexCalls: nil,
	}
	service := newKnowledgeIndexGenerationService()
	service.getProvider = func() vectordb.Provider { return provider }

	if err := service.EnsureActiveCollectionSchema(context.Background()); err != nil {
		t.Fatalf("EnsureActiveCollectionSchema() error = %v", err)
	}

	var requeued models.KnowledgeIndexSyncTask
	if err := db.First(&requeued, task.ID).Error; err != nil {
		t.Fatalf("reload task error = %v", err)
	}
	if requeued.Status != "pending" || requeued.NextAttemptAt == nil {
		t.Fatalf("task status = %q next = %v, want pending with next attempt", requeued.Status, requeued.NextAttemptAt)
	}
	var reloaded models.KnowledgeDocument
	if err := db.First(&reloaded, document.ID).Error; err != nil {
		t.Fatalf("reload document error = %v", err)
	}
	if reloaded.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("document index status = %q, want pending", reloaded.IndexStatus)
	}
	secondDocument := &models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: knowledgeBase.ID, Title: "第二篇文档",
		Content: "second published document", ContentHash: "hash-second",
		ReviewStatus: "published", Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(secondDocument).Error; err != nil {
		t.Fatalf("create second document error = %v", err)
	}
	secondRevision := &models.KnowledgeRevision{
		TenantID: 1, KnowledgeBaseID: knowledgeBase.ID, EntryType: "document", EntryID: secondDocument.ID,
		VersionNo: 1, ContentHash: secondDocument.ContentHash, ReviewStatus: "published",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(secondRevision).Error; err != nil {
		t.Fatalf("create second revision error = %v", err)
	}
	if err := db.Model(secondDocument).Updates(map[string]any{
		"current_revision_id":   secondRevision.ID,
		"published_revision_id": secondRevision.ID,
	}).Error; err != nil {
		t.Fatalf("update second document revision error = %v", err)
	}
	if err := db.Create(&models.KnowledgeChunk{
		TenantID: 1, KnowledgeBaseID: knowledgeBase.ID, DocumentID: secondDocument.ID, RevisionID: secondRevision.ID,
		IndexGenerationID: active.ID, VectorID: "second-lost-vector", Status: enums.StatusOk,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create second chunk error = %v", err)
	}
	var taskCountBefore int64
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Where("index_generation_id = ?", active.ID).Count(&taskCountBefore).Error; err != nil {
		t.Fatalf("count tasks before second ensure error = %v", err)
	}
	if err := service.EnsureActiveCollectionSchema(context.Background()); err != nil {
		t.Fatalf("second EnsureActiveCollectionSchema() error = %v", err)
	}
	var taskCountAfter int64
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Where("index_generation_id = ?", active.ID).Count(&taskCountAfter).Error; err != nil {
		t.Fatalf("count tasks after second ensure error = %v", err)
	}
	if taskCountAfter != taskCountBefore {
		t.Fatalf("task count after second ensure = %d, want %d while backfill is already pending", taskCountAfter, taskCountBefore)
	}
	state, err := service.ResolveAliasState(context.Background())
	if err != nil {
		t.Fatalf("ResolveAliasState() error = %v", err)
	}
	if !state.VectorPointDrift || state.VectorPointDriftCause != "missing_vectors" || state.CollectionPointCount != 0 || state.DatabaseChunkCount != 2 {
		t.Fatalf("vector state = %#v, want missing vector drift 0/2", state)
	}
}

func TestEnsureActiveCollectionSchemaRejectsGenerationCollectionDimensionMismatch(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.KnowledgeIndexGeneration{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{RAG: config.RAGConfig{ActiveCollectionAlias: "knowledge_chunks_active"}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })

	active := &models.KnowledgeIndexGeneration{
		CollectionName:  "knowledge_chunks_v2_text_embedding_v3_active",
		CollectionAlias: "knowledge_chunks_active",
		Dimension:       1024,
		Status:          "active",
		AuditFields:     models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create generation error = %v", err)
	}

	provider := &legacyGenerationTestProvider{
		aliasTarget: active.CollectionName,
		collectionDims: map[string]int{
			active.CollectionName: 8,
		},
	}
	service := newKnowledgeIndexGenerationService()
	service.getProvider = func() vectordb.Provider { return provider }

	err = service.EnsureActiveCollectionSchema(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dimension mismatch") {
		t.Fatalf("EnsureActiveCollectionSchema() error = %v, want dimension mismatch", err)
	}
	if provider.switchCalls != 0 || len(provider.payloadIndexCalls) != 0 {
		t.Fatalf("invalid collection should not be activated: switch=%d indexes=%#v", provider.switchCalls, provider.payloadIndexCalls)
	}
}
