package services

import (
	"errors"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	ticketDispatchOutcomePending    = "pending"
	ticketDispatchOutcomeAccepted   = "accepted"
	ticketDispatchOutcomeTimedOut   = "timed_out"
	ticketDispatchOutcomeSuperseded = "superseded"
	ticketDispatchOutcomeEscalated  = "escalated"
	ticketDispatchOutcomeClosed     = "closed"
	ticketDispatchOutcomeCancelled  = "cancelled"
	ticketDispatchOutcomeFailed     = "failed"
)

func ticketAssignmentTrackingUpdates(now time.Time) map[string]any {
	return ticketAssignmentTrackingUpdatesForTicket(nil, now)
}

func ticketAssignmentTrackingUpdatesForTicket(ticket *models.Ticket, now time.Time) map[string]any {
	return ticketAssignmentTrackingUpdatesForTicketDB(sqls.DB(), ticket, now)
}

func ticketAssignmentTrackingUpdatesForTicketDB(db *gorm.DB, ticket *models.Ticket, now time.Time) map[string]any {
	deadline := ticketAssignmentDeadlineDB(db, ticket, now)
	return map[string]any{
		"assigned_at":                  now,
		"accepted_at":                  nil,
		"accept_deadline_at":           deadline,
		"dispatch_deferred_until":      nil,
		"last_dispatch_failure_reason": "",
	}
}

func addTicketAssignmentTracking(updates map[string]any, now time.Time) {
	for key, value := range ticketAssignmentTrackingUpdates(now) {
		updates[key] = value
	}
}

func addTicketAssignmentTrackingForTicket(updates map[string]any, ticket *models.Ticket, now time.Time) {
	for key, value := range ticketAssignmentTrackingUpdatesForTicket(ticket, now) {
		updates[key] = value
	}
}

func ticketAssignmentDeadline(ticket *models.Ticket, now time.Time) time.Time {
	return ticketAssignmentDeadlineDB(sqls.DB(), ticket, now)
}

func addTicketAssignmentTrackingForTicketDB(updates map[string]any, db *gorm.DB, ticket *models.Ticket, now time.Time) {
	for key, value := range ticketAssignmentTrackingUpdatesForTicketDB(db, ticket, now) {
		updates[key] = value
	}
}

func addTicketDispatchSettlementUpdates(updates map[string]any) {
	updates["accept_deadline_at"] = nil
	updates["dispatch_deferred_until"] = nil
	updates["last_dispatch_failure_reason"] = ""
}

func addTicketDispatchPoolUpdates(updates map[string]any) {
	updates["current_assignee_id"] = 0
	updates["assigned_at"] = nil
	updates["accepted_at"] = nil
	addTicketDispatchSettlementUpdates(updates)
}

func ticketAssignmentDeadlineDB(db *gorm.DB, ticket *models.Ticket, now time.Time) time.Time {
	deadline := now.Add(time.Duration(acceptDeadlineMinutes()) * time.Minute)
	if slaDeadline, ok := ticketAssignmentSLADeadlineDB(db, ticket); ok && slaDeadline.Before(deadline) {
		return slaDeadline
	}
	return deadline
}

func ticketAssignmentSLADeadline(ticket *models.Ticket) (time.Time, bool) {
	return ticketAssignmentSLADeadlineDB(sqls.DB(), ticket)
}

func ticketAssignmentSLADeadlineDB(db *gorm.DB, ticket *models.Ticket) (time.Time, bool) {
	if ticket == nil || ticket.TenantID <= 0 || ticket.CreatedAt.IsZero() {
		return time.Time{}, false
	}
	if db == nil || !db.Migrator().HasTable(&SLAPolicy{}) {
		return time.Time{}, false
	}
	var policy SLAPolicy
	err := db.Where("tenant_id = ? AND priority = ? AND status = ? AND assignment_minutes > ?",
		formatID(ticket.TenantID), ticketSLAPriority(*ticket), "active", 0).
		Order("updated_at DESC").
		First(&policy).Error
	if err != nil {
		return time.Time{}, false
	}
	pausedDuration := time.Duration(0)
	if ticket.ID > 0 && db.Migrator().HasTable(&SLAPauseRecord{}) {
		pausedDuration = time.Duration(totalPausedDurationForTenantDB(db, formatID(ticket.TenantID), formatID(ticket.ID))) * time.Second
	}
	return ticket.CreatedAt.Add(time.Duration(policy.AssignmentMinutes) * time.Minute).Add(pausedDuration), true
}

func totalPausedDurationForTenantDB(db *gorm.DB, tenantID, ticketID string) int64 {
	if db == nil || tenantID == "" || ticketID == "" {
		return 0
	}
	var total int64
	db.Model(&SLAPauseRecord{}).
		Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Select("COALESCE(SUM(duration), 0)").
		Scan(&total)
	var active SLAPauseRecord
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND resumed_at IS NULL", tenantID, ticketID).
		Order("paused_at DESC").
		First(&active).Error; err == nil {
		total += int64(time.Since(active.PausedAt).Seconds())
	}
	return total
}

