package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupEnterpriseKnowledgeTestDB(t *testing.T) (*gorm.DB, int64) {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.Product{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeRevision{},
		&models.ProductKnowledgeLink{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	tenant := &models.Tenant{
		Name:   "Knowledge Tenant",
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	return db, tenant.ID
}

func TestEnterpriseKnowledgePublishCreatesReusableRevision(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	knowledgeBase := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Service KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(knowledgeBase).Error; err != nil {
		t.Fatalf("create knowledge base error = %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: knowledgeBase.ID, Title: "Reset controller after E42 thermal shutdown",
		Content:      "Root cause: the controller protection state remains latched after an E42 thermal shutdown.\n\nSteps:\n1. Check that the cooling path is clear and confirm the temperature is below 45 C.\n2. Disconnect power for 30 seconds, reconnect it, and verify stable operation for another 30 minutes.",
		ReviewStatus: "draft", Language: "en", Status: enums.StatusDisabled,
		TagsJSON: `["controller","thermal"]`, FaultCodesJSON: `["E42"]`,
		IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	document.ContentHash = knowledgeContentHash(document.Content)
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document error = %v", err)
	}

	product := &models.Product{TenantID: tenantID, Code: "CTRL-E42", Name: "Controller", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeLink{
		TenantID: tenantID, ProductID: product.ID, KnowledgeBaseID: knowledgeBase.ID, KnowledgeEntryID: document.ID,
		LinkType: "document", Language: "en", PublishStatus: "draft", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product knowledge link error = %v", err)
	}

	encodedID := encodeEnterpriseKnowledgeID("document", document.ID)
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodedID, "published", nil); err == nil {
		t.Fatal("manual knowledge entry was published without review")
	}
	if err := db.Model(knowledgeBase).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatalf("disable knowledge base error = %v", err)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodedID, "review", nil); err == nil {
		t.Fatal("entry entered review while its knowledge base was disabled")
	}
	if err := db.Model(knowledgeBase).Update("status", enums.StatusOk).Error; err != nil {
		t.Fatalf("reactivate knowledge base error = %v", err)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodedID, "review", nil); err != nil {
		t.Fatalf("submit entry for review error = %v", err)
	}
	published, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodedID, "published", nil)
	if err != nil {
		t.Fatalf("UpdateEntryStatus() error = %v", err)
	}
	if published.Status != "published" {
		t.Fatalf("published status = %q, want published", published.Status)
	}
	reloaded := repositories.KnowledgeDocumentRepository.Get(db, document.ID)
	if reloaded == nil || reloaded.PublishedRevisionID <= 0 || reloaded.CurrentRevisionID != reloaded.PublishedRevisionID {
		t.Fatalf("published document revision ids = %#v", reloaded)
	}
	if reloaded.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("index status = %q, want pending", reloaded.IndexStatus)
	}
	revision := repositories.KnowledgeRevisionRepository.Get(db, reloaded.PublishedRevisionID)
	if revision == nil || revision.EntryID != document.ID || revision.Content != document.Content || revision.ReviewStatus != "published" {
		t.Fatalf("published revision = %#v", revision)
	}

	again, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodedID, "published", nil)
	if err != nil {
		t.Fatalf("second UpdateEntryStatus() error = %v", err)
	}
	if again == nil {
		t.Fatal("second publish returned nil entry")
	}
	var revisionCount int64
	if err := db.Model(&models.KnowledgeRevision{}).Where("entry_type = ? AND entry_id = ?", "document", document.ID).Count(&revisionCount).Error; err != nil {
		t.Fatalf("count revisions error = %v", err)
	}
	if revisionCount != 1 {
		t.Fatalf("revision count = %d, want 1 for idempotent republish", revisionCount)
	}
}

