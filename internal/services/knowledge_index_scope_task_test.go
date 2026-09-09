package services

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestKnowledgeIndexTaskKeyChangesWithProductScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.ProductModel{}, &models.ProductKnowledgeLink{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	const tenantID = int64(71)
	productA := &models.Product{TenantID: tenantID, Code: "A", Name: "Product A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenantID, Code: "B", Name: "Product B", Status: enums.StatusOk}
	if err := db.Create([]*models.Product{productA, productB}).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}

	service := newKnowledgeIndexSyncService()
	generation := &models.KnowledgeIndexGeneration{ID: 3, CollectionName: "knowledge-v3"}
	commonTask, err := service.buildEntryTask(db, tenantID, 81, "knowledge_document", 91, 101, "hash", "upsert", generation)
	if err != nil {
		t.Fatalf("build common task: %v", err)
	}

	linkA := &models.ProductKnowledgeLink{
		TenantID: tenantID, ProductID: productA.ID, KnowledgeBaseID: 81, KnowledgeEntryID: 91,
		LinkType: "document", PublishStatus: "published", Status: enums.StatusOk,
	}
	if err := db.Create(linkA).Error; err != nil {
		t.Fatalf("create product A link: %v", err)
	}
	productATask, err := service.buildEntryTask(db, tenantID, 81, "knowledge_document", 91, 101, "hash", "upsert", generation)
	if err != nil {
		t.Fatalf("build product A task: %v", err)
	}
	if productATask.IdempotencyKey == commonTask.IdempotencyKey {
		t.Fatalf("scope change reused task key %q", productATask.IdempotencyKey)
	}
	if productATask.ProductID != productA.ID {
		t.Fatalf("single-product task ProductID = %d, want %d", productATask.ProductID, productA.ID)
	}

	linkB := &models.ProductKnowledgeLink{
		TenantID: tenantID, ProductID: productB.ID, KnowledgeBaseID: 81, KnowledgeEntryID: 91,
		LinkType: "document", PublishStatus: "published", Status: enums.StatusOk,
	}
	if err := db.Create(linkB).Error; err != nil {
		t.Fatalf("create product B link: %v", err)
	}
	multiProductTask, err := service.buildEntryTask(db, tenantID, 81, "knowledge_document", 91, 101, "hash", "upsert", generation)
	if err != nil {
		t.Fatalf("build multi-product task: %v", err)
	}
	if multiProductTask.IdempotencyKey == productATask.IdempotencyKey {
		t.Fatalf("multi-product scope reused task key %q", multiProductTask.IdempotencyKey)
	}
	if multiProductTask.ProductID != 0 {
		t.Fatalf("multi-product task ProductID = %d, want tenant-scoped task marker 0", multiProductTask.ProductID)
	}
	if !strings.Contains(multiProductTask.InputVersion, ":scope:") {
		t.Fatalf("InputVersion = %q, want scope fingerprint", multiProductTask.InputVersion)
	}
}
