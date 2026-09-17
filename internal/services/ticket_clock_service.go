package services

import (
	"slices"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	TicketClockPauseReasonExternalExpert        = "external_expert"
	TicketClockPauseSourceSupplierCollaboration = "supplier_collaboration"
)

var TicketClockService = newTicketClockService()

func newTicketClockService() *ticketClockService {
	return &ticketClockService{}
}

type ticketClockService struct{}

type ticketClockInterval struct {
	start time.Time
	end   time.Time
	open  bool
}

func (s *ticketClockService) Compute(ticket models.Ticket, now time.Time) dto.TicketClockDTO {
	startAt := ticket.CreatedAt
	endAt := ticketEndAt(ticket, now)
	if startAt.IsZero() {
		startAt = endAt
	}
	if endAt.Before(startAt) {
		endAt = startAt
	}
	e2eSeconds := int64(endAt.Sub(startAt) / time.Second)
	pausedSeconds := ticket.DaypopClockPausedSeconds
	if ticket.DaypopClockPausedAt != nil {
		activeEnd := now
		if activeEnd.After(endAt) {
			activeEnd = endAt
		}
		if activeEnd.After(*ticket.DaypopClockPausedAt) {
			pausedSeconds += int64(activeEnd.Sub(*ticket.DaypopClockPausedAt) / time.Second)
		}
	}
	if pausedSeconds < 0 {
		pausedSeconds = 0
	}
	if pausedSeconds > e2eSeconds {
		pausedSeconds = e2eSeconds
	}
	reason := ""
	if ticket.DaypopClockPausedAt != nil {
		reason = TicketClockPauseReasonExternalExpert
	}
	return dto.TicketClockDTO{
		E2EStartAt:          formatEnterpriseTime(startAt),
		E2EEndAt:            formatEnterpriseTime(endAt),
		E2ESeconds:          e2eSeconds,
		PausedSeconds:       pausedSeconds,
		AccountableSeconds:  maxInt64(0, e2eSeconds-pausedSeconds),
		ExternalWaitSeconds: pausedSeconds,
		PauseActive:         ticket.DaypopClockPausedAt != nil,
		PauseReason:         reason,
	}
}

func ticketEndAt(ticket models.Ticket, now time.Time) time.Time {
	if ticket.ResolvedAt != nil {
		return *ticket.ResolvedAt
	}
	if ticket.HandledAt != nil {
		return *ticket.HandledAt
	}
	if ticketCaseClosed(ticket) {
		if !ticket.UpdatedAt.IsZero() {
			return ticket.UpdatedAt
		}
	}
	return now
}

func (s *ticketClockService) OpenSupplierWaitTx(
	db *gorm.DB,
	ticketID, collaborationID int64,
	now time.Time,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || ticketID <= 0 || collaborationID <= 0 || !db.Migrator().HasTable(&models.TicketClockPause{}) {
		return nil
	}
	var ticket models.Ticket
	if err := db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		return err
	}
	_, err := repositories.TicketClockPauseRepository.Create(db, &models.TicketClockPause{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		ReasonCode:   TicketClockPauseReasonExternalExpert,
		SourceType:   TicketClockPauseSourceSupplierCollaboration,
		SourceID:     collaborationID,
		StartedAt:    now,
		ApprovedBy:   auditOperatorID(operator),
		EvidenceJSON: `{"approved":"system_supplier_wait"}`,
		Status:       enums.StatusOk,
		AuditFields:  buildClockAuditFields(operator, now),
	})
	if err != nil || ticket.DaypopClockPausedAt != nil {
		return err
	}
	updates := map[string]any{
		"daypop_clock_paused_at":                now,
		"daypop_clock_paused_remaining_seconds": int64(0),
		"updated_at":                            now,
		"update_user_id":                        auditOperatorID(operator),
		"update_user_name":                      auditOperatorName(operator),
	}
	if ticket.SLADueAt != nil {
		updates["daypop_clock_paused_remaining_seconds"] = int64(ticket.SLADueAt.Sub(now) / time.Second)
	}
	return db.Model(&models.Ticket{}).Where("id = ? AND tenant_id = ?", ticket.ID, ticket.TenantID).Updates(updates).Error
}

