package services

import (
	"testing"
	"time"

	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func TestTicketCaseWorkflowRepairCannotReactivateTerminalCase(t *testing.T) {
	for _, status := range []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusCancelled} {
		t.Run(string(status), func(t *testing.T) {
			db, op, ticket := setupCaseLifecycle(t)
			runCaseAction(t, db, ticket.ID, "acknowledge", "实际受理", op)
			op.Roles = []string{EnterpriseRoleAdmin}
			now := time.Now().Add(-time.Hour)
			if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{
				"status": status, "case_status": string(status), "handled_at": now, "product_id": 72,
			}).Error; err != nil {
				t.Fatal(err)
			}
			before := repositories.TicketRepository.Get(db, ticket.ID)
			_, err := TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
				TicketID: ticket.ID, MarkResolved: true, FaultCode: "BOOT-001", RootCause: "已确认根因",
				Solution: "已执行修复", TestResult: "passed",
			}, op)
			if err == nil {
				t.Fatal("repair conclusion reactivated a terminal case without reopening")
			}
			after := repositories.TicketRepository.Get(db, ticket.ID)
			if after.Status != before.Status || after.CaseStatus != before.CaseStatus || after.CaseRevision != before.CaseRevision {
				t.Fatalf("terminal case changed: %+v", after)
			}
			var count int64
			if err := db.Model(&models.TicketRepairRecord{}).Where("ticket_id = ?", ticket.ID).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("rejected conclusion wrote a repair record: count=%d err=%v", count, err)
			}
		})
	}
}

func TestTicketCaseWorkflowConversationTeamChangeRetainsAcceptance(t *testing.T) {
	db, op, ticket := setupCaseLifecycle(t)
	runCaseAction(t, db, ticket.ID, "acknowledge", "实际受理", op)
	acceptedAt := time.Now().Add(-30 * time.Minute)
	assignedAt := acceptedAt.Add(-time.Minute)
	if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{
		"status": enums.TicketStatusProcessing, "case_status": "assigned", "conversation_id": 88,
		"current_team_id": 1, "current_assignee_id": op.UserID, "accepted_at": acceptedAt, "assigned_at": assignedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return TicketService.SyncConversationDispatchTx(tx, 88, 2, op.UserID, "调整服务组，处理人不变", op)
	}); err != nil {
		t.Fatal(err)
	}
	after := repositories.TicketRepository.Get(db, ticket.ID)
	if after.CurrentTeamID != 2 || after.CaseStatus != "assigned" || after.Status != enums.TicketStatusProcessing || after.AcceptedAt == nil || !after.AcceptedAt.Equal(acceptedAt) || after.AssignedAt == nil || !after.AssignedAt.Equal(assignedAt) || after.AcceptDeadlineAt != nil {
		t.Fatalf("team-only synchronization reset engineer acceptance: %+v", after)
	}
}

func TestTicketCaseWorkflowResolvedAssignmentRequiresReopen(t *testing.T) {
	for _, state := range []string{"resolved", "closure_pending"} {
		t.Run(state, func(t *testing.T) {
			db := setupHumanDispatchRealtimeTestDB(t)
			createHumanDispatchRealtimeTeam(t, db, 1)
			createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
			createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
			now := time.Now().Add(-time.Hour)
			status := enums.TicketStatusResolved
			if state == "closure_pending" {
				status = enums.TicketStatusPendingCustomerConfirm
			}
			ticket := models.Ticket{TenantID: 1, TicketNo: "CASE-RESOLVED-" + state, Title: "已解决工单",
				Status: status, CaseStatus: state, CaseOwnerID: 101, CurrentAssigneeID: 101, CurrentTeamID: 1,
				ConversationID: 88, AcknowledgedAt: &now, AcceptedAt: &now, AssignedAt: &now, ResolvedAt: &now,
				AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
			if err := db.Create(&ticket).Error; err != nil {
				t.Fatal(err)
			}
			op := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}
			if err := TicketLifecycleService.Assign(ticket.ID, 102, "改派处理人", op); err == nil {
				t.Error("manual assignment changed a resolved case without reopening")
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				return TicketService.SyncConversationDispatchTx(tx, 88, 1, 102, "随会话改派", op)
			}); err == nil {
				t.Error("conversation assignment changed a resolved case without reopening")
			}
			after := repositories.TicketRepository.Get(db, ticket.ID)
			if after.CurrentAssigneeID != 101 || after.Status != status || after.CaseStatus != state || after.ResolvedAt == nil || !after.ResolvedAt.Equal(now) {
				t.Fatalf("assignment changed resolved lifecycle: %+v", after)
			}
		})
	}
}

