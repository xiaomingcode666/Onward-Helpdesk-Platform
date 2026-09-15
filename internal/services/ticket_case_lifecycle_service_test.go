package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func setupCaseLifecycle(t *testing.T) (*gorm.DB, *dto.AuthPrincipal, models.Ticket) {
	t.Helper()
	db := setupCaseOwnerTestDB(t)
	if err := db.AutoMigrate(&models.TicketCaseOperation{}, &models.MeetingRoomJitsi{}, &models.TicketRepairRecord{},
		&models.TicketDispatchAttempt{}, &models.TicketSupplierCollaboration{}, &models.TicketSupplierCollaborationParticipant{},
		&models.PartnerAuthorizationScope{}, &models.KnowledgeCandidate{}, &models.TicketQualityClue{}, &models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatal(err)
	}
	user, _, _ := seedCaseOwnerMember(t, db, 91, "case-customer-service", constants.PermissionTicketChangeStatus.Code)
	op := caseOwnerPrincipal(user)
	op.Permissions = append(op.Permissions, constants.PermissionTicketChangeStatus.Code)
	ticket := models.Ticket{TenantID: 91, TicketNo: "CASE-" + t.Name(), Title: "通用咨询", Status: enums.TicketStatusPendingAcceptance, CaseStatus: "new", AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	return db, op, ticket
}

func runCaseAction(t *testing.T, db *gorm.DB, ticketID int64, action, reason string, op *dto.AuthPrincipal) models.Ticket {
	t.Helper()
	ticket := repositories.TicketRepository.Get(db, ticketID)
	if err := ExecuteTicketCaseCommand(ticketID, TicketCaseCommand{Action: action, Reason: reason, ExpectedStatus: models.EffectiveTicketCaseStatus(*ticket), ExpectedRevision: &ticket.CaseRevision, IdempotencyKey: fmt.Sprintf("%s-%d", action, ticket.CaseRevision)}, op); err != nil {
		t.Fatalf("%s: %v", action, err)
	}
	return *repositories.TicketRepository.Get(db, ticketID)
}

func TestTicketCaseLifecycleTenStatesAndOwnerRetention(t *testing.T) {
	db, op, ticket := setupCaseLifecycle(t)
	accepted := runCaseAction(t, db, ticket.ID, "acknowledge", "客服实际受理", op)
	if accepted.CaseStatus != "acknowledged" || accepted.CaseOwnerID != op.UserID || accepted.AcknowledgedAt == nil || accepted.AcceptedAt != nil {
		t.Fatalf("reception must be separate from engineer acceptance: %+v", accepted)
	}
	runCaseAction(t, db, ticket.ID, "triage", "", op)
	if err := repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"status": enums.TicketStatusPendingAssigneeAccept, "current_assignee_id": int64(902), "current_team_id": int64(72)}); err != nil {
		t.Fatal(err)
	}
	assigned := repositories.TicketRepository.Get(db, ticket.ID)
	if assigned.CaseStatus != "assigned" || assigned.CaseOwnerID != op.UserID {
		t.Fatalf("assignment lost lifecycle/owner: %+v", assigned)
	}
	waiting := runCaseAction(t, db, ticket.ID, "wait", "内部排查线索，不应公开", op)
	if waiting.CaseStatus != "waiting" || waiting.ResolvedAt != nil {
		t.Fatal("waiting is not resolution")
	}
	if err := TicketLifecycleService.Close(ticket.ID, "不能跳过解决", op); err == nil {
		t.Fatal("waiting case closed")
	}
	if err := repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"current_assignee_id": int64(903), "current_team_id": int64(73)}); err != nil {
		t.Fatal(err)
	}
	resumed := runCaseAction(t, db, ticket.ID, "resume", "", op)
	if resumed.CaseStatus != "assigned" || resumed.Status != enums.TicketStatusPendingAssigneeAccept || resumed.AcceptedAt != nil || resumed.WaitingReason != "" {
		t.Fatalf("resume fabricated acceptance or lost state: %+v", resumed)
	}
	restored := runCaseAction(t, db, ticket.ID, "restore", "已核对服务可用，根因仍待修复", op)
	if restored.CaseStatus != "restored" || restored.RestoredAt == nil || restored.ResolvedAt != nil || ticketResolutionSLAStopped(restored) {
		t.Fatal("restoration must not stop the resolution clock")
	}
	if err := TicketLifecycleService.Close(ticket.ID, "不能仅凭恢复关闭", op); err == nil {
		t.Fatal("restored case closed")
	}
	triaged := runCaseAction(t, db, ticket.ID, "triage", "继续排查根因", op)
	if triaged.RestoredAt == nil || !triaged.RestoredAt.Equal(*restored.RestoredAt) {
		t.Fatal("continuing analysis erased an actual restoration timestamp")
	}
	resolved := runCaseAction(t, db, ticket.ID, "resolve", "根因已修复并完成验证", op)
	if resolved.CaseStatus != "resolved" || resolved.ResolvedAt == nil || resolved.CaseResolution == "" {
		t.Fatal("resolution evidence missing")
	}
	pending := runCaseAction(t, db, ticket.ID, "request_closure", "", op)
	if pending.CaseStatus != "closure_pending" {
		t.Fatal("missing closure pending")
	}
	if err := TicketLifecycleService.Close(ticket.ID, "客户确认解决", op); err != nil {
		t.Fatal(err)
	}
	closed := repositories.TicketRepository.Get(db, ticket.ID)
	if closed.CaseStatus != "closed" || closed.HandledAt == nil || closed.CaseOwnerID != op.UserID {
		t.Fatalf("bad close: %+v", closed)
	}
	if err := TicketLifecycleService.Reopen(ticket.ID, "客户反馈问题再次发生", op); err != nil {
		t.Fatal(err)
	}
	reopened := repositories.TicketRepository.Get(db, ticket.ID)
	if reopened.CaseStatus != "in_triage" || reopened.CaseOwnerID != op.UserID || reopened.ResolvedAt != nil || reopened.RestoredAt != nil || reopened.CaseResolution != "" {
		t.Fatalf("reopen must retain owner and reset current resolution: %+v", reopened)
	}
	if err := TicketLifecycleService.Cancel(ticket.ID, "客户撤销本次请求", op); err != nil {
		t.Fatal(err)
	}
	cancelled := repositories.TicketRepository.Get(db, ticket.ID)
	if cancelled.CaseStatus != "cancelled" || cancelled.CaseOwnerID != op.UserID {
		t.Fatalf("bad cancellation: %+v", cancelled)
	}
	var publicProgress []models.TicketProgress
	if err := db.Where("ticket_id = ? AND visible_to_customer = ?", ticket.ID, true).Find(&publicProgress).Error; err != nil {
		t.Fatal(err)
	}
	if len(publicProgress) < 5 {
		t.Fatal("customer lifecycle progress missing")
	}
	for _, p := range publicProgress {
		if strings.Contains(p.Content+p.MetadataJSON, "内部排查线索") {
			t.Fatal("internal reason leaked to customer")
		}
	}
}

