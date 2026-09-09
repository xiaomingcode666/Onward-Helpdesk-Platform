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

func TestFixProductKnowledgeLinkUniqueScopePreservesLanguageAndVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ProductKnowledgeLink{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	links := []models.ProductKnowledgeLink{
		{TenantID: 1, ProductID: 2, KnowledgeBaseID: 3, KnowledgeEntryID: 4, LinkType: "knowledge_article", Language: "zh-CN", Version: "v1", PublishStatus: "draft", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, ProductID: 2, KnowledgeBaseID: 3, KnowledgeEntryID: 4, LinkType: "knowledge_article", Language: "zh-CN", Version: "v1", PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, ProductID: 2, KnowledgeBaseID: 3, KnowledgeEntryID: 4, LinkType: "knowledge_article", Language: "en-US", Version: "v1", PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatalf("create links: %v", err)
	}
	if err := fixProductKnowledgeLinkUniqueScope(db); err != nil {
		t.Fatalf("fix unique scope: %v", err)
	}
	var active []models.ProductKnowledgeLink
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("language ASC").Find(&active).Error; err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(active) != 2 || active[0].Language != "en-US" || active[1].Language != "zh-CN" || active[1].PublishStatus != "published" {
		t.Fatalf("active language links = %+v", active)
	}
	duplicate := links[1]
	duplicate.ID = 0
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("corrected unique index accepted an exact duplicate")
	}
	differentVersion := links[1]
	differentVersion.ID = 0
	differentVersion.Version = "v2"
	if err := db.Create(&differentVersion).Error; err != nil {
		t.Fatalf("corrected unique index rejected a distinct version: %v", err)
	}
	if err := fixProductKnowledgeLinkUniqueScope(db); err != nil {
		t.Fatalf("fix unique scope should be idempotent: %v", err)
	}
}

func TestRepairLegacyKnowledgeCandidateDocumentReferences(t *testing.T) {
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
		TenantID: 1, Title: "candidate", ReviewStatus: "approved", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	canonical := &models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "published", SourceType: "knowledge_candidate", SourceReferenceID: candidate.ID,
		ReviewStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	duplicate := &models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "deleted draft", SourceType: "knowledge_entry", SourceReferenceID: candidate.ID,
		ReviewStatus: "draft", Status: enums.StatusDeleted, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(canonical).Error; err != nil {
		t.Fatalf("create canonical document: %v", err)
	}
	if err := db.Create(duplicate).Error; err != nil {
		t.Fatalf("create duplicate document: %v", err)
	}
	if err := db.Model(candidate).Updates(map[string]any{"knowledge_base_id": 7, "knowledge_entry_id": duplicate.ID*10 + 1}).Error; err != nil {
		t.Fatalf("set stale candidate pointer: %v", err)
	}
	links := []models.ProductKnowledgeLink{
		{TenantID: 1, ProductID: 2, KnowledgeBaseID: 7, KnowledgeEntryID: canonical.ID, LinkType: "document", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, ProductID: 2, KnowledgeBaseID: 7, KnowledgeEntryID: duplicate.ID, LinkType: "document", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatalf("create product links: %v", err)
	}
	if err := repairLegacyKnowledgeCandidateDocumentReferences(db); err != nil {
		t.Fatalf("repair candidate references: %v", err)
	}
	if err := fixProductKnowledgeLinkUniqueScope(db); err != nil {
		t.Fatalf("deduplicate repaired links: %v", err)
	}
	if err := db.First(candidate, candidate.ID).Error; err != nil {
		t.Fatalf("reload candidate: %v", err)
	}
	if candidate.KnowledgeEntryID != canonical.ID*10+1 {
		t.Fatalf("candidate knowledge entry = %d, want canonical %d", candidate.KnowledgeEntryID, canonical.ID*10+1)
	}
	if err := db.First(duplicate, duplicate.ID).Error; err != nil {
		t.Fatalf("reload duplicate: %v", err)
	}
	if duplicate.SourceType != "knowledge_candidate_duplicate" || duplicate.ReviewStatus != "deprecated" || duplicate.Status != enums.StatusDisabled || duplicate.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("repaired duplicate = %+v", duplicate)
	}
	var activeLinkCount int64
	if err := db.Model(&models.ProductKnowledgeLink{}).Where("status <> ?", enums.StatusDeleted).Count(&activeLinkCount).Error; err != nil {
		t.Fatalf("count active links: %v", err)
	}
	if activeLinkCount != 1 {
		t.Fatalf("active repaired links = %d, want 1", activeLinkCount)
	}
}