func TestTicketCaseWorkflowAcceptRetryPreservesOriginalAcceptance(t *testing.T) {
	for _, status := range []enums.TicketStatus{enums.TicketStatusWaitingCustomer, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport} {
		t.Run(string(status), func(t *testing.T) {
			db := setupHumanDispatchRealtimeTestDB(t)
			createHumanDispatchRealtimeTeam(t, db, 1)
			createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
			now := time.Now().Add(-time.Hour)
			state := "assigned"
			if status == enums.TicketStatusWaitingCustomer {
				state = "waiting"
			}
			ticket := models.Ticket{TenantID: 1, TicketNo: "CASE-ACCEPT-" + string(status), Title: "处理中的工单",
				Status: status, CaseStatus: state, CaseOwnerID: 101, CurrentAssigneeID: 101, CurrentTeamID: 1,
				AcknowledgedAt: &now, AcceptedAt: &now, AssignedAt: &now,
				AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
			if err := db.Create(&ticket).Error; err != nil {
				t.Fatal(err)
			}
			op := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}
			if err := TicketLifecycleService.Accept(ticket.ID, op.UserID, op); err != nil {
				t.Fatal(err)
			}
			after := repositories.TicketRepository.Get(db, ticket.ID)
			if after.AcceptedAt == nil || !after.AcceptedAt.Equal(now) || after.CaseStatus != state || after.Status != status || after.CaseRevision != ticket.CaseRevision {
				t.Fatalf("accept retry changed an already accepted workflow: %+v", after)
			}
		})
	}
}

func TestTicketCaseWorkflowLegacyStatusChangeCannotRaceAdoption(t *testing.T) {
	for _, action := range []string{"change_status", "transition"} {
		t.Run(action, func(t *testing.T) {
			db, op, ticket := setupCaseLifecycle(t)
			if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{
				"case_status": "", "status": enums.TicketStatusProcessing,
			}).Error; err != nil {
				t.Fatal(err)
			}
			adopted := false
			if err := db.Callback().Query().After("gorm:query").Register("adopt_between_legacy_read_and_write", func(tx *gorm.DB) {
				if adopted || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Ticket" {
					return
				}
				adopted = true
				err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{
					"case_status": "in_triage", "case_owner_id": op.UserID, "acknowledged_at": time.Now(), "case_revision": 1,
				}).Error
				if err != nil {
					tx.AddError(err)
				}
			}); err != nil {
				t.Fatal(err)
			}
			var err error
			if action == "change_status" {
				err = TicketService.ChangeStatus(request.ChangeTicketStatusRequest{TicketID: ticket.ID, Status: string(enums.TicketStatusVideoSupport)}, op)
			} else {
				_, err = TicketService.Transition(request.TransitionTicketRequest{TicketID: ticket.ID, Status: string(enums.TicketStatusVideoSupport)}, op)
			}
			if !adopted {
				t.Fatal("adoption hook did not execute")
			}
			if err == nil {
				t.Error("legacy status write bypassed canonical operations after concurrent adoption")
			}
			after := repositories.TicketRepository.Get(db, ticket.ID)
			if after.Status != enums.TicketStatusProcessing || after.CaseStatus != "in_triage" || after.CaseRevision != 1 {
				t.Fatalf("stale legacy write mutated newly adopted lifecycle: %+v", after)
			}
		})
	}
}