func TestEnterpriseKnowledgeVersionsReturnsStoredRevisionsAndCurrentDraft(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	kb := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Version KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Current draft title", Content: "Current draft content with updated repair steps.",
		ReviewStatus: "draft", Status: enums.StatusDisabled, Language: "en-US", IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	firstPublishedAt := now.Add(-2 * time.Hour)
	secondPublishedAt := now.Add(-time.Hour)
	revisions := []models.KnowledgeRevision{
		{TenantID: tenantID, KnowledgeBaseID: kb.ID, EntryType: "document", EntryID: document.ID, VersionNo: 1, VersionLabel: "v1", Title: "First title", Content: "First published content", Language: "en-US", ReviewStatus: "published", PublishedAt: &firstPublishedAt, PublishedByName: "reviewer-a", SupersededAt: &secondPublishedAt, AuditFields: models.AuditFields{CreatedAt: firstPublishedAt, UpdatedAt: firstPublishedAt}},
		{TenantID: tenantID, KnowledgeBaseID: kb.ID, EntryType: "document", EntryID: document.ID, VersionNo: 2, VersionLabel: "v2", Title: "Second title", Content: "Second published content", Language: "en-US", ReviewStatus: "published", PublishedAt: &secondPublishedAt, PublishedByName: "reviewer-b", AuditFields: models.AuditFields{CreatedAt: secondPublishedAt, UpdatedAt: secondPublishedAt}},
	}
	if err := db.Create(&revisions).Error; err != nil {
		t.Fatalf("create revisions: %v", err)
	}
	versions, err := EnterpriseKnowledgeService.Versions(tenantID, encodeEnterpriseKnowledgeID("document", document.ID))
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 3 || versions[0].Version != 3 || versions[0].ChangeNote != "Current draft (unpublished)" || versions[1].Version != 2 || versions[2].Version != 1 {
		t.Fatalf("knowledge versions = %+v", versions)
	}
	if versions[1].UpdatedBy != "reviewer-b" || versions[2].ChangeNote != "Superseded published revision" {
		t.Fatalf("revision audit history = %+v", versions)
	}
}

