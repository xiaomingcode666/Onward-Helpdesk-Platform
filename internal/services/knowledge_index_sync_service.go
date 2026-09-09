package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeIndexSyncService = newKnowledgeIndexSyncService()

func newKnowledgeIndexSyncService() *knowledgeIndexSyncService {
	return &knowledgeIndexSyncService{
		now:         time.Now,
		taskTimeout: 2 * time.Minute,
		leaseTTL:    10 * time.Minute,
	}
}

type knowledgeIndexSyncService struct {
	now         func() time.Time
	taskTimeout time.Duration
	leaseTTL    time.Duration
}

func (s *knowledgeIndexSyncService) EnqueueLinkSync(link *models.ProductKnowledgeLink, action, trigger string, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if link == nil {
		return nil, errorsx.InvalidParam("knowledge link is required")
	}
	ctx := ai.WithCapabilityScope(context.Background(), ai.CapabilityScope{
		TenantID:        link.TenantID,
		ProductID:       link.ProductID,
		KnowledgeBaseID: link.KnowledgeBaseID,
	})
	generation, err := KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return nil, err
	}
	spec, err := s.resolveLinkTaskSpec(link)
	if err != nil {
		return nil, err
	}
	entryScope := rag.ResolveKnowledgeEntryProductScope(sqls.DB(), link.TenantID, link.KnowledgeBaseID, spec.SubjectID, spec.SubjectType)
	task := models.KnowledgeIndexSyncTask{
		TenantID:          link.TenantID,
		SubjectType:       spec.SubjectType,
		SubjectID:         spec.SubjectID,
		KnowledgeBaseID:   link.KnowledgeBaseID,
		ProductID:         link.ProductID,
		RevisionID:        spec.RevisionID,
		IndexGenerationID: generation.ID,
		CollectionName:    generation.CollectionName,
		ContentHash:       spec.ContentHash,
		Action:            action,
		InputVersion:      spec.InputVersion + ":scope:" + entryScope.Fingerprint,
	}
	task.IdempotencyKey = buildKnowledgeIndexTaskKey(task.TenantID, spec.EntryKey, task.RevisionID, task.IndexGenerationID, action, entryScope.Fingerprint)
	return s.enqueue(sqls.DB(), &task, operator)
}

func (s *knowledgeIndexSyncService) EnqueueEntrySync(tenantID, knowledgeBaseID int64, entryType string, entryID int64, reviewStatus, action string, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	switch strings.TrimSpace(reviewStatus) {
	case "published", "deprecated":
	default:
		return nil, nil
	}

	ctx := ai.WithCapabilityScope(context.Background(), ai.CapabilityScope{
		TenantID:        tenantID,
		KnowledgeBaseID: knowledgeBaseID,
	})
	generation, err := KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return nil, err
	}
	if generation == nil {
		return nil, errorsx.InvalidParam("index generation is required")
	}

	switch strings.TrimSpace(entryType) {
	case "document", "knowledge_document":
		document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), entryID)
		if document == nil || document.TenantID != tenantID {
			return nil, errorsx.InvalidParam("knowledge document not found")
		}
		revisionID := resolvePublishedDocumentRevisionID(*document)
		if revisionID <= 0 {
			return nil, errorsx.InvalidParam("published document revision is required")
		}
		contentHash := strings.TrimSpace(document.ContentHash)
		if contentHash == "" {
			contentHash = knowledgeContentHash(strings.TrimSpace(document.Content))
		}
		task, err := s.buildEntryTask(sqls.DB(), tenantID, knowledgeBaseID, "knowledge_document", document.ID, revisionID, contentHash, action, generation)
		if err != nil {
			return nil, err
		}
		return s.enqueue(sqls.DB(), task, operator)
	case "faq", "knowledge_faq":
		faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), entryID)
		if faq == nil || faq.TenantID != tenantID {
			return nil, errorsx.InvalidParam("knowledge faq not found")
		}
		revisionID := resolvePublishedFAQRevisionID(*faq)
		if revisionID <= 0 {
			return nil, errorsx.InvalidParam("published faq revision is required")
		}
		contentHash := knowledgeContentHash(strings.TrimSpace(faq.Question) + "\n" + strings.TrimSpace(faq.Answer))
		task, err := s.buildEntryTask(sqls.DB(), tenantID, knowledgeBaseID, "knowledge_faq", faq.ID, revisionID, contentHash, action, generation)
		if err != nil {
			return nil, err
		}
		return s.enqueue(sqls.DB(), task, operator)
	default:
		return nil, errorsx.InvalidParam("invalid subject type")
	}
}

// RequestEntryReindex validates the current published revision and routes a
// manual rebuild through the same durable task pipeline as normal publishing.
func (s *knowledgeIndexSyncService) RequestEntryReindex(ctx context.Context, tenantID int64, entryType string, entryID int64, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || entryID <= 0 {
		return nil, errorsx.InvalidParam("published knowledge entry is required")
	}
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{TenantID: tenantID})
	generation, err := KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return nil, err
	}
	return s.requestPublishedEntryReindexToGeneration(ctx, tenantID, entryType, entryID, generation, operator)
}