func (s *ticketClockService) CloseSupplierWaitTx(
	db *gorm.DB,
	ticketID, collaborationID int64,
	now time.Time,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || ticketID <= 0 || collaborationID <= 0 || !db.Migrator().HasTable(&models.TicketClockPause{}) {
		return nil
	}
	var ticket models.Ticket
	if err := db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		return err
	}
	if err := repositories.TicketClockPauseRepository.CloseBySource(
		db,
		ticket.TenantID,
		TicketClockPauseReasonExternalExpert,
		TicketClockPauseSourceSupplierCollaboration,
		collaborationID,
		now,
		auditOperatorID(operator),
		auditOperatorName(operator),
	); err != nil {
		return err
	}
	if len(repositories.TicketClockPauseRepository.FindActiveByTicket(db, ticket.TenantID, ticket.ID)) > 0 {
		return nil
	}
	return s.clearTicketPauseDB(db, &ticket, now, operator)
}

func (s *ticketClockService) CloseAllForTicketTx(
	db *gorm.DB,
	tenantID, ticketID int64,
	now time.Time,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || tenantID <= 0 || ticketID <= 0 || !db.Migrator().HasTable(&models.TicketClockPause{}) {
		return nil
	}
	if err := db.Model(&models.TicketClockPause{}).
		Where("tenant_id = ? AND ticket_id = ? AND ended_at IS NULL", tenantID, ticketID).
		Updates(map[string]any{
			"ended_at":         now,
			"updated_at":       now,
			"update_user_id":   auditOperatorID(operator),
			"update_user_name": auditOperatorName(operator),
		}).Error; err != nil {
		return err
	}
	var ticket models.Ticket
	if err := db.Where("id = ? AND tenant_id = ?", ticketID, tenantID).First(&ticket).Error; err != nil {
		return err
	}
	return s.clearTicketPauseDB(db, &ticket, now, operator)
}

func (s *ticketClockService) clearTicketPauseDB(
	db *gorm.DB,
	ticket *models.Ticket,
	now time.Time,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || ticket == nil || ticket.DaypopClockPausedAt == nil {
		return nil
	}
	duration := now.Sub(*ticket.DaypopClockPausedAt)
	if duration < 0 {
		duration = 0
	}
	updates := map[string]any{
		"daypop_clock_paused_seconds":           gorm.Expr("daypop_clock_paused_seconds + ?", int64(duration/time.Second)),
		"daypop_clock_paused_at":                nil,
		"daypop_clock_paused_remaining_seconds": int64(0),
		"updated_at":                            now,
		"update_user_id":                        auditOperatorID(operator),
		"update_user_name":                      auditOperatorName(operator),
	}
	if ticket.SLADueAt != nil {
		updates["sla_due_at"] = ticket.SLADueAt.Add(duration)
	}
	return db.Model(&models.Ticket{}).
		Where("id = ? AND tenant_id = ?", ticket.ID, ticket.TenantID).
		Updates(updates).Error
}

