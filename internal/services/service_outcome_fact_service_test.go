package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupServiceOutcomeFactTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
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
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(nil)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestServiceOutcomeFactRebuildsIdempotentlyFromTicketEvidence(t *testing.T) {
	db := setupServiceOutcomeFactTestDB(t)
	const tenantID int64 = 41
	createdAt := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	assignedAt := createdAt.Add(10 * time.Minute)
	acceptedAt := createdAt.Add(15 * time.Minute)
	handledAt := createdAt.Add(20 * time.Minute)
	resolvedAt := createdAt.Add(3 * time.Hour)
	handoffAt := createdAt.Add(5 * time.Minute)

	conversation := models.Conversation{
		TenantID:      tenantID,
		ProductID:     501,
		DeviceID:      601,
		HandoffAt:     &handoffAt,
		AIReplyRounds: 2,
		AuditFields:   models.AuditFields{CreatedAt: createdAt, UpdatedAt: handoffAt},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ticket := models.Ticket{
		TicketNo: "OUTCOME-1", Title: "液压控制器异常", TenantID: tenantID, ProductID: 501, ProductModelID: 502,
		DeviceID: 601, ConversationID: conversation.ID, Status: enums.TicketStatusClosed,
		AssignedAt: &assignedAt, AcceptedAt: &acceptedAt, HandledAt: &handledAt, ResolvedAt: &resolvedAt,
		AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	diagnosis := models.DiagnosisSession{
		ID: "outcome-diagnosis", TenantID: tenantID, ConversationRefID: conversation.ID,
		ConversationID: int642Str(conversation.ID), ProductRefID: ticket.ProductID, ProductID: int642Str(ticket.ProductID),
		Status: "escalated", CreatedAt: createdAt.Add(time.Minute),
		BaseModel: models.BaseModel{CreatedAt: createdAt.Add(time.Minute), UpdatedAt: handoffAt},
	}
	if err := db.Create(&diagnosis).Error; err != nil {
		t.Fatalf("create diagnosis: %v", err)
	}
	repair := models.TicketRepairRecord{
		TenantID: tenantID, TicketID: ticket.ID, ProductID: ticket.ProductID, ProductModelID: ticket.ProductModelID,
		DeviceID: ticket.DeviceID, ServiceMethod: "remote", TestResult: "passed", RemoteResolved: true,
		CostHours: 2.5, MetadataJSON: `{"serviceOutcome":{"avoidedOnsiteVisit":true,"downtimeMinutes":180}}`,
		AuditFields: models.AuditFields{CreatedAt: handledAt, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&repair).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	feedback := models.TicketFeedback{
		TenantID: tenantID, TicketID: ticket.ID, Rating: 5, Status: "submitted", SubmittedAt: resolvedAt.Add(time.Minute),
		AuditFields: models.AuditFields{CreatedAt: resolvedAt.Add(time.Minute), UpdatedAt: resolvedAt.Add(time.Minute)},
	}
	if err := db.Create(&feedback).Error; err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	collaboration := models.TicketSupplierCollaboration{
		TenantID: tenantID, TicketID: ticket.ID, ProductID: ticket.ProductID, ProductModuleID: 1, PartnerCompanyID: 7,
		Status: "resolved", InvitedAt: handledAt, ResolvedAt: &resolvedAt, RecordStatus: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: handledAt, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&collaboration).Error; err != nil {
		t.Fatalf("create supplier collaboration: %v", err)
	}
	meeting := models.MeetingRoomJitsi{
		ID: "outcome-meeting", TenantID: tenantID, TicketID: int642Str(ticket.ID), RoomName: "outcome-room",
		Status: "ended", StartedAt: &handledAt, EndedAt: &resolvedAt,
		BaseModel: models.BaseModel{CreatedAt: handledAt, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	participant := models.MeetingParticipant{
		ID: "outcome-participant", MeetingID: meeting.ID, UserID: "engineer-1", UserType: "enterprise",
		ParticipantName: "工程师", JoinedAt: &handledAt,
		BaseModel: models.BaseModel{CreatedAt: handledAt, UpdatedAt: handledAt},
	}
	if err := db.Create(&participant).Error; err != nil {
		t.Fatalf("create meeting participant: %v", err)
	}
	retrieveLog := models.KnowledgeRetrieveLog{
		TenantID: tenantID, KnowledgeBaseID: 77, ConversationID: conversation.ID, RequestID: "outcome-retrieve",
		HitCount: 1, UsedChunkCount: 1, CitationCount: 1, CreatedAt: createdAt.Add(2 * time.Minute),
	}
	if err := db.Create(&retrieveLog).Error; err != nil {
		t.Fatalf("create retrieve log: %v", err)
	}
	if err := db.Create(&models.KnowledgeRetrieveHit{
		TenantID: tenantID, RetrieveLogID: retrieveLog.ID, KnowledgeBaseID: 77, ChunkID: 1,
		UsedInAnswer: true, IsCitation: true, CreatedAt: createdAt.Add(2 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create retrieve hit: %v", err)
	}
	candidate := models.KnowledgeCandidate{
		TenantID: tenantID, ProductID: ticket.ProductID, TicketID: ticket.ID, SourceType: "ticket_repair",
		SourceID: ticket.TicketNo, KnowledgeEntryID: 88, ReviewStatus: string(enums.KnowledgeCandidateReviewStatusApproved),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: resolvedAt, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&candidate).Error; err != nil {
		t.Fatalf("create knowledge candidate: %v", err)
	}

	// Same ticket/conversation identifiers in another tenant must never enter the fact.
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 99, TicketID: ticket.ID, ServiceMethod: "onsite", TestResult: "failed", CostHours: 100,
		MetadataJSON: `{"downtimeMinutes":9999}`, AuditFields: models.AuditFields{CreatedAt: handledAt, UpdatedAt: resolvedAt},
	}).Error; err != nil {
		t.Fatalf("create cross-tenant repair: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TicketNo: "OUTCOME-OTHER-TENANT-REPEAT", Title: "其他租户同设备重复", TenantID: 99,
		ProductID: ticket.ProductID, ProductModelID: ticket.ProductModelID, DeviceID: ticket.DeviceID,
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: resolvedAt.Add(12 * time.Hour), UpdatedAt: resolvedAt.Add(12 * time.Hour)},
	}).Error; err != nil {
		t.Fatalf("create cross-tenant repeat ticket: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TicketNo: "OUTCOME-LATE-REPEAT", Title: "窗口外同设备重复", TenantID: tenantID,
		ProductID: ticket.ProductID, ProductModelID: ticket.ProductModelID, DeviceID: ticket.DeviceID,
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: resolvedAt.Add(31 * 24 * time.Hour), UpdatedAt: resolvedAt.Add(31 * 24 * time.Hour)},
	}).Error; err != nil {
		t.Fatalf("create out-of-window repeat ticket: %v", err)
	}

	fact, err := ServiceOutcomeFactService.RebuildTicket(context.Background(), tenantID, ticket.ID)
	if err != nil {
		t.Fatalf("RebuildTicket() error = %v", err)
	}
	if !fact.AIEngaged || fact.AISelfServiceResolved || !fact.HumanEscalated || !fact.ExpertInterventionKnown || !fact.ExpertIntervened {
		t.Fatalf("unexpected AI/human outcome: %+v", fact)
	}
	if !fact.SupplierInvolved || fact.SupplierCollaborationCount != 1 || !fact.VideoUsed || fact.MeetingCount != 1 {
		t.Fatalf("unexpected collaboration outcome: %+v", fact)
	}
	if !fact.ResolutionKnown || !fact.RemoteResolved || !fact.OnsiteVisitKnown || fact.OnsiteVisitOccurred ||
		!fact.AvoidedOnsiteVisitKnown || !fact.AvoidedOnsiteVisit {
		t.Fatalf("unexpected resolution/trip outcome: %+v", fact)
	}
	if !fact.FirstTimeFixKnown || !fact.FirstTimeFixEligible || !fact.FirstTimeFix || fact.Reopened || fact.ReopenCount != 0 {
		t.Fatalf("unexpected first-time-fix outcome: %+v", fact)
	}
	if !fact.DowntimeKnown || fact.DowntimeMinutes != 180 || fact.WorkHours != 2.5 {
		t.Fatalf("unexpected downtime/work outcome: %+v", fact)
	}
	if !fact.KnowledgeReuseKnown || !fact.KnowledgeReused || fact.KnowledgeRetrieveCount != 1 || fact.KnowledgeUsedCount != 1 ||
		fact.KnowledgeCitationCount != 1 || fact.KnowledgeCandidateID != candidate.ID || fact.PublishedKnowledgeEntryID != 88 {
		t.Fatalf("unexpected knowledge outcome: %+v", fact)
	}
	if fact.ResponseDurationSeconds == nil || *fact.ResponseDurationSeconds != 20*60 ||
		fact.AcceptanceDurationSeconds == nil || *fact.AcceptanceDurationSeconds != 5*60 ||
		fact.ResolutionDurationSeconds == nil || *fact.ResolutionDurationSeconds != 3*60*60 ||
		!fact.RatingKnown || fact.CustomerRating != 5 {
		t.Fatalf("unexpected duration/rating outcome: %+v", fact)
	}

	sameFact, err := ServiceOutcomeFactService.RebuildTicket(context.Background(), tenantID, ticket.ID)
	if err != nil {
		t.Fatalf("idempotent RebuildTicket() error = %v", err)
	}
	if sameFact.ID != fact.ID || !sameFact.BuiltAt.Equal(fact.BuiltAt) || sameFact.SourceFingerprint != fact.SourceFingerprint {
		t.Fatalf("unchanged rebuild rewrote fact: before=%+v after=%+v", fact, sameFact)
	}
	repeatCreatedAt := resolvedAt.Add(24 * time.Hour)
	if err := db.Create(&models.Ticket{
		TicketNo: "OUTCOME-REPEAT", Title: "同设备故障复发", TenantID: tenantID, ProductID: ticket.ProductID,
		ProductModelID: ticket.ProductModelID, DeviceID: ticket.DeviceID, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: repeatCreatedAt, UpdatedAt: repeatCreatedAt},
	}).Error; err != nil {
		t.Fatalf("create repeat ticket: %v", err)
	}
	repeatedFact, err := ServiceOutcomeFactService.RebuildTicket(context.Background(), tenantID, ticket.ID)
	if err != nil {
		t.Fatalf("rebuild repeated fact: %v", err)
	}
	if repeatedFact.ID != fact.ID || repeatedFact.Reopened || repeatedFact.RepeatTicketCount30Days != 1 || repeatedFact.FirstTimeFix {
		t.Fatalf("30-day repeat ticket was not reflected in first-time fix: %+v", repeatedFact)
	}

	reopenedAt := resolvedAt.Add(time.Hour)
	if err := db.Create(&models.TicketProgress{
		TenantID: tenantID, TicketID: ticket.ID, EventType: enums.TicketProgressEventReopened,
		Content: "客户反馈故障复现", CreatedAt: reopenedAt,
	}).Error; err != nil {
		t.Fatalf("create reopen progress: %v", err)
	}
	reopenedFact, err := ServiceOutcomeFactService.RebuildTicket(context.Background(), tenantID, ticket.ID)
	if err != nil {
		t.Fatalf("rebuild reopened fact: %v", err)
	}
	if reopenedFact.ID != fact.ID || !reopenedFact.Reopened || reopenedFact.ReopenCount != 1 ||
		reopenedFact.RepeatTicketCount30Days != 1 || reopenedFact.FirstTimeFix {
		t.Fatalf("reopen was not reflected in same fact row: %+v", reopenedFact)
	}
	var factCount int64
	if err := db.Model(&models.TicketServiceOutcomeFact{}).Count(&factCount).Error; err != nil || factCount != 1 {
		t.Fatalf("fact count = %d, err = %v", factCount, err)
	}
}

func TestServiceOutcomeFactPreservesUnknownPurchasingEvidence(t *testing.T) {
	db := setupServiceOutcomeFactTestDB(t)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	ticket := models.Ticket{
		TicketNo: "OUTCOME-UNKNOWN", Title: "结果证据不完整", TenantID: 52, ProductID: 701,
		Status: enums.TicketStatusClosed, ResolvedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: 52, TicketID: ticket.ID, ProductID: ticket.ProductID, ServiceMethod: "remote",
		RemoteResolved: true, MetadataJSON: `{}`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	fact, err := ServiceOutcomeFactService.RebuildTicket(context.Background(), 52, ticket.ID)
	if err != nil {
		t.Fatalf("RebuildTicket() error = %v", err)
	}
	if !fact.RemoteResolved || fact.AvoidedOnsiteVisitKnown || fact.AvoidedOnsiteVisit || fact.DowntimeKnown ||
		fact.FirstTimeFixKnown || fact.FirstTimeFixEligible || fact.FirstTimeFix {
		t.Fatalf("missing evidence was converted into a false outcome: %+v", fact)
	}
}

func TestServiceOutcomeFactRebuildMissingBatchBackfillsTerminalTickets(t *testing.T) {
	db := setupServiceOutcomeFactTestDB(t)
	now := time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)
	tickets := []models.Ticket{
		{TenantID: 61, TicketNo: "BACKFILL-CLOSED", Title: "closed", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 61, TicketNo: "BACKFILL-RESOLVED", Title: "resolved", Status: enums.TicketStatusResolved, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 61, TicketNo: "BACKFILL-OPEN", Title: "open", Status: enums.TicketStatusProcessing, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 61, TicketNo: "BACKFILL-CANCELLED", Title: "cancelled", Status: enums.TicketStatusCancelled, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}

	processed, err := ServiceOutcomeFactService.RebuildMissingBatch(context.Background(), 100)
	if err != nil {
		t.Fatalf("RebuildMissingBatch() error = %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}
	var facts []models.TicketServiceOutcomeFact
	if err := db.Order("ticket_id ASC").Find(&facts).Error; err != nil {
		t.Fatalf("load facts: %v", err)
	}
	if len(facts) != 2 || facts[0].TicketID != tickets[0].ID || facts[1].TicketID != tickets[1].ID {
		t.Fatalf("facts = %+v", facts)
	}
	processed, err = ServiceOutcomeFactService.RebuildMissingBatch(context.Background(), 100)
	if err != nil || processed != 0 {
		t.Fatalf("idempotent backfill processed=%d err=%v", processed, err)
	}
}
