package services

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

// KnowledgeEntryUsable is the single server-side gate for formal answers.
func KnowledgeEntryUsable(reviewStatus string, status enums.Status, publishedRevisionID int64, expiresAt *time.Time, now time.Time) bool {
	if strings.TrimSpace(reviewStatus) != "published" || status != enums.StatusOk || publishedRevisionID <= 0 {
		return false
	}
	return expiresAt == nil || expiresAt.After(now)
}

func KnowledgeDocumentUsable(doc *models.KnowledgeDocument, now time.Time) bool {
	return doc != nil && KnowledgeEntryUsable(doc.ReviewStatus, doc.Status, doc.PublishedRevisionID, doc.ExpiresAt, now)
}

func KnowledgeFAQUsable(faq *models.KnowledgeFAQ, now time.Time) bool {
	return faq != nil && KnowledgeEntryUsable(faq.ReviewStatus, faq.Status, faq.PublishedRevisionID, faq.ExpiresAt, now)
}
