package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestKnowledgeIndexSyncBackfillsProductManualStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(&models.KnowledgeDocument{}, &models.ProductManualFile{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	document := &models.KnowledgeDocument{
		TenantID:        701,
		KnowledgeBaseID: 801,
		Title:           "Hydraulic press manual",
		ContentType:     enums.KnowledgeDocumentContentTypeMarkdown,
		ReviewStatus:    "published",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusPending,
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create knowledge document: %v", err)
	}
	manual := &models.ProductManualFile{
		TenantID:            701,
		ProductID:           901,
		AssetID:             1001,
		KnowledgeBaseID:     801,
		KnowledgeDocumentID: document.ID,
		SyncStatus:          enums.KnowledgeDocumentIndexStatusPending,
		Status:              enums.StatusOk,
	}
	if err := db.Create(manual).Error; err != nil {
		t.Fatalf("create product manual: %v", err)
	}
	task := &models.KnowledgeIndexSyncTask{TenantID: 701, SubjectType: "knowledge_document", SubjectID: document.ID}
	service := newKnowledgeIndexSyncService()

	indexedAt := time.Now()
	service.markEntryIndexed(task, indexedAt)
	var indexedManual models.ProductManualFile
	if err := db.First(&indexedManual, manual.ID).Error; err != nil {
		t.Fatalf("reload indexed product manual: %v", err)
	}
	if indexedManual.SyncStatus != enums.KnowledgeDocumentIndexStatusIndexed || indexedManual.SyncedAt == nil || indexedManual.SyncError != "" {
		t.Fatalf("indexed product manual = %#v", indexedManual)
	}

	service.markEntryIndexFailed(task, "embedding_error", errTestKnowledgeIndexFailure{}, false)
	var retryingManual models.ProductManualFile
	if err := db.First(&retryingManual, manual.ID).Error; err != nil {
		t.Fatalf("reload retrying product manual: %v", err)
	}
	if retryingManual.SyncStatus != enums.KnowledgeDocumentIndexStatusPending || !strings.Contains(retryingManual.SyncError, "embedding_error") || retryingManual.SyncedAt != nil {
		t.Fatalf("retrying product manual = %#v", retryingManual)
	}

	service.markEntryIndexFailed(task, "embedding_error", errTestKnowledgeIndexFailure{}, true)
	var failedManual models.ProductManualFile
	if err := db.First(&failedManual, manual.ID).Error; err != nil {
		t.Fatalf("reload failed product manual: %v", err)
	}
	if failedManual.SyncStatus != enums.KnowledgeDocumentIndexStatusFailed || failedManual.SyncedAt != nil {
		t.Fatalf("failed product manual = %#v", failedManual)
	}
}

type errTestKnowledgeIndexFailure struct{}

func (errTestKnowledgeIndexFailure) Error() string { return "embedding provider unavailable" }
