package migration

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBackfillPublishedDocumentRevisionsCreatesRevisionAndScopedTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.ProductModel{},
		&models.ProductKnowledgeLink{},
		&models.KnowledgeDocument{},
		&models.KnowledgeRevision{},
		&models.KnowledgeIndexSyncTask{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	document := &models.KnowledgeDocument{
		TenantID:        41,
		KnowledgeBaseID: 51,
		Title:           "Legacy published manual",
		Content:         "Shared tenant troubleshooting guidance",
		Language:        "en-US",
		ReviewStatus:    "published",
		Status:          enums.StatusOk,
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	generation := &models.KnowledgeIndexGeneration{ID: 61, CollectionName: "knowledge-v2"}
	if err := backfillPublishedDocumentRevisions(db, generation); err != nil {
		t.Fatalf("backfillPublishedDocumentRevisions() error = %v", err)
	}

	var updated models.KnowledgeDocument
	if err := db.First(&updated, document.ID).Error; err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if updated.CurrentRevisionID <= 0 || updated.PublishedRevisionID != updated.CurrentRevisionID {
		t.Fatalf("document revision ids = current:%d published:%d", updated.CurrentRevisionID, updated.PublishedRevisionID)
	}
	var revision models.KnowledgeRevision
	if err := db.First(&revision, updated.PublishedRevisionID).Error; err != nil {
		t.Fatalf("reload revision: %v", err)
	}
	if revision.ReviewStatus != "published" || revision.ContentHash == "" || revision.Content != document.Content {
		t.Fatalf("revision = %#v", revision)
	}
	var tasks []models.KnowledgeIndexSyncTask
	if err := db.Find(&tasks).Error; err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 1 || !strings.Contains(tasks[0].InputVersion, ":scope:") || len(tasks[0].IdempotencyKey) > 128 {
		t.Fatalf("scope backfill tasks = %#v", tasks)
	}

	if err := backfillPublishedDocumentRevisions(db, generation); err != nil {
		t.Fatalf("second backfill error = %v", err)
	}
	var count int64
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("task count after idempotent backfill = %d, want 1", count)
	}
}
