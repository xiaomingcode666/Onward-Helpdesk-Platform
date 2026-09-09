package rag

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestResolveKnowledgeEntryProductScopeSeparatesProductAndTenantSharedKnowledge(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.ProductModel{}, &models.KnowledgeBase{}, &models.ProductKnowledgeLink{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	tenant := &models.Tenant{Name: "scope-tenant", Status: enums.StatusOk}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	productA := &models.Product{TenantID: tenant.ID, Code: "A", Name: "Product A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenant.ID, Code: "B", Name: "Product B", Status: enums.StatusOk}
	if err := db.Create([]*models.Product{productA, productB}).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}

	knowledgeBase := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Documents", KnowledgeType: "document", Status: enums.StatusOk}
	if err := db.Create(knowledgeBase).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	knowledgeBaseID := knowledgeBase.ID
	for _, link := range []*models.ProductKnowledgeLink{
		{TenantID: tenant.ID, ProductID: productA.ID, KnowledgeBaseID: knowledgeBaseID, KnowledgeEntryID: 101, LinkType: "document", PublishStatus: "published", Status: enums.StatusOk},
		{TenantID: tenant.ID, ProductID: productB.ID, KnowledgeBaseID: knowledgeBaseID, KnowledgeEntryID: 102, LinkType: "document", PublishStatus: "draft", Status: enums.StatusOk},
	} {
		if err := repositories.ProductKnowledgeLinkRepository.Create(db, link); err != nil {
			t.Fatalf("create product link: %v", err)
		}
	}

	productScope := ResolveKnowledgeEntryProductScope(db, tenant.ID, knowledgeBaseID, 101, "document")
	if len(productScope.ProductIDs) != 1 || productScope.ProductIDs[0] != productA.ID {
		t.Fatalf("product scope ProductIDs = %#v, want product A", productScope.ProductIDs)
	}
	if len(productScope.ScopeKeys) != 1 || productScope.ScopeKeys[0] != buildKnowledgeProductScopeKey(productA.ID) {
		t.Fatalf("product scope keys = %#v", productScope.ScopeKeys)
	}

	tenantScope := ResolveKnowledgeEntryProductScope(db, tenant.ID, knowledgeBaseID, 103, "document")
	if len(tenantScope.ProductIDs) != 0 || len(tenantScope.ScopeKeys) != 1 || tenantScope.ScopeKeys[0] != buildKnowledgeTenantScopeKey(tenant.ID) {
		t.Fatalf("tenant shared scope = %#v", tenantScope)
	}

	draftScope := ResolveKnowledgeEntryProductScope(db, tenant.ID, knowledgeBaseID, 102, "document")
	if len(draftScope.ProductIDs) != 0 || len(draftScope.ScopeKeys) != 0 {
		t.Fatalf("draft product link must not become tenant shared: %#v", draftScope)
	}

	runtimeKeys := BuildKnowledgeRuntimeScopeKeys(tenant.ID, productA.ID)
	if len(runtimeKeys) != 2 || runtimeKeys[0] != buildKnowledgeTenantScopeKey(tenant.ID) || runtimeKeys[1] != buildKnowledgeProductScopeKey(productA.ID) {
		t.Fatalf("runtime scope keys = %#v", runtimeKeys)
	}

	faqKnowledgeBase := &models.KnowledgeBase{TenantID: tenant.ID, Name: "FAQs", KnowledgeType: "faq", Status: enums.StatusOk}
	if err := db.Create(faqKnowledgeBase).Error; err != nil {
		t.Fatalf("create faq knowledge base: %v", err)
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(db, &models.ProductKnowledgeLink{
		TenantID: tenant.ID, ProductID: productA.ID, KnowledgeBaseID: faqKnowledgeBase.ID,
		KnowledgeEntryID: 201, LinkType: "knowledge_article", PublishStatus: "published", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create faq knowledge article link: %v", err)
	}
	faqScope := ResolveKnowledgeEntryProductScope(db, tenant.ID, faqKnowledgeBase.ID, 201, "faq")
	if len(faqScope.ScopeKeys) != 1 || faqScope.ScopeKeys[0] != buildKnowledgeProductScopeKey(productA.ID) {
		t.Fatalf("faq scope keys = %#v", faqScope.ScopeKeys)
	}
}
