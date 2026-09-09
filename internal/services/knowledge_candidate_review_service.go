package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KnowledgeCandidateReviewService 知识候选处理（§7.4）。
// 批准候选时在事务内创建/更新知识条目与产品知识链接；
// 拒绝与合并均保留审计痕迹。
var KnowledgeCandidateReviewService = newKnowledgeCandidateReviewService()

func newKnowledgeCandidateReviewService() *knowledgeCandidateReviewService {
	return &knowledgeCandidateReviewService{}
}

type knowledgeCandidateReviewService struct{}

// ListCandidates 列出知识候选（可按产品与审核状态过滤）。
func (s *knowledgeCandidateReviewService) ListCandidates(tenantID, productID int64, reviewStatus string) ([]dto.EnterpriseKnowledgeCandidateDTO, error) {
	page, err := s.ListCandidatePage(tenantID, productID, reviewStatus, 1, 100)
	if err != nil {
		return nil, err
	}
	items := append([]dto.EnterpriseKnowledgeCandidateDTO(nil), page.Items...)
	for pageNumber := 2; pageNumber <= page.TotalPages; pageNumber++ {
		next, nextErr := s.ListCandidatePage(tenantID, productID, reviewStatus, pageNumber, 100)
		if nextErr != nil {
			return nil, nextErr
		}
		items = append(items, next.Items...)
	}
	return items, nil
}

func (s *knowledgeCandidateReviewService) ListCandidatePage(tenantID, productID int64, reviewStatus string, page, pageSize int) (*dto.EnterpriseListResponse[dto.EnterpriseKnowledgeCandidateDTO], error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, err
	}
	if err := KnowledgeCandidateScoringService.RescoreStaleCandidatesDB(sqls.DB(), tenantID, productID); err != nil {
		return nil, err
	}
	page, pageSize = normalizeEnterprisePage(page, pageSize)
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).Where("status <> ?", enums.StatusDeleted).
		Desc("candidate_score").Desc("quality_score").Desc("recurrence_count").Desc("id").Page(page, pageSize)
	if productID > 0 {
		cnd.Eq("product_id", productID)
	}
	statuses := splitKnowledgeCandidateReviewStatuses(reviewStatus)
	if len(statuses) > 0 {
		if len(statuses) == 1 {
			cnd.Eq("review_status", statuses[0])
		} else if len(statuses) > 1 {
			cnd.In("review_status", statuses)
		}
	}
	list, paging := repositories.KnowledgeCandidateRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.EnterpriseKnowledgeCandidateDTO, 0, len(list))
	statusFilter := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		statusFilter[status] = struct{}{}
	}
	for i := range list {
		if list[i].ScoreVersion != enums.KnowledgeCandidateScoreVersion {
			if err := KnowledgeCandidateScoringService.RescoreCandidateDB(sqls.DB(), &list[i]); err != nil {
				return nil, err
			}
			if current := repositories.KnowledgeCandidateRepository.Get(sqls.DB(), list[i].ID); current != nil {
				list[i] = *current
			}
		}
		if len(statusFilter) > 0 {
			if _, ok := statusFilter[list[i].ReviewStatus]; !ok {
				continue
			}
		}
		result = append(result, s.buildCandidateDTO(&list[i]))
	}
	totalPages := int((paging.Total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages <= 0 {
		totalPages = 1
	}
	return &dto.EnterpriseListResponse[dto.EnterpriseKnowledgeCandidateDTO]{
		Items: result, Total: paging.Total, Page: page, PageSize: pageSize,
		TotalPages: totalPages, HasMore: page < totalPages,
	}, nil
}

// GetCandidate 获取单个候选（租户隔离）。
func (s *knowledgeCandidateReviewService) GetCandidate(tenantID, candidateID int64) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	item, err := s.requireCandidate(tenantID, candidateID)
	if err != nil {
		return nil, err
	}
	if item.ScoreVersion != enums.KnowledgeCandidateScoreVersion {
		if err := KnowledgeCandidateScoringService.RescoreCandidateDB(sqls.DB(), item); err != nil {
			return nil, err
		}
		item = repositories.KnowledgeCandidateRepository.Get(sqls.DB(), item.ID)
	}
	dtoItem := s.buildCandidateDTO(item)
	return &dtoItem, nil
}

