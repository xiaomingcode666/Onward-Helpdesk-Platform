package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
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

var EnterpriseKnowledgeService = newEnterpriseKnowledgeService()

func newEnterpriseKnowledgeService() *enterpriseKnowledgeService {
	return &enterpriseKnowledgeService{}
}

type enterpriseKnowledgeService struct{}

type EnterpriseKnowledgeQuery struct {
	Page      int
	PageSize  int
	Search    string
	Category  string
	Status    string
	Product   string
	ProductID int64
}

func (s *enterpriseKnowledgeService) ListEntries(tenantID int64, query EnterpriseKnowledgeQuery) (*dto.EnterpriseListResponse[dto.EnterpriseKnowledgeEntryListItemDTO], error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	query.Page, query.PageSize = normalizeEnterprisePage(query.Page, query.PageSize)
	items, total, err := s.pagedEntryListItems(tenantID, query)
	if err != nil {
		return nil, err
	}
	totalPages := int(math.Ceil(float64(total) / float64(query.PageSize)))
	if totalPages <= 0 {
		totalPages = 1
	}
	return &dto.EnterpriseListResponse[dto.EnterpriseKnowledgeEntryListItemDTO]{
		Items:      items,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
		HasMore:    query.Page < totalPages,
	}, nil
}

func (s *enterpriseKnowledgeService) Stats(tenantID int64) (*dto.EnterpriseKnowledgeStatsDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	items := s.allEntryListItems(tenantID)
	stats := &dto.EnterpriseKnowledgeStatsDTO{TotalEntries: int64(len(items))}
	for _, item := range items {
		switch item.Status {
		case "published":
			stats.PublishedCount++
		case "review":
			stats.ReviewCount++
		case "deprecated":
			stats.DeprecatedCount++
		default:
			stats.DraftCount++
		}
		stats.AvgHitRate += int64(item.HitRate)
	}
	if len(items) > 0 {
		stats.AvgHitRate = stats.AvgHitRate / int64(len(items))
	}
	return stats, nil
}

func (s *enterpriseKnowledgeService) GetEntry(tenantID, encodedID int64) (*dto.EnterpriseKnowledgeEntryDTO, error) {
	entryType, actualID, err := decodeEnterpriseKnowledgeID(encodedID)
	if err != nil {
		return nil, err
	}
	switch entryType {
	case "document":
		item := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), actualID)
		if item == nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
			return nil, errorsx.InvalidParam("knowledge entry not found")
		}
		return s.buildDocumentEntry(*item), nil
	case "faq":
		item := repositories.KnowledgeFAQRepository.Get(sqls.DB(), actualID)
		if item == nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
			return nil, errorsx.InvalidParam("knowledge entry not found")
		}
		return s.buildFAQEntry(*item), nil
	default:
		return nil, errorsx.InvalidParam("unsupported knowledge entry type")
	}
}

func (s *enterpriseKnowledgeService) CreateEntry(tenantID int64, req dto.EnterpriseKnowledgeMutationRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeEntryDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, errorsx.InvalidParam("title and content are required")
	}
	entryType := normalizeKnowledgeEntryType(req.Type)
	kb, err := s.resolveMutationKnowledgeBase(tenantID, req.KnowledgeBaseID, req.Category, entryType, operator)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	audit := utils.BuildAuditFields(operator)
	audit.CreatedAt = now
	audit.UpdatedAt = now
	var createdID int64
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		switch entryType {
		case "faq":
			item := &models.KnowledgeFAQ{
				TenantID:         tenantID,
				KnowledgeBaseID:  kb.ID,
				Question:         strings.TrimSpace(req.Title),
				Answer:           strings.TrimSpace(req.Content),
				SimilarQuestions: "[]",
				ReviewStatus:     "draft",
				Language:         normalizeKnowledgeLanguage(req.Language),
				TagsJSON:         knowledgeStringJSON(req.Tags),
				FaultCodesJSON:   knowledgeStringJSON(req.FaultCodes),
				Status:           enums.StatusDisabled,
				IndexStatus:      enums.KnowledgeDocumentIndexStatusPending,
				AuditFields:      audit,
			}
			if err := repositories.KnowledgeFAQRepository.Create(tx, item); err != nil {
				return err
			}
			if err := s.syncRelatedProductsDB(tx, tenantID, kb.ID, item.ID, entryType, item.Language, req.Visibility, item.ReviewStatus, req.RelatedProductIDs, operator); err != nil {
				return err
			}
			createdID = encodeEnterpriseKnowledgeID("faq", item.ID)
		default:
			hash := knowledgeContentHash(req.Content)
			item := &models.KnowledgeDocument{
				TenantID:        tenantID,
				KnowledgeBaseID: kb.ID,
				Title:           strings.TrimSpace(req.Title),
				ContentType:     enums.KnowledgeDocumentContentTypeMarkdown,
				Content:         strings.TrimSpace(req.Content),
				SourceType:      "knowledge_entry",
				ContentHash:     hash,
				ReviewStatus:    "draft",
				Language:        normalizeKnowledgeLanguage(req.Language),
				TagsJSON:        knowledgeStringJSON(req.Tags),
				FaultCodesJSON:  knowledgeStringJSON(req.FaultCodes),
				Status:          enums.StatusDisabled,
				IndexStatus:     enums.KnowledgeDocumentIndexStatusPending,
				AuditFields:     audit,
			}
			if err := repositories.KnowledgeDocumentRepository.Create(tx, item); err != nil {
				return err
			}
			if err := s.syncRelatedProductsDB(tx, tenantID, kb.ID, item.ID, entryType, item.Language, req.Visibility, item.ReviewStatus, req.RelatedProductIDs, operator); err != nil {
				return err
			}
			createdID = encodeEnterpriseKnowledgeID("document", item.ID)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.GetEntry(tenantID, createdID)
}

