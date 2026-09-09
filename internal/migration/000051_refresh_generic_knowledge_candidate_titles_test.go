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

func TestRefreshGenericKnowledgeCandidateTitles(t *testing.T) {
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
		TenantID: 1, ProductID: 11, TicketNo: "TK-PWR-001", FaultCode: "PWR-001",
		Title: "PWR-001", Status: enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 1, TicketID: ticket.ID, ProductID: 11, RootCause: "压力控制板保护输入端线束端子压接不良，运行后低压瞬断",
		Solution: "更换端子并重新压接", TestResult: "passed",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	generic := models.KnowledgeCandidate{
		TenantID: 1, ProductID: 11, TicketID: ticket.ID, Title: "PWR-001", ReviewStatus: "pending", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	descriptive := models.KnowledgeCandidate{
		TenantID: 1, ProductID: 11, TicketID: ticket.ID, Title: "PWR-001 已人工整理的标题", ReviewStatus: "pending", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	approved := models.KnowledgeCandidate{
		TenantID: 1, ProductID: 11, TicketID: ticket.ID, Title: "PWR-001", ReviewStatus: "approved", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&generic).Error; err != nil {
		t.Fatalf("create generic candidate: %v", err)
	}
	if err := db.Create(&descriptive).Error; err != nil {
		t.Fatalf("create descriptive candidate: %v", err)
	}
	if err := db.Create(&approved).Error; err != nil {
		t.Fatalf("create approved candidate: %v", err)
	}

	if err := refreshGenericKnowledgeCandidateTitles(db); err != nil {
		t.Fatalf("refresh titles: %v", err)
	}

	var refreshed models.KnowledgeCandidate
	if err := db.First(&refreshed, generic.ID).Error; err != nil {
		t.Fatalf("load refreshed candidate: %v", err)
	}
	if refreshed.Title != "PWR-001 压力控制板保护输入端线束端子压接不良" {
		t.Fatalf("generic title was not refreshed: %q", refreshed.Title)
	}
	var untouched models.KnowledgeCandidate
	if err := db.First(&untouched, descriptive.ID).Error; err != nil {
		t.Fatalf("load descriptive candidate: %v", err)
	}
	if untouched.Title != descriptive.Title {
		t.Fatalf("descriptive title must be preserved: %q", untouched.Title)
	}
	var approvedAfter models.KnowledgeCandidate
	if err := db.First(&approvedAfter, approved.ID).Error; err != nil {
		t.Fatalf("load approved candidate: %v", err)
	}
	if approvedAfter.Title != approved.Title {
		t.Fatalf("approved title must be preserved: %q", approvedAfter.Title)
	}
}