// RequestKnowledgeBaseReindex enqueues all and only the published revisions in
// a knowledge base. It does not clear the active collection, so a partial
// enqueue failure cannot make the whole knowledge base unavailable.
func (s *knowledgeIndexSyncService) RequestKnowledgeBaseReindex(ctx context.Context, tenantID, knowledgeBaseID int64, operator *dto.AuthPrincipal) (int, error) {
	if operator == nil {
		return 0, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk {
		return 0, errorsx.InvalidParam("knowledge base not found")
	}
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{TenantID: tenantID, KnowledgeBaseID: knowledgeBaseID})
	generation, err := KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return 0, err
	}

	entryType := "document"
	model := any(&models.KnowledgeDocument{})
	if knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		entryType = "faq"
		model = &models.KnowledgeFAQ{}
	}
	var ids []int64
	if err := sqls.DB().Model(model).
		Where("tenant_id = ? AND knowledge_base_id = ? AND review_status = ? AND status = ? AND published_revision_id > 0",
			tenantID, knowledgeBaseID, "published", enums.StatusOk).
		Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}

	enqueued := 0
	errs := make([]error, 0)
	for _, entryID := range ids {
		task, requestErr := s.requestPublishedEntryReindexToGeneration(ctx, tenantID, entryType, entryID, generation, operator)
		if requestErr != nil {
			errs = append(errs, fmt.Errorf("entry %d: %w", entryID, requestErr))
			continue
		}
		if task != nil {
			enqueued++
		}
	}
	return enqueued, errors.Join(errs...)
}

func (s *knowledgeIndexSyncService) requestPublishedEntryReindexToGeneration(ctx context.Context, tenantID int64, entryType string, entryID int64, generation *models.KnowledgeIndexGeneration, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	now := s.now()
	switch strings.TrimSpace(entryType) {
	case "faq", "knowledge_faq":
		item := repositories.KnowledgeFAQRepository.Get(sqls.DB(), entryID)
		if item == nil || item.TenantID != tenantID || item.Status != enums.StatusOk || item.ReviewStatus != "published" || item.PublishedRevisionID <= 0 {
			return nil, errorsx.InvalidParam("knowledge faq is not published")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(
			sqls.DB(), item.PublishedRevisionID, tenantID, item.KnowledgeBaseID, "faq", item.ID,
		) == nil {
			return nil, errorsx.InvalidParam("published faq revision is missing or invalid")
		}
		if err := repositories.KnowledgeFAQRepository.Updates(sqls.DB(), item.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusPending,
			"indexed_at":   nil,
			"index_error":  "",
			"updated_at":   now,
		}); err != nil {
			return nil, err
		}
		contentHash := knowledgeContentHash(strings.TrimSpace(item.Question) + "\n" + strings.TrimSpace(item.Answer))
		return s.EnqueueEntrySyncToGeneration(ctx, tenantID, item.KnowledgeBaseID, "knowledge_faq", item.ID, item.PublishedRevisionID, contentHash, "upsert", generation, operator)
	case "document", "knowledge_document":
		item := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), entryID)
		if item == nil || item.TenantID != tenantID || item.Status != enums.StatusOk || item.ReviewStatus != "published" || item.PublishedRevisionID <= 0 {
			return nil, errorsx.InvalidParam("knowledge document is not published")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(
			sqls.DB(), item.PublishedRevisionID, tenantID, item.KnowledgeBaseID, "document", item.ID,
		) == nil {
			return nil, errorsx.InvalidParam("published document revision is missing or invalid")
		}
		if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), item.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusPending,
			"indexed_at":   nil,
			"index_error":  "",
			"updated_at":   now,
		}); err != nil {
			return nil, err
		}
		contentHash := strings.TrimSpace(item.ContentHash)
		if contentHash == "" {
			contentHash = knowledgeContentHash(strings.TrimSpace(item.Content))
		}
		return s.EnqueueEntrySyncToGeneration(ctx, tenantID, item.KnowledgeBaseID, "knowledge_document", item.ID, item.PublishedRevisionID, contentHash, "upsert", generation, operator)
	default:
		return nil, errorsx.InvalidParam("invalid knowledge entry type")
	}
}

// EnqueueEntryRevisionSync targets a captured revision instead of resolving the
// entry's current revision. It is used when editing an indexed entry so a
// concurrent republish cannot redirect the cleanup task to the new revision.
func (s *knowledgeIndexSyncService) EnqueueEntryRevisionSync(tenantID, knowledgeBaseID int64, entryType string, entryID, revisionID int64, contentHash, action string, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if tenantID <= 0 || knowledgeBaseID <= 0 || entryID <= 0 || revisionID <= 0 {
		return nil, errorsx.InvalidParam("knowledge entry revision scope is required")
	}
	ctx := ai.WithCapabilityScope(context.Background(), ai.CapabilityScope{
		TenantID: tenantID, KnowledgeBaseID: knowledgeBaseID,
	})
	generation, err := KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return nil, err
	}
	subjectType := "knowledge_document"
	if strings.TrimSpace(entryType) == "faq" || strings.TrimSpace(entryType) == "knowledge_faq" {
		subjectType = "knowledge_faq"
	}
	task, err := s.buildEntryTask(sqls.DB(), tenantID, knowledgeBaseID, subjectType, entryID, revisionID, contentHash, action, generation)
	if err != nil {
		return nil, err
	}
	return s.enqueue(sqls.DB(), task, operator)
}

func (s *knowledgeIndexSyncService) EnqueueEntrySyncToGeneration(ctx context.Context, tenantID, knowledgeBaseID int64, subjectType string, subjectID, revisionID int64, contentHash, action string, generation *models.KnowledgeIndexGeneration, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if generation == nil {
		return nil, errorsx.InvalidParam("index generation is required")
	}
	task, err := s.buildEntryTask(sqls.DB(), tenantID, knowledgeBaseID, subjectType, subjectID, revisionID, contentHash, action, generation)
	if err != nil {
		return nil, err
	}
	return s.enqueue(sqls.DB(), task, operator)
}

