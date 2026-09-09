package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupSLATenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "sla_tenant_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{}, &models.TicketProgress{}, &models.TicketDispatchAttempt{}, &models.Message{},
		&models.DomainEvent{}, &models.OutboxRecord{},
		&SLAPolicy{}, &SLAPauseRecord{}, &SLAViolation{}, &EscalationRule{}, &TicketEscalation{},
	); err != nil {
		t.Fatalf("migrate sla models: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)
	return db
}

func TestSLAViolationScanDoesNotCrossTenants(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	policy := SLAPolicy{ID: "policy-1", TenantID: "1", Name: "tenant 1", Priority: "p2", FRTMinutes: 1, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	dueAt := now.Add(-time.Minute)
	tickets := []models.Ticket{
		{TicketNo: "SLA-T1", TenantID: 1, Status: enums.TicketStatusPending, SLADueAt: &dueAt, AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now}},
		{TicketNo: "SLA-T2", TenantID: 2, Status: enums.TicketStatusPending, SLADueAt: &dueAt, AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	violations, err := SLAService.CheckSLAViolations()
	if err != nil {
		t.Fatalf("scan violations: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("violation count = %d, want 2: %+v", len(violations), violations)
	}
	for _, violation := range violations {
		if violation.TenantID != "1" || violation.TicketID != formatID(tickets[0].ID) {
			t.Fatalf("cross-tenant violation detected: %+v", violation)
		}
	}
}

func TestSLAViolationScanChecksAllTargetsAndIsIdempotent(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	policy := SLAPolicy{
		ID: "policy-all", TenantID: "1", Name: "all targets", Priority: "p1",
		FRTMinutes: 1, AssignmentMinutes: 2, ResolutionMinutes: 3,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	createdAt := now.Add(-10 * time.Minute)
	handledAt := createdAt.Add(3 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "SLA-ALL", TenantID: 1, ConversationID: 10, PriorityCode: "p1", Status: enums.TicketStatusInProgress,
		HandledAt: &handledAt, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	respondedAt := createdAt.Add(2 * time.Minute)
	if err := db.Create(&models.Message{
		ConversationID: ticket.ConversationID, ClientMsgID: "sla-response", SenderType: enums.IMSenderTypeAgent,
		MessageType: enums.IMMessageTypeText, SendStatus: enums.IMMessageStatusSent, SentAt: &respondedAt,
	}).Error; err != nil {
		t.Fatalf("create response: %v", err)
	}

	violations, err := SLAService.CheckSLAViolations()
	if err != nil {
		t.Fatalf("scan violations: %v", err)
	}
	if len(violations) != 3 {
		t.Fatalf("violation count = %d, want 3: %+v", len(violations), violations)
	}
	seen := make(map[string]bool)
	for _, violation := range violations {
		seen[violation.ViolationType] = true
	}
	for _, violationType := range []string{"frt", "assignment", "resolution"} {
		if !seen[violationType] {
			t.Fatalf("missing %s violation: %+v", violationType, violations)
		}
	}
	var breachEventCount int64
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventSLAWarning).Count(&breachEventCount).Error; err != nil {
		t.Fatalf("count sla breach events: %v", err)
	}
	if breachEventCount != 3 {
		t.Fatalf("sla breach event count = %d, want 3", breachEventCount)
	}

	repeated, err := SLAService.CheckSLAViolations()
	if err != nil {
		t.Fatalf("repeat scan: %v", err)
	}
	if len(repeated) != 0 {
		t.Fatalf("repeat scan created violations: %+v", repeated)
	}
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventSLAWarning).Count(&breachEventCount).Error; err != nil {
		t.Fatalf("count repeated sla breach events: %v", err)
	}
	if breachEventCount != 3 {
		t.Fatalf("repeat scan created duplicate sla events: %d", breachEventCount)
	}
}

func TestSLAViolationCreateIsIdempotentWithActiveUniqueIndex(t *testing.T) {
	db := setupSLATenantTestDB(t)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_sla_violation_active ON sla_violations(ticket_id, violation_type) WHERE resolved_at IS NULL").Error; err != nil {
		t.Fatalf("create active violation unique index: %v", err)
	}
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "SLA-IDEMPOTENT", TenantID: 1, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	ticketID := formatID(ticket.ID)

	first, created, err := SLAService.createViolation(ticketID, "1", "assignment", 5, 10, now)
	if err != nil {
		t.Fatalf("create first violation: %v", err)
	}
	if !created || first == nil {
		t.Fatalf("first violation created=%v violation=%+v, want created violation", created, first)
	}
	second, created, err := SLAService.createViolation(ticketID, "1", "assignment", 5, 10, now.Add(time.Second))
	if err != nil {
		t.Fatalf("create duplicate violation: %v", err)
	}
	if created || second != nil {
		t.Fatalf("duplicate violation created=%v violation=%+v, want no-op", created, second)
	}

	var violationCount int64
	if err := db.Model(&SLAViolation{}).Where("ticket_id = ? AND violation_type = ? AND resolved_at IS NULL", ticketID, "assignment").Count(&violationCount).Error; err != nil {
		t.Fatalf("count violations: %v", err)
	}
	if violationCount != 1 {
		t.Fatalf("active violation count = %d, want 1", violationCount)
	}
	var breachEventCount int64
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventSLAWarning).Count(&breachEventCount).Error; err != nil {
		t.Fatalf("count sla events: %v", err)
	}
	if breachEventCount != 1 {
		t.Fatalf("sla event count = %d, want 1", breachEventCount)
	}
	var outboxCount int64
	if err := db.Model(&models.OutboxRecord{}).Where("event_type = ?", events.EventSLAWarning).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count sla outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("sla outbox count = %d, want 1", outboxCount)
	}
}

func TestSLAWarningScanEnqueuesRiskEventIdempotently(t *testing.T) {
	db := setupSLATenantTestDB(t)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.SLAWarningLeadMinutes = 5
	cfg.TicketDispatch.SLAWarningRepeatMinutes = 15
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Date(2026, 8, 6, 10, 8, 0, 0, time.UTC)
	policy := SLAPolicy{
		ID: "policy-warning", TenantID: "1", Name: "assignment warning", Priority: "p2",
		AssignmentMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	ticket := models.Ticket{
		TicketNo: "SLA-WARN", TenantID: 1, Status: enums.TicketStatusPendingAssigneeAccept,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-8 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	created, err := SLAService.checkSLAWarningsAt(now)
	if err != nil {
		t.Fatalf("scan sla warnings: %v", err)
	}
	if created != 1 {
		t.Fatalf("warning count = %d, want 1", created)
	}
	repeated, err := SLAService.checkSLAWarningsAt(now.Add(time.Minute))
	if err != nil {
		t.Fatalf("repeat warning scan: %v", err)
	}
	if repeated != 0 {
		t.Fatalf("repeat warning scan created duplicates: %d", repeated)
	}
	var riskEventCount int64
	if err := db.Model(&models.DomainEvent{}).
		Where("event_type = ? AND idempotency_key LIKE ?", events.EventSLAWarning, "%:sla.risk:%").
		Count(&riskEventCount).Error; err != nil {
		t.Fatalf("count sla risk events: %v", err)
	}
	if riskEventCount != 1 {
		t.Fatalf("sla risk event count = %d, want 1", riskEventCount)
	}
	var outboxCount int64
	if err := db.Model(&models.OutboxRecord{}).Where("event_type = ?", events.EventSLAWarning).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count sla risk outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("sla risk outbox count = %d, want 1", outboxCount)
	}
}

func TestSLAWarningDispatchesUnassignedAssignmentRisk(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&SLAPolicy{}, &SLAPauseRecord{}, &SLAViolation{}); err != nil {
		t.Fatalf("migrate sla models: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.SLAWarningLeadMinutes = 5
	cfg.TicketDispatch.SLAWarningRepeatMinutes = 15
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Date(2026, 8, 6, 10, 8, 0, 0, time.UTC)
	policy := SLAPolicy{
		ID: "policy-warning-dispatch", TenantID: "1", Name: "assignment warning dispatch", Priority: "p2",
		AssignmentMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	ticket := models.Ticket{
		TicketNo: "SLA-WARN-DISPATCH", TenantID: 1, CurrentTeamID: 1, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-8 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	created, err := SLAService.checkSLAWarningsAt(now)
	if err != nil {
		t.Fatalf("scan sla warnings: %v", err)
	}
	if created != 1 {
		t.Fatalf("warning count = %d, want 1", created)
	}
	var current models.Ticket
	if err := db.First(&current, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if current.Status != enums.TicketStatusPendingAssigneeAccept || current.CurrentAssigneeID != 101 || current.AcceptDeadlineAt == nil {
		t.Fatalf("assignment warning did not dispatch unassigned ticket: %+v", current)
	}
	expectedDeadline := ticket.CreatedAt.Add(10 * time.Minute)
	if !current.AcceptDeadlineAt.Equal(expectedDeadline) {
		t.Fatalf("accept deadline = %v, want SLA deadline %v", current.AcceptDeadlineAt, expectedDeadline)
	}
}

func TestSLAWarningDoesNotRecycleAssignmentBeforeBreach(t *testing.T) {
	db := setupSLATenantTestDB(t)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.SLAWarningLeadMinutes = 5
	cfg.TicketDispatch.SLAWarningRepeatMinutes = 15
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Date(2026, 8, 6, 10, 8, 0, 0, time.UTC)
	policy := SLAPolicy{
		ID: "policy-warning-no-recycle", TenantID: "1", Name: "assignment warning no recycle", Priority: "p2",
		AssignmentMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	deadline := now.Add(20 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "SLA-WARN-ASSIGNED", TenantID: 1, Status: enums.TicketStatusPendingAssigneeAccept,
		CurrentTeamID: 1, CurrentAssigneeID: 101, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-8 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: now.Add(-8 * time.Minute), AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-8 * time.Minute), UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create dispatch attempt: %v", err)
	}

	created, err := SLAService.checkSLAWarningsAt(now)
	if err != nil {
		t.Fatalf("scan sla warnings: %v", err)
	}
	if created != 1 {
		t.Fatalf("warning count = %d, want 1", created)
	}
	var current models.Ticket
	if err := db.First(&current, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if current.Status != enums.TicketStatusPendingAssigneeAccept || current.CurrentAssigneeID != 101 || current.DispatchAttempts != 0 {
		t.Fatalf("assignment warning recycled before breach: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.First(&attempt, "ticket_id = ?", ticket.ID).Error; err != nil {
		t.Fatalf("reload attempt: %v", err)
	}
	if attempt.Outcome != ticketDispatchOutcomePending || attempt.EndedAt != nil {
		t.Fatalf("assignment warning finished pending attempt before breach: %+v", attempt)
	}
}

func TestEscalationScanAndManualEscalationEnforceTenantScope(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	rule := EscalationRule{ID: "rule-1", TenantID: "1", Name: "tenant 1", Priority: "p2", HoursIdle: 1, TargetLevel: 2, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}
	tickets := []models.Ticket{
		{TicketNo: "ESC-T1", TenantID: 1, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}},
		{TicketNo: "ESC-T2", TenantID: 2, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	escalations, err := EscalationService.CheckAndEscalate()
	if err != nil {
		t.Fatalf("scan escalations: %v", err)
	}
	if len(escalations) != 1 || escalations[0].TenantID != "1" || escalations[0].TicketID != formatID(tickets[0].ID) {
		t.Fatalf("unexpected escalations: %+v", escalations)
	}
	if _, err := EscalationService.EscalateTicket(formatID(tickets[1].ID), "1", "", 0, 2, "manual", "cross tenant"); err == nil {
		t.Fatal("expected cross-tenant manual escalation to be rejected")
	}
}

func TestSLATenantScopedMutationsAndIdempotency(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	tickets := []models.Ticket{
		{TicketNo: "SLA-SCOPE-T1", TenantID: 1, Status: enums.TicketStatusInProgress, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "SLA-SCOPE-T2", TenantID: 2, Status: enums.TicketStatusInProgress, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	policies := []SLAPolicy{
		{ID: "scope-policy-1", TenantID: "1", Name: "tenant 1", Priority: "p1", FRTMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "scope-policy-2", TenantID: "2", Name: "tenant 2", Priority: "p1", FRTMinutes: 20, Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&policies).Error; err != nil {
		t.Fatalf("create policies: %v", err)
	}

	newName := "cross-tenant overwrite"
	if _, err := SLAService.UpdateSLAPolicyForTenant("1", policies[1].ID, UpdateSLAPolicyInput{Name: &newName}); err == nil {
		t.Fatal("expected cross-tenant policy update to fail")
	}
	if got := SLAService.GetSLAPolicy(policies[1].ID); got == nil || got.Name != "tenant 2" {
		t.Fatalf("cross-tenant update changed policy: %+v", got)
	}
	if _, err := SLAService.ToggleSLAPolicyForTenant("1", policies[1].ID); err == nil {
		t.Fatal("expected cross-tenant policy toggle to fail")
	}

	ticket1ID := formatID(tickets[0].ID)
	ticket2ID := formatID(tickets[1].ID)
	if _, err := SLAService.PauseSLAForTenant("1", ticket2ID, "waiting_customer"); err == nil {
		t.Fatal("expected cross-tenant pause to fail")
	}
	if _, err := SLAService.GetSLATimelineForTenant("1", ticket2ID); err == nil {
		t.Fatal("expected cross-tenant timeline to fail")
	}

	firstPause, err := SLAService.PauseSLAForTenant("1", ticket1ID, "waiting_customer")
	if err != nil {
		t.Fatalf("pause SLA: %v", err)
	}
	repeatedPause, err := SLAService.PauseSLAForTenant("1", ticket1ID, "waiting_parts")
	if err != nil {
		t.Fatalf("repeat pause SLA: %v", err)
	}
	if repeatedPause.ID != firstPause.ID || repeatedPause.TenantID != "1" {
		t.Fatalf("pause was not idempotent: first=%+v repeated=%+v", firstPause, repeatedPause)
	}
	var activeCount int64
	if err := db.Model(&SLAPauseRecord{}).Where("tenant_id = ? AND ticket_id = ? AND resumed_at IS NULL", "1", ticket1ID).Count(&activeCount).Error; err != nil {
		t.Fatalf("count active pauses: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("active pause count = %d, want 1", activeCount)
	}

	firstResume, err := SLAService.ResumeSLAForTenant("1", ticket1ID)
	if err != nil {
		t.Fatalf("resume SLA: %v", err)
	}
	repeatedResume, err := SLAService.ResumeSLAForTenant("1", ticket1ID)
	if err != nil {
		t.Fatalf("repeat resume SLA: %v", err)
	}
	if repeatedResume.ID != firstResume.ID || repeatedResume.ResumedAt == nil {
		t.Fatalf("resume was not idempotent: first=%+v repeated=%+v", firstResume, repeatedResume)
	}
}

func TestSLAPolicyRejectsCrossTenantCalendarAndDuplicateActivePriority(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	calendar := ServiceCalendar{
		ID: "tenant-2-calendar", TenantID: "2", Name: "tenant 2", Timezone: "UTC",
		WorkDays: "[1,2,3,4,5]", WorkHoursStart: "09:00", WorkHoursEnd: "18:00", Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.AutoMigrate(&ServiceCalendar{}); err != nil {
		t.Fatalf("migrate calendar: %v", err)
	}
	if err := db.Create(&calendar).Error; err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	if _, err := SLAService.CreateSLAPolicy(CreateSLAPolicyInput{
		TenantID: "1", Name: "cross calendar", Priority: "p2", FRTMinutes: 10, CalendarID: calendar.ID,
	}); err == nil {
		t.Fatal("expected cross-tenant calendar binding to fail")
	}
	if _, err := SLAService.CreateSLAPolicy(CreateSLAPolicyInput{
		TenantID: "1", Name: "primary", Priority: "p2", FRTMinutes: 10,
	}); err != nil {
		t.Fatalf("create primary policy: %v", err)
	}
	if _, err := SLAService.CreateSLAPolicy(CreateSLAPolicyInput{
		TenantID: "1", Name: "duplicate", Priority: "p2", ResolutionMinutes: 30,
	}); err == nil {
		t.Fatal("expected duplicate active priority to fail")
	}
	if _, err := SLAService.CreateSLAPolicy(CreateSLAPolicyInput{
		TenantID: "1", Name: "negative", Priority: "p3", FRTMinutes: -1,
	}); err == nil {
		t.Fatal("expected negative SLA target to fail")
	}
}
