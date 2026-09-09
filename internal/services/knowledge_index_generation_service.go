package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeIndexGenerationService = newKnowledgeIndexGenerationService()

func newKnowledgeIndexGenerationService() *knowledgeIndexGenerationService {
	return &knowledgeIndexGenerationService{
		getProvider:       vectordb.GetProvider,
		getDimension:      ai.Embedding.GetDimension,
		getEmbeddingModel: ai.Embedding.GetModel,
		now:               time.Now,
	}
}

type knowledgeIndexGenerationService struct {
	getProvider       func() vectordb.Provider
	getDimension      func(ctx context.Context) (int, error)
	getEmbeddingModel func(ctx context.Context) (*models.AIConfig, error)
	now               func() time.Time
}

type KnowledgeIndexAliasState struct {
	Alias                 string
	AliasTargetCollection string
	AliasDrift            bool
	CollectionPointCount  int64
	DatabaseChunkCount    int64
	VectorPointDrift      bool
	VectorPointDriftCause string
	ActiveGeneration      *models.KnowledgeIndexGeneration
}

func (s *knowledgeIndexGenerationService) ListGenerations(limit int) []models.KnowledgeIndexGeneration {
	alias := strings.TrimSpace(config.CurrentOrDefault().RAG.Normalized().ActiveCollectionAlias)
	if limit <= 0 {
		limit = 20
	}
	cnd := sqls.NewCnd().Desc("id").Limit(limit)
	if alias != "" {
		cnd.Eq("collection_alias", alias)
	}
	return repositories.KnowledgeIndexGenerationRepository.Find(sqls.DB(), cnd)
}

func (s *knowledgeIndexGenerationService) GetGeneration(generationID int64) *models.KnowledgeIndexGeneration {
	if generationID <= 0 {
		return nil
	}
	return repositories.KnowledgeIndexGenerationRepository.Get(sqls.DB(), generationID)
}

func (s *knowledgeIndexGenerationService) ResolveAliasState(ctx context.Context) (*KnowledgeIndexAliasState, error) {
	alias := strings.TrimSpace(config.CurrentOrDefault().RAG.Normalized().ActiveCollectionAlias)
	state := &KnowledgeIndexAliasState{
		Alias:            alias,
		ActiveGeneration: s.GetActiveGeneration(),
	}
	provider := s.getProvider()
	if state.ActiveGeneration != nil && provider != nil {
		vectorState, err := s.resolveGenerationVectorState(ctx, provider, state.ActiveGeneration)
		if err != nil {
			return nil, err
		}
		state.CollectionPointCount = vectorState.CollectionPointCount
		state.DatabaseChunkCount = vectorState.DatabaseChunkCount
		state.VectorPointDrift = vectorState.VectorPointDrift
		state.VectorPointDriftCause = vectorState.VectorPointDriftCause
	}
	if alias == "" {
		return state, nil
	}
	if provider == nil {
		return state, nil
	}
	aliasProvider, ok := provider.(vectordb.AliasProvider)
	if !ok {
		return state, nil
	}
	target, err := aliasProvider.GetAliasTarget(ctx, alias)
	if err != nil {
		return nil, err
	}
	state.AliasTargetCollection = strings.TrimSpace(target)
	if state.ActiveGeneration != nil && state.AliasTargetCollection != "" && state.AliasTargetCollection != state.ActiveGeneration.CollectionName {
		state.AliasDrift = true
	}
	return state, nil
}

func (s *knowledgeIndexGenerationService) EnsureActiveCollectionSchema(ctx context.Context) error {
	active := s.GetActiveGeneration()
	if active == nil {
		return nil
	}
	return s.ensureActiveGenerationReady(ctx, active)
}