func (s *knowledgeIndexSyncService) EnqueueEntrySyncToGenerationTx(db *gorm.DB, tenantID, knowledgeBaseID int64, subjectType string, subjectID, revisionID int64, contentHash, action string, generation *models.KnowledgeIndexGeneration, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if db == nil {
		db = sqls.DB()
	}
	if generation == nil {
		return nil, errorsx.InvalidParam("index generation is required")
	}
	task, err := s.buildEntryTask(db, tenantID, knowledgeBaseID, subjectType, subjectID, revisionID, contentHash, action, generation)
	if err != nil {
		return nil, err
	}
	return s.enqueue(db, task, operator)
}

func (s *knowledgeIndexSyncService) buildEntryTask(db *gorm.DB, tenantID, knowledgeBaseID int64, subjectType string, subjectID, revisionID int64, contentHash, action string, generation *models.KnowledgeIndexGeneration) (*models.KnowledgeIndexSyncTask, error) {
	entryKey, inputVersion, err := s.resolveEntryTaskKey(subjectType, subjectID, revisionID)
	if err != nil {
		return nil, err
	}
	entryScope := rag.ResolveKnowledgeEntryProductScope(db, tenantID, knowledgeBaseID, subjectID, subjectType)
	task := &models.KnowledgeIndexSyncTask{
		TenantID:          tenantID,
		SubjectType:       subjectType,
		SubjectID:         subjectID,
		KnowledgeBaseID:   knowledgeBaseID,
		RevisionID:        revisionID,
		IndexGenerationID: generation.ID,
		CollectionName:    generation.CollectionName,
		ContentHash:       strings.TrimSpace(contentHash),
		ProviderType:      "internal",
		Action:            action,
		InputVersion:      inputVersion + ":scope:" + entryScope.Fingerprint,
	}
	if len(entryScope.ProductIDs) == 1 {
		task.ProductID = entryScope.ProductIDs[0]
	}
	task.IdempotencyKey = buildKnowledgeIndexTaskKey(task.TenantID, entryKey, task.RevisionID, task.IndexGenerationID, task.Action, entryScope.Fingerprint)
	return task, nil
}

func (s *knowledgeIndexSyncService) enqueue(db *gorm.DB, task *models.KnowledgeIndexSyncTask, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if db == nil {
		db = sqls.DB()
	}
	if task == nil {
		return nil, errorsx.InvalidParam("index task is required")
	}
	if existing := repositories.KnowledgeIndexSyncTaskRepository.GetByIdempotencyKey(db, task.IdempotencyKey); existing != nil {
		if s.requeueSucceededTaskForNewerEntryState(db, existing) {
			return repositories.KnowledgeIndexSyncTaskRepository.Get(db, existing.ID), nil
		}
		return existing, nil
	}
	now := s.now()
	task.ProviderType = "internal"
	task.Status = "pending"
	task.MaxRetries = normalizeKnowledgeTaskMaxRetries(task.MaxRetries)
	task.NextAttemptAt = &now
	task.LockOwner = ""
	task.ErrorCode = ""
	task.ErrorSummary = ""
	task.AuditFields = buildGenerationAuditFields(operator, now)
	if err := repositories.KnowledgeIndexSyncTaskRepository.Create(db, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *knowledgeIndexSyncService) requeueSucceededTaskForNewerEntryState(db *gorm.DB, task *models.KnowledgeIndexSyncTask) bool {
	if db == nil || task == nil || task.Status != "succeeded" {
		return false
	}
	var reviewStatus string
	var indexStatus enums.KnowledgeDocumentIndexStatus
	var entryUpdatedAt time.Time
	switch task.SubjectType {
	case "knowledge_faq":
		var item models.KnowledgeFAQ
		if err := db.Select("review_status", "index_status", "updated_at").First(&item, task.SubjectID).Error; err != nil {
			return false
		}
		reviewStatus, indexStatus, entryUpdatedAt = item.ReviewStatus, item.IndexStatus, item.UpdatedAt
	default:
		var item models.KnowledgeDocument
		if err := db.Select("review_status", "index_status", "updated_at").First(&item, task.SubjectID).Error; err != nil {
			return false
		}
		reviewStatus, indexStatus, entryUpdatedAt = item.ReviewStatus, item.IndexStatus, item.UpdatedAt
	}
	desiredState := reviewStatus == "published"
	if task.Action == "delete" {
		desiredState = reviewStatus != "published"
	}
	if !desiredState || indexStatus != enums.KnowledgeDocumentIndexStatusPending || !entryUpdatedAt.After(task.UpdatedAt) {
		return false
	}
	now := s.now()
	return repositories.KnowledgeIndexSyncTaskRepository.Updates(db, task.ID, map[string]any{
		"status":          "pending",
		"retry_count":     0,
		"next_attempt_at": now,
		"locked_at":       nil,
		"lock_owner":      "",
		"error_code":      "",
		"error_summary":   "",
		"last_attempt_at": nil,
		"finished_at":     nil,
		"updated_at":      now,
	}) == nil
}

func (s *knowledgeIndexSyncService) RetryTask(tenantID, taskID int64, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	task := repositories.KnowledgeIndexSyncTaskRepository.Get(sqls.DB(), taskID)
	if task == nil || task.TenantID != tenantID {
		return nil, errorsx.InvalidParam("index sync task not found")
	}
	now := s.now()
	if err := repositories.KnowledgeIndexSyncTaskRepository.Updates(sqls.DB(), taskID, map[string]any{
		"status":           "pending",
		"error_code":       "",
		"error_summary":    "",
		"retry_count":      0,
		"next_attempt_at":  now,
		"locked_at":        nil,
		"lock_owner":       "",
		"last_attempt_at":  nil,
		"finished_at":      nil,
		"update_user_id":   operator.UserID,
		"update_user_name": operatorUsername(operator),
		"updated_at":       now,
	}); err != nil {
		return nil, err
	}
	s.markEntryIndexPending(task, now)
	return repositories.KnowledgeIndexSyncTaskRepository.Get(sqls.DB(), taskID), nil
}

func (s *knowledgeIndexSyncService) GetTask(tenantID, taskID int64) (*models.KnowledgeIndexSyncTask, error) {
	task := repositories.KnowledgeIndexSyncTaskRepository.Get(sqls.DB(), taskID)
	if task == nil || task.TenantID != tenantID {
		return nil, errorsx.InvalidParam("index sync task not found")
	}
	return task, nil
}

func (s *knowledgeIndexSyncService) ListTasks(tenantID, productID int64) []models.KnowledgeIndexSyncTask {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).Desc("id").Limit(100)
	if productID > 0 {
		cnd.Eq("product_id", productID)
	}
	return repositories.KnowledgeIndexSyncTaskRepository.Find(sqls.DB(), cnd)
}

