package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureKnowledgeCandidateStateRecoveryDeduplicatesCandidateDocumentsAndLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeCandidate{}, &models.KnowledgeDocument{}, &models.ProductKnowledgeLink{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	candidate := &models.KnowledgeCandidate{
		TenantID: 1, TicketID: 10, Title: "candidate", ReviewStatus: "approved", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	documents := []models.KnowledgeDocument{
		{TenantID: 1, KnowledgeBaseID: 20, Title: "draft", SourceType: "knowledge_entry", SourceReferenceID: candidate.ID, ReviewStatus: "draft", Status: enums.StatusDisabled, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, KnowledgeBaseID: 20, Title: "published", SourceType: "knowledge_entry", SourceReferenceID: candidate.ID, ReviewStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, KnowledgeBaseID: 20, Title: "unrelated legacy entry", SourceType: "knowledge_entry", SourceReferenceID: 999, ReviewStatus: "draft", Status: enums.StatusDisabled, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&documents).Error; err != nil {
		t.Fatalf("create documents: %v", err)
	}
	links := []models.ProductKnowledgeLink{
		{TenantID: 1, ProductID: 30, ProductModelID: 40, KnowledgeBaseID: 20, KnowledgeEntryID: documents[1].ID, PublishStatus: "draft", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, ProductID: 30, ProductModelID: 40, KnowledgeBaseID: 20, KnowledgeEntryID: documents[1].ID, PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatalf("create links: %v", err)
	}

	if err := ensureKnowledgeCandidateStateRecoveryColumns(db); err != nil {
		t.Fatalf("run state recovery migration: %v", err)
	}
	if err := db.First(candidate, candidate.ID).Error; err != nil {
		t.Fatalf("reload backfilled candidate: %v", err)
	}
	if candidate.ScoreVersion != enums.KnowledgeCandidateScoreVersion {
		t.Fatalf("candidate score version = %q, want %q", candidate.ScoreVersion, enums.KnowledgeCandidateScoreVersion)
	}
	if candidate.KnowledgeEntryID != documents[1].ID*10+1 || candidate.KnowledgeBaseID != documents[1].KnowledgeBaseID {
		t.Fatalf("candidate knowledge pointer = base:%d entry:%d, want canonical document", candidate.KnowledgeBaseID, candidate.KnowledgeEntryID)
	}
	var activeCandidateDocuments []models.KnowledgeDocument
	if err := db.Where("tenant_id = ? AND source_type = ? AND source_reference_id = ? AND status <> ?", 1, "knowledge_candidate", candidate.ID, enums.StatusDeleted).
		Find(&activeCandidateDocuments).Error; err != nil {
		t.Fatalf("list active candidate documents: %v", err)
	}
	if len(activeCandidateDocuments) != 1 || activeCandidateDocuments[0].Title != "published" {
		t.Fatalf("active candidate documents = %+v, want the published document", activeCandidateDocuments)
	}
	var duplicateDocument models.KnowledgeDocument
	if err := db.First(&duplicateDocument, documents[0].ID).Error; err != nil {
		t.Fatalf("load duplicate document: %v", err)
	}
	if duplicateDocument.SourceType != "knowledge_candidate_duplicate" || duplicateDocument.ReviewStatus != "deprecated" || duplicateDocument.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("duplicate document cleanup state = %+v", duplicateDocument)
	}
	var unrelated models.KnowledgeDocument
	if err := db.First(&unrelated, documents[2].ID).Error; err != nil {
		t.Fatalf("load unrelated legacy document: %v", err)
	}
	if unrelated.SourceType != "knowledge_entry" || unrelated.Status == enums.StatusDeleted {
		t.Fatalf("unrelated legacy document was incorrectly normalized: %+v", unrelated)
	}
	conflictingDocument := &models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 20, Title: "conflict", SourceType: "knowledge_candidate", SourceReferenceID: candidate.ID,
		ReviewStatus: "draft", Status: enums.StatusDisabled, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conflictingDocument).Error; err == nil {
		t.Fatal("candidate source unique index accepted a duplicate active document")
	}
	var activeLinks []models.ProductKnowledgeLink
	if err := db.Where("tenant_id = ? AND product_id = ? AND product_model_id = ? AND knowledge_base_id = ? AND knowledge_entry_id = ? AND status <> ?",
		1, 30, 40, 20, documents[1].ID, enums.StatusDeleted).Find(&activeLinks).Error; err != nil {
		t.Fatalf("list active links: %v", err)
	}
	if len(activeLinks) != 1 || activeLinks[0].PublishStatus != "published" {
		t.Fatalf("active product links = %+v, want one published link", activeLinks)
	}
	conflictingLink := &models.ProductKnowledgeLink{
		TenantID: 1, ProductID: 30, ProductModelID: 40, KnowledgeBaseID: 20, KnowledgeEntryID: documents[1].ID,
		PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conflictingLink).Error; err == nil {
		t.Fatal("active product knowledge link unique index accepted a duplicate")
	}
	localizedLink := &models.ProductKnowledgeLink{
		TenantID: 1, ProductID: 30, ProductModelID: 40, KnowledgeBaseID: 20, KnowledgeEntryID: documents[1].ID,
		LinkType: "knowledge_article", Language: "en-US", Version: "v1", PublishStatus: "published", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(localizedLink).Error; err != nil {
		t.Fatalf("unique index rejected a distinct language/version link: %v", err)
	}
	if err := ensureKnowledgeCandidateStateRecoveryColumns(db); err != nil {
		t.Fatalf("state recovery migration should be idempotent: %v", err)
	}
}
