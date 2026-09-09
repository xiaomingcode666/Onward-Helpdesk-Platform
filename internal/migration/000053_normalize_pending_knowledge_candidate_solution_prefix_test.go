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

func TestNormalizePendingKnowledgeCandidateSolutionPrefixes(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	pending := models.KnowledgeCandidate{
		TenantID: 1, TicketID: 10, Title: "E42", ReviewStatus: "pending", Status: enums.StatusOk,
		SolutionSummary: "处理方案：清理散热通道；维修结论：复测通过",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	approved := models.KnowledgeCandidate{
		TenantID: 1, TicketID: 11, Title: "E42", ReviewStatus: "approved", KnowledgeEntryID: 1001, Status: enums.StatusOk,
		SolutionSummary: "处理方案：已发布内容保持不动",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending: %v", err)
	}
	if err := db.Create(&approved).Error; err != nil {
		t.Fatalf("create approved: %v", err)
	}

	if err := normalizePendingKnowledgeCandidateSolutionPrefixes(db); err != nil {
		t.Fatalf("normalize prefixes: %v", err)
	}

	var refreshed models.KnowledgeCandidate
	if err := db.First(&refreshed, pending.ID).Error; err != nil {
		t.Fatalf("load pending: %v", err)
	}
	if refreshed.SolutionSummary != "清理散热通道；维修结论：复测通过" {
		t.Fatalf("pending prefix was not normalized: %q", refreshed.SolutionSummary)
	}
	var approvedAfter models.KnowledgeCandidate
	if err := db.First(&approvedAfter, approved.ID).Error; err != nil {
		t.Fatalf("load approved: %v", err)
	}
	if approvedAfter.SolutionSummary != approved.SolutionSummary {
		t.Fatalf("approved candidate must not be changed: %q", approvedAfter.SolutionSummary)
	}
}