func (s *knowledgeIndexSyncService) ProcessDueTasks(ctx context.Context, limit int) int {
	ragCfg := config.CurrentOrDefault().RAG.Normalized()
	if !ragCfg.Enabled || !ragCfg.IngestionEnabled {
		return 0
	}
	if limit <= 0 {
		limit = ragCfg.TaskBatchSize
	}
	now := s.now()
	lockOwner := fmt.Sprintf("rag-worker-%d", now.UnixNano())
	tasks, err := repositories.KnowledgeIndexSyncTaskRepository.ClaimDueTasks(sqls.DB(), now, limit, lockOwner, now.Add(s.leaseTTL))
	if err != nil {
		slog.Warn("claim knowledge index tasks failed", "error", err)
		return 0
	}
	processed := 0
	for i := range tasks {
		item := tasks[i]
		if err := s.ProcessTask(ctx, &item); err != nil {
			slog.Warn("process knowledge index task failed", "task_id", item.ID, "error", err)
		}
		processed++
	}
	return processed
}

func (s *knowledgeIndexSyncService) RecoverTimedOutTasks() int64 {
	now := s.now()
	recovered, err := repositories.KnowledgeIndexSyncTaskRepository.RecoverExpiredRunningTasks(sqls.DB(), now.Add(-s.leaseTTL), now)
	if err != nil {
		slog.Warn("recover knowledge index tasks failed", "error", err)
		return 0
	}
	return recovered
}

// ReconcileEntryTasks repairs the gap where an entry status transaction
// committed but its index task could not be created or was terminally failed.
func (s *knowledgeIndexSyncService) ReconcileEntryTasks(limit int) int {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	recovered := s.reconcileSupersededRevisionTasks(limit)
	remaining := limit - recovered
	if remaining <= 0 {
		return recovered
	}
	var documents []models.KnowledgeDocument
	if err := sqls.DB().Where("status <> ? AND index_status <> ? AND (review_status IN ? OR (review_status IN ? AND published_revision_id > 0))",
		enums.StatusDeleted, enums.KnowledgeDocumentIndexStatusIndexed, []string{"published", "deprecated"}, []string{"draft", "review"}).
		Order("CASE WHEN index_status = 'pending' THEN 0 ELSE 1 END ASC, updated_at ASC, id ASC").Limit(remaining).Find(&documents).Error; err == nil {
		for i := range documents {
			action := "upsert"
			knowledgeBaseID := documents[i].KnowledgeBaseID
			reviewStatus := documents[i].ReviewStatus
			if documents[i].ReviewStatus != "published" {
				action = "delete"
				reviewStatus = "deprecated"
				knowledgeBaseID = resolveKnowledgeIndexRevisionBaseID(documents[i].PublishedRevisionID, knowledgeBaseID)
			}
			operator := &dto.AuthPrincipal{TenantID: documents[i].TenantID, Username: "knowledge-index-reconciler"}
			task, err := s.EnqueueEntrySync(documents[i].TenantID, knowledgeBaseID, "document", documents[i].ID, reviewStatus, action, operator)
			if err == nil && task != nil {
				if s.requeueTerminalTask(task) {
					recovered++
				} else if task.Status == "pending" || task.Status == "waiting_retry" || task.Status == "running" {
					recovered++
				}
			}
		}
	}
	remaining = limit - recovered
	if remaining <= 0 {
		return recovered
	}
	var faqs []models.KnowledgeFAQ
	if err := sqls.DB().Where("status <> ? AND index_status <> ? AND (review_status IN ? OR (review_status IN ? AND published_revision_id > 0))",
		enums.StatusDeleted, enums.KnowledgeDocumentIndexStatusIndexed, []string{"published", "deprecated"}, []string{"draft", "review"}).
		Order("CASE WHEN index_status = 'pending' THEN 0 ELSE 1 END ASC, updated_at ASC, id ASC").Limit(remaining).Find(&faqs).Error; err == nil {
		for i := range faqs {
			action := "upsert"
			knowledgeBaseID := faqs[i].KnowledgeBaseID
			reviewStatus := faqs[i].ReviewStatus
			if faqs[i].ReviewStatus != "published" {
				action = "delete"
				reviewStatus = "deprecated"
				knowledgeBaseID = resolveKnowledgeIndexRevisionBaseID(faqs[i].PublishedRevisionID, knowledgeBaseID)
			}
			operator := &dto.AuthPrincipal{TenantID: faqs[i].TenantID, Username: "knowledge-index-reconciler"}
			task, err := s.EnqueueEntrySync(faqs[i].TenantID, knowledgeBaseID, "faq", faqs[i].ID, reviewStatus, action, operator)
			if err == nil && task != nil {
				if s.requeueTerminalTask(task) {
					recovered++
				} else if task.Status == "pending" || task.Status == "waiting_retry" || task.Status == "running" {
					recovered++
				}
			}
		}
	}
	return recovered
}

