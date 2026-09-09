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

func TestRefreshPendingKnowledgeCandidateSummaries(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.TicketRepairRecord{}, &models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	ticket := models.Ticket{
		TenantID: 1, ProductID: 11, TicketNo: "TK-E42", FaultCode: "E42",
		Title: "E42", Status: enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 1, TicketID: ticket.ID, ProductID: 11, RootCause: "散热通道积尘导致控制器温度升至 86℃",
		Solution: "清理散热通道并确认风扇恢复", Conclusion: "温度降至 44℃ 后无负载复测 30 分钟未再出现 E42",
		TestResult: "passed", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 1, TicketID: ticket.ID, ProductID: 11, RootCause: "最终确认风扇供电端子松动",
		Solution: "重新压接供电端子并满载复测", Conclusion: "满载复测 60 分钟温度稳定在 42℃",
		TestResult: "passed", AuditFields: models.AuditFields{CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
	}).Error; err != nil {
		t.Fatalf("create latest repair: %v", err)
	}
	pending := models.KnowledgeCandidate{
		TenantID: 1, ProductID: 11, TicketID: ticket.ID, Title: "E42",
		RootCauseSummary: "旧根因", SolutionSummary: "清理散热通道并确认风扇恢复",
		ReviewStatus: "pending", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	approved := models.KnowledgeCandidate{
		TenantID: 1, ProductID: 11, TicketID: ticket.ID, Title: "E42",
		RootCauseSummary: "旧根因", SolutionSummary: "旧方案",
		ReviewStatus: "approved", KnowledgeEntryID: 1001, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending candidate: %v", err)
	}
	if err := db.Create(&approved).Error; err != nil {
		t.Fatalf("create approved candidate: %v", err)
	}

	if err := refreshPendingKnowledgeCandidateSummaries(db); err != nil {
		t.Fatalf("refresh summaries: %v", err)
	}

	var refreshed models.KnowledgeCandidate
	if err := db.First(&refreshed, pending.ID).Error; err != nil {
		t.Fatalf("load refreshed candidate: %v", err)
	}
	if refreshed.Title != "E42 最终确认风扇供电端子松动" {
		t.Fatalf("pending title was not refreshed: %q", refreshed.Title)
	}
	if refreshed.RootCauseSummary != "最终确认风扇供电端子松动" ||
		!strings.Contains(refreshed.SolutionSummary, "重新压接供电端子并满载复测") ||
		!strings.Contains(refreshed.SolutionSummary, "维修结论：满载复测 60 分钟温度稳定在 42℃") ||
		strings.Contains(refreshed.SolutionSummary, "清理散热通道") {
		t.Fatalf("pending solution summary must preserve solution and conclusion: %q", refreshed.SolutionSummary)
	}
	var approvedAfter models.KnowledgeCandidate
	if err := db.First(&approvedAfter, approved.ID).Error; err != nil {
		t.Fatalf("load approved candidate: %v", err)
	}
	if approvedAfter.SolutionSummary != "旧方案" {
		t.Fatalf("approved candidate must not be changed: %+v", approvedAfter)
	}
}
