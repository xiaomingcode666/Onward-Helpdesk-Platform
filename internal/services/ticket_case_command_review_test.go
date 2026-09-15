package services

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func TestTicketCaseTerminalCommandsRejectStaleWritesAndReplayOriginalResult(t *testing.T) {
	for _, action := range []string{"close", "cancel", "reopen"} {
		t.Run(action, func(t *testing.T) {
			db, op, ticket := setupCaseLifecycle(t)
			runCaseAction(t, db, ticket.ID, "acknowledge", "受理", op)
			runCaseAction(t, db, ticket.ID, "triage", "", op)
			if action == "cancel" {
				runCaseAction(t, db, ticket.ID, "wait", "等待客户确认", op)
			} else {
				runCaseAction(t, db, ticket.ID, "resolve", "已核对问题解决", op)
			}
			before := repositories.TicketRepository.Get(db, ticket.ID)
			command := TicketCaseCommand{Action: action, Reason: "客户确认本次操作", ExpectedStatus: before.CaseStatus, ExpectedRevision: &before.CaseRevision, IdempotencyKey: "terminal-original"}
			if err := ExecuteTicketCaseCommand(ticket.ID, command, op); err != nil {
				t.Fatal(err)
			}
			if action == "cancel" {
				cancelled := repositories.TicketRepository.Get(db, ticket.ID)
				if cancelled.WaitingReason != "" || cancelled.CaseResumeStatus != "" || cancelled.CaseResumeTechnicalStatus != "" {
					t.Fatal("cancelled case still presents an active waiting period")
				}
			}
			first, err := GetTicketCaseCommandResult(ticket.TenantID, ticket.ID, command.IdempotencyKey)
			if err != nil || first.Revision != before.CaseRevision+1 {
				t.Fatalf("missing committed receipt: %+v, %v", first, err)
			}
			switch action {
			case "close":
				runCaseAction(t, db, ticket.ID, "reopen", "问题再次出现", op)
				runCaseAction(t, db, ticket.ID, "resolve", "再次修复并验证", op)
			case "cancel":
				next, _, _ := seedCaseOwnerMember(t, db, ticket.TenantID, "replacement", constants.PermissionTicketChangeStatus.Code)
				if err := TicketCaseOwnerService.TransferWithKey(ticket.ID, op.UserID, next.ID, "客服换班", "after-cancellation", op); err != nil {
					t.Fatal(err)
				}
			case "reopen":
				runCaseAction(t, db, ticket.ID, "resolve", "重新验证完成", op)
				runCaseAction(t, db, ticket.ID, "close", "客户确认", op)
			}
			latest := repositories.TicketRepository.Get(db, ticket.ID)
			var progressBefore int64
			db.Model(&models.TicketProgress{}).Where("ticket_id = ?", ticket.ID).Count(&progressBefore)
			if err := ExecuteTicketCaseCommand(ticket.ID, command, op); err != nil {
				t.Fatalf("lost response retry failed: %v", err)
			}
			after := repositories.TicketRepository.Get(db, ticket.ID)
			replayed, err := GetTicketCaseCommandResult(ticket.TenantID, ticket.ID, command.IdempotencyKey)
			if err != nil || *replayed != *first || after.CaseRevision != latest.CaseRevision || after.CaseStatus != latest.CaseStatus {
				t.Fatalf("retry changed newer state/result: before=%+v after=%+v receipt=%+v err=%v", latest, after, replayed, err)
			}
			var progressAfter int64
			db.Model(&models.TicketProgress{}).Where("ticket_id = ?", ticket.ID).Count(&progressAfter)
			if progressBefore != progressAfter {
				t.Fatal("retry duplicated audit records")
			}
			changedPayload := command
			changedPayload.Reason = "different payload"
			if err := ExecuteTicketCaseCommand(ticket.ID, changedPayload, op); !errors.Is(err, ErrTicketCaseConflict) {
				t.Fatalf("reused key accepted changed payload: %v", err)
			}
			stale := command
			stale.IdempotencyKey = "new-key-stale-screen"
			if err := ExecuteTicketCaseCommand(ticket.ID, stale, op); !errors.Is(err, ErrTicketCaseConflict) {
				t.Fatalf("stale terminal command accepted: %v", err)
			}
		})
	}
}

func TestTicketCaseTerminalReceiptFailureRollsBackCancellation(t *testing.T) {
	db, op, ticket := setupCaseLifecycle(t)
	runCaseAction(t, db, ticket.ID, "acknowledge", "受理", op)
	before := runCaseAction(t, db, ticket.ID, "wait", "等待资料", op)
	if err := db.Callback().Create().Before("gorm:create").Register("fail_terminal_receipt", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "TicketCaseOperation" {
			tx.AddError(errors.New("receipt storage unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := ExecuteTicketCaseCommand(ticket.ID, TicketCaseCommand{Action: "cancel", Reason: "客户撤回", ExpectedStatus: before.CaseStatus, ExpectedRevision: &before.CaseRevision, IdempotencyKey: "failed-cancel"}, op)
	if err == nil {
		t.Fatal("cancellation committed without receipt")
	}
	after := repositories.TicketRepository.Get(db, ticket.ID)
	if after.CaseStatus != before.CaseStatus || after.Status != before.Status || after.HandledAt != nil || after.CaseRevision != before.CaseRevision {
		t.Fatalf("failed receipt left a cancelled ticket: %+v", after)
	}
}

func TestTicketCaseWaitingAndTriagePreserveActiveTechnicalWork(t *testing.T) {
	for _, status := range []enums.TicketStatus{enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport} {
		t.Run(string(status), func(t *testing.T) {
			db, op, ticket := setupCaseLifecycle(t)
			runCaseAction(t, db, ticket.ID, "acknowledge", "受理", op)
			now := time.Now()
			if err := repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"status": status, "current_assignee_id": op.UserID, "accepted_at": now}); err != nil {
				t.Fatal(err)
			}
			runCaseAction(t, db, ticket.ID, "restore", "服务已恢复，协作继续", op)
			triaged := runCaseAction(t, db, ticket.ID, "triage", "继续分析", op)
			if triaged.Status != status {
				t.Fatalf("triage erased active collaboration: %s", triaged.Status)
			}
			runCaseAction(t, db, ticket.ID, "wait", "等待客户资料", op)
			resumed := runCaseAction(t, db, ticket.ID, "resume", "资料收到", op)
			if resumed.Status != status || resumed.CaseResumeTechnicalStatus != "" || resumed.AcceptedAt == nil || !resumed.AcceptedAt.Equal(now) {
				t.Fatalf("resume lost technical work/acceptance: %+v", resumed)
			}
			runCaseAction(t, db, ticket.ID, "wait", "等待外部反馈", op)
			if err := repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"status": enums.TicketStatusProcessing}); err != nil {
				t.Fatal(err)
			}
			resumed = runCaseAction(t, db, ticket.ID, "resume", "协作已结束", op)
			if resumed.Status != enums.TicketStatusProcessing {
				t.Fatal("resume resurrected completed collaboration")
			}
		})
	}
}