// EnrichCandidate 补充未终结候选的结构化内容并重新执行评分、分流与去重。
func (s *knowledgeCandidateReviewService) EnrichCandidate(tenantID, candidateID int64, req dto.EnterpriseKnowledgeCandidateEnrichRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	updates := make(map[string]any)
	for key, value := range map[string]string{
		"title":              req.Title,
		"suggestion":         req.Suggestion,
		"root_cause_summary": req.RootCauseSummary,
		"solution_summary":   req.SolutionSummary,
	} {
		if value = strings.TrimSpace(value); value != "" {
			updates[key] = value
		}
	}
	if len(updates) == 0 {
		return nil, errorsx.InvalidParam("provide candidate content to enrich")
	}
	updates["update_user_id"] = operator.UserID
	updates["update_user_name"] = operator.Username
	updates["updated_at"] = time.Now()
	var item *models.KnowledgeCandidate
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		current := &models.KnowledgeCandidate{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(current, candidateID).Error; err != nil || current.TenantID != tenantID || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("knowledge candidate not found")
		}
		switch current.ReviewStatus {
		case string(enums.KnowledgeCandidateReviewStatusApproved):
			return errorsx.InvalidParam("complete a new verified repair round before reassessing this knowledge candidate")
		case string(enums.KnowledgeCandidateReviewStatusRejected),
			string(enums.KnowledgeCandidateReviewStatusMerged):
			return errorsx.InvalidParam("knowledge candidate has already been finalized")
		}
		if err := s.requireCandidateSourceClosed(current); err != nil {
			return err
		}
		oldSimilarityHash := current.SimilarityHash
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, current.ID, updates); err != nil {
			return err
		}
		item = repositories.KnowledgeCandidateRepository.Get(tx, current.ID)
		if err := KnowledgeCandidateScoringService.RescoreCandidateDB(tx, item); err != nil {
			return err
		}
		if oldSimilarityHash != "" && oldSimilarityHash != item.SimilarityHash {
			if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, oldSimilarityHash); err != nil {
				return err
			}
		}
		item = repositories.KnowledgeCandidateRepository.Get(tx, current.ID)
		return nil
	}); err != nil {
		return nil, err
	}
	dtoItem := s.buildCandidateDTO(item)
	return &dtoItem, nil
}

// DetachDuplicateCandidate marks a detected duplicate as an independent case
// and reruns scoring without allowing automatic regrouping.
func (s *knowledgeCandidateReviewService) DetachDuplicateCandidate(tenantID, candidateID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var item *models.KnowledgeCandidate
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		current := &models.KnowledgeCandidate{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(current, candidateID).Error; err != nil || current.TenantID != tenantID || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("knowledge candidate not found")
		}
		if current.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusDuplicate) {
			return errorsx.InvalidParam("knowledge candidate is not grouped as a duplicate")
		}
		oldSimilarityHash := current.SimilarityHash
		current.DeduplicationOverride = true
		current.MergedToCandidateID = 0
		if err := KnowledgeCandidateScoringService.RescoreCandidateDB(tx, current); err != nil {
			return err
		}
		if oldSimilarityHash != "" && oldSimilarityHash != current.SimilarityHash {
			if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, oldSimilarityHash); err != nil {
				return err
			}
		}
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, current.ID, map[string]any{
			"deduplication_override": true,
			"update_user_id":         operator.UserID,
			"update_user_name":       operator.Username,
			"updated_at":             time.Now(),
		}); err != nil {
			return err
		}
		item = repositories.KnowledgeCandidateRepository.Get(tx, current.ID)
		return nil
	}); err != nil {
		return nil, err
	}
	result := s.buildCandidateDTO(item)
	return &result, nil
}