func (s *enterpriseKnowledgeService) UpdateEntry(tenantID, encodedID int64, req dto.EnterpriseKnowledgeMutationRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeEntryDTO, error) {
	entryType, actualID, err := decodeEnterpriseKnowledgeID(encodedID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, errorsx.InvalidParam("title and content are required")
	}
	kb, err := s.resolveMutationKnowledgeBase(tenantID, req.KnowledgeBaseID, req.Category, entryType, operator)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	language := normalizeKnowledgeLanguage(req.Language)
	switch entryType {
	case "document":
		current := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), actualID)
		if current == nil || current.TenantID != tenantID || current.Status == enums.StatusDeleted {
			return nil, errorsx.InvalidParam("knowledge entry not found")
		}
		reviewStatus := "draft"
		previousKnowledgeBaseID := current.KnowledgeBaseID
		previousPublishedRevisionID := int64(0)
		previousContentHash := ""
		if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
			locked := &models.KnowledgeDocument{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND tenant_id = ? AND status <> ?", actualID, tenantID, enums.StatusDeleted).
				First(locked).Error; err != nil {
				return errorsx.InvalidParam("knowledge entry not found")
			}
			current = locked
			previousKnowledgeBaseID = current.KnowledgeBaseID
			previousPublishedRevisionID = current.PublishedRevisionID
			previousContentHash = current.ContentHash
			if current.KnowledgeBaseID != kb.ID {
				if err := s.deactivateEntryLinksDB(tx, tenantID, current.KnowledgeBaseID, actualID); err != nil {
					return err
				}
			}
			if err := repositories.KnowledgeDocumentRepository.Updates(tx, actualID, map[string]any{
				"knowledge_base_id": kb.ID,
				"title":             strings.TrimSpace(req.Title),
				"content_type":      enums.KnowledgeDocumentContentTypeMarkdown,
				"content":           strings.TrimSpace(req.Content),
				"content_hash":      knowledgeContentHash(req.Content),
				"language":          language,
				"tags_json":         knowledgeStringJSON(req.Tags),
				"fault_codes_json":  knowledgeStringJSON(req.FaultCodes),
				"index_status":      enums.KnowledgeDocumentIndexStatusPending,
				"indexed_at":        nil,
				"index_error":       "",
				"review_status":     reviewStatus,
				"status":            enums.StatusDisabled,
				"update_user_id":    operatorUserID(operator),
				"update_user_name":  operatorUsername(operator),
				"updated_at":        now,
			}); err != nil {
				return err
			}
			return s.syncRelatedProductsDB(tx, tenantID, kb.ID, actualID, entryType, language, req.Visibility, reviewStatus, req.RelatedProductIDs, operator)
		}); err != nil {
			return nil, err
		}
		if previousPublishedRevisionID > 0 {
			if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(tenantID, previousKnowledgeBaseID, entryType, actualID, previousPublishedRevisionID, previousContentHash, "delete", operator); err != nil {
				slog.Warn("enqueue previous knowledge revision cleanup failed", "entry_type", entryType, "entry_id", actualID, "revision_id", previousPublishedRevisionID, "error", err)
			}
		}
		updated := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), actualID)
		return s.buildDocumentEntry(*updated), nil
	case "faq":
		current := repositories.KnowledgeFAQRepository.Get(sqls.DB(), actualID)
		if current == nil || current.TenantID != tenantID || current.Status == enums.StatusDeleted {
			return nil, errorsx.InvalidParam("knowledge entry not found")
		}
		reviewStatus := "draft"
		previousKnowledgeBaseID := current.KnowledgeBaseID
		previousPublishedRevisionID := int64(0)
		previousContentHash := ""
		if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
			locked := &models.KnowledgeFAQ{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND tenant_id = ? AND status <> ?", actualID, tenantID, enums.StatusDeleted).
				First(locked).Error; err != nil {
				return errorsx.InvalidParam("knowledge entry not found")
			}
			current = locked
			previousKnowledgeBaseID = current.KnowledgeBaseID
			previousPublishedRevisionID = current.PublishedRevisionID
			previousContentHash = knowledgeContentHash(strings.TrimSpace(current.Question) + "\n" + strings.TrimSpace(current.Answer))
			if current.KnowledgeBaseID != kb.ID {
				if err := s.deactivateEntryLinksDB(tx, tenantID, current.KnowledgeBaseID, actualID); err != nil {
					return err
				}
			}
			if err := repositories.KnowledgeFAQRepository.Updates(tx, actualID, map[string]any{
				"knowledge_base_id": kb.ID,
				"question":          strings.TrimSpace(req.Title),
				"answer":            strings.TrimSpace(req.Content),
				"language":          language,
				"tags_json":         knowledgeStringJSON(req.Tags),
				"fault_codes_json":  knowledgeStringJSON(req.FaultCodes),
				"index_status":      enums.KnowledgeDocumentIndexStatusPending,
				"indexed_at":        nil,
				"index_error":       "",
				"review_status":     reviewStatus,
				"status":            enums.StatusDisabled,
				"update_user_id":    operatorUserID(operator),
				"update_user_name":  operatorUsername(operator),
				"updated_at":        now,
			}); err != nil {
				return err
			}
			return s.syncRelatedProductsDB(tx, tenantID, kb.ID, actualID, entryType, language, req.Visibility, reviewStatus, req.RelatedProductIDs, operator)
		}); err != nil {
			return nil, err
		}
		if previousPublishedRevisionID > 0 {
			if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(tenantID, previousKnowledgeBaseID, entryType, actualID, previousPublishedRevisionID, previousContentHash, "delete", operator); err != nil {
				slog.Warn("enqueue previous knowledge revision cleanup failed", "entry_type", entryType, "entry_id", actualID, "revision_id", previousPublishedRevisionID, "error", err)
			}
		}
		updated := repositories.KnowledgeFAQRepository.Get(sqls.DB(), actualID)
		return s.buildFAQEntry(*updated), nil
	default:
		return nil, errorsx.InvalidParam("unsupported knowledge entry type")
	}
}

func knowledgeEntryGateError(assessment knowledgeEntryAssessment) error {
	return errorsx.InvalidParam(fmt.Sprintf(
		"knowledge entry score does not meet review thresholds: quality %d/%d, value %d/%d",
		assessment.QualityScore,
		enums.KnowledgeEntryMinimumQualityScore,
		assessment.ValueScore,
		enums.KnowledgeEntryMinimumValueScore,
	))
}

