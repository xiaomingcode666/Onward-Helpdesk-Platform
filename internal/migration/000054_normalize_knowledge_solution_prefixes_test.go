package migration

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeKnowledgeSolutionPrefixes(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeDocument{}, &models.KnowledgeChunk{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	document := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 10, Title: "PWR-001 端子氧化",
		Content:     "根因：端子氧化\n\n处理方案：处理方案：更换端子并重新压接",
		ContentHash: "old-doc-hash", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	untouchedDocument := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 10, Title: "E42",
		Content:     "处理方案：清理散热通道",
		ContentHash: "keep-doc-hash", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := db.Create(&untouchedDocument).Error; err != nil {
		t.Fatalf("create untouched document: %v", err)
	}
	chunk := models.KnowledgeChunk{
		TenantID: 1, KnowledgeBaseID: 10, DocumentID: document.ID, ChunkNo: 1,
		Title:       "PWR-001 端子氧化",
		Content:     "处理方案：处理方案：更换端子并重新压接；维修结论：复测通过",
		ContentHash: "old-chunk-hash", Status: enums.StatusOk,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&chunk).Error; err != nil {
		t.Fatalf("create chunk: %v", err)
	}

	if err := normalizeKnowledgeSolutionPrefixes(db); err != nil {
		t.Fatalf("normalize knowledge prefixes: %v", err)
	}

	var refreshedDocument models.KnowledgeDocument
	if err := db.First(&refreshedDocument, document.ID).Error; err != nil {
		t.Fatalf("load refreshed document: %v", err)
	}
	if strings.Contains(refreshedDocument.Content, "处理方案：处理方案：") ||
		!strings.Contains(refreshedDocument.Content, "处理方案：更换端子并重新压接") {
		t.Fatalf("document prefix was not normalized: %q", refreshedDocument.Content)
	}
	if refreshedDocument.ContentHash == "" || refreshedDocument.ContentHash == document.ContentHash {
		t.Fatalf("document content hash was not refreshed: %q", refreshedDocument.ContentHash)
	}

	var refreshedChunk models.KnowledgeChunk
	if err := db.First(&refreshedChunk, chunk.ID).Error; err != nil {
		t.Fatalf("load refreshed chunk: %v", err)
	}
	if strings.Contains(refreshedChunk.Content, "处理方案：处理方案：") ||
		!strings.Contains(refreshedChunk.Content, "处理方案：更换端子并重新压接") {
		t.Fatalf("chunk prefix was not normalized: %q", refreshedChunk.Content)
	}
	if refreshedChunk.ContentHash == "" || refreshedChunk.ContentHash == chunk.ContentHash {
		t.Fatalf("chunk content hash was not refreshed: %q", refreshedChunk.ContentHash)
	}

	var afterUntouched models.KnowledgeDocument
	if err := db.First(&afterUntouched, untouchedDocument.ID).Error; err != nil {
		t.Fatalf("load untouched document: %v", err)
	}
	if afterUntouched.ContentHash != untouchedDocument.ContentHash {
		t.Fatalf("document without duplicated prefix must not be changed: %q", afterUntouched.ContentHash)
	}
}
