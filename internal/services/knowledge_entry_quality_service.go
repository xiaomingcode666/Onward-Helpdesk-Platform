package services

import (
	"strings"
	"unicode/utf8"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type knowledgeEntryAssessment struct {
	QualityScore   int
	ValueScore     int
	EntryScore     int
	ReviewEligible bool
	QualityFlags   []string
}

func assessKnowledgeDocument(db *gorm.DB, item *models.KnowledgeDocument) knowledgeEntryAssessment {
	if item == nil {
		return knowledgeEntryAssessment{}
	}
	if item.SourceType == "knowledge_candidate" && item.SourceReferenceID > 0 {
		if candidate := loadKnowledgeEntrySourceCandidate(db, item.TenantID, item.SourceReferenceID); candidate != nil {
			flags := knowledgeStringSlice(candidate.QualityFlagsJSON)
			contentMatchesCandidate := knowledgeCandidateApprovalContentMatches(candidate, item.Title, item.Content)
			if !contentMatchesCandidate {
				flags = append(flags, "candidate_source_content_mismatch")
			}
			return knowledgeEntryAssessment{
				QualityScore: candidate.QualityScore,
				ValueScore:   candidate.ValueScore,
				EntryScore:   candidate.CandidateScore,
				ReviewEligible: candidate.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusApproved) &&
					candidate.QualityScore >= enums.KnowledgeCandidateMinimumQualityScore &&
					candidate.ValueScore >= enums.KnowledgeCandidateMinimumValueScore &&
					!candidate.RequiresReassessment && contentMatchesCandidate,
				QualityFlags: flags,
			}
		}
	}
	checkDuplicate := item.ReviewStatus == "" || item.ReviewStatus == "draft" || item.ReviewStatus == "review"
	return assessManualKnowledgeEntry(db, item.TenantID, item.KnowledgeBaseID, "document", item.ID, item.Title, item.Content, item.TagsJSON, item.FaultCodesJSON, checkDuplicate)
}

func assessKnowledgeFAQ(db *gorm.DB, item *models.KnowledgeFAQ) knowledgeEntryAssessment {
	if item == nil {
		return knowledgeEntryAssessment{}
	}
	checkDuplicate := item.ReviewStatus == "" || item.ReviewStatus == "draft" || item.ReviewStatus == "review"
	return assessManualKnowledgeEntry(db, item.TenantID, item.KnowledgeBaseID, "faq", item.ID, item.Question, item.Answer, item.TagsJSON, item.FaultCodesJSON, checkDuplicate)
}

func assessManualKnowledgeEntry(db *gorm.DB, tenantID, knowledgeBaseID int64, entryType string, entryID int64, title, content, tagsJSON, faultCodesJSON string, checkDuplicate bool) knowledgeEntryAssessment {
	result := knowledgeEntryAssessment{QualityFlags: make([]string, 0, 5)}
	titleLength := utf8.RuneCountInString(strings.TrimSpace(title))
	contentLength := utf8.RuneCountInString(strings.TrimSpace(content))
	if titleLength >= 6 {
		result.QualityScore += 15
	} else if titleLength >= 3 {
		result.QualityScore += 8
	} else {
		result.QualityFlags = append(result.QualityFlags, "title_too_short")
	}
	switch {
	case contentLength >= 120:
		result.QualityScore += 45
	case contentLength >= 60:
		result.QualityScore += 35
	case contentLength >= 30:
		result.QualityScore += 20
	default:
		result.QualityScore += 5
		result.QualityFlags = append(result.QualityFlags, "content_too_short")
	}
	if knowledgeEntryHasStructure(content) {
		result.QualityScore += 15
	} else {
		result.QualityFlags = append(result.QualityFlags, "missing_structure")
	}
	if knowledgeEntryHasActionableContent(content) {
		result.QualityScore += 15
	} else {
		result.QualityFlags = append(result.QualityFlags, "missing_actionable_steps")
	}
	tags := knowledgeStringSlice(tagsJSON)
	faultCodes := knowledgeStringSlice(faultCodesJSON)
	if len(tags) > 0 || len(faultCodes) > 0 {
		result.QualityScore += 10
	} else {
		result.QualityFlags = append(result.QualityFlags, "missing_context_tags")
	}
	tenantSharedScope := knowledgeEntryUsesTenantSharedKnowledgeBase(db, tenantID, knowledgeBaseID)
	productCount := int64(0)
	if db != nil && db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
		_ = db.Model(&models.ProductKnowledgeLink{}).
			Where("tenant_id = ? AND knowledge_base_id = ? AND knowledge_entry_id = ? AND status <> ?", tenantID, knowledgeBaseID, entryID, enums.StatusDeleted).
			Count(&productCount).Error
	}
	if productCount > 0 || tenantSharedScope {
		result.ValueScore += 25
	} else {
		result.QualityFlags = append(result.QualityFlags, "missing_product_scope")
	}
	if len(faultCodes) > 0 {
		result.ValueScore += 25
	}
	if len(tags) > 0 {
		result.ValueScore += 10
	}
	if contentLength >= 120 {
		result.ValueScore += 25
	} else if contentLength >= 60 {
		result.ValueScore += 15
	}
	if titleLength >= 6 {
		result.ValueScore += 15
	}
	duplicatePublishedEntry := checkDuplicate && knowledgeEntryHasPublishedEquivalent(db, tenantID, knowledgeBaseID, entryType, entryID, title, content, faultCodes)
	if duplicatePublishedEntry {
		result.ValueScore -= 40
		result.QualityFlags = append(result.QualityFlags, "duplicate_published_entry")
	}
	result.QualityScore = clampScore(result.QualityScore)
	result.ValueScore = clampScore(result.ValueScore)
	result.EntryScore = (result.QualityScore*65 + result.ValueScore*35 + 50) / 100
	result.ReviewEligible = result.QualityScore >= enums.KnowledgeEntryMinimumQualityScore &&
		result.ValueScore >= enums.KnowledgeEntryMinimumValueScore &&
		!duplicatePublishedEntry &&
		!containsString(result.QualityFlags, "missing_product_scope") &&
		!containsString(result.QualityFlags, "missing_actionable_steps")
	return result
}