func (s *enterpriseKnowledgeService) UpdateEntryStatus(tenantID, encodedID int64, next string, operator *dto.AuthPrincipal) (*dto.EnterpriseKnowledgeEntryDTO, error) {
	entryType, actualID, err := decodeEnterpriseKnowledgeID(encodedID)
	if err != nil {
		return nil, err
	}
	reviewStatus, modelStatus, indexStatus, err := knowledgeWorkflowColumns(next)
	if err != nil {
		return nil, err
	}
	buildColumns := func(now time.Time) map[string]any {
		columns := map[string]any{
			"review_status":    reviewStatus,
			"status":           modelStatus,
			"index_status":     indexStatus,
			"update_user_id":   operatorUserID(operator),
			"update_user_name": operatorUsername(operator),
			"updated_at":       now,
		}
		if reviewStatus == "published" || reviewStatus == "deprecated" {
			columns["indexed_at"] = nil
			columns["index_error"] = ""
		}
		return columns
	}
	switch entryType {
	case "document":
		var updated *models.KnowledgeDocument
		var supersededRevision *models.KnowledgeRevision
		shouldEnqueue := false
		if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
			current := &models.KnowledgeDocument{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND tenant_id = ? AND status <> ?", actualID, tenantID, enums.StatusDeleted).
				First(current).Error; err != nil {
				return errorsx.InvalidParam("knowledge entry not found")
			}
			candidateApproved := isApprovedCandidateKnowledgeDocument(tx, current)
			if err := validateKnowledgeEntryTransition(current.ReviewStatus, reviewStatus, candidateApproved); err != nil {
				return err
			}
			if current.ReviewStatus == reviewStatus {
				updated = current
				return nil
			}
			if reviewStatus == "review" || reviewStatus == "published" {
				if err := requireActiveKnowledgeBaseForEntryTx(tx, tenantID, current.KnowledgeBaseID, "document"); err != nil {
					return err
				}
				assessment := assessKnowledgeDocument(tx, current)
				if !assessment.ReviewEligible {
					return knowledgeEntryGateError(assessment)
				}
			}
			now := time.Now()
			columns := buildColumns(now)
			if reviewStatus == "published" {
				previousPublishedRevisionID := current.PublishedRevisionID
				revision, revisionErr := s.publishDocumentRevision(tx, *current, now, operator)
				if revisionErr != nil {
					return revisionErr
				}
				columns["current_revision_id"] = revision.ID
				columns["published_revision_id"] = revision.ID
				if previousPublishedRevisionID > 0 && previousPublishedRevisionID != revision.ID {
					supersededRevision = repositories.KnowledgeRevisionRepository.Get(tx, previousPublishedRevisionID)
				}
			}
			if updateErr := repositories.KnowledgeDocumentRepository.Updates(tx, actualID, columns); updateErr != nil {
				return updateErr
			}
			if err := s.syncEntryLinkPublishStatusTx(tx, tenantID, current.KnowledgeBaseID, actualID, reviewStatus); err != nil {
				return err
			}
			updated = repositories.KnowledgeDocumentRepository.Get(tx, actualID)
			shouldEnqueue = true
			return nil
		}); err != nil {
			return nil, err
		}
		if shouldEnqueue && (reviewStatus == "published" || reviewStatus == "deprecated") {
			s.enqueueEntryIndexSync(tenantID, updated.KnowledgeBaseID, entryType, actualID, reviewStatus, next, operator)
		}
		s.enqueueSupersededRevisionCleanup(supersededRevision, operator)
		return s.buildDocumentEntry(*updated), nil
	case "faq":
		var updated *models.KnowledgeFAQ
		var supersededRevision *models.KnowledgeRevision
		shouldEnqueue := false
		if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
			current := &models.KnowledgeFAQ{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND tenant_id = ? AND status <> ?", actualID, tenantID, enums.StatusDeleted).
				First(current).Error; err != nil {
				return errorsx.InvalidParam("knowledge entry not found")
			}
			if err := validateKnowledgeEntryTransition(current.ReviewStatus, reviewStatus, false); err != nil {
				return err
			}
			if current.ReviewStatus == reviewStatus {
				updated = current
				return nil
			}
			if reviewStatus == "review" || reviewStatus == "published" {
				if err := requireActiveKnowledgeBaseForEntryTx(tx, tenantID, current.KnowledgeBaseID, "faq"); err != nil {
					return err
				}
				assessment := assessKnowledgeFAQ(tx, current)
				if !assessment.ReviewEligible {
					return knowledgeEntryGateError(assessment)
				}
			}
			now := time.Now()
			columns := buildColumns(now)
			if reviewStatus == "published" {
				previousPublishedRevisionID := current.PublishedRevisionID
				revision, revisionErr := s.publishFAQRevision(tx, *current, now, operator)
				if revisionErr != nil {
					return revisionErr
				}
				columns["current_revision_id"] = revision.ID
				columns["published_revision_id"] = revision.ID
				if previousPublishedRevisionID > 0 && previousPublishedRevisionID != revision.ID {
					supersededRevision = repositories.KnowledgeRevisionRepository.Get(tx, previousPublishedRevisionID)
				}
			}
			if updateErr := repositories.KnowledgeFAQRepository.Updates(tx, actualID, columns); updateErr != nil {
				return updateErr
			}
			if err := s.syncEntryLinkPublishStatusTx(tx, tenantID, current.KnowledgeBaseID, actualID, reviewStatus); err != nil {
				return err
			}
			updated = repositories.KnowledgeFAQRepository.Get(tx, actualID)
			shouldEnqueue = true
			return nil
		}); err != nil {
			return nil, err
		}
		if shouldEnqueue && (reviewStatus == "published" || reviewStatus == "deprecated") {
			s.enqueueEntryIndexSync(tenantID, updated.KnowledgeBaseID, entryType, actualID, reviewStatus, next, operator)
		}
		s.enqueueSupersededRevisionCleanup(supersededRevision, operator)
		return s.buildFAQEntry(*updated), nil
	default:
		return nil, errorsx.InvalidParam("unsupported knowledge entry type")
	}
}

func (s *enterpriseKnowledgeService) enqueueSupersededRevisionCleanup(revision *models.KnowledgeRevision, operator *dto.AuthPrincipal) {
	if revision == nil || revision.ID <= 0 {
		return
	}
	if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(
		revision.TenantID, revision.KnowledgeBaseID, revision.EntryType, revision.EntryID,
		revision.ID, revision.ContentHash, "delete", operator,
	); err != nil {
		slog.Warn("enqueue superseded knowledge revision cleanup failed", "entry_type", revision.EntryType, "entry_id", revision.EntryID, "revision_id", revision.ID, "error", err)
	}
}

func isApprovedCandidateKnowledgeDocument(db *gorm.DB, document *models.KnowledgeDocument) bool {
	if document == nil || document.SourceReferenceID <= 0 || (document.SourceType != "knowledge_candidate" && document.SourceType != "knowledge_entry") {
		return false
	}
	candidate := loadKnowledgeEntrySourceCandidate(db, document.TenantID, document.SourceReferenceID)
	return candidate != nil && candidate.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusApproved) && !candidate.RequiresReassessment
}

func validateKnowledgeEntryTransition(current, next string, candidateApproved bool) error {
	current = strings.TrimSpace(current)
	if current == "" {
		current = "draft"
	}
	if current == next {
		return nil
	}
	valid := false
	switch current {
	case "draft":
		valid = next == "review" || (next == "published" && candidateApproved)
	case "review":
		valid = next == "draft" || next == "published"
	case "published":
		valid = next == "deprecated"
	case "deprecated":
		valid = next == "draft"
	}
	if !valid {
		return errorsx.InvalidParam(fmt.Sprintf("invalid knowledge status transition: %s -> %s", current, next))
	}
	return nil
}

func requireActiveKnowledgeBaseForEntryTx(db *gorm.DB, tenantID, knowledgeBaseID int64, entryType string) error {
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk {
		return errorsx.InvalidParam("activate the knowledge base before reviewing or publishing its entries")
	}
	isFAQ := strings.TrimSpace(entryType) == "faq"
	if (knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ)) != isFAQ {
		return errorsx.InvalidParam("knowledge entry type does not match its knowledge base")
	}
	return nil
}

func (s *enterpriseKnowledgeService) publishDocumentRevision(db *gorm.DB, document models.KnowledgeDocument, now time.Time, operator *dto.AuthPrincipal) (*models.KnowledgeRevision, error) {
	contentHash := strings.TrimSpace(document.ContentHash)
	if contentHash == "" {
		contentHash = knowledgeContentHash(document.Content)
	}
	return s.publishKnowledgeRevision(db, models.KnowledgeRevision{
		TenantID:        document.TenantID,
		KnowledgeBaseID: document.KnowledgeBaseID,
		EntryType:       "document",
		EntryID:         document.ID,
		Title:           document.Title,
		Content:         document.Content,
		Language:        normalizeKnowledgeLanguage(document.Language),
		Visibility:      "internal",
		TagsJSON:        firstNonBlank(document.TagsJSON, "[]"),
		FaultCodesJSON:  firstNonBlank(document.FaultCodesJSON, "[]"),
		ContentHash:     contentHash,
	}, document.PublishedRevisionID, now, operator)
}

func (s *enterpriseKnowledgeService) publishFAQRevision(db *gorm.DB, faq models.KnowledgeFAQ, now time.Time, operator *dto.AuthPrincipal) (*models.KnowledgeRevision, error) {
	return s.publishKnowledgeRevision(db, models.KnowledgeRevision{
		TenantID:        faq.TenantID,
		KnowledgeBaseID: faq.KnowledgeBaseID,
		EntryType:       "faq",
		EntryID:         faq.ID,
		Title:           faq.Question,
		Content:         faq.Answer,
		Language:        normalizeKnowledgeLanguage(faq.Language),
		Visibility:      "internal",
		TagsJSON:        firstNonBlank(faq.TagsJSON, "[]"),
		FaultCodesJSON:  firstNonBlank(faq.FaultCodesJSON, "[]"),
		ContentHash:     knowledgeContentHash(strings.TrimSpace(faq.Question) + "\n" + strings.TrimSpace(faq.Answer)),
	}, faq.PublishedRevisionID, now, operator)
}