func ticketAssignmentStatus(current enums.TicketStatus) enums.TicketStatus {
	if enums.IsValidAfterSalesStatus(string(current)) {
		return enums.TicketStatusPendingAssigneeAccept
	}
	return enums.TicketStatusAssigned
}

func canAssignTicketStatus(current enums.TicketStatus) bool {
	switch current {
	case enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled:
		return false
	default:
		return true
	}
}

func createTicketDispatchAttemptTx(db *gorm.DB, ticket *models.Ticket, teamID, assigneeID int64, reason string, operator *dto.AuthPrincipal, now time.Time) error {
	if db == nil || ticket == nil || ticket.ID <= 0 || assigneeID <= 0 {
		return nil
	}
	if previous, err := repositories.TicketDispatchAttemptRepository.FindPendingForUpdate(db, ticket.ID); err == nil {
		if err := repositories.TicketDispatchAttemptRepository.Updates(db, previous.ID, map[string]any{
			"outcome":    ticketDispatchOutcomeSuperseded,
			"ended_at":   now,
			"updated_at": now,
		}); err != nil {
			return err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	count, err := repositories.TicketDispatchAttemptRepository.CountByTicket(db, ticket.ID)
	if err != nil {
		return err
	}
	deadline := ticketAssignmentDeadlineDB(db, ticket, now)
	audit := utils.BuildAuditFields(operator)
	audit.CreatedAt = now
	audit.UpdatedAt = now
	return repositories.TicketDispatchAttemptRepository.Create(db, &models.TicketDispatchAttempt{
		TenantID:         ticket.TenantID,
		TicketID:         ticket.ID,
		TeamID:           teamID,
		AssigneeID:       assigneeID,
		AttemptNo:        int(count) + 1,
		Outcome:          ticketDispatchOutcomePending,
		Reason:           strings.TrimSpace(reason),
		AssignedAt:       now,
		AcceptDeadlineAt: &deadline,
		AuditFields:      audit,
	})
}

func finishPendingTicketDispatchAttemptTx(db *gorm.DB, ticketID int64, outcome string, acceptedAt *time.Time, now time.Time) error {
	if db == nil || ticketID <= 0 {
		return nil
	}
	attempt, err := repositories.TicketDispatchAttemptRepository.FindPendingForUpdate(db, ticketID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return repositories.TicketDispatchAttemptRepository.Updates(db, attempt.ID, map[string]any{
		"outcome":     outcome,
		"accepted_at": acceptedAt,
		"ended_at":    now,
		"updated_at":  now,
	})
}

func createFinishedTicketDispatchAttemptTx(db *gorm.DB, ticket *models.Ticket, teamID, assigneeID int64, outcome, reason string, operator *dto.AuthPrincipal, now time.Time) error {
	if db == nil || ticket == nil || ticket.ID <= 0 || assigneeID <= 0 {
		return nil
	}
	count, err := repositories.TicketDispatchAttemptRepository.CountByTicket(db, ticket.ID)
	if err != nil {
		return err
	}
	audit := utils.BuildAuditFields(operator)
	audit.CreatedAt = now
	audit.UpdatedAt = now
	endedAt := now
	var acceptedAt *time.Time
	if outcome == ticketDispatchOutcomeAccepted {
		acceptedAt = &now
	}
	return repositories.TicketDispatchAttemptRepository.Create(db, &models.TicketDispatchAttempt{
		TenantID:    ticket.TenantID,
		TicketID:    ticket.ID,
		TeamID:      teamID,
		AssigneeID:  assigneeID,
		AttemptNo:   int(count) + 1,
		Outcome:     outcome,
		Reason:      strings.TrimSpace(reason),
		AssignedAt:  now,
		AcceptedAt:  acceptedAt,
		EndedAt:     &endedAt,
		AuditFields: audit,
	})
}

func createFailedTicketDispatchAttemptTx(db *gorm.DB, ticket *models.Ticket, teamID int64, reason string, operator *dto.AuthPrincipal, now time.Time) error {
	if db == nil || ticket == nil || ticket.ID <= 0 {
		return nil
	}
	count, err := repositories.TicketDispatchAttemptRepository.CountByTicket(db, ticket.ID)
	if err != nil {
		return err
	}
	audit := utils.BuildAuditFields(operator)
	audit.CreatedAt = now
	audit.UpdatedAt = now
	endedAt := now
	return repositories.TicketDispatchAttemptRepository.Create(db, &models.TicketDispatchAttempt{
		TenantID:    ticket.TenantID,
		TicketID:    ticket.ID,
		TeamID:      teamID,
		AssigneeID:  0,
		AttemptNo:   int(count) + 1,
		Outcome:     ticketDispatchOutcomeFailed,
		Reason:      normalizeDispatchFailureReason(reason),
		AssignedAt:  now,
		EndedAt:     &endedAt,
		AuditFields: audit,
	})
}