func knowledgeEntryUsesTenantSharedKnowledgeBase(db *gorm.DB, tenantID, knowledgeBaseID int64) bool {
	if db == nil || knowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeBase{}) {
		return false
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, knowledgeBaseID)
	return knowledgeBase != nil &&
		knowledgeBase.TenantID == tenantID &&
		knowledgeBase.Status == enums.StatusOk &&
		knowledgeBase.AccessScope == string(enums.KnowledgeBaseAccessScopeTenant)
}

func knowledgeEntryHasPublishedEquivalent(db *gorm.DB, tenantID, knowledgeBaseID int64, entryType string, entryID int64, title, content string, faultCodes []string) bool {
	if db == nil {
		return false
	}
	candidateText := strings.TrimSpace(title + " " + content)
	faultCodeSet := make(map[string]struct{}, len(faultCodes))
	for _, faultCode := range faultCodes {
		if normalized := normalizeKnowledgeCandidateIdentity(faultCode); normalized != "" {
			faultCodeSet[normalized] = struct{}{}
		}
	}
	if db.Migrator().HasTable(&models.KnowledgeDocument{}) {
		lastID := int64(0)
		for {
			var documents []models.KnowledgeDocument
			query := db.Where("tenant_id = ? AND knowledge_base_id = ? AND status = ? AND review_status = ? AND published_revision_id > 0", tenantID, knowledgeBaseID, enums.StatusOk, "published")
			if entryType == "document" && entryID > 0 {
				query = query.Where("id <> ?", entryID)
			}
			if lastID > 0 {
				query = query.Where("id < ?", lastID)
			}
			if err := query.Order("id DESC").Limit(500).Find(&documents).Error; err != nil {
				return true
			}
			for i := range documents {
				if knowledgeEntryTextsEquivalent(candidateText, documents[i].Title+" "+documents[i].Content, faultCodeSet, documents[i].FaultCodesJSON) {
					return true
				}
			}
			if len(documents) < 500 {
				break
			}
			lastID = documents[len(documents)-1].ID
		}
	}
	if db.Migrator().HasTable(&models.KnowledgeFAQ{}) {
		lastID := int64(0)
		for {
			var faqs []models.KnowledgeFAQ
			query := db.Where("tenant_id = ? AND knowledge_base_id = ? AND status = ? AND review_status = ? AND published_revision_id > 0", tenantID, knowledgeBaseID, enums.StatusOk, "published")
			if entryType == "faq" && entryID > 0 {
				query = query.Where("id <> ?", entryID)
			}
			if lastID > 0 {
				query = query.Where("id < ?", lastID)
			}
			if err := query.Order("id DESC").Limit(500).Find(&faqs).Error; err != nil {
				return true
			}
			for i := range faqs {
				if knowledgeEntryTextsEquivalent(candidateText, faqs[i].Question+" "+faqs[i].Answer, faultCodeSet, faqs[i].FaultCodesJSON) {
					return true
				}
			}
			if len(faqs) < 500 {
				break
			}
			lastID = faqs[len(faqs)-1].ID
		}
	}
	return false
}

func knowledgeEntryTextsEquivalent(left, right string, faultCodes map[string]struct{}, otherFaultCodesJSON string) bool {
	similarity := knowledgeCandidateTextSimilarity(left, right)
	if similarity >= 0.82 {
		return true
	}
	if similarity < 0.45 || len(faultCodes) == 0 {
		return false
	}
	for faultCode := range faultCodes {
		if knowledgeFaultCodeJSONContains(otherFaultCodesJSON, faultCode) {
			return true
		}
	}
	return false
}

func loadKnowledgeEntrySourceCandidate(db *gorm.DB, tenantID, candidateID int64) *models.KnowledgeCandidate {
	if db == nil {
		db = sqls.DB()
	}
	if db == nil || candidateID <= 0 || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	var candidate models.KnowledgeCandidate
	if result := db.Where("id = ? AND tenant_id = ? AND status <> ?", candidateID, tenantID, enums.StatusDeleted).First(&candidate); result.Error != nil {
		return nil
	}
	return &candidate
}

func knowledgeEntryHasStructure(content string) bool {
	value := strings.ToLower(content)
	return strings.Count(value, "\n") >= 2 || strings.Contains(value, "1.") || strings.Contains(value, "- ") ||
		strings.Contains(value, "步骤") || strings.Contains(value, "根因") || strings.Contains(value, "solution") || strings.Contains(value, "cause")
}

func knowledgeEntryHasActionableContent(content string) bool {
	value := strings.ToLower(content)
	for _, keyword := range []string{"检查", "确认", "更换", "清理", "重新", "执行", "验证", "复测", "check", "replace", "verify", "restart", "inspect"} {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}