func (s *knowledgeIndexSyncService) reconcileSupersededRevisionTasks(limit int) int {
	if limit <= 0 || !sqls.DB().Migrator().HasTable(&models.KnowledgeChunk{}) || !sqls.DB().Migrator().HasTable(&models.KnowledgeRevision{}) {
		return 0
	}
	generation := KnowledgeIndexGenerationService.GetActiveGeneration()
	if generation == nil {
		return 0
	}
	revisionIDs := sqls.DB().Model(&models.KnowledgeChunk{}).
		Select("DISTINCT revision_id").
		Where("index_generation_id = ? AND revision_id > 0", generation.ID)
	var revisions []models.KnowledgeRevision
	if err := sqls.DB().Where("id IN (?) AND superseded_at IS NOT NULL", revisionIDs).
		Order("superseded_at ASC, id ASC").Limit(limit).Find(&revisions).Error; err != nil {
		return 0
	}
	recovered := 0
	for i := range revisions {
		operator := &dto.AuthPrincipal{TenantID: revisions[i].TenantID, Username: "knowledge-index-reconciler"}
		task, err := s.EnqueueEntryRevisionSync(
			revisions[i].TenantID, revisions[i].KnowledgeBaseID, revisions[i].EntryType, revisions[i].EntryID,
			revisions[i].ID, revisions[i].ContentHash, "delete", operator,
		)
		if err == nil && task != nil {
			if s.requeueTerminalTask(task) {
				recovered++
			} else if task.Status == "pending" || task.Status == "waiting_retry" || task.Status == "running" {
				recovered++
			}
		}
	}
	return recovered
}

func resolveKnowledgeIndexRevisionBaseID(revisionID, fallbackKnowledgeBaseID int64) int64 {
	if revisionID <= 0 {
		return fallbackKnowledgeBaseID
	}
	revision := repositories.KnowledgeRevisionRepository.Get(sqls.DB(), revisionID)
	if revision == nil || revision.KnowledgeBaseID <= 0 {
		return fallbackKnowledgeBaseID
	}
	return revision.KnowledgeBaseID
}

func (s *knowledgeIndexSyncService) requeueTerminalTask(task *models.KnowledgeIndexSyncTask) bool {
	if task == nil || (task.Status != "failed" && task.Status != "cancelled") {
		return false
	}
	now := s.now()
	if !recoverableLocalEmbeddingFailure(task) && !task.UpdatedAt.IsZero() && task.UpdatedAt.After(now.Add(-time.Hour)) {
		return false
	}
	if err := repositories.KnowledgeIndexSyncTaskRepository.Updates(sqls.DB(), task.ID, map[string]any{
		"status":          "pending",
		"retry_count":     0,
		"next_attempt_at": now,
		"locked_at":       nil,
		"lock_owner":      "",
		"error_code":      "",
		"error_summary":   "",
		"finished_at":     nil,
		"updated_at":      now,
	}); err != nil {
		return false
	}
	s.markEntryIndexPending(task, now)
	return true
}

func recoverableLocalEmbeddingFailure(task *models.KnowledgeIndexSyncTask) bool {
	if task == nil || task.Status != "failed" || task.ErrorCode != "embedding_error" {
		return false
	}
	summary := strings.ToLower(task.ErrorSummary)
	return strings.Contains(summary, "127.0.0.1:18099") || strings.Contains(summary, "localhost:18099")
}

func (s *knowledgeIndexSyncService) ProcessTask(ctx context.Context, task *models.KnowledgeIndexSyncTask) error {
	if task == nil {
		return errorsx.InvalidParam("task is required")
	}
	taskCtx := ctx
	if taskCtx == nil {
		taskCtx = context.Background()
	}
	var cancel context.CancelFunc
	taskCtx, cancel = context.WithTimeout(taskCtx, s.taskTimeout)
	defer cancel()

	now := s.now()
	_ = repositories.KnowledgeIndexSyncTaskRepository.Updates(sqls.DB(), task.ID, map[string]any{
		"status":          "running",
		"last_attempt_at": now,
		"locked_at":       now,
		"updated_at":      now,
	})

	markEntry, err := s.reconcileTaskRevision(taskCtx, task)
	if err != nil {
		return s.failTask(task, err)
	}
	return s.finishTask(task, markEntry)
}

// reconcileTaskRevision makes every task converge its revision to the entry's
// latest state. This prevents a delayed upsert from reviving deprecated content
// and a delayed delete from removing a revision that has just been republished.
func (s *knowledgeIndexSyncService) reconcileTaskRevision(ctx context.Context, task *models.KnowledgeIndexSyncTask) (bool, error) {
	for attempt := 0; attempt < 3; attempt++ {
		desiredUpsert, _ := s.taskRevisionDesiredState(task)
		var err error
		if desiredUpsert {
			err = s.processInternalUpsert(ctx, task)
		} else {
			err = s.processInternalDelete(ctx, task)
		}
		if err != nil {
			return false, err
		}
		currentDesiredUpsert, markEntry := s.taskRevisionDesiredState(task)
		if currentDesiredUpsert == desiredUpsert {
			return markEntry, nil
		}
	}
	return false, fmt.Errorf("knowledge entry index state changed repeatedly during synchronization")
}