// ApproveCandidate 批准候选：创建知识条目 + 产品知识链接，并回写候选审核结果。
func (s *knowledgeCandidateReviewService) ApproveCandidate(tenantID, candidateID int64, req dto.EnterpriseKnowledgeCandidateApproveRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireCandidate(tenantID, candidateID)
	if err != nil {
		return nil, err
	}
	if item.RequiresReassessment {
		return nil, errorsx.InvalidParam("complete a new verified repair round before reassessing this knowledge candidate")
	}
	if item.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) {
		if err := s.requireReviewableCandidate(item); err != nil {
			return nil, err
		}
	}
	if err := s.requireCandidateSourceClosed(item); err != nil {
		return nil, err
	}

	var updated *models.KnowledgeCandidate
	var documentID int64
	var knowledgeBaseID int64
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		current := &models.KnowledgeCandidate{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(current, candidateID).Error; err != nil || current.TenantID != tenantID || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("knowledge candidate not found")
		}
		if current.RequiresReassessment {
			return errorsx.InvalidParam("complete a new verified repair round before reassessing this knowledge candidate")
		}
		if current.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) &&
			!enums.IsKnowledgeCandidateReviewReady(current.ReviewStatus, current.QualityScore, current.ValueScore, current.MergedToCandidateID) {
			return errorsx.InvalidParam("knowledge candidate does not meet the quality and value thresholds for review")
		}
		if err := s.requireCandidateSourceClosed(current); err != nil {
			return err
		}
		resolvedKnowledgeBaseID, resolveErr := s.resolveCandidateKnowledgeBaseDB(tx, current)
		if resolveErr != nil {
			return resolveErr
		}
		knowledgeBaseID = resolvedKnowledgeBaseID
		title, content := buildKnowledgeCandidateDocumentContent(current, req)
		if title == "" || content == "" {
			return errorsx.InvalidParam("candidate has no title or content to approve")
		}
		if !knowledgeCandidateApprovalContentMatches(current, title, content) {
			return errorsx.InvalidParam("knowledge entry content does not match the verified candidate evidence")
		}
		document, createErr := s.ensureCandidateKnowledgeDocumentTx(tx, current, knowledgeBaseID, title, content, req, operator)
		if createErr != nil {
			return createErr
		}
		documentID = document.ID
		if req.Publish {
			if err := s.publishCandidateKnowledgeDocumentTx(tx, document, operator); err != nil {
				return err
			}
		}
		now := time.Now()
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, current.ID, map[string]any{
			"review_status":         enums.KnowledgeCandidateReviewStatusApproved,
			"review_remark":         "",
			"requires_reassessment": false,
			"knowledge_base_id":     knowledgeBaseID,
			"knowledge_entry_id":    encodeEnterpriseKnowledgeID("document", document.ID),
			"reviewed_at":           now,
			"reviewer_id":           operator.UserID,
			"update_user_id":        operator.UserID,
			"update_user_name":      operator.Username,
			"updated_at":            now,
		}); err != nil {
			return err
		}
		updated = repositories.KnowledgeCandidateRepository.Get(tx, current.ID)
		return nil
	}); err != nil {
		return nil, err
	}
	if req.Publish {
		EnterpriseKnowledgeService.enqueueEntryIndexSync(tenantID, knowledgeBaseID, "document", documentID, "published", "published", operator)
	}
	if updated != nil && updated.TicketID > 0 && sqls.DB().Migrator().HasTable(&models.TicketServiceOutcomeFact{}) {
		if _, rebuildErr := ServiceOutcomeFactService.RebuildTicket(context.Background(), tenantID, updated.TicketID); rebuildErr != nil {
			slog.Warn("rebuild service outcome after knowledge approval failed", "tenantId", tenantID, "ticketId", updated.TicketID, "error", rebuildErr)
		}
	}
	dtoItem := s.buildCandidateDTO(updated)
	return &dtoItem, nil
}

func knowledgeCandidateApprovalContentMatches(candidate *models.KnowledgeCandidate, title, content string) bool {
	if candidate == nil {
		return false
	}
	candidateText := strings.TrimSpace(candidate.Title + " " + candidate.Suggestion + " " + candidate.RootCauseSummary + " " + candidate.SolutionSummary)
	documentText := strings.TrimSpace(title + " " + content)
	return knowledgeCandidateTextSimilarity(candidateText, documentText) >= 0.35
}

func buildKnowledgeCandidateDocumentContent(candidate *models.KnowledgeCandidate, req dto.EnterpriseKnowledgeCandidateApproveRequest) (string, string) {
	title := strings.TrimSpace(req.Title)
	if title == "" && candidate != nil {
		title = strings.TrimSpace(candidate.Title)
	}
	content := strings.TrimSpace(req.Content)
	if content != "" || candidate == nil {
		return title, content
	}
	parts := make([]string, 0, 3)
	if strings.TrimSpace(candidate.Suggestion) != "" {
		parts = append(parts, strings.TrimSpace(candidate.Suggestion))
	}
	if strings.TrimSpace(candidate.RootCauseSummary) != "" {
		parts = append(parts, "根因："+strings.TrimSpace(candidate.RootCauseSummary))
	}
	if strings.TrimSpace(candidate.SolutionSummary) != "" {
		parts = append(parts, formatKnowledgeCandidateSolutionContent(candidate.SolutionSummary))
	}
	return title, strings.Join(parts, "\n\n")
}

