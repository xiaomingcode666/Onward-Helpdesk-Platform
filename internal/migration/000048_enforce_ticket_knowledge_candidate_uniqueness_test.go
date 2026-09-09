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

func TestEnforceTicketKnowledgeCandidateUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	rows := []models.KnowledgeCandidate{
		{TenantID: 1, TicketID: 9, SourceType: "ticket_manual", Title: "manual", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, TicketID: 9, SourceType: "ticket_repair", Title: "repair", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed candidates: %v", err)
	}
	if err := enforceTicketKnowledgeCandidateUniqueness(db); err != nil {
		t.Fatalf("enforce uniqueness: %v", err)
	}
	var active []models.KnowledgeCandidate
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status <> ?", 1, 9, enums.StatusDeleted).Find(&active).Error; err != nil {
		t.Fatalf("list active candidates: %v", err)
	}
	if len(active) != 1 || active[0].SourceType != "ticket_repair" {
		t.Fatalf("active candidates = %+v, want only ticket_repair", active)
	}
	duplicate := &models.KnowledgeCandidate{TenantID: 1, TicketID: 9, SourceType: "ticket_manual", Title: "duplicate", Status: enums.StatusOk}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("unique index accepted a second active candidate for one ticket")
	}
	if !db.Migrator().HasIndex(&models.KnowledgeCandidate{}, "ux_knowledge_candidate_active_ticket") {
		t.Fatal("active ticket knowledge candidate unique index was not created")
	}
}