func (s *ticketClockService) Backfill() error {
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(&models.TicketClockPause{}) {
		return nil
	}
	var collaborations []models.TicketSupplierCollaboration
	if err := db.Where("record_status <> ?", enums.StatusDeleted).Order("id ASC").Find(&collaborations).Error; err != nil {
		return err
	}
	ticketByID := make(map[int64]models.Ticket)
	if ids := uniqueCollaborationTicketIDs(collaborations); len(ids) > 0 {
		var tickets []models.Ticket
		if err := db.Where("id IN ?", ids).Find(&tickets).Error; err != nil {
			return err
		}
		for i := range tickets {
			ticketByID[tickets[i].ID] = tickets[i]
		}
	}
	ticketIDs := make([]int64, 0)
	for i := range collaborations {
		item := collaborations[i]
		endedAt := item.ResolvedAt
		if ticket, ok := ticketByID[item.TicketID]; ok && endedAt == nil && ticketResolutionSLAStopped(ticket) {
			end := ticketEndAt(ticket, time.Now())
			endedAt = &end
		}
		if existing := repositories.TicketClockPauseRepository.FindBySource(
			db,
			item.TenantID,
			TicketClockPauseReasonExternalExpert,
			TicketClockPauseSourceSupplierCollaboration,
			item.ID,
		); existing != nil {
			if existing.EndedAt == nil && endedAt != nil {
				if err := db.Model(&models.TicketClockPause{}).Where("id = ?", existing.ID).Updates(map[string]any{
					"ended_at":   *endedAt,
					"updated_at": *endedAt,
				}).Error; err != nil {
					return err
				}
				ticketIDs = append(ticketIDs, item.TicketID)
			}
			continue
		}
		created, err := repositories.TicketClockPauseRepository.Create(db, &models.TicketClockPause{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			ReasonCode:   TicketClockPauseReasonExternalExpert,
			SourceType:   TicketClockPauseSourceSupplierCollaboration,
			SourceID:     item.ID,
			StartedAt:    item.InvitedAt,
			EndedAt:      endedAt,
			ApprovedBy:   0,
			EvidenceJSON: `{"backfill":"supplier_collaboration"}`,
			Status:       enums.StatusOk,
			AuditFields:  models.AuditFields{CreatedAt: item.InvitedAt, UpdatedAt: item.InvitedAt},
		})
		if err != nil {
			return err
		}
		if created {
			ticketIDs = append(ticketIDs, item.TicketID)
		}
	}
	slices.Sort(ticketIDs)
	ticketIDs = slices.Compact(ticketIDs)
	for _, ticketID := range ticketIDs {
		if err := s.recalculateFromPausesDB(db, ticketID, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func uniqueCollaborationTicketIDs(items []models.TicketSupplierCollaboration) []int64 {
	ids := make([]int64, 0, len(items))
	for i := range items {
		if items[i].TicketID > 0 {
			ids = append(ids, items[i].TicketID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func (s *ticketClockService) recalculateFromPausesDB(db *gorm.DB, ticketID int64, now time.Time) error {
	var ticket models.Ticket
	if err := db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		return err
	}
	rows := repositories.TicketClockPauseRepository.FindByTicket(db, ticket.TenantID, ticket.ID)
	intervals := make([]ticketClockInterval, 0, len(rows))
	for i := range rows {
		if rows[i].StartedAt.IsZero() {
			continue
		}
		end := now
		open := rows[i].EndedAt == nil
		if rows[i].EndedAt != nil {
			end = *rows[i].EndedAt
		}
		if end.Before(rows[i].StartedAt) {
			end = rows[i].StartedAt
		}
		intervals = append(intervals, ticketClockInterval{start: rows[i].StartedAt, end: end, open: open})
	}
	merged := mergeTicketClockIntervals(intervals)
	var pausedSeconds int64
	var pausedAt *time.Time
	for i := range merged {
		if merged[i].open {
			start := merged[i].start
			pausedAt = &start
			continue
		}
		pausedSeconds += int64(merged[i].end.Sub(merged[i].start) / time.Second)
	}
	updates := map[string]any{
		"daypop_clock_paused_seconds":           pausedSeconds,
		"daypop_clock_paused_at":                pausedAt,
		"daypop_clock_paused_remaining_seconds": int64(0),
		"updated_at":                            now,
	}
	if pausedAt != nil && ticket.SLADueAt != nil {
		updates["daypop_clock_paused_remaining_seconds"] = int64(ticket.SLADueAt.Sub(*pausedAt) / time.Second)
	}
	return db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error
}

func mergeTicketClockIntervals(items []ticketClockInterval) []ticketClockInterval {
	if len(items) == 0 {
		return items
	}
	slices.SortFunc(items, func(left, right ticketClockInterval) int {
		return left.start.Compare(right.start)
	})
	merged := make([]ticketClockInterval, 0, len(items))
	for _, item := range items {
		if len(merged) == 0 || item.start.After(merged[len(merged)-1].end) {
			merged = append(merged, item)
			continue
		}
		last := &merged[len(merged)-1]
		if item.end.After(last.end) {
			last.end = item.end
		}
		last.open = last.open || item.open
	}
	return merged
}

func buildClockAuditFields(operator *dto.AuthPrincipal, now time.Time) models.AuditFields {
	return models.AuditFields{
		CreatedAt:      now,
		CreateUserID:   auditOperatorID(operator),
		CreateUserName: auditOperatorName(operator),
		UpdatedAt:      now,
		UpdateUserID:   auditOperatorID(operator),
		UpdateUserName: auditOperatorName(operator),
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