func (s *knowledgeCandidateReviewService) ensureCandidateKnowledgeDocumentTx(tx *gorm.DB, candidate *models.KnowledgeCandidate, knowledgeBaseID int64, title, content string, req dto.EnterpriseKnowledgeCandidateApproveRequest, operator *dto.AuthPrincipal) (*models.KnowledgeDocument, error) {
	var document models.KnowledgeDocument
	find := tx.Where("tenant_id = ? AND source_type IN ? AND source_reference_id = ? AND status <> ?", candidate.TenantID, []string{"knowledge_candidate", "knowledge_entry"}, candidate.ID, enums.StatusDeleted).
		Order("id ASC").First(&document)
	if find.Error == nil {
		if candidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) || candidate.RequiresReassessment {
			if err := repositories.KnowledgeDocumentRepository.Updates(tx, document.ID, map[string]any{
				"title":            strings.TrimSpace(title),
				"content":          strings.TrimSpace(content),
				"content_hash":     knowledgeContentHash(content),
				"review_status":    "draft",
				"status":           enums.StatusDisabled,
				"index_status":     enums.KnowledgeDocumentIndexStatusPending,
				"indexed_at":       nil,
				"index_error":      "",
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       time.Now(),
			}); err != nil {
				return nil, err
			}
			document.Title = strings.TrimSpace(title)
			document.Content = strings.TrimSpace(content)
			document.ContentHash = knowledgeContentHash(content)
			document.ReviewStatus = "draft"
			document.Status = enums.StatusDisabled
		}
		return &document, s.ensureCandidateProductLinkTx(tx, candidate, &document, req, operator)
	}
	if find.Error != gorm.ErrRecordNotFound {
		return nil, find.Error
	}
	now := time.Now()
	document = models.KnowledgeDocument{
		TenantID:          candidate.TenantID,
		KnowledgeBaseID:   knowledgeBaseID,
		Title:             strings.TrimSpace(title),
		ContentType:       enums.KnowledgeDocumentContentTypeMarkdown,
		Content:           strings.TrimSpace(content),
		SourceType:        "knowledge_candidate",
		SourceReferenceID: candidate.ID,
		ContentHash:       knowledgeContentHash(content),
		ReviewStatus:      "draft",
		Language:          normalizeKnowledgeLanguage(req.Language),
		TagsJSON:          "[]",
		FaultCodesJSON:    "[]",
		Status:            enums.StatusDisabled,
		IndexStatus:       enums.KnowledgeDocumentIndexStatusPending,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	document.CreatedAt = now
	document.UpdatedAt = now
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&document)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := tx.Where("tenant_id = ? AND source_type IN ? AND source_reference_id = ? AND status <> ?", candidate.TenantID, []string{"knowledge_candidate", "knowledge_entry"}, candidate.ID, enums.StatusDeleted).
			Order("id ASC").First(&document).Error; err != nil {
			return nil, err
		}
	}
	if err := s.ensureCandidateProductLinkTx(tx, candidate, &document, req, operator); err != nil {
		return nil, err
	}
	return &document, nil
}

func (s *knowledgeCandidateReviewService) ensureCandidateProductLinkTx(tx *gorm.DB, candidate *models.KnowledgeCandidate, document *models.KnowledgeDocument, req dto.EnterpriseKnowledgeCandidateApproveRequest, operator *dto.AuthPrincipal) error {
	if candidate.ProductID <= 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&models.ProductKnowledgeLink{}).
		Where("tenant_id = ? AND product_id = ? AND product_model_id = ? AND knowledge_base_id = ? AND knowledge_entry_id = ? AND status <> ?",
			candidate.TenantID, candidate.ProductID, candidate.ProductModelID, document.KnowledgeBaseID, document.ID, enums.StatusDeleted).
		Count(&count).Error; err != nil || count > 0 {
		return err
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ProductKnowledgeLink{
		TenantID:         candidate.TenantID,
		ProductID:        candidate.ProductID,
		ProductModelID:   candidate.ProductModelID,
		KnowledgeBaseID:  document.KnowledgeBaseID,
		KnowledgeEntryID: document.ID,
		LinkType:         "document",
		Language:         document.Language,
		Visibility:       normalizeVisibility(req.Visibility),
		PublishStatus:    document.ReviewStatus,
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	}).Error
}

