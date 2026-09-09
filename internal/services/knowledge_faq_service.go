package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm/clause"
)

var KnowledgeFAQService = newKnowledgeFAQService()

func newKnowledgeFAQService() *knowledgeFAQService {
	return &knowledgeFAQService{}
}

type knowledgeFAQService struct{}

func (s *knowledgeFAQService) Get(id int64) *models.KnowledgeFAQ {
	return repositories.KnowledgeFAQRepository.Get(sqls.DB(), id)
}

func (s *knowledgeFAQService) FindPageByCnd(cnd *sqls.Cnd) (list []models.KnowledgeFAQ, paging *sqls.Paging) {
	return repositories.KnowledgeFAQRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *knowledgeFAQService) FindPageByParams(queryParams *params.QueryParams) (list []models.KnowledgeFAQ, paging *sqls.Paging) {
	return repositories.KnowledgeFAQRepository.FindPageByParams(sqls.DB(), queryParams)
}

func (s *knowledgeFAQService) Count(cnd *sqls.Cnd) int64 {
	return repositories.KnowledgeFAQRepository.Count(sqls.DB(), cnd)
}

func (s *knowledgeFAQService) CreateKnowledgeFAQ(req request.CreateKnowledgeFAQRequest, operator *dto.AuthPrincipal) (*models.KnowledgeFAQ, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	kb, err := s.requireFAQKnowledgeBaseForOperator(req.KnowledgeBaseID, operator)
	if err != nil {
		return nil, err
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return nil, err
	}
	item, err := s.buildKnowledgeFAQModel(req)
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
	if err := repositories.KnowledgeFAQRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return s.Get(item.ID), nil
}

func (s *knowledgeFAQService) UpdateKnowledgeFAQ(req request.UpdateKnowledgeFAQRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.Get(req.ID)
	if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
		return errorsx.InvalidParamI18n("error.e0025")
	}
	if _, err := s.requireFAQKnowledgeBaseForOperator(req.KnowledgeBaseID, operator); err != nil {
		return err
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	item, err := s.buildKnowledgeFAQModel(req.CreateKnowledgeFAQRequest)
	if err != nil {
		return err
	}
	oldKnowledgeBaseID := current.KnowledgeBaseID
	previousPublishedRevisionID := current.PublishedRevisionID
	previousContentHash := knowledgeContentHash(strings.TrimSpace(current.Question) + "\n" + strings.TrimSpace(current.Answer))
	previousReviewStatus := current.ReviewStatus
	previousIndexStatus := current.IndexStatus
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := &models.KnowledgeFAQ{}
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(locked, req.ID).Error; err != nil || locked.TenantID != current.TenantID || locked.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0025")
		}
		oldKnowledgeBaseID = locked.KnowledgeBaseID
		previousPublishedRevisionID = locked.PublishedRevisionID
		previousContentHash = knowledgeContentHash(strings.TrimSpace(locked.Question) + "\n" + strings.TrimSpace(locked.Answer))
		previousReviewStatus = locked.ReviewStatus
		previousIndexStatus = locked.IndexStatus
		if err := repositories.KnowledgeFAQRepository.Updates(ctx.Tx, req.ID, map[string]any{
			"knowledge_base_id": item.KnowledgeBaseID,
			"directory_id":      item.DirectoryID,
			"question":          item.Question,
			"answer":            item.Answer,
			"similar_questions": item.SimilarQuestions,
			"review_status":     "draft",
			"status":            enums.StatusDisabled,
			"index_status":      enums.KnowledgeDocumentIndexStatusPending,
			"indexed_at":        nil,
			"index_error":       "",
			"remark":            item.Remark,
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
		if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(current.TenantID, oldKnowledgeBaseID, "faq", req.ID, previousPublishedRevisionID, previousContentHash, "delete", operator); err != nil {
			slog.Warn("enqueue edited dashboard faq cleanup failed", "faq_id", req.ID, "revision_id", previousPublishedRevisionID, "error", err)
		}
		return nil
	}
	if previousReviewStatus == "published" || previousIndexStatus == enums.KnowledgeDocumentIndexStatusIndexed {
		if err := rag.Index.RemoveFAQIndex(context.Background(), req.ID); err != nil {
			slog.Warn("remove legacy dashboard faq index failed", "faq_id", req.ID, "error", err)
		}
	}
	return nil
}

