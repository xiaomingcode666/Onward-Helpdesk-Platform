package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm/clause"
)

var KnowledgeDocumentService = newKnowledgeDocumentService()

func newKnowledgeDocumentService() *knowledgeDocumentService {
	return &knowledgeDocumentService{}
}

type knowledgeDocumentService struct {
}

func (s *knowledgeDocumentService) Get(id int64) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Get(sqls.DB(), id)
}

func (s *knowledgeDocumentService) Take(where ...interface{}) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Take(sqls.DB(), where...)
}

func (s *knowledgeDocumentService) Find(cnd *sqls.Cnd) []models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Find(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindOne(cnd *sqls.Cnd) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.FindOne(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindPageByParams(params *params.QueryParams) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageByParams(sqls.DB(), params)
}

func (s *knowledgeDocumentService) FindPageByCnd(cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindPageListByCnd(cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageListByCnd(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) Count(cnd *sqls.Cnd) int64 {
	return repositories.KnowledgeDocumentRepository.Count(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) Create(t *models.KnowledgeDocument) error {
	return repositories.KnowledgeDocumentRepository.Create(sqls.DB(), t)
}

func (s *knowledgeDocumentService) Update(t *models.KnowledgeDocument) error {
	return repositories.KnowledgeDocumentRepository.Update(sqls.DB(), t)
}

func (s *knowledgeDocumentService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), id, columns)
}

func (s *knowledgeDocumentService) UpdateColumn(id int64, name string, value interface{}) error {
	return repositories.KnowledgeDocumentRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *knowledgeDocumentService) Delete(id int64) {
	repositories.KnowledgeDocumentRepository.Delete(sqls.DB(), id)
}

func (s *knowledgeDocumentService) CreateKnowledgeDocument(req request.CreateKnowledgeDocumentRequest, operator *dto.AuthPrincipal) (*models.KnowledgeDocument, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	kb := KnowledgeBaseService.GetForOperator(req.KnowledgeBaseID, operator)
	if kb == nil {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return nil, err
	}
	item, err := s.buildKnowledgeDocumentModel(req)
	if err != nil {
		return nil, err
	}
	item.TenantID = kb.TenantID
	item.ReviewStatus = "draft"
	item.Status = enums.StatusDisabled
	item.IndexStatus = enums.KnowledgeDocumentIndexStatusPending
	item.IndexError = ""
	item.IndexedAt = nil
	item.AuditFields = utils.BuildAuditFields(operator)
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Create(item).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	item = s.Get(item.ID)
	return item, nil
}

func (s *knowledgeDocumentService) UpdateKnowledgeDocument(req request.UpdateKnowledgeDocumentRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.Get(req.ID)
	if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
		return errorsx.InvalidParamI18n("error.e0218")
	}
	kb := KnowledgeBaseService.GetForOperator(req.KnowledgeBaseID, operator)
	if kb == nil {
		return errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	item, err := s.buildKnowledgeDocumentModel(req.CreateKnowledgeDocumentRequest)
	if err != nil {
		return err
	}
	item.TenantID = kb.TenantID
	oldKnowledgeBaseID := current.KnowledgeBaseID
	previousPublishedRevisionID := current.PublishedRevisionID
	previousContentHash := current.ContentHash
	previousReviewStatus := current.ReviewStatus
	previousIndexStatus := current.IndexStatus
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := &models.KnowledgeDocument{}
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(locked, req.ID).Error; err != nil || locked.TenantID != current.TenantID || locked.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0218")
		}
		oldKnowledgeBaseID = locked.KnowledgeBaseID
		previousPublishedRevisionID = locked.PublishedRevisionID
		previousContentHash = locked.ContentHash
		previousReviewStatus = locked.ReviewStatus
		previousIndexStatus = locked.IndexStatus
		if err := repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, req.ID, map[string]any{
			"tenant_id":         item.TenantID,
			"knowledge_base_id": item.KnowledgeBaseID,
			"directory_id":      item.DirectoryID,
			"title":             item.Title,
			"content_type":      item.ContentType,
			"content_hash":      item.ContentHash,
			"content":           item.Content,
			"review_status":     "draft",
			"status":            enums.StatusDisabled,
			"index_status":      enums.KnowledgeDocumentIndexStatusPending,
			"indexed_at":        nil,
			"index_error":       "",
			"update_user_id":    operator.UserID,
			"update_user_name":  operator.Username,
			"updated_at":        time.Now(),
		}); err != nil {
			return err
		}
		if oldKnowledgeBaseID != item.KnowledgeBaseID {
			return EnterpriseKnowledgeService.deactivateEntryLinksDB(ctx.Tx, current.TenantID, oldKnowledgeBaseID, req.ID)
		}
		return EnterpriseKnowledgeService.syncEntryLinkPublishStatusTx(ctx.Tx, current.TenantID, oldKnowledgeBaseID, req.ID, "draft")
	}); err != nil {
		return err
	}
	if previousPublishedRevisionID > 0 {
		if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(current.TenantID, oldKnowledgeBaseID, "document", req.ID, previousPublishedRevisionID, previousContentHash, "delete", operator); err != nil {
			slog.Warn("enqueue edited dashboard knowledge cleanup failed", "document_id", req.ID, "revision_id", previousPublishedRevisionID, "error", err)
		}
	} else if previousReviewStatus == "published" || previousIndexStatus == enums.KnowledgeDocumentIndexStatusIndexed {
		if err := rag.Index.RemoveDocumentIndex(context.Background(), req.ID); err != nil {
			slog.Warn("remove legacy dashboard knowledge index failed", "document_id", req.ID, "error", err)
		}
	}
	return nil
}

