package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestProductKnowledgeLinkServiceRejectsCrossTenantWrites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:product_knowledge_link_tenant_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.Product{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.ProductKnowledgeLink{},
	); err != nil {
		t.Fatal(err)
	}

	tenantA := &models.Tenant{Name: "Tenant A", Status: enums.StatusOk}
	tenantB := &models.Tenant{Name: "Tenant B", Status: enums.StatusOk}
	if err := db.Create(tenantA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(tenantB).Error; err != nil {
		t.Fatal(err)
	}
	productA := &models.Product{TenantID: tenantA.ID, Code: "A", Name: "Product A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenantB.ID, Code: "B", Name: "Product B", Status: enums.StatusOk}
	if err := repositories.ProductRepository.Create(db, productA); err != nil {
		t.Fatal(err)
	}
	if err := repositories.ProductRepository.Create(db, productB); err != nil {
		t.Fatal(err)
	}
	kbA := &models.KnowledgeBase{TenantID: tenantA.ID, Name: "KB A", KnowledgeType: "document", Status: enums.StatusOk}
	kbB := &models.KnowledgeBase{TenantID: tenantB.ID, Name: "KB B", KnowledgeType: "document", Status: enums.StatusOk}
	if err := db.Create(kbA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(kbB).Error; err != nil {
		t.Fatal(err)
	}
	docA := &models.KnowledgeDocument{TenantID: tenantA.ID, KnowledgeBaseID: kbA.ID, Title: "Doc A", ReviewStatus: "published", Status: enums.StatusOk}
	docB := &models.KnowledgeDocument{TenantID: tenantB.ID, KnowledgeBaseID: kbB.ID, Title: "Doc B", ReviewStatus: "published", Status: enums.StatusOk}
	if err := repositories.KnowledgeDocumentRepository.Create(db, docA); err != nil {
		t.Fatal(err)
	}
	if err := repositories.KnowledgeDocumentRepository.Create(db, docB); err != nil {
		t.Fatal(err)
	}
	link := &models.ProductKnowledgeLink{
		TenantID:         tenantA.ID,
		ProductID:        productA.ID,
		KnowledgeBaseID:  kbA.ID,
		KnowledgeEntryID: docA.ID,
		LinkType:         "document",
		Visibility:       "customer",
		PublishStatus:    "published",
		Status:           enums.StatusOk,
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(db, link); err != nil {
		t.Fatal(err)
	}

	operatorA := &dto.AuthPrincipal{TenantID: tenantA.ID, UserID: 1001, Username: "tenant-a-admin"}
	if _, err := ProductKnowledgeLinkService.CreateLink(CreateProductKnowledgeLinkRequest{
		TenantID:         tenantB.ID,
		ProductID:        productB.ID,
		KnowledgeBaseID:  kbB.ID,
		KnowledgeEntryID: docB.ID,
		LinkType:         "document",
	}, operatorA); err == nil {
		t.Fatal("CreateLink() error = nil, want cross-tenant operator rejection")
	}

	if err := ProductKnowledgeLinkService.UpdateLink(UpdateProductKnowledgeLinkRequest{
		ID:               link.ID,
		KnowledgeBaseID:  kbB.ID,
		KnowledgeEntryID: docB.ID,
		LinkType:         "document",
		Visibility:       "customer",
	}, operatorA); err == nil {
		t.Fatal("UpdateLink() error = nil, want cross-tenant knowledge entry rejection")
	}
	stored := ProductKnowledgeLinkService.Get(link.ID)
	if stored == nil || stored.KnowledgeBaseID != kbA.ID || stored.KnowledgeEntryID != docA.ID {
		t.Fatalf("stored link changed after rejected update: %+v", stored)
	}
}

func TestProductKnowledgeLinkPublishRequiresPublishedEntryRevision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(
		&models.Tenant{}, &models.Product{}, &models.KnowledgeBase{},
		&models.KnowledgeDocument{}, &models.KnowledgeRevision{}, &models.ProductKnowledgeLink{},
	); err != nil {
		t.Fatal(err)
	}
	tenant := &models.Tenant{Name: "Tenant", Status: enums.StatusOk}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	product := &models.Product{TenantID: tenant.ID, Code: "P-1", Name: "Product", Status: enums.StatusOk}
	knowledgeBase := &models.KnowledgeBase{TenantID: tenant.ID, Name: "KB", KnowledgeType: "document", Status: enums.StatusOk}
	if err := db.Create(product).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(knowledgeBase).Error; err != nil {
		t.Fatal(err)
	}
	document := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: knowledgeBase.ID, Title: "Draft repair guide",
		ReviewStatus: "draft", Status: enums.StatusDisabled,
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatal(err)
	}
	link := &models.ProductKnowledgeLink{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: knowledgeBase.ID,
		KnowledgeEntryID: document.ID, LinkType: "document", PublishStatus: "draft", Status: enums.StatusOk,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 7, Username: "reviewer"}
	if err := ProductKnowledgeLinkService.PublishLinkScoped(tenant.ID, product.ID, link.ID, operator); err == nil {
		t.Fatal("draft knowledge entry link was published")
	}
	if err := db.First(link, link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if link.PublishStatus != "draft" {
		t.Fatalf("link status = %q, want draft", link.PublishStatus)
	}
	if err := ProductKnowledgeLinkService.PublishLink(link.ID, operator); err == nil {
		t.Fatal("compatibility publish bypassed the entry review gate")
	}

	published := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: knowledgeBase.ID, Title: "Published repair guide",
		ReviewStatus: "published", Status: enums.StatusOk, PublishedRevisionID: 11, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed,
	}
	if err := db.Create(published).Error; err != nil {
		t.Fatal(err)
	}
	if err := ProductKnowledgeLinkService.UpdateLink(UpdateProductKnowledgeLinkRequest{
		ID: link.ID, KnowledgeBaseID: knowledgeBase.ID, KnowledgeEntryID: published.ID,
		LinkType: "document", Visibility: "internal",
	}, operator); err != nil {
		t.Fatalf("bind published-looking entry: %v", err)
	}
	if err := ProductKnowledgeLinkService.PublishLinkScoped(tenant.ID, product.ID, link.ID, operator); err == nil {
		t.Fatal("entry with a nonexistent published revision was accepted")
	}
	revision := &models.KnowledgeRevision{
		TenantID: tenant.ID, KnowledgeBaseID: knowledgeBase.ID, EntryType: "document", EntryID: published.ID,
		VersionNo: 1, Title: published.Title, ReviewStatus: "published",
	}
	if err := db.Create(revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(published).Updates(map[string]any{
		"current_revision_id":   revision.ID,
		"published_revision_id": revision.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := ProductKnowledgeLinkService.PublishLinkScoped(tenant.ID, product.ID, link.ID, operator); err != nil {
		t.Fatalf("publish valid knowledge link: %v", err)
	}
	if err := db.First(link, link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if link.PublishStatus != "published" {
		t.Fatalf("link status = %q, want published", link.PublishStatus)
	}
	if err := ProductKnowledgeLinkService.UpdateLink(UpdateProductKnowledgeLinkRequest{
		ID: link.ID, KnowledgeBaseID: knowledgeBase.ID, KnowledgeEntryID: document.ID,
		LinkType: "document", Visibility: "internal",
	}, operator); err != nil {
		t.Fatalf("rebind published link to draft entry: %v", err)
	}
	if err := db.First(link, link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if link.PublishStatus != "draft" || link.KnowledgeEntryID != document.ID {
		t.Fatalf("rebound link = %+v, want draft status", link)
	}
	if err := db.First(published, published.ID).Error; err != nil {
		t.Fatal(err)
	}
	if published.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("previously scoped entry index status = %q, want pending for reconciliation", published.IndexStatus)
	}
}

func TestPublishedRevisionResolverRequiresExplicitPublishedRevision(t *testing.T) {
	document := models.KnowledgeDocument{ID: 11, CurrentRevisionID: 22}
	faq := models.KnowledgeFAQ{ID: 33, CurrentRevisionID: 44}
	if got := resolvePublishedDocumentRevisionID(document); got != 0 {
		t.Fatalf("document revision fallback = %d, want 0", got)
	}
	if got := resolvePublishedFAQRevisionID(faq); got != 0 {
		t.Fatalf("faq revision fallback = %d, want 0", got)
	}
	document.PublishedRevisionID = 55
	faq.PublishedRevisionID = 66
	if got := resolvePublishedDocumentRevisionID(document); got != 55 {
		t.Fatalf("document published revision = %d, want 55", got)
	}
	if got := resolvePublishedFAQRevisionID(faq); got != 66 {
		t.Fatalf("faq published revision = %d, want 66", got)
	}
}