func (s *knowledgeFAQService) DeleteKnowledgeFAQ(id int64) error {
	current := s.Get(id)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0025")
	}
	if err := repositories.KnowledgeFAQRepository.Delete(sqls.DB(), id); err != nil {
		return err
	}
	return rag.Index.RemoveFAQIndex(context.Background(), id)
}

func (s *knowledgeFAQService) DeleteKnowledgeFAQForOperator(id int64, operator *dto.AuthPrincipal) error {
	current := s.Get(id)
	if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
		return errorsx.InvalidParamI18n("error.e0025")
	}
	return s.DeleteKnowledgeFAQ(id)
}

func (s *knowledgeFAQService) BatchMoveKnowledgeFAQs(req request.BatchMoveKnowledgeFAQRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0332")
	}
	kb, err := s.requireFAQKnowledgeBaseForOperator(req.KnowledgeBaseID, operator)
	if err != nil {
		return err
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	publishedIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		current := s.Get(id)
		if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
			return errorsx.InvalidParamI18n("error.e0025")
		}
		if current.KnowledgeBaseID != req.KnowledgeBaseID {
			return errorsx.InvalidParamI18n("error.e0138")
		}
		if current.ReviewStatus == "published" && current.Status == enums.StatusOk && current.PublishedRevisionID > 0 {
			publishedIDs = append(publishedIDs, id)
		}
	}
	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		for _, id := range ids {
			if err := repositories.KnowledgeFAQRepository.Updates(ctx.Tx, id, map[string]any{
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
		if _, err := KnowledgeIndexSyncService.EnqueueEntrySync(kb.TenantID, req.KnowledgeBaseID, "faq", id, "published", "upsert", operator); err != nil {
			slog.Warn("enqueue moved knowledge faq reindex failed", "faq_id", id, "error", err)
		}
	}
	return nil
}

func (s *knowledgeFAQService) BatchDeleteKnowledgeFAQs(req request.BatchDeleteKnowledgeFAQRequest) error {
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0330")
	}
	for _, id := range ids {
		if err := s.DeleteKnowledgeFAQ(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *knowledgeFAQService) BatchDeleteKnowledgeFAQsForOperator(req request.BatchDeleteKnowledgeFAQRequest, operator *dto.AuthPrincipal) error {
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0330")
	}
	for _, id := range ids {
		current := s.Get(id)
		if current == nil || requireKnowledgeTenantOwnership(current.TenantID, operator) != nil {
			return errorsx.InvalidParamI18n("error.e0025")
		}
	}
	return s.BatchDeleteKnowledgeFAQs(req)
}

func (s *knowledgeFAQService) buildKnowledgeFAQModel(req request.CreateKnowledgeFAQRequest) (*models.KnowledgeFAQ, error) {
	if req.KnowledgeBaseID <= 0 {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if req.Question == "" {
		return nil, errorsx.InvalidParamI18n("error.e0340")
	}
	if req.Answer == "" {
		return nil, errorsx.InvalidParamI18n("error.e0292")
	}
	similarQuestions, err := json.Marshal(normalizeSimilarQuestions(req.SimilarQuestions))
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.e0280")
	}
	return &models.KnowledgeFAQ{
		KnowledgeBaseID:  req.KnowledgeBaseID,
		DirectoryID:      req.DirectoryID,
		Question:         req.Question,
		Answer:           req.Answer,
		SimilarQuestions: string(similarQuestions),
		Remark:           req.Remark,
	}, nil
}

func (s *knowledgeFAQService) requireFAQKnowledgeBase(knowledgeBaseID int64) (*models.KnowledgeBase, error) {
	kb := KnowledgeBaseService.Get(knowledgeBaseID)
	if kb == nil {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType != "faq" {
		return nil, errorsx.InvalidParamI18n("error.e0199")
	}
	return kb, nil
}

func (s *knowledgeFAQService) requireFAQKnowledgeBaseForOperator(knowledgeBaseID int64, operator *dto.AuthPrincipal) (*models.KnowledgeBase, error) {
	kb := KnowledgeBaseService.GetForOperator(knowledgeBaseID, operator)
	if kb == nil {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType != "faq" {
		return nil, errorsx.InvalidParamI18n("error.e0199")
	}
	return kb, nil
}

func normalizeSimilarQuestions(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		value := item
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}
