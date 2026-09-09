package event_handlers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestServiceOutcomeProjectorIsIdempotentAcrossCloseAndKnowledgeEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "service-outcome-projector.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{},
		&models.Conversation{},
		&models.DiagnosisSession{},
		&models.TicketRepairRecord{},
		&models.TicketFeedback{},
		&models.TicketSupplierCollaboration{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.KnowledgeRetrieveLog{},
		&models.KnowledgeRetrieveHit{},
		&models.KnowledgeCandidate{},
		&models.TicketProgress{},
		&models.TicketServiceOutcomeFact{},
	); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(nil) })

	now := time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC)
	ticket := &models.Ticket{
		TenantID: 71, TicketNo: "OUTCOME-EVENT-1", Title: "event projection", ProductID: 801,
		Status: enums.TicketStatusClosed, ResolvedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	closeEvent := events.TicketClosedEvent{EventID: "close-1", TenantID: 71, TicketID: ticket.ID, OccurredAt: now}
	if err := projectTicketClosedToServiceOutcome(context.Background(), closeEvent); err != nil {
		t.Fatalf("project close: %v", err)
	}
	if err := projectTicketClosedToServiceOutcome(context.Background(), closeEvent); err != nil {
		t.Fatalf("replay close: %v", err)
	}

	candidate := &models.KnowledgeCandidate{
		TenantID: 71, ProductID: ticket.ProductID, TicketID: ticket.ID, SourceType: "ticket_repair",
		SourceID: ticket.TicketNo, ReviewStatus: string(enums.KnowledgeCandidateReviewStatusApproved),
		KnowledgeEntryID: 991, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now.Add(time.Minute)},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if err := projectKnowledgeCandidateToServiceOutcome(context.Background(), events.KnowledgeCandidateCreatedEvent{
		EventID: "candidate-1", TenantID: 71, CandidateID: candidate.ID,
	}); err != nil {
		t.Fatalf("project candidate: %v", err)
	}

	var facts []models.TicketServiceOutcomeFact
	if err := db.Find(&facts).Error; err != nil {
		t.Fatalf("load facts: %v", err)
	}
	if len(facts) != 1 || facts[0].TicketID != ticket.ID || facts[0].KnowledgeCandidateID != candidate.ID || facts[0].PublishedKnowledgeEntryID != 991 {
		t.Fatalf("facts = %+v", facts)
	}
}