func (s *knowledgeIndexGenerationService) ensureActiveGenerationReady(ctx context.Context, active *models.KnowledgeIndexGeneration) error {
	provider := s.getProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	if err := validateKnowledgeGenerationCollection(ctx, provider, active); err != nil {
		return err
	}
	if aliasProvider, ok := provider.(vectordb.AliasProvider); ok {
		aliasTarget, err := aliasProvider.GetAliasTarget(ctx, active.CollectionAlias)
		if err != nil {
			return err
		}
		if strings.TrimSpace(aliasTarget) != active.CollectionName {
			if strings.TrimSpace(aliasTarget) == "" {
				legacyExists, existsErr := aliasProvider.CollectionExists(ctx, active.CollectionAlias)
				if existsErr != nil {
					return existsErr
				}
				if legacyExists {
					migrator, migratorOK := provider.(vectordb.LegacyCollectionAliasMigrator)
					if !migratorOK {
						return fmt.Errorf("vectordb provider cannot migrate legacy collection %s to an alias", active.CollectionAlias)
					}
					if _, migrateErr := migrator.MigrateLegacyCollectionToAlias(ctx, active.CollectionAlias, active.CollectionName, map[string]any{
						"index_generation_id": active.ID,
					}); migrateErr != nil {
						return migrateErr
					}
				} else if err := aliasProvider.SwitchAlias(ctx, active.CollectionAlias, active.CollectionName); err != nil {
					return err
				}
			} else if err := aliasProvider.SwitchAlias(ctx, active.CollectionAlias, active.CollectionName); err != nil {
				return err
			}
			slog.Warn("reconciled knowledge index alias with active generation",
				"alias", active.CollectionAlias,
				"previous_target", aliasTarget,
				"active_collection", active.CollectionName,
				"generation_id", active.ID)
		}
	}
	s.reconcileActiveGenerationVectorDrift(ctx, provider, active)
	ensurer, ok := provider.(vectordb.PayloadIndexEnsurer)
	if !ok {
		return nil
	}
	if err := ensurer.EnsurePayloadIndexes(ctx, active.CollectionName); err != nil {
		return fmt.Errorf("ensure active knowledge collection schema: %w", err)
	}
	return nil
}

type knowledgeGenerationVectorState struct {
	CollectionPointCount  int64
	DatabaseChunkCount    int64
	VectorPointDrift      bool
	VectorPointDriftCause string
}

func (s *knowledgeIndexGenerationService) resolveGenerationVectorState(ctx context.Context, provider vectordb.Provider, generation *models.KnowledgeIndexGeneration) (*knowledgeGenerationVectorState, error) {
	state := &knowledgeGenerationVectorState{}
	if provider == nil || generation == nil || generation.ID <= 0 {
		return state, nil
	}
	info, err := provider.GetCollection(ctx, generation.CollectionName)
	if err != nil {
		return nil, fmt.Errorf("load knowledge index generation collection %s: %w", generation.CollectionName, err)
	}
	if info == nil {
		return nil, fmt.Errorf("knowledge index generation collection %s is unavailable", generation.CollectionName)
	}
	state.CollectionPointCount = int64(info.PointCount)
	state.DatabaseChunkCount = repositories.KnowledgeChunkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("index_generation_id", generation.ID))
	switch {
	case state.CollectionPointCount < state.DatabaseChunkCount:
		state.VectorPointDrift = true
		state.VectorPointDriftCause = "missing_vectors"
	case state.CollectionPointCount > state.DatabaseChunkCount:
		state.VectorPointDrift = true
		state.VectorPointDriftCause = "stale_vectors"
	}
	return state, nil
}

