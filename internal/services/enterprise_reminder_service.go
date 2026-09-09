package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	enterpriseReminderLookahead     = time.Hour
	enterpriseReminderDefaultSince  = 2 * time.Minute
	enterpriseReminderMaxSinceRange = 24 * time.Hour
	enterpriseReminderLimit         = 10
)

var EnterpriseReminderService = newEnterpriseReminderService()

type enterpriseReminderService struct{}

type enterpriseReminderProductGroupScope struct {
	IsEngineer bool
	TeamIDs    []int64
	ProductIDs []int64
}

func newEnterpriseReminderService() *enterpriseReminderService {
	return &enterpriseReminderService{}
}

func (s *enterpriseReminderService) Poll(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal, since, now time.Time) (*dto.EnterpriseReminderPollDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if operator == nil || operator.UserID <= 0 {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	since = normalizeEnterpriseReminderSince(since, now)
	return &dto.EnterpriseReminderPollDTO{
		CheckedAt:        formatEnterpriseTime(now),
		MeetingReminders: s.meetingReminders(ctx, tenantID, operator, now),
		TicketReminders:  s.ticketReminders(tenantID, operator, since, now),
	}, nil
}

func normalizeEnterpriseReminderSince(since, now time.Time) time.Time {
	if since.IsZero() {
		return now.Add(-enterpriseReminderDefaultSince)
	}
	since = since.UTC()
	if since.After(now) {
		return now.Add(-enterpriseReminderDefaultSince)
	}
	if now.Sub(since) > enterpriseReminderMaxSinceRange {
		return now.Add(-enterpriseReminderMaxSinceRange)
	}
	return since
}

func (s *enterpriseReminderService) meetingReminders(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal, now time.Time) []dto.EnterpriseMeetingReminderDTO {
	if operator == nil || operator.UserID <= 0 {
		return []dto.EnterpriseMeetingReminderDTO{}
	}
	MeetingService.ReconcileStaleMeetingPresenceForTenant(ctx, tenantID, enterpriseReminderLimit)
	query := sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ?", tenantID).
		Where("status IN ?", []string{"waiting", "scheduled"}).
		Where("scheduled_at IS NOT NULL").
		Where("scheduled_at >= ? AND scheduled_at <= ?", now, now.Add(enterpriseReminderLookahead))
	participantMeetingIDs := MeetingService.personalMeetingIDSet(operator.UserID)
	participantIDs := make([]string, 0, len(participantMeetingIDs))
	for meetingID := range participantMeetingIDs {
		participantIDs = append(participantIDs, meetingID)
	}
	if len(participantIDs) > 0 {
		query = query.Where("created_by = ? OR id IN ?", strconv.FormatInt(operator.UserID, 10), participantIDs)
	} else {
		query = query.Where("created_by = ?", strconv.FormatInt(operator.UserID, 10))
	}

	meetings := make([]models.MeetingRoomJitsi, 0, enterpriseReminderLimit)
	if err := query.Order("scheduled_at ASC").Limit(enterpriseReminderLimit).Find(&meetings).Error; err != nil {
		return []dto.EnterpriseMeetingReminderDTO{}
	}
	ticketByID := s.loadMeetingTickets(tenantID, meetings)
	ret := make([]dto.EnterpriseMeetingReminderDTO, 0, len(meetings))
	for _, meeting := range meetings {
		if meeting.ScheduledAt == nil {
			continue
		}
		ticketID, _ := strconv.ParseInt(meeting.TicketID, 10, 64)
		ticket := ticketByID[ticketID]
		ticketNo := ""
		title := "视频协作提醒"
		if ticket != nil {
			ticketNo = ticket.TicketNo
			title = firstNonEmptyString(ticket.Title, title)
		}
		ret = append(ret, dto.EnterpriseMeetingReminderDTO{
			ID:              "meeting:" + meeting.ID + ":" + meeting.ScheduledAt.Format(time.RFC3339),
			MeetingID:       meeting.ID,
			TicketID:        ticketID,
			TicketNo:        ticketNo,
			Title:           title,
			RoomName:        meeting.RoomName,
			Status:          meeting.Status,
			ScheduledAt:     formatEnterpriseTimePtr(meeting.ScheduledAt),
			StartsInMinutes: minutesUntil(now, *meeting.ScheduledAt),
			ActionURL:       enterpriseMeetingReminderPath(meeting.ID, ticketID),
		})
	}
	return ret
}

func (s *enterpriseReminderService) ticketReminders(tenantID int64, operator *dto.AuthPrincipal, since, now time.Time) []dto.EnterpriseTicketReminderDTO {
	scope := s.productGroupScope(tenantID, operator)
	if !scope.IsEngineer || len(scope.TeamIDs) == 0 {
		return []dto.EnterpriseTicketReminderDTO{}
	}
	terminal := []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("current_assignee_id", 0).
		Where("status NOT IN ?", terminal).
		Where("created_at > ? AND created_at <= ?", since, now).
		Desc("created_at").Desc("id").
		Limit(enterpriseReminderLimit)
	if len(scope.ProductIDs) > 0 {
		cnd.Where("product_id IN ? OR (product_id = 0 AND current_team_id IN ?)", scope.ProductIDs, scope.TeamIDs)
	} else {
		cnd.Where("product_id = 0 AND current_team_id IN ?", scope.TeamIDs)
	}
	tickets := repositories.TicketRepository.Find(sqls.DB(), cnd)
	ret := make([]dto.EnterpriseTicketReminderDTO, 0, len(tickets))
	for _, ticket := range tickets {
		item := EnterpriseTicketService.BuildListItem(ticket)
		ret = append(ret, dto.EnterpriseTicketReminderDTO{
			ID:          fmt.Sprintf("ticket:%d:%s", ticket.ID, ticket.CreatedAt.UTC().Format(time.RFC3339)),
			TicketID:    ticket.ID,
			TicketNo:    item.TicketNo,
			Title:       item.Title,
			ProductName: item.ProductName,
			TeamName:    item.TeamName,
			Priority:    item.Priority,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt,
			ActionURL:   fmt.Sprintf("/enterprise/ticket-workbench?ticket_id=%d", ticket.ID),
		})
	}
	return ret
}

func (s *enterpriseReminderService) productGroupScope(tenantID int64, operator *dto.AuthPrincipal) enterpriseReminderProductGroupScope {
	if operator == nil || operator.UserID <= 0 || tenantID <= 0 || isBlockedEngineerWorkStatusSession(operator) {
		return enterpriseReminderProductGroupScope{}
	}
	profile := AgentProfileService.GetByUserID(operator.UserID)
	if profile == nil || profile.TenantID != tenantID || profile.Status != enums.StatusOk {
		return enterpriseReminderProductGroupScope{}
	}
	teamIDs := AgentTeamMemberService.FindTeamIDsByUserIDs(sqls.DB(), tenantID, []int64{operator.UserID})[operator.UserID]
	teams := AgentTeamService.FindByIds(teamIDs)
	scope := enterpriseReminderProductGroupScope{IsEngineer: true}
	for _, team := range teams {
		if team.TenantID != tenantID || team.Status != enums.StatusOk {
			continue
		}
		scope.TeamIDs = append(scope.TeamIDs, team.ID)
		if team.ProductID > 0 {
			scope.ProductIDs = append(scope.ProductIDs, team.ProductID)
		}
	}
	scope.TeamIDs = uniqueServiceInt64s(scope.TeamIDs)
	scope.ProductIDs = uniqueServiceInt64s(scope.ProductIDs)
	return scope
}

func (s *enterpriseReminderService) loadMeetingTickets(tenantID int64, meetings []models.MeetingRoomJitsi) map[int64]*models.Ticket {
	ret := make(map[int64]*models.Ticket)
	ticketIDs := make([]int64, 0, len(meetings))
	for _, meeting := range meetings {
		ticketID, err := strconv.ParseInt(strings.TrimSpace(meeting.TicketID), 10, 64)
		if err == nil && ticketID > 0 {
			ticketIDs = append(ticketIDs, ticketID)
		}
	}
	ticketIDs = uniqueServiceInt64s(ticketIDs)
	if len(ticketIDs) == 0 {
		return ret
	}
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		In("id", ticketIDs).
		Limit(len(ticketIDs)))
	for i := range tickets {
		ret[tickets[i].ID] = &tickets[i]
	}
	return ret
}

func minutesUntil(now, target time.Time) int64 {
	if !target.After(now) {
		return 0
	}
	delta := target.Sub(now)
	minutes := delta / time.Minute
	if delta%time.Minute != 0 {
		minutes++
	}
	return int64(minutes)
}

func enterpriseMeetingReminderPath(meetingID string, ticketID int64) string {
	if ticketID > 0 {
		return fmt.Sprintf("/enterprise/ticket-workbench?ticket_id=%d", ticketID)
	}
	return enterpriseMeetingRoomPath(meetingID, 0)
}
