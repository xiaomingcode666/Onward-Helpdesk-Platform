package models

import (
	"remotehelpdesk/internal/pkg/enums"
	"time"
)

// TicketCaseOperation deduplicates commands under the ticket's transaction lock.
type TicketCaseOperation struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	TenantID       int64  `gorm:"not null;uniqueIndex:uk_case_operation,priority:1"`
	TicketID       int64  `gorm:"not null;uniqueIndex:uk_case_operation,priority:2"`
	OperationKey   string `gorm:"type:varchar(100);not null;uniqueIndex:uk_case_operation,priority:3"`
	PayloadHash    string `gorm:"type:varchar(64);not null"`
	ResultStatus   string `gorm:"type:varchar(32);not null"`
	ResultRevision int64  `gorm:"not null"`
	CreatedAt      time.Time
}

// EffectiveTicketCaseStatus interprets historical workflow values without writing
// missing ownership or timestamps. Recorded case state is the authoritative value.
func EffectiveTicketCaseStatus(ticket Ticket) string {
	if ticket.CaseStatus != "" {
		return ticket.CaseStatus
	}
	switch ticket.Status {
	case enums.TicketStatusAccepted:
		return "acknowledged"
	case enums.TicketStatusAssigned, enums.TicketStatusPendingAssigneeAccept:
		return "assigned"
	case enums.TicketStatusWaitingCustomer:
		return "waiting"
	case enums.TicketStatusPendingCustomerConfirm:
		if ticket.ResolvedAt != nil {
			return "closure_pending"
		}
		return "waiting"
	case enums.TicketStatusResolved:
		return "resolved"
	case enums.TicketStatusQualityReview:
		if ticket.HandledAt != nil {
			return "closed"
		}
		if ticket.ResolvedAt != nil {
			return "resolved"
		}
		return "in_triage"
	case enums.TicketStatusClosed, enums.TicketStatusDone:
		return "closed"
	case enums.TicketStatusCancelled:
		return "cancelled"
	case enums.TicketStatusPendingDispatch, enums.TicketStatusInProgress,
		enums.TicketStatusProcessing, enums.TicketStatusEscalated,
		enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusReopened:
		if ticket.CurrentAssigneeID > 0 {
			return "assigned"
		}
		return "in_triage"
	default:
		return "new"
	}
}