func (s *knowledgeIndexGenerationService) reconcileActiveGenerationVectorDrift(ctx context.Context, provider vectordb.Provider, generation *models.KnowledgeIndexGeneration) {
	ragCfg := config.CurrentOrDefault().RAG.Normalized()
	if !ragCfg.Enabled || !ragCfg.IngestionEnabled || generation == nil {
		return
	}
	state, err := s.resolveGenerationVectorState(ctx, provider, generation)
	if err != nil {
		slog.Warn("check knowledge index vector drift failed", "generation_id", generation.ID, "error", err)
		return
	}
	if !state.VectorPointDrift || state.CollectionPointCount >= state.DatabaseChunkCount {
		return
	}
	pending, err := s.hasActiveGenerationBackfillWork(generation.ID)
	if err != nil {
		slog.Warn("check knowledge index vector drift backfill failed", "generation_id", generation.ID, "error", err)
		return
	}
	if pending {
		return
	}
	limit := ragCfg.TaskBatchSize * 10
	if limit < 100 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	count, err := s.EnqueueBackfillForGeneration(ctx, generation.ID, &dto.AuthPrincipal{Username: "knowledge-index-reconciler"}, limit)
	if err != nil {
		slog.Warn("enqueue knowledge index vector drift backfill failed",
			"generation_id", generation.ID,
			"collection_points", state.CollectionPointCount,
			"database_chunks", state.DatabaseChunkCount,
			"error", err)
		return
	}
	if count > 0 {
		slog.Warn("enqueued knowledge index vector drift backfill",
			"generation_id", generation.ID,
			"collection_points", state.CollectionPointCount,
			"database_chunks", state.DatabaseChunkCount,
			"enqueued_count", count)
	}
}

func (s *knowledgeIndexGenerationService) hasActiveGenerationBackfillWork(generationID int64) (bool, error) {
	if generationID <= 0 {
		return false, nil
	}
	var count int64
	err := sqls.DB().Model(&models.KnowledgeIndexSyncTask{}).
		Where("index_generation_id = ?", generationID).
		Where("status IN ?", []string{"pending", "running", "waiting_retry"}).
		Count(&count).Error
	return count > 0, err
}

func (s *knowledgeIndexGenerationService) EnsureActiveGeneration(ctx context.Context, operator *dto.AuthPrincipal) (*models.KnowledgeIndexGeneration, error) {
	ragCfg := config.CurrentOrDefault().RAG.Normalized()
	alias := strings.TrimSpace(ragCfg.ActiveCollectionAlias)
	if alias == "" {
		return nil, fmt.Errorf("rag active collection alias is required")
	}

	dimension, err := s.getDimension(ctx)
	if err != nil {
		return nil, err
	}
	model, err := s.getEmbeddingModel(ctx)
	if err != nil {
		return nil, err
	}
	if active := repositories.KnowledgeIndexGenerationRepository.FindActiveByAlias(sqls.DB(), alias); active != nil {
		if knowledgeGenerationMatches(active, ragCfg, model.ModelName, dimension) {
			if err := s.ensureActiveGenerationReady(ctx, active); err != nil {
				return nil, err
			}
			return active, nil
		}
		return nil, fmt.Errorf("active knowledge index generation uses embedding model %s/%d, requested %s/%d; prepare, backfill, and activate a compatible generation before switching embedding source",
			active.EmbeddingModel, active.Dimension, strings.TrimSpace(model.ModelName), dimension)
	}
	generation, err := s.prepareCompatibleGeneration(ctx, ragCfg, model.ModelName, dimension, operator)
	if err != nil {
		return nil, err
	}
	if err := s.ActivateGeneration(ctx, generation.ID, operator); err != nil {
		return nil, err
	}
	return repositories.KnowledgeIndexGenerationRepository.Get(sqls.DB(), generation.ID), nil
}

func (s *knowledgeIndexGenerationService) ensureCollectionSchema(ctx context.Context, collectionName string) error {
	provider := s.getProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	ensurer, ok := provider.(vectordb.PayloadIndexEnsurer)
	if !ok {
		return nil
	}
	return ensurer.EnsurePayloadIndexes(ctx, collectionName)
}

