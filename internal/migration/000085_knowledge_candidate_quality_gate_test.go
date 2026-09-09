package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEnsureKnowledgeCandidateQualityGateBackfillsAndSeparatesReviewQueue(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.TicketRepairRecord{}, &models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	highTicket := &models.Ticket{
		TenantID: 1, ProductID: 10, ProductModelID: 20, DeviceID: 30,
		TicketNo: "TK-HIGH", Title: "控制器过热停机", FaultCode: "TEMP-001",
		PriorityCode: "p2", Status: enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	lowTicket := &models.Ticket{
		TenantID: 1, ProductID: 10, TicketNo: "TK-LOW", Title: "咨询",
		PriorityCode: "p4", Status: enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create([]*models.Ticket{highTicket, lowTicket}).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 1, TicketID: highTicket.ID, ProductID: 10, ProductModelID: 20, DeviceID: 30,
		RootCause:  "散热风扇接头松动导致控制器温度过高",
		Solution:   "重新固定风扇接头并清理散热通道后连续运行三十分钟",
		Conclusion: "复测期间温度稳定且未再次停机", TestResult: "passed",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	high := &models.KnowledgeCandidate{
		TenantID: 1, ProductID: 10, ProductModelID: 20, TicketID: highTicket.ID,
		SourceType: "ticket_repair", SourceID: highTicket.TicketNo, Title: "TEMP-001 控制器过热停机",
		Suggestion:       "复测期间温度稳定且未再次停机",
		RootCauseSummary: "散热风扇接头松动导致控制器温度过高",
		SolutionSummary:  "重新固定风扇接头并清理散热通道后连续运行三十分钟",
		ReviewStatus:     "pending", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	low := &models.KnowledgeCandidate{
		TenantID: 1, ProductID: 10, TicketID: lowTicket.ID,
		SourceType: "ticket_repair", SourceID: lowTicket.TicketNo, Title: "咨询",
		Suggestion: "已回复", ReviewStatus: "pending", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create([]*models.KnowledgeCandidate{high, low}).Error; err != nil {
		t.Fatalf("create candidates: %v", err)
	}

	if err := ensureKnowledgeCandidateQualityGate(db); err != nil {
		t.Fatalf("ensure quality gate: %v", err)
	}
	if err := db.First(high, high.ID).Error; err != nil {
		t.Fatalf("reload high candidate: %v", err)
	}
	if err := db.First(low, low.ID).Error; err != nil {
		t.Fatalf("reload low candidate: %v", err)
	}
	if high.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) ||
		high.QualityScore < enums.KnowledgeCandidateMinimumQualityScore ||
		high.ValueScore < enums.KnowledgeCandidateMinimumValueScore {
		t.Fatalf("high candidate should enter review queue: %+v", high)
	}
	if low.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusPending) || low.QualityScore >= 60 {
		t.Fatalf("low candidate should be withheld from review: %+v", low)
	}
	if err := ensureKnowledgeCandidateQualityGate(db); err != nil {
		t.Fatalf("quality gate migration should be idempotent: %v", err)
	}
}