func TestTicketCaseLifecycleAcknowledgementAuditAndAtomicFailure(t *testing.T) {
	db, op, ticket := setupCaseLifecycle(t)
	runCaseAction(t, db, ticket.ID, "acknowledge", "受理", op)
	var audit models.TicketProgress
	if err := db.Where("ticket_id = ? AND visible_to_customer = ?", ticket.ID, false).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(audit.MetadataJSON), &metadata); err != nil || metadata["case_owner_id"] != float64(op.UserID) {
		t.Fatalf("acknowledgement must audit real owner: %s", audit.MetadataJSON)
	}
	before := repositories.TicketRepository.Get(db, ticket.ID)
	if err := db.Callback().Create().Before("gorm:create").Register("fail_case_audit", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "TicketProgress" {
			tx.AddError(errors.New("audit unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := ExecuteTicketCaseCommand(ticket.ID, TicketCaseCommand{Action: "triage", ExpectedStatus: before.CaseStatus, ExpectedRevision: &before.CaseRevision, IdempotencyKey: "failed-audit"}, op)
	if err == nil {
		t.Fatal("mutation succeeded without audit")
	}
	after := repositories.TicketRepository.Get(db, ticket.ID)
	if after.CaseRevision != before.CaseRevision || after.CaseStatus != before.CaseStatus {
		t.Fatal("audit error did not roll back case mutation")
	}
	var count int64
	db.Model(&models.TicketCaseOperation{}).Where("operation_key = ?", "failed-audit").Count(&count)
	if count != 0 {
		t.Fatal("failed operation left an idempotency receipt")
	}
}

func TestTicketCaseLifecycleConflictsAndBypassGuards(t *testing.T) {
	db, op, ticket := setupCaseLifecycle(t)
	cmd := TicketCaseCommand{Action: "acknowledge", ExpectedStatus: "new", ExpectedRevision: &ticket.CaseRevision, IdempotencyKey: "stable"}
	if err := ExecuteTicketCaseCommand(ticket.ID, cmd, op); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteTicketCaseCommand(ticket.ID, cmd, op); err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}
	cmd.Reason = "different payload"
	if err := ExecuteTicketCaseCommand(ticket.ID, cmd, op); !errors.Is(err, ErrTicketCaseConflict) {
		t.Fatalf("changed payload error = %v", err)
	}
	cmd.IdempotencyKey = "stale"
	if err := ExecuteTicketCaseCommand(ticket.ID, cmd, op); !errors.Is(err, ErrTicketCaseConflict) {
		t.Fatalf("stale revision error = %v", err)
	}
	if err := TicketService.ChangeStatus(request.ChangeTicketStatusRequest{TicketID: ticket.ID, Status: "in_progress"}, op); err == nil {
		t.Fatal("legacy direct state API bypassed canonical flow")
	}
	if _, err := TicketService.Transition(request.TransitionTicketRequest{TicketID: ticket.ID, Status: "pending_dispatch"}, op); err == nil {
		t.Fatal("legacy transition bypassed canonical flow")
	}
	readOnly := *op
	readOnly.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketAssign.Code}
	if actions := BuildTicketCaseLifecycle(repositories.TicketRepository.Get(db, ticket.ID), &readOnly).AllowedActions; len(actions) != 0 {
		t.Fatalf("offered actions absent handler permission: %v", actions)
	}
}

func TestTicketCaseLifecycleLegacyReadAndAutomaticAssignmentDoNotInventReception(t *testing.T) {
	db, _, ticket := setupCaseLifecycle(t)
	if err := repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"status": enums.TicketStatusPendingAssigneeAccept, "current_assignee_id": int64(303)}); err != nil {
		t.Fatal(err)
	}
	current := repositories.TicketRepository.Get(db, ticket.ID)
	if current.CaseStatus != "new" || current.CaseOwnerID != 0 || current.AcknowledgedAt != nil || current.AcceptedAt != nil {
		t.Fatalf("automatic assignment fabricated reception: %+v", current)
	}
	legacy := models.Ticket{Status: enums.TicketStatusPendingCustomerConfirm}
	lifecycle := BuildTicketCaseLifecycle(&legacy, nil)
	if lifecycle.Status != "waiting" || !lifecycle.LegacyRecord || lifecycle.OwnerID != 0 || lifecycle.AcknowledgedAt != "" {
		t.Fatalf("legacy waiting falsely resolved/owned: %+v", lifecycle)
	}
}