func (s *knowledgeIndexGenerationService) PrepareCompatibleGeneration(ctx context.Context, operator *dto.AuthPrincipal) (*models.KnowledgeIndexGeneration, error) {
	ragCfg := config.CurrentOrDefault().RAG.Normalized()
	if strings.TrimSpace(ragCfg.ActiveCollectionAlias) == "" {
		return nil, fmt.Errorf("rag active collection alias is required")
	}
	dimension, err := s.getDimension(ctx)
	if err != nil {
		return nil, err
	}
	model, err := s.getEmbeddingModel(ctx)
	if err != nil {
		return nil, err
	}
	if active := repositories.KnowledgeIndexGenerationRepository.FindActiveByAlias(sqls.DB(), ragCfg.ActiveCollectionAlias); knowledgeGenerationMatches(active, ragCfg, model.ModelName, dimension) {
		return active, nil
	}
	return s.prepareCompatibleGeneration(ctx, ragCfg, model.ModelName, dimension, operator)
}

func (s *knowledgeIndexGenerationService) prepareCompatibleGeneration(ctx context.Context, ragCfg config.RAGConfig, modelName string, dimension int, operator *dto.AuthPrincipal) (*models.KnowledgeIndexGeneration, error) {
	alias := strings.TrimSpace(ragCfg.ActiveCollectionAlias)

	provider := s.getProvider()
	if provider == nil {
		return nil, fmt.Errorf("vectordb provider not initialized")
	}
	if _, ok := provider.(vectordb.AliasProvider); !ok {
		return nil, fmt.Errorf("vectordb provider does not support alias management")
	}

	if ready := repositories.KnowledgeIndexGenerationRepository.FindLatestByAlias(sqls.DB(), alias); reusableKnowledgeGeneration(ready, ragCfg, modelName, dimension) {
		return ready, nil
	}

	now := s.now()
	collectionName := buildKnowledgeGenerationCollectionName(ragCfg.CollectionPrefix, ragCfg.SchemaVersion, modelName, now)
	generation := &models.KnowledgeIndexGeneration{
		CollectionName:   collectionName,
		CollectionAlias:  alias,
		SchemaVersion:    ragCfg.SchemaVersion,
		EmbeddingModel:   strings.TrimSpace(modelName),
		EmbeddingVersion: strings.TrimSpace(modelName),
		Dimension:        dimension,
		Status:           "building",
		AuditFields:      buildGenerationAuditFields(operator, now),
	}

	if err := provider.CreateCollection(ctx, generation.CollectionName, generation.Dimension); err != nil {
		return nil, err
	}
	if err := validateKnowledgeGenerationCollection(ctx, provider, generation); err != nil {
		return nil, err
	}
	generation.Status = "ready"
	if err := repositories.KnowledgeIndexGenerationRepository.Create(sqls.DB(), generation); err != nil {
		return nil, err
	}
	return generation, nil
}