func (s *enterpriseKnowledgeService) publishKnowledgeRevision(db *gorm.DB, snapshot models.KnowledgeRevision, previousPublishedRevisionID int64, now time.Time, operator *dto.AuthPrincipal) (*models.KnowledgeRevision, error) {
	latest := repositories.KnowledgeRevisionRepository.FindLatestByEntry(db, snapshot.TenantID, snapshot.EntryType, snapshot.EntryID)
	if latest != nil && latest.ReviewStatus == "published" && latest.Title == snapshot.Title && latest.ContentHash == snapshot.ContentHash && latest.Language == snapshot.Language {
		return latest, nil
	}

	versionNo := 1
	if latest != nil && latest.VersionNo >= versionNo {
		versionNo = latest.VersionNo + 1
	}
	snapshot.VersionNo = versionNo
	snapshot.VersionLabel = "v" + strconv.Itoa(versionNo)
	snapshot.ReviewStatus = "published"
	snapshot.PublishedAt = &now
	snapshot.PublishedByID = operatorUserID(operator)
	snapshot.PublishedByName = operatorUsername(operator)
	snapshot.AuditFields = utils.BuildAuditFields(operator)
	snapshot.CreatedAt = now
	snapshot.UpdatedAt = now
	if err := repositories.KnowledgeRevisionRepository.Create(db, &snapshot); err != nil {
		return nil, err
	}
	if previousPublishedRevisionID > 0 && previousPublishedRevisionID != snapshot.ID {
		if err := repositories.KnowledgeRevisionRepository.Updates(db, previousPublishedRevisionID, map[string]any{
			"superseded_at":    now,
			"update_user_id":   operatorUserID(operator),
			"update_user_name": operatorUsername(operator),
			"updated_at":       now,
		}); err != nil {
			return nil, err
		}
	}
	return &snapshot, nil
}

// enqueueEntryIndexSync 知识发布/修订/废弃时生成索引同步任务（§8.3）。
// 失败不阻塞知识生命周期主流程。
func (s *enterpriseKnowledgeService) enqueueEntryIndexSync(tenantID, knowledgeBaseID int64, entryType string, entryID int64, reviewStatus, action string, operator *dto.AuthPrincipal) {
	syncAction := "upsert"
	if action == "deprecate" || reviewStatus == "deprecated" {
		syncAction = "delete"
	}
	if _, err := KnowledgeIndexSyncService.EnqueueEntrySync(tenantID, knowledgeBaseID, entryType, entryID, reviewStatus, syncAction, operator); err != nil {
		slog.Warn("enqueue knowledge index sync failed", "entry_type", entryType, "entry_id", entryID, "error", err)
	}
}

func (s *enterpriseKnowledgeService) Versions(tenantID, encodedID int64) ([]dto.EnterpriseKnowledgeVersionDTO, error) {
	entry, err := s.GetEntry(tenantID, encodedID)
	if err != nil {
		return nil, err
	}
	entryType, actualID, err := decodeEnterpriseKnowledgeID(encodedID)
	if err != nil {
		return nil, err
	}
	revisions := repositories.KnowledgeRevisionRepository.FindByEntry(sqls.DB(), tenantID, entryType, actualID)
	result := make([]dto.EnterpriseKnowledgeVersionDTO, 0, len(revisions)+1)
	latestVersion := 0
	currentMatchesLatest := false
	if len(revisions) > 0 {
		latestVersion = revisions[0].VersionNo
		currentMatchesLatest = revisions[0].Title == entry.Title && revisions[0].Content == entry.Content
	}
	if entry.Status != "published" && (!currentMatchesLatest || len(revisions) == 0) {
		result = append(result, dto.EnterpriseKnowledgeVersionDTO{
			Version: latestVersion + 1, EntryID: encodedID, Title: entry.Title, Content: entry.Content,
			Language: firstNonBlank(firstString(entry.Languages), "default"), ChangeNote: "Current draft (unpublished)",
			UpdatedBy: entry.AuthorName, UpdatedAt: entry.UpdatedAt,
		})
	}
	for i := range revisions {
		updatedAt := revisions[i].UpdatedAt
		if revisions[i].PublishedAt != nil {
			updatedAt = *revisions[i].PublishedAt
		}
		changeNote := "Published revision"
		if revisions[i].SupersededAt != nil {
			changeNote = "Superseded published revision"
		}
		result = append(result, dto.EnterpriseKnowledgeVersionDTO{
			Version: revisions[i].VersionNo, EntryID: encodedID, Title: revisions[i].Title, Content: revisions[i].Content,
			Language: revisions[i].Language, ChangeNote: changeNote,
			UpdatedBy: revisions[i].PublishedByName, UpdatedAt: formatEnterpriseTime(updatedAt),
		})
	}
	if len(result) == 0 {
		result = append(result, dto.EnterpriseKnowledgeVersionDTO{
			Version: 1, EntryID: encodedID, Title: entry.Title, Content: entry.Content,
			Language: firstNonBlank(firstString(entry.Languages), "default"), ChangeNote: "Current stored version",
			UpdatedBy: entry.AuthorName, UpdatedAt: entry.UpdatedAt,
		})
	}
	return result, nil
}

func (s *enterpriseKnowledgeService) Quality(tenantID, encodedID int64) (*dto.EnterpriseKnowledgeQualityDTO, error) {
	entry, err := s.GetEntry(tenantID, encodedID)
	if err != nil {
		return nil, err
	}
	return &dto.EnterpriseKnowledgeQualityDTO{
		CompletenessScore: entry.CompletenessScore,
		HitRate:           entry.HitRate,
		PositiveFeedback:  0,
		NegativeFeedback:  0,
		TotalViews:        0,
	}, nil
}

func (s *enterpriseKnowledgeService) Translations(tenantID, encodedID int64) ([]dto.EnterpriseKnowledgeTranslationDTO, error) {
	entry, err := s.GetEntry(tenantID, encodedID)
	if err != nil {
		return nil, err
	}
	language := firstNonBlank(firstString(entry.Languages), "default")
	return []dto.EnterpriseKnowledgeTranslationDTO{{
		Language:  language,
		Title:     entry.Title,
		Content:   entry.Content,
		Status:    entry.Status,
		UpdatedAt: entry.UpdatedAt,
	}}, nil
}

