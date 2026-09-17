package repositories

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"time"
)

// syncTicketCaseColumns keeps legacy technical operations and the standard case
// lifecycle in one transaction. Explicit case commands provide their own state.
func SyncTicketCaseColumns(db *gorm.DB, id int64, columns map[string]interface{}) error {
	if err := GuardTicketRelationsDB(db, id, columns); err != nil {
		return err
	}
	_, statusChanged := columns["status"]
	_, assigneeChanged := columns["current_assignee_id"]
	if _, explicit := columns["case_status"]; explicit || (!statusChanged && !assigneeChanged) {
		return nil
	}
	var ticket models.Ticket
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, id).Error; err != nil {
		return err
	}
	if ticket.CaseStatus == "" {
		return nil
	}
	next := ticket.CaseStatus
	status := ticket.Status
	if statusChanged {
		status = enums.TicketStatus(fmt.Sprint(columns["status"]))
	}
	assignee := ticket.CurrentAssigneeID
	if assigneeChanged {
		switch v := columns["current_assignee_id"].(type) {
		case int64:
			assignee = v
		case int:
			assignee = int64(v)
		}
	}
	switch status {
	case enums.TicketStatusClosed, enums.TicketStatusDone:
		next = "closed"
	case enums.TicketStatusCancelled:
		next = "cancelled"
	case enums.TicketStatusReopened:
		next = "in_triage"
		columns["restored_at"] = nil
		columns["case_resolution"] = ""
		columns["waiting_reason"] = ""
		columns["case_resume_status"] = ""
		columns["case_resume_technical_status"] = ""
	case enums.TicketStatusResolved:
		next = "resolved"
	case enums.TicketStatusWaitingCustomer:
		next = "waiting"
	case enums.TicketStatusPendingCustomerConfirm:
		if ticket.ResolvedAt != nil || columns["resolved_at"] != nil {
			next = "closure_pending"
		} else {
			next = "waiting"
		}
	default:
		// Reassignment does not silently resume waiting or revoke restore/resolve.
		if next == "acknowledged" || next == "in_triage" || next == "assigned" {
			if assignee > 0 {
				next = "assigned"
			} else if next == "assigned" {
				next = "in_triage"
			}
		}
	}
	// Ownership follows the engineer who currently handles the case; there is no
	// separate support-agent owner. Returning to the dispatch pool clears it.
	if assigneeChanged {
		if _, explicitlyPreserved := columns["case_owner_id"]; !explicitlyPreserved {
			columns["case_owner_id"] = assignee
		}
	}
	if next == ticket.CaseStatus {
		return nil
	}
	if ticket.CaseStatus == "waiting" && next != "waiting" {
		// Reasons remain in progress history; the current summary must no longer
		// describe an ended waiting period after a repair, cancellation or close.
		columns["waiting_reason"] = ""
		columns["case_resume_status"] = ""
		columns["case_resume_technical_status"] = ""
	}
	columns["case_status"] = next
	columns["case_revision"] = gorm.Expr("case_revision + 1")
	metadata, _ := json.Marshal(map[string]any{"from_case_status": ticket.CaseStatus, "to_case_status": next, "source": "technical_workflow"})
	actorID := int64(0)
	switch v := columns["update_user_id"].(type) {
	case int64:
		actorID = v
	case int:
		actorID = int64(v)
	}
	return db.Create(&models.TicketProgress{TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: "case_status_changed", Content: "工单主状态：" + ticket.CaseStatus + " → " + next, MetadataJSON: string(metadata), AuthorID: actorID, CreatedAt: time.Now()}).Error
}