func (s *knowledgeIndexGenerationService) ActivateGeneration(ctx context.Context, generationID int64, operator *dto.AuthPrincipal) error {
	generation := repositories.KnowledgeIndexGenerationRepository.Get(sqls.DB(), generationID)
	if generation == nil {
		return fmt.Errorf("knowledge index generation not found")
	}
	provider := s.getProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	aliasProvider, ok := provider.(vectordb.AliasProvider)
	if !ok {
		return fmt.Errorf("vectordb provider does not support alias management")
	}
	exists, err := aliasProvider.CollectionExists(ctx, generation.CollectionName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("knowledge index generation collection %s does not exist", generation.CollectionName)
	}
	if err := validateKnowledgeGenerationCollection(ctx, provider, generation); err != nil {
		return err
	}

	aliasTarget, err := aliasProvider.GetAliasTarget(ctx, generation.CollectionAlias)
	if err != nil {
		return err
	}
	legacyCollectionExists := false
	if aliasTarget == "" && generation.CollectionAlias != generation.CollectionName {
		legacyCollectionExists, err = aliasProvider.CollectionExists(ctx, generation.CollectionAlias)
		if err != nil {
			return err
		}
	}

	migratedPointCount := -1
	if legacyCollectionExists {
		migrator, ok := provider.(vectordb.LegacyCollectionAliasMigrator)
		if !ok {
			return fmt.Errorf("vectordb provider cannot migrate legacy collection %s to an alias", generation.CollectionAlias)
		}
		migratedPointCount, err = migrator.MigrateLegacyCollectionToAlias(ctx, generation.CollectionAlias, generation.CollectionName, map[string]any{
			"index_generation_id": generation.ID,
		})
		if err != nil {
			return err
		}
	} else {
		if err := aliasProvider.SwitchAlias(ctx, generation.CollectionAlias, generation.CollectionName); err != nil {
			return err
		}
		// Recover an interrupted legacy migration where the source was already
		// removed and the copied target was preserved before alias creation.
		if (aliasTarget == "" || aliasTarget == generation.CollectionName) && generation.Status == "ready" && generation.PointCount == 0 {
			if info, infoErr := provider.GetCollection(ctx, generation.CollectionName); infoErr == nil && info != nil && info.PointCount > 0 {
				migratedPointCount = info.PointCount
			}
		}
	}
	if migratedPointCount >= 0 {
		if err := s.finishLegacyGenerationMigration(generation.ID, migratedPointCount); err != nil {
			return err
		}
	}

	now := s.now()
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.KnowledgeIndexGeneration{}).
			Where("collection_alias = ? AND status = ?", generation.CollectionAlias, "active").
			Where("id <> ?", generation.ID).
			Updates(map[string]any{
				"status":           "retired",
				"retired_at":       now,
				"updated_at":       now,
				"update_user_id":   operatorUserID(operator),
				"update_user_name": operatorUsername(operator),
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.KnowledgeIndexGeneration{}).Where("id = ?", generation.ID).Updates(map[string]any{
			"status":           "active",
			"activated_at":     now,
			"retired_at":       nil,
			"updated_at":       now,
			"update_user_id":   operatorUserID(operator),
			"update_user_name": operatorUsername(operator),
		}).Error
	})
}

func validateKnowledgeGenerationCollection(ctx context.Context, provider vectordb.Provider, generation *models.KnowledgeIndexGeneration) error {
	if generation == nil {
		return fmt.Errorf("knowledge index generation is required")
	}
	info, err := provider.GetCollection(ctx, generation.CollectionName)
	if err != nil {
		return fmt.Errorf("load knowledge index generation collection %s: %w", generation.CollectionName, err)
	}
	if info == nil {
		return fmt.Errorf("knowledge index generation collection %s is unavailable", generation.CollectionName)
	}
	if generation.Dimension <= 0 {
		return fmt.Errorf("knowledge index generation %d has invalid dimension %d", generation.ID, generation.Dimension)
	}
	if info.Dimension != generation.Dimension {
		return fmt.Errorf("knowledge index generation %d dimension mismatch: collection %s uses %d, generation requires %d", generation.ID, generation.CollectionName, info.Dimension, generation.Dimension)
	}
	return nil
}

func reusableKnowledgeGeneration(generation *models.KnowledgeIndexGeneration, ragCfg config.RAGConfig, embeddingModel string, dimension int) bool {
	if generation == nil || generation.Status != "ready" {
		return false
	}
	return knowledgeGenerationMatches(generation, ragCfg, embeddingModel, dimension)
}

func knowledgeGenerationMatches(generation *models.KnowledgeIndexGeneration, ragCfg config.RAGConfig, embeddingModel string, dimension int) bool {
	if generation == nil {
		return false
	}
	return generation.CollectionAlias == strings.TrimSpace(ragCfg.ActiveCollectionAlias) &&
		generation.SchemaVersion == ragCfg.SchemaVersion &&
		generation.EmbeddingModel == strings.TrimSpace(embeddingModel) &&
		generation.Dimension == dimension
}