func (s *knowledgeIndexSyncService) taskRevisionDesiredState(task *models.KnowledgeIndexSyncTask) (bool, bool) {
	if task == nil {
		return false, false
	}
	switch task.SubjectType {
	case "knowledge_faq":
		item := repositories.KnowledgeFAQRepository.Get(sqls.DB(), task.SubjectID)
		if item == nil || item.TenantID != task.TenantID {
			return false, false
		}
		matchesRevision := item.PublishedRevisionID == task.RevisionID
		return matchesRevision && item.ReviewStatus == "published" && item.Status == enums.StatusOk, matchesRevision
	default:
		item := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), task.SubjectID)
		if item == nil || item.TenantID != task.TenantID {
			return false, false
		}
		matchesRevision := item.PublishedRevisionID == task.RevisionID
		return matchesRevision && item.ReviewStatus == "published" && item.Status == enums.StatusOk, matchesRevision
	}
}

func (s *knowledgeIndexSyncService) processInternalUpsert(ctx context.Context, task *models.KnowledgeIndexSyncTask) error {
	revision := repositories.KnowledgeRevisionRepository.Get(sqls.DB(), task.RevisionID)
	if revision == nil || revision.TenantID != task.TenantID || revision.KnowledgeBaseID != task.KnowledgeBaseID {
		return fmt.Errorf("revision not found")
	}
	expectedEntryType := "document"
	if task.SubjectType == "knowledge_faq" {
		expectedEntryType = "faq"
	}
	if revision.EntryType != expectedEntryType || revision.EntryID != task.SubjectID {
		return fmt.Errorf("revision does not belong to the task subject")
	}
	if strings.TrimSpace(revision.ReviewStatus) != "published" {
		return fmt.Errorf("revision is not published")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), task.KnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != task.TenantID || knowledgeBase.Status != enums.StatusOk {
		return fmt.Errorf("knowledge base not found")
	}
	target := rag.IndexTarget{
		CollectionName:    task.CollectionName,
		IndexGenerationID: task.IndexGenerationID,
	}
	switch task.SubjectType {
	case "knowledge_faq":
		current := repositories.KnowledgeFAQRepository.Get(sqls.DB(), task.SubjectID)
		if current == nil || current.TenantID != task.TenantID {
			return fmt.Errorf("knowledge faq not found")
		}
		snapshot := *current
		snapshot.Question = revision.Title
		snapshot.Answer = revision.Content
		snapshot.Language = revision.Language
		snapshot.ReviewStatus = revision.ReviewStatus
		snapshot.CurrentRevisionID = revision.ID
		snapshot.PublishedRevisionID = revision.ID
		return rag.Index.IndexFAQToTarget(ctx, snapshot, *knowledgeBase, target)
	default:
		current := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), task.SubjectID)
		if current == nil || current.TenantID != task.TenantID {
			return fmt.Errorf("knowledge document not found")
		}
		snapshot := *current
		snapshot.Title = revision.Title
		snapshot.Content = revision.Content
		snapshot.ContentHash = revision.ContentHash
		snapshot.Language = revision.Language
		snapshot.ReviewStatus = revision.ReviewStatus
		snapshot.CurrentRevisionID = revision.ID
		snapshot.PublishedRevisionID = revision.ID
		return rag.Index.IndexDocumentToTarget(ctx, snapshot, *knowledgeBase, target)
	}
}

func (s *knowledgeIndexSyncService) processInternalDelete(ctx context.Context, task *models.KnowledgeIndexSyncTask) error {
	provider := vectordb.GetProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	entryKey, _, err := s.resolveEntryTaskKey(task.SubjectType, task.SubjectID, task.RevisionID)
	if err != nil {
		return err
	}
	filter := &vectordb.SearchFilter{
		TenantID:          task.TenantID,
		EntryKeys:         []string{entryKey},
		KnowledgeBaseIDs:  []int64{task.KnowledgeBaseID},
		ReviewStatus:      "published",
		RevisionIDs:       []int64{task.RevisionID},
		IndexGenerationID: task.IndexGenerationID,
	}
	if deleteProvider, ok := provider.(vectordb.FilterDeleteProvider); ok {
		if err := deleteProvider.DeleteByFilter(ctx, task.CollectionName, filter); err != nil {
			return err
		}
	} else {
		vectorIDs := s.collectTaskChunkVectorIDs(task)
		if len(vectorIDs) > 0 {
			if err := provider.DeleteVectors(ctx, task.CollectionName, vectorIDs); err != nil {
				return err
			}
		}
	}
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		switch task.SubjectType {
		case "knowledge_faq":
			return tx.Where("faq_id = ? AND revision_id = ? AND index_generation_id = ?", task.SubjectID, task.RevisionID, task.IndexGenerationID).
				Delete(&models.KnowledgeChunk{}).Error
		default:
			return tx.Where("document_id = ? AND revision_id = ? AND index_generation_id = ?", task.SubjectID, task.RevisionID, task.IndexGenerationID).
				Delete(&models.KnowledgeChunk{}).Error
		}
	})
}

