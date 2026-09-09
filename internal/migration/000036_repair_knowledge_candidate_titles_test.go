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

func TestRepairPendingTicketKnowledgeCandidateTitles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.KnowledgeCandidate{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ticket := models.Ticket{TenantID: 1, ProductID: 2, TicketNo: "TK-KB-1", Title: "generic fallback", FaultCode: "电气控制故障", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	generated := models.KnowledgeCandidate{TenantID: 1, ProductID: 2, TicketID: ticket.ID, SourceType: "ticket_repair", Title: ticket.Title, ReviewStatus: "pending", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	custom := models.KnowledgeCandidate{TenantID: 1, ProductID: 2, TicketID: ticket.ID, SourceType: "ticket_repair", Title: "人工整理标题", ReviewStatus: "pending", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&generated).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&custom).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairPendingTicketKnowledgeCandidateTitles(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&generated, generated.ID).Error; err != nil || generated.Title != ticket.FaultCode {
		t.Fatalf("generated title was not repaired: title=%q err=%v", generated.Title, err)
	}
	if err := db.First(&custom, custom.ID).Error; err != nil || custom.Title != "人工整理标题" {
		t.Fatalf("custom title should be preserved: title=%q err=%v", custom.Title, err)
	}
}