func (s *knowledgeIndexGenerationService) finishLegacyGenerationMigration(generationID int64, pointCount int) error {
	now := s.now()
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.KnowledgeIndexGeneration{}).Where("id = ?", generationID).Updates(map[string]any{
			"point_count": pointCount,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.KnowledgeChunk{}).Where("index_generation_id = ?", 0).Updates(map[string]any{
			"index_generation_id": generationID,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.KnowledgeRevision{}).Where("index_generation_id = ?", 0).Updates(map[string]any{
			"index_generation_id": generationID,
			"updated_at":          now,
		}).Error
	})
}

func (s *knowledgeIndexGenerationService) GetActiveGeneration() *models.KnowledgeIndexGeneration {
	alias := strings.TrimSpace(config.CurrentOrDefault().RAG.Normalized().ActiveCollectionAlias)
	if alias == "" {
		return nil
	}
	return repositories.KnowledgeIndexGenerationRepository.FindActiveByAlias(sqls.DB(), alias)
}

func (s *knowledgeIndexGenerationService) EnqueueBackfillForGeneration(ctx context.Context, generationID int64, operator *dto.AuthPrincipal, limit int) (int, error) {
	generation := repositories.KnowledgeIndexGenerationRepository.Get(sqls.DB(), generationID)
	if generation == nil {
		return 0, fmt.Errorf("knowledge index generation not found")
	}
	if limit <= 0 {
		limit = 100
	}
	count := 0
	documents := repositories.KnowledgeDocumentRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("review_status", "published").
		Eq("status", enums.StatusOk).
		Gt("published_revision_id", 0).
		Asc("id").
		Limit(limit))
	for i := range documents {
		task, err := KnowledgeIndexSyncService.requestPublishedEntryReindexToGeneration(
			ctx,
			documents[i].TenantID,
			"document",
			documents[i].ID,
			generation,
			operator,
		)
		if err != nil {
			return count, err
		}
		if task != nil {
			if KnowledgeIndexSyncService.requeueTerminalTask(task) {
				count++
			} else if isKnowledgeIndexTaskActive(task) {
				count++
			}
		}
	}
	if count >= limit {
		return count, nil
	}
	faqs := repositories.KnowledgeFAQRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("review_status", "published").
		Eq("status", enums.StatusOk).
		Gt("published_revision_id", 0).
		Asc("id").
		Limit(limit-count))
	for i := range faqs {
		task, err := KnowledgeIndexSyncService.requestPublishedEntryReindexToGeneration(
			ctx,
			faqs[i].TenantID,
			"faq",
			faqs[i].ID,
			generation,
			operator,
		)
		if err != nil {
			return count, err
		}
		if task != nil {
			if KnowledgeIndexSyncService.requeueTerminalTask(task) {
				count++
			} else if isKnowledgeIndexTaskActive(task) {
				count++
			}
		}
	}
	slog.Info("knowledge index generation backfill enqueued", "generation_id", generationID, "count", count)
	return count, nil
}

func isKnowledgeIndexTaskActive(task *models.KnowledgeIndexSyncTask) bool {
	if task == nil {
		return false
	}
	return task.Status == "pending" || task.Status == "running" || task.Status == "waiting_retry"
}

func buildKnowledgeGenerationCollectionName(prefix string, schemaVersion int, embeddingModel string, now time.Time) string {
	safeModel := strings.NewReplacer("/", "_", ":", "_", ".", "_", "-", "_", " ", "_").Replace(strings.TrimSpace(embeddingModel))
	if safeModel == "" {
		safeModel = "embedding"
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "knowledge_chunks"
	}
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	return fmt.Sprintf("%s_v%d_%s_%s", prefix, schemaVersion, safeModel, now.UTC().Format("20060102_150405"))
}

func buildGenerationAuditFields(operator *dto.AuthPrincipal, now time.Time) models.AuditFields {
	fields := models.AuditFields{
		CreatedAt: now,
		UpdatedAt: now,
	}
	if operator != nil {
		fields = models.AuditFields{
			CreatedAt:      now,
			UpdatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operatorUsername(operator),
			UpdateUserID:   operator.UserID,
			UpdateUserName: operatorUsername(operator),
		}
	}
	return fields
}
