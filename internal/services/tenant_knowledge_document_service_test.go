package services_test

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func setupTenantKnowledgeDocumentTestDB(t *testing.T) {
	t.Helper()
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + filepath.Join(t.TempDir(), "tenant-knowledge-document.db") + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.KnowledgeDocument{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
}

func TestListEnterpriseTenantDocumentsEnforcesTenantScope(t *testing.T) {
	setupTenantKnowledgeDocumentTestDB(t)
	tenantKnowledgeBase := &models.KnowledgeBase{
		TenantID:      41,
		Name:          "Tenant knowledge",
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		AccessScope:   string(enums.KnowledgeBaseAccessScopeTenant),
		Status:        enums.StatusOk,
	}
	productKnowledgeBase := &models.KnowledgeBase{
		TenantID:      41,
		Name:          "Product knowledge",
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		AccessScope:   string(enums.KnowledgeBaseAccessScopeProduct),
		Status:        enums.StatusOk,
	}
	if err := sqls.DB().Create(tenantKnowledgeBase).Error; err != nil {
		t.Fatalf("create tenant knowledge base: %v", err)
	}
	if err := sqls.DB().Create(productKnowledgeBase).Error; err != nil {
		t.Fatalf("create product knowledge base: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID:        41,
		KnowledgeBaseID: tenantKnowledgeBase.ID,
		Title:           "General service policy",
		ContentType:     enums.KnowledgeDocumentContentTypeMarkdown,
		Content:         "General service policy content",
		SourceType:      "knowledge_entry",
		ReviewStatus:    "published",
		Status:          enums.StatusOk,
	}
	if err := sqls.DB().Create(document).Error; err != nil {
		t.Fatalf("create knowledge document: %v", err)
	}

	items, err := services.KnowledgeDocumentService.ListEnterpriseTenantDocuments(41, tenantKnowledgeBase.ID, 1, 20)
	if err != nil {
		t.Fatalf("ListEnterpriseTenantDocuments() error = %v", err)
	}
	if len(items.Items) != 1 || items.Items[0].ID != document.ID || items.Items[0].ProductID != 0 {
		t.Fatalf("items = %+v, want tenant-only document", items)
	}
	if _, err := services.KnowledgeDocumentService.ListEnterpriseTenantDocuments(41, productKnowledgeBase.ID, 1, 20); err == nil {
		t.Fatal("product-scoped knowledge base must be rejected")
	}
	if _, err := services.KnowledgeDocumentService.ListEnterpriseTenantDocuments(42, tenantKnowledgeBase.ID, 1, 20); err == nil {
		t.Fatal("cross-tenant knowledge base must be rejected")
	}
}
