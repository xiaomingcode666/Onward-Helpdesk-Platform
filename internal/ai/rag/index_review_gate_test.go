package rag

import (
	"context"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestKnowledgeIndexRejectsUnpublishedEntriesBeforeVectorization(t *testing.T) {
	knowledgeBase := models.KnowledgeBase{ID: 10, TenantID: 20, KnowledgeType: string(enums.KnowledgeBaseTypeDocument)}
	document := models.KnowledgeDocument{
		ID: 30, TenantID: 20, KnowledgeBaseID: 10, ReviewStatus: "draft",
		Status: enums.StatusDisabled, PublishedRevisionID: 0,
	}
	if err := Index.IndexDocumentToTarget(context.Background(), document, knowledgeBase, IndexTarget{}); err == nil || !strings.Contains(err.Error(), "not a published revision") {
		t.Fatalf("draft document index error = %v", err)
	}

	faqBase := models.KnowledgeBase{ID: 11, TenantID: 20, KnowledgeType: string(enums.KnowledgeBaseTypeFAQ)}
	faq := models.KnowledgeFAQ{
		ID: 31, TenantID: 20, KnowledgeBaseID: 11, ReviewStatus: "review",
		Status: enums.StatusDisabled, PublishedRevisionID: 0,
	}
	if err := Index.IndexFAQToTarget(context.Background(), faq, faqBase, IndexTarget{}); err == nil || !strings.Contains(err.Error(), "not a published revision") {
		t.Fatalf("review faq index error = %v", err)
	}
}

func TestKnowledgeIndexRejectsPublishedLabelWithoutRevision(t *testing.T) {
	knowledgeBase := models.KnowledgeBase{ID: 10, TenantID: 20, KnowledgeType: string(enums.KnowledgeBaseTypeDocument)}
	document := models.KnowledgeDocument{
		ID: 30, TenantID: 20, KnowledgeBaseID: 10, ReviewStatus: "published",
		Status: enums.StatusOk, PublishedRevisionID: 0,
	}
	if err := Index.IndexDocumentToTarget(context.Background(), document, knowledgeBase, IndexTarget{}); err == nil || !strings.Contains(err.Error(), "not a published revision") {
		t.Fatalf("revisionless document index error = %v", err)
	}
}