func (s *knowledgeIndexSyncService) finishTask(task *models.KnowledgeIndexSyncTask, markEntry bool) error {
	now := s.now()
	if err := repositories.KnowledgeIndexSyncTaskRepository.Updates(sqls.DB(), task.ID, map[string]any{
		"status":        "succeeded",
		"error_code":    "",
		"error_summary": "",
		"locked_at":     nil,
		"lock_owner":    "",
		"finished_at":   now,
		"updated_at":    now,
	}); err != nil {
		return err
	}
	if markEntry {
		s.markEntryIndexed(task, now)
	}
	s.updateGenerationPointCount(task.IndexGenerationID)
	return nil
}

func (s *knowledgeIndexSyncService) failTask(task *models.KnowledgeIndexSyncTask, cause error) error {
	retryCount := task.RetryCount + 1
	now := s.now()
	var nextAttemptAt any = now.Add(backoffForKnowledgeTask(retryCount))
	status := "waiting_retry"
	finishedAt := any(nil)
	if retryCount >= normalizeKnowledgeTaskMaxRetries(task.MaxRetries) {
		status = "failed"
		nextAttemptAt = nil
		finishedAt = now
	}
	errorCode := classifyKnowledgeTaskError(cause)
	persistErr := repositories.KnowledgeIndexSyncTaskRepository.Updates(sqls.DB(), task.ID, map[string]any{
		"status":          status,
		"retry_count":     retryCount,
		"next_attempt_at": nextAttemptAt,
		"locked_at":       nil,
		"lock_owner":      "",
		"error_code":      errorCode,
		"error_summary":   truncateErrorSummary(cause.Error()),
		"finished_at":     finishedAt,
		"updated_at":      now,
	})
	s.markEntryIndexFailed(task, errorCode, cause, status == "failed")
	if persistErr != nil {
		return errors.Join(cause, fmt.Errorf("persist knowledge index retry state: %w", persistErr))
	}
	return cause
}

func (s *knowledgeIndexSyncService) resolveLinkTaskSpec(link *models.ProductKnowledgeLink) (*knowledgeEntryTaskSpec, error) {
	switch {
	case s.entryIsFAQ(link.TenantID, link.KnowledgeBaseID, link.KnowledgeEntryID):
		faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID)
		if faq == nil || faq.TenantID != link.TenantID || faq.Status != enums.StatusOk || faq.ReviewStatus != "published" {
			return nil, errorsx.InvalidParam("knowledge faq not found")
		}
		revisionID := resolvePublishedFAQRevisionID(*faq)
		if revisionID <= 0 {
			return nil, errorsx.InvalidParam("published faq revision is required")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(sqls.DB(), revisionID, link.TenantID, link.KnowledgeBaseID, "faq", faq.ID) == nil {
			return nil, errorsx.InvalidParam("published faq revision is missing or invalid")
		}
		return &knowledgeEntryTaskSpec{
			SubjectType:  "knowledge_faq",
			SubjectID:    faq.ID,
			EntryKey:     fmt.Sprintf("faq:%d", faq.ID),
			RevisionID:   revisionID,
			ContentHash:  knowledgeContentHash(strings.TrimSpace(faq.Question) + "\n" + strings.TrimSpace(faq.Answer)),
			InputVersion: fmt.Sprintf("rev:%d", revisionID),
		}, nil
	default:
		document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID)
		if document == nil || document.TenantID != link.TenantID || document.Status != enums.StatusOk || document.ReviewStatus != "published" {
			return nil, errorsx.InvalidParam("knowledge document not found")
		}
		revisionID := resolvePublishedDocumentRevisionID(*document)
		if revisionID <= 0 {
			return nil, errorsx.InvalidParam("published document revision is required")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(sqls.DB(), revisionID, link.TenantID, link.KnowledgeBaseID, "document", document.ID) == nil {
			return nil, errorsx.InvalidParam("published document revision is missing or invalid")
		}
		return &knowledgeEntryTaskSpec{
			SubjectType:  "knowledge_document",
			SubjectID:    document.ID,
			EntryKey:     fmt.Sprintf("document:%d", document.ID),
			RevisionID:   revisionID,
			ContentHash:  strings.TrimSpace(document.ContentHash),
			InputVersion: fmt.Sprintf("rev:%d", revisionID),
		}, nil
	}
}

func (s *knowledgeIndexSyncService) resolveEntryTaskKey(subjectType string, subjectID, revisionID int64) (string, string, error) {
	switch strings.TrimSpace(subjectType) {
	case "knowledge_faq":
		return fmt.Sprintf("faq:%d", subjectID), fmt.Sprintf("rev:%d", revisionID), nil
	case "knowledge_document":
		return fmt.Sprintf("document:%d", subjectID), fmt.Sprintf("rev:%d", revisionID), nil
	default:
		return "", "", errorsx.InvalidParam("invalid subject type")
	}
}

func (s *knowledgeIndexSyncService) entryIsFAQ(tenantID, knowledgeBaseID, entryID int64) bool {
	if knowledgeBaseID > 0 {
		if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID); kb != nil && kb.KnowledgeType == "faq" {
			return true
		}
	}
	if faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), entryID); faq != nil && faq.TenantID == tenantID {
		return true
	}
	return false
}