func (s *knowledgeDocumentService) DeleteKnowledgeDocument(id int64) error {
	if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), id, map[string]any{
		"status":     enums.StatusDeleted,
		"updated_at": time.Now(),
	}); err != nil {
		return err
	}
	return rag.Index.RemoveDocumentIndex(context.Background(), id)
}

func (s *knowledgeDocumentService) DeleteKnowledgeDocumentForOperator(id int64, operator *dto.AuthPrincipal) error {
	current := s.Get(id)
	if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
		return errorsx.InvalidParamI18n("error.e0218")
	}
	return s.DeleteKnowledgeDocument(id)
}

func (s *knowledgeDocumentService) BatchMoveKnowledgeDocuments(req request.BatchMoveKnowledgeDocumentRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0333")
	}
	kb := KnowledgeBaseService.GetForOperator(req.KnowledgeBaseID, operator)
	if kb == nil {
		return errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	publishedIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		current := s.Get(id)
		if current == nil || current.Status == enums.StatusDeleted || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
			return errorsx.InvalidParamI18n("error.e0218")
		}
		if current.KnowledgeBaseID != req.KnowledgeBaseID {
			return errorsx.InvalidParamI18n("error.e0139")
		}
		if current.ReviewStatus == "published" && current.Status == enums.StatusOk && current.PublishedRevisionID > 0 {
			publishedIDs = append(publishedIDs, id)
		}
	}
	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		for _, id := range ids {
			if err := repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, id, map[string]any{
				"directory_id":     req.DirectoryID,
				"index_status":     enums.KnowledgeDocumentIndexStatusPending,
				"indexed_at":       nil,
				"index_error":      "",
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, id := range publishedIDs {
		if _, err := KnowledgeIndexSyncService.EnqueueEntrySync(kb.TenantID, req.KnowledgeBaseID, "document", id, "published", "upsert", operator); err != nil {
			slog.Warn("enqueue moved knowledge document reindex failed", "document_id", id, "error", err)
		}
	}
	return nil
}

func (s *knowledgeDocumentService) BatchDeleteKnowledgeDocuments(req request.BatchDeleteKnowledgeDocumentRequest) error {
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0331")
	}
	for _, id := range ids {
		if err := s.DeleteKnowledgeDocument(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *knowledgeDocumentService) BatchDeleteKnowledgeDocumentsForOperator(req request.BatchDeleteKnowledgeDocumentRequest, operator *dto.AuthPrincipal) error {
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0331")
	}
	for _, id := range ids {
		current := s.Get(id)
		if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
			return errorsx.InvalidParamI18n("error.e0218")
		}
	}
	return s.BatchDeleteKnowledgeDocuments(req)
}

func (s *knowledgeDocumentService) buildKnowledgeDocumentModel(req request.CreateKnowledgeDocumentRequest) (*models.KnowledgeDocument, error) {
	if strs.IsBlank(string(req.ContentType)) {
		req.ContentType = enums.KnowledgeDocumentContentTypeHTML
	}
	if req.ContentType != enums.KnowledgeDocumentContentTypeHTML && req.ContentType != enums.KnowledgeDocumentContentTypeMarkdown {
		return nil, errorsx.InvalidParamI18n("error.e0129")
	}

	plainText := rag.ExtractPlainText(req.Content, req.ContentType)
	item := &models.KnowledgeDocument{
		KnowledgeBaseID: req.KnowledgeBaseID,
		DirectoryID:     req.DirectoryID,
		Title:           req.Title,
		ContentType:     req.ContentType,
		Content:         req.Content,
	}
	if plainText != "" {
		hash := sha256.Sum256([]byte(plainText))
		item.ContentHash = hex.EncodeToString(hash[:])
	}
	return item, nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