func (s *knowledgeCandidateReviewService) publishCandidateKnowledgeDocumentTx(tx *gorm.DB, document *models.KnowledgeDocument, operator *dto.AuthPrincipal) error {
	if document.ReviewStatus == "published" {
		return nil
	}
	now := time.Now()
	revision, err := EnterpriseKnowledgeService.publishDocumentRevision(tx, *document, now, operator)
	if err != nil {
		return err
	}
	if err := repositories.KnowledgeDocumentRepository.Updates(tx, document.ID, map[string]any{
		"review_status":         "published",
		"status":                enums.StatusOk,
		"index_status":          enums.KnowledgeDocumentIndexStatusPending,
		"current_revision_id":   revision.ID,
		"published_revision_id": revision.ID,
		"indexed_at":            nil,
		"index_error":           "",
		"update_user_id":        operator.UserID,
		"update_user_name":      operator.Username,
		"updated_at":            now,
	}); err != nil {
		return err
	}
	document.ReviewStatus = "published"
	document.Status = enums.StatusOk
	document.PublishedRevisionID = revision.ID
	return EnterpriseKnowledgeService.syncEntryLinkPublishStatusTx(tx, document.TenantID, document.KnowledgeBaseID, document.ID, "published")
}

func formatKnowledgeCandidateSolutionContent(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	if strings.HasPrefix(summary, "处理方案：") || strings.HasPrefix(summary, "维修结论：") {
		return summary
	}
	return "处理方案：" + summary
}

func (s *knowledgeCandidateReviewService) resolveCandidateKnowledgeBaseDB(db *gorm.DB, item *models.KnowledgeCandidate) (int64, error) {
	if item.KnowledgeBaseID > 0 {
		kb := repositories.KnowledgeBaseRepository.Get(db, item.KnowledgeBaseID)
		if kb != nil && kb.TenantID == item.TenantID && kb.Status == enums.StatusOk {
			return kb.ID, nil
		}
	}
	if item.ProductID <= 0 {
		return 0, errorsx.InvalidParam("knowledge candidate is not associated with a product knowledge base")
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, item.ProductID)
	if profile == nil || profile.TenantID != item.TenantID || profile.Status == enums.StatusDeleted || profile.DefaultKnowledgeBaseID <= 0 {
		return 0, errorsx.InvalidParam("create the product knowledge base before approving this entry")
	}
	kb := repositories.KnowledgeBaseRepository.Get(db, profile.DefaultKnowledgeBaseID)
	if kb == nil || kb.TenantID != item.TenantID || kb.Status != enums.StatusOk {
		return 0, errorsx.InvalidParam("create the product knowledge base before approving this entry")
	}
	return kb.ID, nil
}

// RejectCandidate 拒绝候选并记录原因。
func (s *knowledgeCandidateReviewService) RejectCandidate(tenantID, candidateID int64, remark string, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var updated *models.KnowledgeCandidate
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		item := &models.KnowledgeCandidate{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(item, candidateID).Error; err != nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("knowledge candidate not found")
		}
		if item.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusRejected) {
			updated = item
			return nil
		}
		if err := s.requireRejectableCandidate(item); err != nil {
			return err
		}
		now := time.Now()
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, item.ID, map[string]any{
			"review_status":    enums.KnowledgeCandidateReviewStatusRejected,
			"review_remark":    strings.TrimSpace(remark),
			"reviewed_at":      now,
			"reviewer_id":      operator.UserID,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		}); err != nil {
			return err
		}
		if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, item.SimilarityHash); err != nil {
			return err
		}
		if err := s.writeCandidateRejectionProgressTx(tx, item, strings.TrimSpace(remark), now, operator); err != nil {
			return err
		}
		updated = repositories.KnowledgeCandidateRepository.Get(tx, item.ID)
		return nil
	}); err != nil {
		return nil, err
	}
	dtoItem := s.buildCandidateDTO(updated)
	return &dtoItem, nil
}

func (s *knowledgeCandidateReviewService) writeCandidateRejectionProgressTx(tx *gorm.DB, item *models.KnowledgeCandidate, remark string, now time.Time, operator *dto.AuthPrincipal) error {
	if tx == nil || item == nil || item.TicketID <= 0 {
		return nil
	}
	ticket := repositories.TicketRepository.Get(tx, item.TicketID)
	if ticket == nil || ticket.TenantID != item.TenantID || ticket.Status == enums.TicketStatusCancelled {
		return nil
	}
	content := "知识候选审核已驳回"
	if remark != "" {
		content += "：" + remark
	}
	metadata, _ := json.Marshal(map[string]any{
		"source":       "knowledge_candidate_review",
		"candidateId":  item.ID,
		"reviewStatus": enums.KnowledgeCandidateReviewStatusRejected,
		"reviewRemark": remark,
	})
	authorID := int64(0)
	if operator != nil {
		authorID = operator.UserID
	}
	return repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
		TenantID:          item.TenantID,
		TicketID:          item.TicketID,
		EventType:         enums.TicketProgressEventProgress,
		Content:           content,
		VisibleToCustomer: false,
		MetadataJSON:      string(metadata),
		AuthorID:          authorID,
		CreatedAt:         now,
	})
}

