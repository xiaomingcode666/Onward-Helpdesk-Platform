package services

import (
	"strings"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"
)

// Keep the historical SQL projection consistent with EffectiveTicketCaseStatus.
// It reads legacy evidence without inventing acknowledgement or restore dates.
const ticketEffectiveCaseStatusSQL = `COALESCE(NULLIF(case_status, ''), CASE
	WHEN status = 'accepted' THEN 'acknowledged'
	WHEN status IN ('assigned', 'pending_assignee_accept') THEN 'assigned'
	WHEN status IN ('pending_dispatch', 'in_progress', 'processing', 'escalated', 'video_support', 'supplier_support', 'reopened')
		THEN CASE WHEN current_assignee_id > 0 THEN 'assigned' ELSE 'in_triage' END
	WHEN status = 'waiting_customer' THEN 'waiting'
	WHEN status = 'pending_customer_confirm' THEN CASE WHEN resolved_at IS NOT NULL THEN 'closure_pending' ELSE 'waiting' END
	WHEN status = 'resolved' THEN 'resolved'
	WHEN status = 'quality_review' THEN CASE WHEN handled_at IS NOT NULL THEN 'closed' WHEN resolved_at IS NOT NULL THEN 'resolved' ELSE 'in_triage' END
	WHEN status IN ('closed', 'done') THEN 'closed'
	WHEN status = 'cancelled' THEN 'cancelled'
	ELSE 'new' END)`

const ticketCaseOpenSQL = "(" + ticketEffectiveCaseStatusSQL + " NOT IN ('closed', 'cancelled'))"
const ticketResolutionSLARunningSQL = "(" + ticketCaseOpenSQL + " AND resolved_at IS NULL)"
const ticketCaseAwaitingClosureSQL = "(" + ticketEffectiveCaseStatusSQL + " IN ('resolved', 'closure_pending'))"

func ticketCaseAwaitingClosure(ticket models.Ticket) bool {
	state := models.EffectiveTicketCaseStatus(ticket)
	return state == "resolved" || state == "closure_pending"
}

func validateTicketCaseStatusFilter(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "", "new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled":
		return status, nil
	default:
		return "", errorsx.InvalidParam("invalid case_status")
	}
}

func buildTicketCaseSummary(ticket models.Ticket) dto.TicketCaseSummaryDTO {
	ownerName := ""
	if ticket.CaseOwnerID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ticket.CaseOwnerID); user != nil {
			ownerName = customerPortalFirstNonEmptyString(user.Nickname, user.Username)
		}
	}
	return ticketCaseSummaryWithOwner(ticket, ownerName)
}

func ticketCaseSummaryWithOwner(ticket models.Ticket, ownerName string) dto.TicketCaseSummaryDTO {
	return dto.TicketCaseSummaryDTO{
		MergedIntoID: ticket.MergedIntoID,
		CaseType:     ticket.CaseType, PriorityLevel: ticketEffectivePriority(ticket), PriorityReviewRequired: false,
		CaseStatus: models.EffectiveTicketCaseStatus(ticket), CaseStatusRecorded: ticket.CaseStatus != "",
		CaseOwnerID: ticket.CaseOwnerID, CaseOwnerName: ownerName,
		AcknowledgedAt: formatEnterpriseTimePtr(ticket.AcknowledgedAt), RestoredAt: formatEnterpriseTimePtr(ticket.RestoredAt),
	}
}

// BuildTicketCaseSummary reuses caller-preloaded names without performing a query.
func BuildTicketCaseSummary(ticket models.Ticket, ownerName string) dto.TicketCaseSummaryDTO {
	return ticketCaseSummaryWithOwner(ticket, ownerName)
}

func ticketCaseClosed(ticket models.Ticket) bool {
	switch models.EffectiveTicketCaseStatus(ticket) {
	case "closed", "cancelled":
		return true
	default:
		return false
	}
}

func ticketResolutionSLAStopped(ticket models.Ticket) bool {
	return ticketCaseClosed(ticket) || ticket.ResolvedAt != nil
}