func (s *knowledgeIndexSyncService) collectTaskChunkVectorIDs(task *models.KnowledgeIndexSyncTask) []string {
	var chunks []models.KnowledgeChunk
	switch task.SubjectType {
	case "knowledge_faq":
		chunks = repositories.KnowledgeChunkRepository.FindByFAQRevisionGeneration(sqls.DB(), task.SubjectID, task.RevisionID, task.IndexGenerationID)
	default:
		chunks = repositories.KnowledgeChunkRepository.FindByDocumentRevisionGeneration(sqls.DB(), task.SubjectID, task.RevisionID, task.IndexGenerationID)
	}
	vectorIDs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.VectorID) != "" {
			vectorIDs = append(vectorIDs, chunk.VectorID)
		}
	}
	return vectorIDs
}

func (s *knowledgeIndexSyncService) markEntryIndexed(task *models.KnowledgeIndexSyncTask, indexedAt time.Time) {
	updates := map[string]any{
		"index_status": "indexed",
		"indexed_at":   indexedAt,
		"index_error":  "",
		"updated_at":   indexedAt,
	}
	switch task.SubjectType {
	case "knowledge_faq":
		_ = repositories.KnowledgeFAQRepository.Updates(sqls.DB(), task.SubjectID, updates)
	default:
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), task.SubjectID, updates)
		_ = sqls.DB().Model(&models.ProductManualFile{}).
			Where("tenant_id = ? AND knowledge_document_id = ? AND status <> ?", task.TenantID, task.SubjectID, enums.StatusDeleted).
			Updates(map[string]any{
				"sync_status": "indexed",
				"sync_error":  "",
				"synced_at":   indexedAt,
				"updated_at":  indexedAt,
			}).Error
	}
}

func (s *knowledgeIndexSyncService) markEntryIndexFailed(task *models.KnowledgeIndexSyncTask, errorCode string, cause error, terminal bool) {
	now := s.now()
	indexStatus := "pending"
	if terminal {
		indexStatus = "failed"
	}
	updates := map[string]any{
		"index_status": indexStatus,
		"index_error":  fmt.Sprintf("%s: %s", errorCode, truncateErrorSummary(cause.Error())),
		"updated_at":   now,
	}
	switch task.SubjectType {
	case "knowledge_faq":
		_ = repositories.KnowledgeFAQRepository.Updates(sqls.DB(), task.SubjectID, updates)
	default:
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), task.SubjectID, updates)
		manualStatus := "pending"
		if terminal {
			manualStatus = "failed"
		}
		_ = sqls.DB().Model(&models.ProductManualFile{}).
			Where("tenant_id = ? AND knowledge_document_id = ? AND status <> ?", task.TenantID, task.SubjectID, enums.StatusDeleted).
			Updates(map[string]any{
				"sync_status": manualStatus,
				"sync_error":  updates["index_error"],
				"synced_at":   nil,
				"updated_at":  now,
			}).Error
	}
}

func (s *knowledgeIndexSyncService) markEntryIndexPending(task *models.KnowledgeIndexSyncTask, now time.Time) {
	updates := map[string]any{
		"index_status": "pending",
		"index_error":  "",
		"indexed_at":   nil,
		"updated_at":   now,
	}
	switch task.SubjectType {
	case "knowledge_faq":
		_ = repositories.KnowledgeFAQRepository.Updates(sqls.DB(), task.SubjectID, updates)
	default:
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), task.SubjectID, updates)
		_ = sqls.DB().Model(&models.ProductManualFile{}).
			Where("tenant_id = ? AND knowledge_document_id = ? AND status <> ?", task.TenantID, task.SubjectID, enums.StatusDeleted).
			Updates(map[string]any{
				"sync_status": "pending",
				"sync_error":  "",
				"synced_at":   nil,
				"updated_at":  now,
			}).Error
	}
}

func (s *knowledgeIndexSyncService) updateGenerationPointCount(generationID int64) {
	if generationID <= 0 {
		return
	}
	count := repositories.KnowledgeChunkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("index_generation_id", generationID))
	_ = repositories.KnowledgeIndexGenerationRepository.Updates(sqls.DB(), generationID, map[string]any{
		"point_count": count,
		"updated_at":  s.now(),
	})
}

func buildKnowledgeIndexTaskKey(tenantID int64, entryKey string, revisionID, generationID int64, action, scopeFingerprint string) string {
	raw := fmt.Sprintf("tenant:%d:entry:%s:revision:%d:generation:%d:action:%s:scope:%s", tenantID, strings.TrimSpace(entryKey), revisionID, generationID, strings.TrimSpace(action), strings.TrimSpace(scopeFingerprint))
	sum := sha256.Sum256([]byte(raw))
	return "knowledge:" + hex.EncodeToString(sum[:])
}

func normalizeKnowledgeTaskMaxRetries(value int) int {
	if value > 0 {
		return value
	}
	return config.CurrentOrDefault().RAG.Normalized().MaxRetries
}

func backoffForKnowledgeTask(retryCount int) time.Duration {
	if retryCount <= 1 {
		return time.Minute
	}
	delay := time.Minute * time.Duration(1<<(retryCount-1))
	if delay > 30*time.Minute {
		return 30 * time.Minute
	}
	return delay
}

func classifyKnowledgeTaskError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(value, "not found"):
		return "not_found"
	case strings.Contains(value, "published"):
		return "invalid_revision"
	case strings.Contains(value, "vectordb"):
		return "vectordb_error"
	case strings.Contains(value, "embedding"):
		return "embedding_error"
	default:
		return "sync_failed"
	}
}

func truncateErrorSummary(value string) string {
	if len(value) > 500 {
		return value[:500]
	}
	return value
}

type knowledgeEntryTaskSpec struct {
	SubjectType  string
	SubjectID    int64
	EntryKey     string
	RevisionID   int64
	ContentHash  string
	InputVersion string
}
