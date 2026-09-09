package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestLegacyKnowledgeEditorsReturnContentToGovernedDraft(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{}, &models.KnowledgeBase{}, &models.KnowledgeDocument{}, &models.KnowledgeFAQ{},
		&models.KnowledgeRevision{}, &models.ProductKnowledgeLink{}, &models.KnowledgeIndexGeneration{}, &models.KnowledgeIndexSyncTask{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	tenant := &models.Tenant{Name: "legacy-review-gate", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	documentKB := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Document KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	faqKB := &models.KnowledgeBase{TenantID: tenant.ID, Name: "FAQ KB", KnowledgeType: string(enums.KnowledgeBaseTypeFAQ), Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&[]*models.KnowledgeBase{documentKB, faqKB}).Error; err != nil {
		t.Fatalf("create knowledge bases: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 9, Username: "dashboard-editor"}
	createdDocument, err := KnowledgeDocumentService.CreateKnowledgeDocument(request.CreateKnowledgeDocumentRequest{
		KnowledgeBaseID: documentKB.ID, Title: "Controller thermal repair", ContentType: enums.KnowledgeDocumentContentTypeMarkdown,
		Content: "Check the cooling path and verify the controller temperature after repair.",
	}, operator)
	if err != nil {
		t.Fatalf("create legacy document: %v", err)
	}
	if createdDocument.ReviewStatus != "draft" || createdDocument.Status != enums.StatusDisabled {
		t.Fatalf("created legacy document = %+v, want governed draft", createdDocument)
	}
	createdFAQ, err := KnowledgeFAQService.CreateKnowledgeFAQ(request.CreateKnowledgeFAQRequest{
		KnowledgeBaseID: faqKB.ID, Question: "How should TEMP-001 be checked?", Answer: "Check the fan connector and verify temperature stability.",
	}, operator)
	if err != nil {
		t.Fatalf("create legacy faq: %v", err)
	}
	if createdFAQ.ReviewStatus != "draft" || createdFAQ.Status != enums.StatusDisabled {
		t.Fatalf("created legacy faq = %+v, want governed draft", createdFAQ)
	}

	documentRevision := &models.KnowledgeRevision{TenantID: tenant.ID, KnowledgeBaseID: documentKB.ID, EntryType: "document", EntryID: createdDocument.ID, VersionNo: 1, ReviewStatus: "published", ContentHash: createdDocument.ContentHash, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	faqRevision := &models.KnowledgeRevision{TenantID: tenant.ID, KnowledgeBaseID: faqKB.ID, EntryType: "faq", EntryID: createdFAQ.ID, VersionNo: 1, ReviewStatus: "published", ContentHash: knowledgeContentHash(createdFAQ.Question + "\n" + createdFAQ.Answer), AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(documentRevision).Error; err != nil {
		t.Fatalf("create document revision: %v", err)
	}
	if err := db.Create(faqRevision).Error; err != nil {
		t.Fatalf("create faq revision: %v", err)
	}
	if err := db.Model(createdDocument).Updates(map[string]any{"review_status": "published", "status": enums.StatusOk, "published_revision_id": documentRevision.ID, "current_revision_id": documentRevision.ID, "index_status": enums.KnowledgeDocumentIndexStatusIndexed}).Error; err != nil {
		t.Fatalf("publish document fixture: %v", err)
	}
	if err := db.Model(createdFAQ).Updates(map[string]any{"review_status": "published", "status": enums.StatusOk, "published_revision_id": faqRevision.ID, "current_revision_id": faqRevision.ID, "index_status": enums.KnowledgeDocumentIndexStatusIndexed}).Error; err != nil {
		t.Fatalf("publish faq fixture: %v", err)
	}
	documentLink := &models.ProductKnowledgeLink{TenantID: tenant.ID, ProductID: 10, KnowledgeBaseID: documentKB.ID, KnowledgeEntryID: createdDocument.ID, LinkType: "document", PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	faqLink := &models.ProductKnowledgeLink{TenantID: tenant.ID, ProductID: 10, KnowledgeBaseID: faqKB.ID, KnowledgeEntryID: createdFAQ.ID, LinkType: "faq", PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&[]*models.ProductKnowledgeLink{documentLink, faqLink}).Error; err != nil {
		t.Fatalf("create links: %v", err)
	}
	if err := KnowledgeDocumentService.UpdateKnowledgeDocument(request.UpdateKnowledgeDocumentRequest{
		ID: createdDocument.ID, CreateKnowledgeDocumentRequest: request.CreateKnowledgeDocumentRequest{
			KnowledgeBaseID: documentKB.ID, Title: "Updated controller thermal repair", ContentType: enums.KnowledgeDocumentContentTypeMarkdown,
			Content: "Check and secure the cooling connector, then verify stable temperature for thirty minutes.",
		},
	}, operator); err != nil {
		t.Fatalf("update legacy document: %v", err)
	}
	if err := KnowledgeFAQService.UpdateKnowledgeFAQ(request.UpdateKnowledgeFAQRequest{
		ID: createdFAQ.ID, CreateKnowledgeFAQRequest: request.CreateKnowledgeFAQRequest{
			KnowledgeBaseID: faqKB.ID, Question: "How should TEMP-001 be repaired?", Answer: "Secure the fan connector and verify temperature for thirty minutes.",
		},
	}, operator); err != nil {
		t.Fatalf("update legacy faq: %v", err)
	}
	if err := db.First(createdDocument, createdDocument.ID).Error; err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if err := db.First(createdFAQ, createdFAQ.ID).Error; err != nil {
		t.Fatalf("reload faq: %v", err)
	}
	if createdDocument.ReviewStatus != "draft" || createdDocument.Status != enums.StatusDisabled || createdDocument.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("updated legacy document = %+v", createdDocument)
	}
	if createdFAQ.ReviewStatus != "draft" || createdFAQ.Status != enums.StatusDisabled || createdFAQ.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("updated legacy faq = %+v", createdFAQ)
	}
	if err := db.First(documentLink, documentLink.ID).Error; err != nil {
		t.Fatalf("reload document link: %v", err)
	}
	if err := db.First(faqLink, faqLink.ID).Error; err != nil {
		t.Fatalf("reload faq link: %v", err)
	}
	if documentLink.PublishStatus != "draft" || faqLink.PublishStatus != "draft" {
		t.Fatalf("legacy edit link statuses = document:%q faq:%q", documentLink.PublishStatus, faqLink.PublishStatus)
	}
}