// MergeCandidate 将候选合并到已有知识条目：校验条目归属后回写引用，
// 并为候选的产品补建产品知识链接。
func (s *knowledgeCandidateReviewService) MergeCandidate(tenantID, candidateID int64, encodedEntryID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeCandidateDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	entryType, actualID, err := decodeEnterpriseKnowledgeID(encodedEntryID)
	if err != nil {
		return nil, err
	}
	var updated *models.KnowledgeCandidate
	var knowledgeBaseID int64
	shouldReindexEntry := false
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		item := &models.KnowledgeCandidate{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(item, candidateID).Error; err != nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("knowledge candidate not found")
		}
		if item.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusMerged) && item.KnowledgeEntryID == encodedEntryID {
			updated = item
			return nil
		}
		if item.ScoreVersion != enums.KnowledgeCandidateScoreVersion {
			if err := KnowledgeCandidateScoringService.RescoreCandidateDB(tx, item); err != nil {
				return err
			}
			item = repositories.KnowledgeCandidateRepository.Get(tx, item.ID)
			if item == nil {
				return errorsx.InvalidParam("knowledge candidate not found")
			}
		}
		if item.RequiresReassessment {
			return errorsx.InvalidParam("complete a new verified repair round before reassessing this knowledge candidate")
		}
		if !enums.IsKnowledgeCandidateReviewReady(item.ReviewStatus, item.QualityScore, item.ValueScore, item.MergedToCandidateID) {
			return errorsx.InvalidParam("knowledge candidate does not meet the quality and value thresholds for review")
		}
		if err := s.requireCandidateSourceClosed(item); err != nil {
			return err
		}
		resolvedKnowledgeBaseID, validateErr := validateCandidateMergeEntryTx(tx, tenantID, entryType, actualID, item)
		if validateErr != nil {
			return validateErr
		}
		knowledgeBaseID = resolvedKnowledgeBaseID
		scopeChanged, linkErr := ensureCandidateMergedProductLinkTx(tx, item, knowledgeBaseID, entryType, actualID, operator)
		if linkErr != nil {
			return linkErr
		}
		shouldReindexEntry = shouldReindexEntry || scopeChanged
		now := time.Now()
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, item.ID, map[string]any{
			"review_status":      enums.KnowledgeCandidateReviewStatusMerged,
			"review_remark":      "",
			"knowledge_base_id":  knowledgeBaseID,
			"knowledge_entry_id": encodedEntryID,
			"reviewed_at":        now,
			"reviewer_id":        operator.UserID,
			"update_user_id":     operator.UserID,
			"update_user_name":   operator.Username,
			"updated_at":         now,
		}); err != nil {
			return err
		}
		updated = repositories.KnowledgeCandidateRepository.Get(tx, item.ID)
		return nil
	}); err != nil {
		return nil, err
	}
	if shouldReindexEntry {
		EnterpriseKnowledgeService.enqueueEntryIndexSync(tenantID, knowledgeBaseID, entryType, actualID, "published", "merge_scope_change", operator)
	}
	dtoItem := s.buildCandidateDTO(updated)
	return &dtoItem, nil
}