func TestEnterpriseKnowledgeReviewGateBlocksLowScoreAndPublishedDuplicate(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	kb := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Controller KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	low := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Tip", Content: "Restart it.",
		ReviewStatus: "draft", Status: enums.StatusDisabled, IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(low).Error; err != nil {
		t.Fatalf("create low-score document: %v", err)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodeEnterpriseKnowledgeID("document", low.ID), "review", nil); err == nil {
		t.Fatal("low-score manual entry entered review")
	}

	product := &models.Product{TenantID: tenantID, Code: "CTRL-1", Name: "Controller", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	content := "Root cause: the fan connector is loose and the controller overheats.\n\nSteps:\n1. Check the fan connector and cooling path.\n2. Reconnect the cable, clean the cooling path, and verify stable operation for 30 minutes."
	published := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Controller overheating repair", Content: content,
		ReviewStatus: "published", Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed, PublishedRevisionID: 1,
		TagsJSON: `["thermal"]`, FaultCodesJSON: `["TEMP-001"]`, ContentHash: knowledgeContentHash(content),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(published).Error; err != nil {
		t.Fatalf("create published document: %v", err)
	}
	duplicate := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Repair controller overheating", Content: content,
		ReviewStatus: "draft", Status: enums.StatusDisabled, IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		TagsJSON: `["thermal"]`, FaultCodesJSON: `["TEMP-001"]`, ContentHash: knowledgeContentHash(content),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(duplicate).Error; err != nil {
		t.Fatalf("create duplicate draft: %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeLink{
		TenantID: tenantID, ProductID: product.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: duplicate.ID,
		LinkType: "document", PublishStatus: "draft", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create duplicate product link: %v", err)
	}
	assessment := assessKnowledgeDocument(db, duplicate)
	if assessment.ReviewEligible || !containsString(assessment.QualityFlags, "duplicate_published_entry") {
		t.Fatalf("published duplicate assessment = %+v, want blocked duplicate", assessment)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodeEnterpriseKnowledgeID("document", duplicate.ID), "review", nil); err == nil {
		t.Fatal("published duplicate entered review")
	}
}

func TestEnterpriseKnowledgeReviewGateRequiresProductScopeAndActionableSteps(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	kb := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Repair KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		AccessScope: string(enums.KnowledgeBaseAccessScopeProduct), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	content := "Root cause: repeated thermal shutdown occurs because the cooling path accumulates debris and airflow declines over time.\n\nBackground:\nThis condition is typically visible after prolonged operation and produces a stable temperature alarm pattern across controller logs."
	document := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Controller thermal shutdown analysis", Content: content,
		ReviewStatus: "draft", Status: enums.StatusDisabled, IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		TagsJSON: `["thermal"]`, FaultCodesJSON: `["TEMP-001"]`, ContentHash: knowledgeContentHash(content),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	assessment := assessKnowledgeDocument(db, document)
	if assessment.ReviewEligible || !containsString(assessment.QualityFlags, "missing_product_scope") || !containsString(assessment.QualityFlags, "missing_actionable_steps") {
		t.Fatalf("unscoped non-actionable document assessment = %+v", assessment)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodeEnterpriseKnowledgeID("document", document.ID), "review", nil); err == nil {
		t.Fatal("unscoped non-actionable document entered review")
	}
}

func TestEnterpriseKnowledgeReviewGateAllowsTenantSharedKnowledgeWithoutProductScope(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	kb := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Tenant shared KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		AccessScope: string(enums.KnowledgeBaseAccessScopeTenant), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	content := "Scope: customers who do not know the exact equipment model can use this general after-sales checklist.\n\nActions:\n1. Keep the equipment powered off before inspection.\n2. Check cables, grounding, guards, ventilation and visible corrosion.\n3. If red lights, smoke, smell, leakage or overheating appear, stop immediately and do not repeatedly power on."
	document := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "General restart safety checklist", Content: content,
		ReviewStatus: "draft", Status: enums.StatusDisabled, IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		TagsJSON: `["general","safety"]`, ContentHash: knowledgeContentHash(content),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	assessment := assessKnowledgeDocument(db, document)
	if !assessment.ReviewEligible || containsString(assessment.QualityFlags, "missing_product_scope") {
		t.Fatalf("tenant shared document assessment = %+v, want review eligible without product scope", assessment)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, encodeEnterpriseKnowledgeID("document", document.ID), "review", nil); err != nil {
		t.Fatalf("tenant shared document should enter review: %v", err)
	}
}

func TestEnterpriseKnowledgeReviewGateFindsDuplicatesBeyondFirstBatch(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()
	kb := &models.KnowledgeBase{
		TenantID: tenantID, Name: "Large Repair KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	matchingContent := "Root cause: the fan connector is loose and blocks cooling.\n\nSteps:\n1. Check and secure the fan connector.\n2. Clean the cooling path and verify stable operation for thirty minutes."
	oldMatching := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Controller fan connector repair", Content: matchingContent,
		ReviewStatus: "published", Status: enums.StatusOk, PublishedRevisionID: 1, FaultCodesJSON: `["TEMP-001"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(oldMatching).Error; err != nil {
		t.Fatalf("create old matching document: %v", err)
	}
	noise := make([]models.KnowledgeDocument, 0, 500)
	for i := 0; i < 500; i++ {
		noise = append(noise, models.KnowledgeDocument{
			TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Unrelated network article " + strconv.Itoa(i),
			Content:      "Network configuration reference for an unrelated gateway model and protocol setting.",
			ReviewStatus: "published", Status: enums.StatusOk, PublishedRevisionID: 1,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		})
	}
	if err := db.CreateInBatches(&noise, 100).Error; err != nil {
		t.Fatalf("create unrelated documents: %v", err)
	}
	draft := &models.KnowledgeDocument{
		TenantID: tenantID, KnowledgeBaseID: kb.ID, Title: "Repair controller fan connector", Content: matchingContent,
		ReviewStatus: "draft", Status: enums.StatusDisabled, TagsJSON: `["thermal"]`, FaultCodesJSON: `["TEMP-001"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(draft).Error; err != nil {
		t.Fatalf("create duplicate draft: %v", err)
	}
	product := &models.Product{TenantID: tenantID, Code: "LARGE-KB", Name: "Controller", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeLink{
		TenantID: tenantID, ProductID: product.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: draft.ID,
		LinkType: "document", PublishStatus: "draft", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product link: %v", err)
	}
	assessment := assessKnowledgeDocument(db, draft)
	if assessment.ReviewEligible || !containsString(assessment.QualityFlags, "duplicate_published_entry") {
		t.Fatalf("old published duplicate was missed: %+v", assessment)
	}
}

func TestEnterpriseKnowledgeListEntriesFiltersByProductID(t *testing.T) {
	db, tenantID := setupEnterpriseKnowledgeTestDB(t)
	now := time.Now()

	productA := &models.Product{
		TenantID: tenantID,
		Code:     "HP-300",
		Name:     "Hydraulic Pump",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	productB := &models.Product{
		TenantID: tenantID,
		Code:     "HP-300-EU",
		Name:     "Hydraulic Pump EU",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	for _, product := range []*models.Product{productA, productB} {
		if err := db.Create(product).Error; err != nil {
			t.Fatalf("create product error = %v", err)
		}
	}

	kb := &models.KnowledgeBase{
		TenantID:      tenantID,
		Name:          "Hydraulic KB",
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		Status:        enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base error = %v", err)
	}

	doc := &models.KnowledgeDocument{
		TenantID:        tenantID,
		KnowledgeBaseID: kb.ID,
		Title:           "Hydraulic noise troubleshooting",
		ContentType:     enums.KnowledgeDocumentContentTypeMarkdown,
		Content:         "Check oil pressure and inspect the pump housing.",
		ReviewStatus:    "published",
		Language:        "en-US",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash:     "hash-1",
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := repositories.KnowledgeDocumentRepository.Create(db, doc); err != nil {
		t.Fatalf("create knowledge document error = %v", err)
	}

	link := &models.ProductKnowledgeLink{
		TenantID:         tenantID,
		ProductID:        productA.ID,
		KnowledgeBaseID:  kb.ID,
		KnowledgeEntryID: doc.ID,
		LinkType:         "document",
		Language:         "en-US",
		PublishStatus:    "published",
		Status:           enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(db, link); err != nil {
		t.Fatalf("create product knowledge link error = %v", err)
	}

	matched, err := EnterpriseKnowledgeService.ListEntries(tenantID, EnterpriseKnowledgeQuery{
		Page:      1,
		PageSize:  20,
		ProductID: productA.ID,
	})
	if err != nil {
		t.Fatalf("ListEntries(productA) error = %v", err)
	}
	if matched.Total != 1 || len(matched.Items) != 1 {
		t.Fatalf("ListEntries(productA) total=%d len=%d, want 1/1", matched.Total, len(matched.Items))
	}
	if len(matched.Items[0].ProductIDs) != 1 || matched.Items[0].ProductIDs[0] != productA.ID {
		t.Fatalf("ListEntries(productA) product IDs = %+v, want [%d]", matched.Items[0].ProductIDs, productA.ID)
	}

	unmatched, err := EnterpriseKnowledgeService.ListEntries(tenantID, EnterpriseKnowledgeQuery{
		Page:      1,
		PageSize:  20,
		ProductID: productB.ID,
	})
	if err != nil {
		t.Fatalf("ListEntries(productB) error = %v", err)
	}
	if unmatched.Total != 0 || len(unmatched.Items) != 0 {
		t.Fatalf("ListEntries(productB) total=%d len=%d, want 0/0", unmatched.Total, len(unmatched.Items))
	}
}