func (s *enterpriseKnowledgeService) allEntryListItems(tenantID int64) []dto.EnterpriseKnowledgeEntryListItemDTO {
	kbNames := s.knowledgeBaseNames(tenantID)
	items := make([]dto.EnterpriseKnowledgeEntryListItemDTO, 0)
	documents := repositories.KnowledgeDocumentRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("source_asset_id", 0).
		NotEq("status", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id"))
	for _, item := range documents {
		items = append(items, s.buildDocumentItem(item, kbNames))
	}
	faqs := repositories.KnowledgeFAQRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id"))
	for _, item := range faqs {
		items = append(items, s.buildFAQItem(item, kbNames))
	}
	return items
}

type enterpriseKnowledgeEntryPageRef struct {
	EntryType string    `gorm:"column:entry_type"`
	ID        int64     `gorm:"column:id"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (s *enterpriseKnowledgeService) pagedEntryListItems(tenantID int64, query EnterpriseKnowledgeQuery) ([]dto.EnterpriseKnowledgeEntryListItemDTO, int64, error) {
	db := sqls.DB()
	documentSQL, documentArgs := s.entryPageSelectSQL(db, "document", tenantID, query)
	faqSQL, faqArgs := s.entryPageSelectSQL(db, "faq", tenantID, query)
	unionSQL := documentSQL + " UNION ALL " + faqSQL
	args := make([]any, 0, len(documentArgs)+len(faqArgs)+2)
	args = append(args, documentArgs...)
	args = append(args, faqArgs...)

	var total int64
	if err := db.Raw("SELECT COUNT(*) FROM ("+unionSQL+") AS knowledge_entry_page_refs", args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.EnterpriseKnowledgeEntryListItemDTO{}, 0, nil
	}

	offset := (query.Page - 1) * query.PageSize
	pageArgs := append(append([]any{}, args...), query.PageSize, offset)
	var refs []enterpriseKnowledgeEntryPageRef
	if err := db.Raw("SELECT entry_type, id, updated_at FROM ("+unionSQL+") AS knowledge_entry_page_refs ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?", pageArgs...).Scan(&refs).Error; err != nil {
		return nil, 0, err
	}
	return s.buildEntryListPage(tenantID, refs), total, nil
}

func (s *enterpriseKnowledgeService) entryPageSelectSQL(db *gorm.DB, entryType string, tenantID int64, query EnterpriseKnowledgeQuery) (string, []any) {
	table := db.NamingStrategy.TableName("KnowledgeDocument")
	titleColumn := "entries.title"
	if entryType == "faq" {
		table = db.NamingStrategy.TableName("KnowledgeFAQ")
		titleColumn = "entries.question"
	}
	knowledgeBaseTable := db.NamingStrategy.TableName("KnowledgeBase")
	productLinkTable := db.NamingStrategy.TableName("ProductKnowledgeLink")
	productTable := db.NamingStrategy.TableName("Product")
	args := []any{entryType, enums.StatusDeleted, tenantID, enums.StatusDeleted}
	joins := " JOIN " + knowledgeBaseTable + " AS knowledge_bases ON knowledge_bases.id = entries.knowledge_base_id AND knowledge_bases.tenant_id = entries.tenant_id AND knowledge_bases.status <> ?"
	where := []string{"entries.tenant_id = ?", "entries.status <> ?"}
	if entryType == "document" {
		where = append(where, "entries.source_asset_id = 0")
	}
	if search := strings.ToLower(strings.TrimSpace(query.Search)); search != "" {
		pattern := "%" + search + "%"
		where = append(where, "(LOWER("+titleColumn+") LIKE ? OR "+s.entryProductNameExistsSQL(productLinkTable, productTable)+")")
		args = append(args, pattern, pattern, pattern)
	}
	if category := strings.TrimSpace(query.Category); category != "" && category != "All" {
		where = append(where, "knowledge_bases.name = ?")
		args = append(args, category)
	}
	if status := strings.TrimSpace(query.Status); status != "" && status != "all" {
		statusSQL, statusArgs := entryStatusSQL(status)
		where = append(where, statusSQL)
		args = append(args, statusArgs...)
	}
	if query.ProductID > 0 {
		where = append(where, s.entryProductIDExistsSQL(productLinkTable))
		args = append(args, query.ProductID)
	}
	if product := strings.ToLower(strings.TrimSpace(query.Product)); product != "" {
		pattern := "%" + product + "%"
		where = append(where, s.entryProductNameExistsSQL(productLinkTable, productTable))
		args = append(args, pattern, pattern)
	}
	return "SELECT ? AS entry_type, entries.id, entries.updated_at FROM " + table + " AS entries" + joins + " WHERE " + strings.Join(where, " AND "), args
}

func (s *enterpriseKnowledgeService) entryProductIDExistsSQL(linkTable string) string {
	return "EXISTS (SELECT 1 FROM " + linkTable + " AS product_knowledge_links WHERE product_knowledge_links.tenant_id = entries.tenant_id AND product_knowledge_links.knowledge_base_id = entries.knowledge_base_id AND product_knowledge_links.product_id = ? AND product_knowledge_links.status <> " + strconv.Itoa(int(enums.StatusDeleted)) + " AND " + s.entryProductLinkScopeSQL(linkTable) + ")"
}

func (s *enterpriseKnowledgeService) entryProductNameExistsSQL(linkTable, productTable string) string {
	productFilter := "(LOWER(products.name) LIKE ? OR LOWER(products.code) LIKE ?)"
	return "EXISTS (SELECT 1 FROM " + linkTable + " AS product_knowledge_links JOIN " + productTable + " AS products ON products.id = product_knowledge_links.product_id AND products.tenant_id = product_knowledge_links.tenant_id AND products.status <> " + strconv.Itoa(int(enums.StatusDeleted)) + " WHERE product_knowledge_links.tenant_id = entries.tenant_id AND product_knowledge_links.knowledge_base_id = entries.knowledge_base_id AND product_knowledge_links.status <> " + strconv.Itoa(int(enums.StatusDeleted)) + " AND " + s.entryProductLinkScopeSQL(linkTable) + " AND " + productFilter + ")"
}

func (s *enterpriseKnowledgeService) entryProductLinkScopeSQL(linkTable string) string {
	return "(product_knowledge_links.knowledge_entry_id = entries.id OR (product_knowledge_links.knowledge_entry_id = 0 AND NOT EXISTS (SELECT 1 FROM " + linkTable + " AS entry_product_knowledge_links WHERE entry_product_knowledge_links.tenant_id = entries.tenant_id AND entry_product_knowledge_links.knowledge_base_id = entries.knowledge_base_id AND entry_product_knowledge_links.knowledge_entry_id = entries.id AND entry_product_knowledge_links.status <> " + strconv.Itoa(int(enums.StatusDeleted)) + ")))"
}

func entryStatusSQL(status string) (string, []any) {
	switch status {
	case "published":
		return "(entries.review_status = 'published' OR (entries.review_status NOT IN ('review', 'published', 'deprecated') AND entries.status = " + strconv.Itoa(int(enums.StatusOk)) + " AND entries.index_status = ?))", []any{string(enums.KnowledgeDocumentIndexStatusIndexed)}
	case "review":
		return "(entries.review_status = 'review' OR (entries.review_status NOT IN ('review', 'published', 'deprecated') AND entries.status = " + strconv.Itoa(int(enums.StatusOk)) + " AND entries.index_status = ?))", []any{string(enums.KnowledgeDocumentIndexStatusPending)}
	case "deprecated":
		return "entries.review_status = 'deprecated'", nil
	case "draft":
		return "(entries.review_status = 'draft' AND NOT (entries.status = " + strconv.Itoa(int(enums.StatusOk)) + " AND entries.index_status IN (?, ?)))", []any{string(enums.KnowledgeDocumentIndexStatusIndexed), string(enums.KnowledgeDocumentIndexStatusPending)}
	default:
		return "1 = 1", nil
	}
}

func (s *enterpriseKnowledgeService) buildEntryListPage(tenantID int64, refs []enterpriseKnowledgeEntryPageRef) []dto.EnterpriseKnowledgeEntryListItemDTO {
	if len(refs) == 0 {
		return []dto.EnterpriseKnowledgeEntryListItemDTO{}
	}
	documentIDs := make([]int64, 0, len(refs))
	faqIDs := make([]int64, 0, len(refs))
	for _, ref := range refs {
		if ref.EntryType == "faq" {
			faqIDs = append(faqIDs, ref.ID)
			continue
		}
		documentIDs = append(documentIDs, ref.ID)
	}
	documents := make(map[int64]models.KnowledgeDocument, len(documentIDs))
	if len(documentIDs) > 0 {
		var rows []models.KnowledgeDocument
		sqls.DB().Where("tenant_id = ? AND id IN ? AND status <> ?", tenantID, documentIDs, enums.StatusDeleted).Find(&rows)
		for _, row := range rows {
			documents[row.ID] = row
		}
	}
	faqs := make(map[int64]models.KnowledgeFAQ, len(faqIDs))
	if len(faqIDs) > 0 {
		var rows []models.KnowledgeFAQ
		sqls.DB().Where("tenant_id = ? AND id IN ? AND status <> ?", tenantID, faqIDs, enums.StatusDeleted).Find(&rows)
		for _, row := range rows {
			faqs[row.ID] = row
		}
	}
	kbNames := s.knowledgeBaseNames(tenantID)
	items := make([]dto.EnterpriseKnowledgeEntryListItemDTO, 0, len(refs))
	for _, ref := range refs {
		if ref.EntryType == "faq" {
			if item, ok := faqs[ref.ID]; ok {
				items = append(items, s.buildFAQItem(item, kbNames))
			}
			continue
		}
		if item, ok := documents[ref.ID]; ok {
			items = append(items, s.buildDocumentItem(item, kbNames))
		}
	}
	return items
}

func (s *enterpriseKnowledgeService) buildDocumentItem(item models.KnowledgeDocument, kbNames map[int64]string) dto.EnterpriseKnowledgeEntryListItemDTO {
	status := knowledgeStatus(item.Status, string(item.IndexStatus), item.ReviewStatus)
	assessment := assessKnowledgeDocument(sqls.DB(), &item)
	relatedProducts := s.relatedProducts(item.TenantID, item.KnowledgeBaseID, item.ID)
	return dto.EnterpriseKnowledgeEntryListItemDTO{
		ID:              encodeEnterpriseKnowledgeID("document", item.ID),
		KnowledgeBaseID: item.KnowledgeBaseID,
		Title:           item.Title,
		Type:            "document",
		Category:        firstNonBlank(kbNames[item.KnowledgeBaseID], "Document"),
		Status:          status,
		Tags:            knowledgeStringSlice(item.TagsJSON),
		Languages:       []string{normalizeKnowledgeLanguage(item.Language)},
		ProductScope:    productScopeFromRelatedProducts(relatedProducts),
		ProductIDs:      productIDsFromRelatedProducts(relatedProducts),
		RelevanceScore:  assessment.EntryScore,
		QualityScore:    assessment.QualityScore,
		ValueScore:      assessment.ValueScore,
		EntryScore:      assessment.EntryScore,
		ReviewEligible:  assessment.ReviewEligible,
		QualityFlags:    assessment.QualityFlags,
		HitRate:         knowledgeRelevanceScore(status),
		AuthorName:      item.CreateUserName,
		UpdatedAt:       formatEnterpriseTime(item.UpdatedAt),
	}
}

func (s *enterpriseKnowledgeService) buildFAQItem(item models.KnowledgeFAQ, kbNames map[int64]string) dto.EnterpriseKnowledgeEntryListItemDTO {
	status := knowledgeStatus(item.Status, string(item.IndexStatus), item.ReviewStatus)
	assessment := assessKnowledgeFAQ(sqls.DB(), &item)
	relatedProducts := s.relatedProducts(item.TenantID, item.KnowledgeBaseID, item.ID)
	return dto.EnterpriseKnowledgeEntryListItemDTO{
		ID:              encodeEnterpriseKnowledgeID("faq", item.ID),
		KnowledgeBaseID: item.KnowledgeBaseID,
		Title:           item.Question,
		Type:            "faq",
		Category:        firstNonBlank(kbNames[item.KnowledgeBaseID], "FAQ"),
		Status:          status,
		Tags:            knowledgeStringSlice(item.TagsJSON),
		Languages:       []string{normalizeKnowledgeLanguage(item.Language)},
		ProductScope:    productScopeFromRelatedProducts(relatedProducts),
		ProductIDs:      productIDsFromRelatedProducts(relatedProducts),
		RelevanceScore:  assessment.EntryScore,
		QualityScore:    assessment.QualityScore,
		ValueScore:      assessment.ValueScore,
		EntryScore:      assessment.EntryScore,
		ReviewEligible:  assessment.ReviewEligible,
		QualityFlags:    assessment.QualityFlags,
		HitRate:         knowledgeRelevanceScore(status),
		AuthorName:      item.CreateUserName,
		UpdatedAt:       formatEnterpriseTime(item.UpdatedAt),
	}
}

func (s *enterpriseKnowledgeService) filterEntries(items []dto.EnterpriseKnowledgeEntryListItemDTO, query EnterpriseKnowledgeQuery) []dto.EnterpriseKnowledgeEntryListItemDTO {
	search := strings.ToLower(strings.TrimSpace(query.Search))
	category := strings.TrimSpace(query.Category)
	status := strings.TrimSpace(query.Status)
	product := strings.ToLower(strings.TrimSpace(query.Product))
	productID := query.ProductID
	filtered := make([]dto.EnterpriseKnowledgeEntryListItemDTO, 0, len(items))
	for _, item := range items {
		if search != "" && !strings.Contains(strings.ToLower(item.Title), search) && !strings.Contains(strings.ToLower(item.ProductScope), search) {
			continue
		}
		if category != "" && category != "All" && item.Category != category {
			continue
		}
		if status != "" && status != "all" && item.Status != status {
			continue
		}
		if productID > 0 && !containsInt64(item.ProductIDs, productID) {
			continue
		}
		if product != "" && !strings.Contains(strings.ToLower(item.ProductScope), product) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (s *enterpriseKnowledgeService) knowledgeBaseNames(tenantID int64) map[int64]string {
	bases := repositories.KnowledgeBaseRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted))
	result := make(map[int64]string, len(bases))
	for _, item := range bases {
		result[item.ID] = item.Name
	}
	return result
}

func (s *enterpriseKnowledgeService) buildDocumentEntry(item models.KnowledgeDocument) *dto.EnterpriseKnowledgeEntryDTO {
	category := "Document"
	if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), item.KnowledgeBaseID); kb != nil {
		category = kb.Name
	}
	status := knowledgeStatus(item.Status, string(item.IndexStatus), item.ReviewStatus)
	tags := knowledgeStringSlice(item.TagsJSON)
	faultCodes := knowledgeStringSlice(item.FaultCodesJSON)
	relatedProducts := s.relatedProducts(item.TenantID, item.KnowledgeBaseID, item.ID)
	assessment := assessKnowledgeDocument(sqls.DB(), &item)
	return &dto.EnterpriseKnowledgeEntryDTO{
		ID:                encodeEnterpriseKnowledgeID("document", item.ID),
		KnowledgeBaseID:   item.KnowledgeBaseID,
		Title:             item.Title,
		Content:           item.Content,
		Category:          category,
		Status:            status,
		Tags:              tags,
		RelatedProducts:   relatedProducts,
		FaultCodes:        faultCodes,
		Languages:         []string{normalizeKnowledgeLanguage(item.Language)},
		HitRate:           knowledgeRelevanceScore(status),
		CompletenessScore: knowledgeCompletenessScore(item.Title, item.Content, category, tags, faultCodes, relatedProducts),
		QualityScore:      assessment.QualityScore,
		ValueScore:        assessment.ValueScore,
		EntryScore:        assessment.EntryScore,
		ReviewEligible:    assessment.ReviewEligible,
		QualityFlags:      assessment.QualityFlags,
		AuthorName:        item.CreateUserName,
		CreatedAt:         formatEnterpriseTime(item.CreatedAt),
		UpdatedAt:         formatEnterpriseTime(item.UpdatedAt),
		PublishedAt:       publishedAt(status, item.IndexedAt),
	}
}

func (s *enterpriseKnowledgeService) buildFAQEntry(item models.KnowledgeFAQ) *dto.EnterpriseKnowledgeEntryDTO {
	category := "FAQ"
	if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), item.KnowledgeBaseID); kb != nil {
		category = kb.Name
	}
	status := knowledgeStatus(item.Status, string(item.IndexStatus), item.ReviewStatus)
	tags := knowledgeStringSlice(item.TagsJSON)
	faultCodes := knowledgeStringSlice(item.FaultCodesJSON)
	relatedProducts := s.relatedProducts(item.TenantID, item.KnowledgeBaseID, item.ID)
	assessment := assessKnowledgeFAQ(sqls.DB(), &item)
	return &dto.EnterpriseKnowledgeEntryDTO{
		ID:                encodeEnterpriseKnowledgeID("faq", item.ID),
		KnowledgeBaseID:   item.KnowledgeBaseID,
		Title:             item.Question,
		Content:           item.Answer,
		Category:          category,
		Status:            status,
		Tags:              tags,
		RelatedProducts:   relatedProducts,
		FaultCodes:        faultCodes,
		Languages:         []string{normalizeKnowledgeLanguage(item.Language)},
		HitRate:           knowledgeRelevanceScore(status),
		CompletenessScore: knowledgeCompletenessScore(item.Question, item.Answer, category, tags, faultCodes, relatedProducts),
		QualityScore:      assessment.QualityScore,
		ValueScore:        assessment.ValueScore,
		EntryScore:        assessment.EntryScore,
		ReviewEligible:    assessment.ReviewEligible,
		QualityFlags:      assessment.QualityFlags,
		AuthorName:        item.CreateUserName,
		CreatedAt:         formatEnterpriseTime(item.CreatedAt),
		UpdatedAt:         formatEnterpriseTime(item.UpdatedAt),
		PublishedAt:       publishedAt(status, item.IndexedAt),
	}
}

func (s *enterpriseKnowledgeService) productScopeForKnowledgeBase(knowledgeBaseID int64) string {
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().Eq("knowledge_base_id", knowledgeBaseID).NotEq("status", enums.StatusDeleted))
	if len(links) == 0 {
		return "All"
	}
	names := make([]string, 0, len(links))
	seen := make(map[int64]struct{})
	for _, link := range links {
		if link.ProductID <= 0 {
			continue
		}
		if _, ok := seen[link.ProductID]; ok {
			continue
		}
		seen[link.ProductID] = struct{}{}
		if product := repositories.ProductRepository.Get(sqls.DB(), link.ProductID); product != nil {
			names = append(names, product.Name)
		}
	}
	if len(names) == 0 {
		return "All"
	}
	return strings.Join(names, ", ")
}

func productScopeFromRelatedProducts(products []dto.EnterpriseKnowledgeRelatedProductDTO) string {
	if len(products) == 0 {
		return "All"
	}
	names := make([]string, 0, len(products))
	for _, item := range products {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		names = append(names, item.Name)
	}
	if len(names) == 0 {
		return "All"
	}
	return strings.Join(names, ", ")
}

func productIDsFromRelatedProducts(products []dto.EnterpriseKnowledgeRelatedProductDTO) []int64 {
	if len(products) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(products))
	for _, item := range products {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *enterpriseKnowledgeService) ensureKnowledgeBase(tenantID int64, category string, entryType string, operator *dto.AuthPrincipal) (*models.KnowledgeBase, error) {
	name := firstNonBlank(category, "General Knowledge")
	knowledgeType := string(enums.KnowledgeBaseTypeDocument)
	if entryType == "faq" {
		knowledgeType = string(enums.KnowledgeBaseTypeFAQ)
	}
	item := repositories.KnowledgeBaseRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("name", name).
		Eq("knowledge_type", knowledgeType).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).
		NotEq("status", enums.StatusDeleted))
	if item != nil {
		return item, nil
	}
	kb := &models.KnowledgeBase{
		TenantID:              tenantID,
		Name:                  name,
		KnowledgeType:         knowledgeType,
		AccessScope:           string(enums.KnowledgeBaseAccessScopeTenant),
		Status:                enums.StatusOk,
		DefaultTopK:           10,
		DefaultScoreThreshold: 0.5,
		DefaultRerankLimit:    5,
		ChunkProvider:         "structured",
		ChunkTargetTokens:     300,
		ChunkMaxTokens:        400,
		ChunkOverlapTokens:    40,
		AnswerMode:            1,
		AuditFields:           utils.BuildAuditFields(operator),
	}
	if err := repositories.KnowledgeBaseRepository.Create(sqls.DB(), kb); err != nil {
		return nil, err
	}
	return kb, nil
}

func (s *enterpriseKnowledgeService) resolveMutationKnowledgeBase(
	tenantID, knowledgeBaseID int64,
	category, entryType string,
	operator *dto.AuthPrincipal,
) (*models.KnowledgeBase, error) {
	if knowledgeBaseID <= 0 {
		return s.ensureKnowledgeBase(tenantID, category, entryType, operator)
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if kb == nil || kb.TenantID != tenantID || kb.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("knowledge base not found for this tenant")
	}
	if entryType == "faq" && kb.KnowledgeType != string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParam("FAQ entry requires a FAQ knowledge base")
	}
	if entryType != "faq" && kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParam("document entry requires a document knowledge base")
	}
	return kb, nil
}

func (s *enterpriseKnowledgeService) relatedProducts(tenantID, knowledgeBaseID, entryID int64) []dto.EnterpriseKnowledgeRelatedProductDTO {
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", entryID).
		NotEq("status", enums.StatusDeleted).
		Asc("sort_no").
		Asc("id"))
	if len(links) == 0 {
		links = repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("knowledge_base_id", knowledgeBaseID).
			Eq("knowledge_entry_id", 0).
			NotEq("status", enums.StatusDeleted).
			Asc("sort_no").
			Asc("id"))
	}
	result := make([]dto.EnterpriseKnowledgeRelatedProductDTO, 0, len(links))
	seen := make(map[int64]struct{}, len(links))
	for _, link := range links {
		if link.ProductID <= 0 {
			continue
		}
		if _, ok := seen[link.ProductID]; ok {
			continue
		}
		seen[link.ProductID] = struct{}{}
		product := repositories.ProductRepository.GetByTenant(sqls.DB(), link.ProductID, tenantID)
		if product == nil || product.Status == enums.StatusDeleted {
			continue
		}
		result = append(result, dto.EnterpriseKnowledgeRelatedProductDTO{
			ID:   product.ID,
			Name: product.Name,
			Code: product.Code,
		})
	}
	return result
}

func (s *enterpriseKnowledgeService) syncRelatedProducts(tenantID, knowledgeBaseID, entryID int64, entryType, language, visibility, reviewStatus string, productIDs []int64, operator *dto.AuthPrincipal) error {
	if err := s.syncRelatedProductsDB(sqls.DB(), tenantID, knowledgeBaseID, entryID, entryType, language, visibility, reviewStatus, productIDs, operator); err != nil {
		return err
	}
	if strings.TrimSpace(reviewStatus) == "published" {
		s.enqueueEntryIndexSync(tenantID, knowledgeBaseID, entryType, entryID, reviewStatus, "scope_refresh", operator)
	}
	return nil
}

func (s *enterpriseKnowledgeService) syncRelatedProductsDB(db *gorm.DB, tenantID, knowledgeBaseID, entryID int64, entryType, language, visibility, reviewStatus string, productIDs []int64, operator *dto.AuthPrincipal) error {
	selected := make(map[int64]struct{})
	for _, id := range productIDs {
		if id <= 0 {
			continue
		}
		product := repositories.ProductRepository.GetByTenant(db, id, tenantID)
		if product == nil || product.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("related product not found")
		}
		selected[id] = struct{}{}
	}
	existing := repositories.ProductKnowledgeLinkRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", entryID).
		NotEq("status", enums.StatusDeleted))
	now := time.Now()
	for _, link := range existing {
		if _, keep := selected[link.ProductID]; keep {
			delete(selected, link.ProductID)
			updates := map[string]any{
				"link_type":        entryType,
				"language":         language,
				"publish_status":   reviewStatus,
				"status":           enums.StatusOk,
				"update_user_id":   operatorUserID(operator),
				"update_user_name": operatorUsername(operator),
				"updated_at":       now,
			}
			if strings.TrimSpace(visibility) != "" {
				updates["visibility"] = normalizeVisibility(visibility)
			}
			if err := repositories.ProductKnowledgeLinkRepository.Updates(db, link.ID, updates); err != nil {
				return err
			}
			continue
		}
		if err := repositories.ProductKnowledgeLinkRepository.Updates(db, link.ID, map[string]any{
			"status":           enums.StatusDeleted,
			"update_user_id":   operatorUserID(operator),
			"update_user_name": operatorUsername(operator),
			"updated_at":       now,
		}); err != nil {
			return err
		}
	}
	for productID := range selected {
		link := &models.ProductKnowledgeLink{
			TenantID:         tenantID,
			ProductID:        productID,
			KnowledgeBaseID:  knowledgeBaseID,
			KnowledgeEntryID: entryID,
			LinkType:         entryType,
			Language:         language,
			Visibility:       normalizeVisibility(visibility),
			PublishStatus:    reviewStatus,
			Status:           enums.StatusOk,
			AuditFields:      utils.BuildAuditFields(operator),
		}
		if err := repositories.ProductKnowledgeLinkRepository.Create(db, link); err != nil {
			return err
		}
	}
	return nil
}

func (s *enterpriseKnowledgeService) deactivateEntryLinks(tenantID, knowledgeBaseID, entryID int64) error {
	return s.deactivateEntryLinksDB(sqls.DB(), tenantID, knowledgeBaseID, entryID)
}

func (s *enterpriseKnowledgeService) deactivateEntryLinksDB(db *gorm.DB, tenantID, knowledgeBaseID, entryID int64) error {
	links := repositories.ProductKnowledgeLinkRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", entryID).
		NotEq("status", enums.StatusDeleted))
	for _, link := range links {
		if err := repositories.ProductKnowledgeLinkRepository.Updates(db, link.ID, map[string]any{
			"status":     enums.StatusDeleted,
			"updated_at": time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *enterpriseKnowledgeService) syncEntryLinkPublishStatus(tenantID, knowledgeBaseID, entryID int64, reviewStatus string) error {
	return s.syncEntryLinkPublishStatusTx(sqls.DB(), tenantID, knowledgeBaseID, entryID, reviewStatus)
}

func (s *enterpriseKnowledgeService) syncEntryLinkPublishStatusTx(db *gorm.DB, tenantID, knowledgeBaseID, entryID int64, reviewStatus string) error {
	links := repositories.ProductKnowledgeLinkRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", entryID).
		NotEq("status", enums.StatusDeleted))
	for _, link := range links {
		if err := repositories.ProductKnowledgeLinkRepository.Updates(db, link.ID, map[string]any{
			"publish_status": reviewStatus,
			"updated_at":     time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func knowledgeStatus(status enums.Status, indexStatus string, reviewStatus string) string {
	switch strings.TrimSpace(reviewStatus) {
	case "review", "published", "deprecated":
		return strings.TrimSpace(reviewStatus)
	case "draft":
		if status != enums.StatusOk {
			return "draft"
		}
	}
	if status == enums.StatusDeleted {
		return "deprecated"
	}
	if status == enums.StatusDisabled {
		return "draft"
	}
	if status != enums.StatusOk {
		return "draft"
	}
	if indexStatus == string(enums.KnowledgeDocumentIndexStatusIndexed) {
		return "published"
	}
	if indexStatus == string(enums.KnowledgeDocumentIndexStatusPending) {
		return "review"
	}
	return "draft"
}

func knowledgeRelevanceScore(status string) int {
	if status == "published" {
		return 100
	}
	return 0
}

func encodeEnterpriseKnowledgeID(entryType string, actualID int64) int64 {
	if entryType == "faq" {
		return actualID*10 + 2
	}
	return actualID*10 + 1
}

func decodeEnterpriseKnowledgeID(encodedID int64) (string, int64, error) {
	if encodedID <= 0 {
		return "", 0, errorsx.InvalidParam("invalid knowledge entry id")
	}
	switch encodedID % 10 {
	case 1:
		return "document", encodedID / 10, nil
	case 2:
		return "faq", encodedID / 10, nil
	default:
		return "", 0, errorsx.InvalidParam("invalid knowledge entry id")
	}
}

func normalizeKnowledgeEntryType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "faq":
		return "faq"
	default:
		return "document"
	}
}

func normalizeKnowledgeLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	return value
}

func knowledgeStringSlice(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return sanitizeKnowledgeStrings(values)
}

func knowledgeStringJSON(values []string) string {
	data, err := json.Marshal(sanitizeKnowledgeStrings(values))
	if err != nil {
		return "[]"
	}
	return string(data)
}

func sanitizeKnowledgeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func knowledgeContentHash(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func knowledgeWorkflowColumns(next string) (string, enums.Status, enums.KnowledgeDocumentIndexStatus, error) {
	switch next {
	case "draft":
		return "draft", enums.StatusDisabled, enums.KnowledgeDocumentIndexStatusPending, nil
	case "review":
		return "review", enums.StatusOk, enums.KnowledgeDocumentIndexStatusPending, nil
	case "published":
		return "published", enums.StatusOk, enums.KnowledgeDocumentIndexStatusPending, nil
	case "deprecated":
		return "deprecated", enums.StatusDisabled, enums.KnowledgeDocumentIndexStatusPending, nil
	default:
		return "", enums.StatusDisabled, "", errorsx.InvalidParam("invalid knowledge status")
	}
}

func knowledgeCompletenessScore(title, content, category string, tags, faultCodes []string, relatedProducts []dto.EnterpriseKnowledgeRelatedProductDTO) int {
	score := 0
	if strings.TrimSpace(title) != "" {
		score += 20
	}
	if len(strings.TrimSpace(content)) >= 40 {
		score += 40
	} else if strings.TrimSpace(content) != "" {
		score += 20
	}
	if strings.TrimSpace(category) != "" {
		score += 10
	}
	if len(tags) > 0 {
		score += 10
	}
	if len(faultCodes) > 0 {
		score += 10
	}
	if len(relatedProducts) > 0 {
		score += 10
	}
	if score > 100 {
		return 100
	}
	return score
}

func publishedAt(status string, indexedAt *time.Time) string {
	if status != "published" {
		return ""
	}
	return formatEnterpriseTimePtr(indexedAt)
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func operatorUserID(operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return 0
	}
	return operator.UserID
}

func operatorUsername(operator *dto.AuthPrincipal) string {
	if operator == nil {
		return "enterprise-api"
	}
	return firstNonBlank(operator.Username, "enterprise-api")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