func validateCandidateMergeEntryTx(tx *gorm.DB, tenantID int64, entryType string, actualID int64, candidate *models.KnowledgeCandidate) (int64, error) {
	faultCode := ""
	if candidate != nil {
		if ticket := loadCandidateTicket(tx, candidate.TenantID, candidate.TicketID); ticket != nil {
			faultCode = ticket.FaultCode
		}
	}
	candidateText := ""
	if candidate != nil {
		candidateText = strings.TrimSpace(candidate.Title + " " + candidate.Suggestion + " " + candidate.RootCauseSummary + " " + candidate.SolutionSummary)
	}
	switch entryType {
	case "document":
		doc := repositories.KnowledgeDocumentRepository.Get(tx, actualID)
		if doc == nil || doc.TenantID != tenantID || doc.Status != enums.StatusOk || doc.ReviewStatus != "published" || doc.PublishedRevisionID <= 0 {
			return 0, errorsx.InvalidParam("published knowledge entry not found")
		}
		knowledgeBase := repositories.KnowledgeBaseRepository.Get(tx, doc.KnowledgeBaseID)
		if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk || knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
			return 0, errorsx.InvalidParam("active document knowledge base not found")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(tx, doc.PublishedRevisionID, tenantID, doc.KnowledgeBaseID, "document", doc.ID) == nil {
			return 0, errorsx.InvalidParam("published knowledge revision is missing or does not belong to this entry")
		}
		if !knowledgeCandidateEntryEquivalent(faultCode, candidateText, doc.Title+" "+doc.Content, doc.FaultCodesJSON) {
			return 0, errorsx.InvalidParam("published knowledge entry does not match the candidate evidence")
		}
		return doc.KnowledgeBaseID, nil
	case "faq":
		faq := repositories.KnowledgeFAQRepository.Get(tx, actualID)
		if faq == nil || faq.TenantID != tenantID || faq.Status != enums.StatusOk || faq.ReviewStatus != "published" || faq.PublishedRevisionID <= 0 {
			return 0, errorsx.InvalidParam("published knowledge entry not found")
		}
		knowledgeBase := repositories.KnowledgeBaseRepository.Get(tx, faq.KnowledgeBaseID)
		if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk || knowledgeBase.KnowledgeType != string(enums.KnowledgeBaseTypeFAQ) {
			return 0, errorsx.InvalidParam("active FAQ knowledge base not found")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(tx, faq.PublishedRevisionID, tenantID, faq.KnowledgeBaseID, "faq", faq.ID) == nil {
			return 0, errorsx.InvalidParam("published knowledge revision is missing or does not belong to this entry")
		}
		if !knowledgeCandidateEntryEquivalent(faultCode, candidateText, faq.Question+" "+faq.Answer, faq.FaultCodesJSON) {
			return 0, errorsx.InvalidParam("published knowledge entry does not match the candidate evidence")
		}
		return faq.KnowledgeBaseID, nil
	default:
		return 0, errorsx.InvalidParam("unsupported knowledge entry type")
	}
}

func ensureCandidateMergedProductLinkTx(tx *gorm.DB, candidate *models.KnowledgeCandidate, knowledgeBaseID int64, entryType string, entryID int64, operator *dto.AuthPrincipal) (bool, error) {
	if candidate == nil || candidate.ProductID <= 0 {
		return false, nil
	}
	product := repositories.ProductRepository.GetByTenant(tx, candidate.ProductID, candidate.TenantID)
	if product == nil || product.Status != enums.StatusOk {
		return false, errorsx.InvalidParam("candidate product is not active")
	}
	if candidate.ProductModelID > 0 {
		model := repositories.ProductModelRepository.Get(tx, candidate.ProductModelID)
		if model == nil || model.TenantID != candidate.TenantID || model.ProductID != candidate.ProductID || model.Status != enums.StatusOk {
			return false, errorsx.InvalidParam("candidate product model is not active")
		}
	}
	var links []models.ProductKnowledgeLink
	if err := tx.Where(
		"tenant_id = ? AND product_id = ? AND product_model_id = ? AND knowledge_base_id = ? AND knowledge_entry_id = ? AND status <> ?",
		candidate.TenantID, candidate.ProductID, candidate.ProductModelID, knowledgeBaseID, entryID, enums.StatusDeleted,
	).Order("CASE WHEN publish_status = 'published' THEN 0 ELSE 1 END ASC, id ASC").Find(&links).Error; err != nil {
		return false, err
	}
	for i := range links {
		if links[i].PublishStatus == "published" && links[i].Status == enums.StatusOk {
			return false, nil
		}
	}
	link := &models.ProductKnowledgeLink{
		TenantID: candidate.TenantID, ProductID: candidate.ProductID, ProductModelID: candidate.ProductModelID,
		KnowledgeBaseID: knowledgeBaseID, KnowledgeEntryID: entryID, LinkType: entryType,
		Visibility: "internal", PublishStatus: "published", Status: enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if len(links) > 0 {
		link = &links[0]
		if err := repositories.ProductKnowledgeLinkRepository.Updates(tx, link.ID, map[string]any{
			"publish_status":   "published",
			"status":           enums.StatusOk,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return false, err
		}
		link.PublishStatus = "published"
		link.Status = enums.StatusOk
	} else if err := tx.Create(link).Error; err != nil {
		return false, err
	}
	if err := ProductKnowledgeLinkService.markLinkEntryIndexPendingDB(tx, link); err != nil {
		return false, err
	}
	return true, nil
}

func (s *knowledgeCandidateReviewService) requireCandidate(tenantID, candidateID int64) (*models.KnowledgeCandidate, error) {
	item := repositories.KnowledgeCandidateRepository.Get(sqls.DB(), candidateID)
	if item == nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("knowledge candidate not found")
	}
	return item, nil
}

func (s *knowledgeCandidateReviewService) requireReviewableCandidate(item *models.KnowledgeCandidate) error {
	if item.ScoreVersion != enums.KnowledgeCandidateScoreVersion {
		if err := KnowledgeCandidateScoringService.RescoreCandidateDB(sqls.DB(), item); err != nil {
			return err
		}
		if current := repositories.KnowledgeCandidateRepository.Get(sqls.DB(), item.ID); current != nil {
			*item = *current
		}
	}
	reviewStatus := strings.TrimSpace(item.ReviewStatus)
	if reviewStatus == "" {
		reviewStatus = "pending"
	}
	if !enums.IsKnowledgeCandidateReviewReady(reviewStatus, item.QualityScore, item.ValueScore, item.MergedToCandidateID) {
		return errorsx.InvalidParam("knowledge candidate does not meet the quality and value thresholds for review")
	}
	return s.requireCandidateSourceClosed(item)
}

func (s *knowledgeCandidateReviewService) requireRejectableCandidate(item *models.KnowledgeCandidate) error {
	switch strings.TrimSpace(item.ReviewStatus) {
	case string(enums.KnowledgeCandidateReviewStatusApproved), string(enums.KnowledgeCandidateReviewStatusMerged):
		return errorsx.InvalidParam("knowledge candidate has already been finalized")
	}
	return s.requireCandidateSourceClosed(item)
}

func (s *knowledgeCandidateReviewService) requireCandidateSourceClosed(item *models.KnowledgeCandidate) error {
	if item.TicketID <= 0 || (item.SourceType != "ticket_repair" && item.SourceType != "ticket_manual") {
		return nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID)
	if ticket == nil || ticket.TenantID != item.TenantID {
		return errorsx.InvalidParam("knowledge candidate ticket is not available")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status != enums.TicketStatusClosed && status != enums.TicketStatusDone {
		return errorsx.InvalidParam("close the ticket before reviewing its knowledge candidate")
	}
	return nil
}

func (s *knowledgeCandidateReviewService) buildCandidateDTO(item *models.KnowledgeCandidate) dto.EnterpriseKnowledgeCandidateDTO {
	reviewStatus := item.ReviewStatus
	if reviewStatus == "" {
		reviewStatus = "pending"
	}
	breakdown := dto.KnowledgeCandidateScoreBreakdownDTO{}
	_ = json.Unmarshal([]byte(item.ScoreBreakdownJSON), &breakdown)
	flags := make([]string, 0)
	_ = json.Unmarshal([]byte(item.QualityFlagsJSON), &flags)
	return dto.EnterpriseKnowledgeCandidateDTO{
		ID:                    item.ID,
		ProductID:             item.ProductID,
		ProductModelID:        item.ProductModelID,
		SourceType:            item.SourceType,
		SourceID:              item.SourceID,
		TicketID:              item.TicketID,
		Title:                 item.Title,
		Suggestion:            item.Suggestion,
		RootCauseSummary:      item.RootCauseSummary,
		SolutionSummary:       item.SolutionSummary,
		KnowledgeBaseID:       item.KnowledgeBaseID,
		KnowledgeEntryID:      item.KnowledgeEntryID,
		QualityScore:          item.QualityScore,
		ValueScore:            item.ValueScore,
		CandidateScore:        item.CandidateScore,
		ScoreBreakdown:        breakdown,
		QualityFlags:          flags,
		ScoreVersion:          item.ScoreVersion,
		ScoredAt:              formatEnterpriseTimePtr(item.ScoredAt),
		SimilarityHash:        item.SimilarityHash,
		DuplicateGroupID:      item.DuplicateGroupID,
		MergedToCandidateID:   item.MergedToCandidateID,
		DeduplicationOverride: item.DeduplicationOverride,
		RecurrenceCount:       item.RecurrenceCount,
		AffectedDeviceCount:   item.AffectedDeviceCount,
		RequiresReassessment:  item.RequiresReassessment,
		ReviewEligible:        enums.IsKnowledgeCandidateReviewReady(reviewStatus, item.QualityScore, item.ValueScore, item.MergedToCandidateID),
		ReviewStatus:          reviewStatus,
		ReviewRemark:          item.ReviewRemark,
		ReviewedAt:            formatEnterpriseTimePtr(item.ReviewedAt),
		CreatedAt:             formatEnterpriseTime(item.CreatedAt),
	}
}

func splitKnowledgeCandidateReviewStatuses(value string) []string {
	result := make([]string, 0, 4)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
