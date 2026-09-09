package services

import (
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	AgentWorkStatusAvailable = "available"
	AgentWorkStatusBusy      = "busy"
	AgentWorkStatusLeave     = "leave"
	AgentWorkStatusOffline   = "offline"
	AgentWorkStatusCustom    = "custom"

	agentBriefingTicketLimit = 10
)

var AgentWorkStatusService = newAgentWorkStatusService()

type agentWorkStatusService struct{}

type EngineerBriefingResult struct {
	IsEngineer            bool
	WorkStatus            models.AgentWorkStatus
	NeedsConfirmation     bool
	Teams                 []models.AgentTeam
	UnassignedTicketCount int64
	MyOpenTicketCount     int64
	UnassignedTickets     []models.Ticket
	MyOpenTickets         []models.Ticket
}

func newAgentWorkStatusService() *agentWorkStatusService {
	return &agentWorkStatusService{}
}

func isEmployeePortalSupportSession(operator *dto.AuthPrincipal) bool {
	return operator != nil &&
		strings.TrimSpace(operator.SupportMode) == models.SupportModeEmployeePortal &&
		operator.SubjectType == models.SubjectTypeTenantMember
}

func isBlockedEngineerWorkStatusSession(operator *dto.AuthPrincipal) bool {
	if isEmployeePortalSupportSession(operator) {
		return false
	}
	return operator != nil && (operator.SupportGrantID > 0 || strings.TrimSpace(operator.SupportMode) != "")
}

func (s *agentWorkStatusService) GetBriefing(operator *dto.AuthPrincipal, now time.Time) (*EngineerBriefingResult, error) {
	if operator == nil || operator.UserID <= 0 || operator.EffectiveTenantID() <= 0 {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	if isBlockedEngineerWorkStatusSession(operator) {
		return &EngineerBriefingResult{IsEngineer: false, Teams: []models.AgentTeam{}, UnassignedTickets: []models.Ticket{}, MyOpenTickets: []models.Ticket{}}, nil
	}
	profile := AgentProfileService.GetByUserID(operator.UserID)
	if profile == nil || profile.TenantID != tenantID || profile.Status != enums.StatusOk {
		return &EngineerBriefingResult{IsEngineer: false, Teams: []models.AgentTeam{}, UnassignedTickets: []models.Ticket{}, MyOpenTickets: []models.Ticket{}}, nil
	}
	status, err := s.ensureDefaultStatus(operator, profile, now)
	if err != nil {
		return nil, err
	}
	teamIDs := AgentTeamMemberService.FindTeamIDsByUserIDs(sqls.DB(), tenantID, []int64{operator.UserID})[operator.UserID]
	teams := AgentTeamService.FindByIds(teamIDs)
	visibleTeams := make([]models.AgentTeam, 0, len(teams))
	visibleTeamIDs := make([]int64, 0, len(teams))
	visibleProductIDs := make([]int64, 0, len(teams))
	for _, team := range teams {
		if team.TenantID == tenantID && team.Status == enums.StatusOk {
			visibleTeams = append(visibleTeams, team)
			visibleTeamIDs = append(visibleTeamIDs, team.ID)
			if team.ProductID > 0 {
				visibleProductIDs = append(visibleProductIDs, team.ProductID)
			}
		}
	}
	nonTerminal := "status NOT IN ?"
	terminal := []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}
	unassigned := make([]models.Ticket, 0)
	var unassignedCount int64
	if len(visibleTeamIDs) > 0 {
		unassignedCnd := sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_assignee_id", 0).
			Where(nonTerminal, terminal).
			Asc("created_at").Asc("id")
		if len(visibleProductIDs) > 0 {
			unassignedCnd.Where("product_id IN ? OR (product_id = 0 AND current_team_id IN ?)", visibleProductIDs, visibleTeamIDs)
		} else {
			unassignedCnd.Where("product_id = 0 AND current_team_id IN ?", visibleTeamIDs)
		}
		unassignedCount = repositories.TicketRepository.Count(sqls.DB(), unassignedCnd)
		unassignedCnd.Limit(agentBriefingTicketLimit)
		unassigned = repositories.TicketRepository.Find(sqls.DB(), unassignedCnd)
	}
	mineCnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("current_assignee_id", operator.UserID).
		Where(nonTerminal, terminal).
		Asc("created_at").Asc("id")
	myOpenCount := repositories.TicketRepository.Count(sqls.DB(), mineCnd)
	mine := repositories.TicketRepository.Find(sqls.DB(), mineCnd.Limit(agentBriefingTicketLimit))
	return &EngineerBriefingResult{
		IsEngineer:            true,
		WorkStatus:            *status,
		NeedsConfirmation:     status.ConfirmedAt.IsZero() || status.Status == AgentWorkStatusOffline || (status.AvailableAt != nil && !now.Before(*status.AvailableAt)),
		Teams:                 visibleTeams,
		UnassignedTickets:     unassigned,
		MyOpenTickets:         mine,
		UnassignedTicketCount: unassignedCount,
		MyOpenTicketCount:     myOpenCount,
	}, nil
}

func (s *agentWorkStatusService) UpdateMyStatus(req request.UpdateAgentWorkStatusRequest, operator *dto.AuthPrincipal, now time.Time) (*models.AgentWorkStatus, error) {
	if operator == nil || operator.UserID <= 0 || operator.EffectiveTenantID() <= 0 {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if isBlockedEngineerWorkStatusSession(operator) {
		return nil, errorsx.InvalidParam("temporary support sessions cannot change engineer work status")
	}
	tenantID := operator.EffectiveTenantID()
	profile := AgentProfileService.GetByUserID(operator.UserID)
	if profile == nil || profile.TenantID != tenantID || profile.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("current user is not an active engineer")
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if !isValidAgentWorkStatus(status) {
		return nil, errorsx.InvalidParam("invalid engineer work status")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 255 {
		return nil, errorsx.InvalidParam("status note must not exceed 255 characters")
	}
	var availableAt *time.Time
	if value := strings.TrimSpace(req.AvailableAt); value != "" && status != AgentWorkStatusAvailable {
		parsed, err := parseDateTimeValue(value)
		if err != nil {
			return nil, errorsx.InvalidParam("invalid expected recovery time")
		}
		if !parsed.After(now) {
			return nil, errorsx.InvalidParam("expected recovery time must be in the future")
		}
		availableAt = &parsed
	}
	current := repositories.AgentWorkStatusRepository.GetByTenantAndUser(sqls.DB(), tenantID, operator.UserID)
	changedAt := now
	if current != nil && current.Status == status && current.Note == note && equalAgentStatusTime(current.AvailableAt, availableAt) {
		changedAt = current.StatusChangedAt
	}
	item := &models.AgentWorkStatus{
		TenantID:        tenantID,
		UserID:          operator.UserID,
		Status:          status,
		Note:            note,
		AvailableAt:     availableAt,
		ConfirmedAt:     now,
		StatusChangedAt: changedAt,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if current != nil {
		item.ID = current.ID
		item.CreatedAt = current.CreatedAt
		item.CreateUserID = current.CreateUserID
		item.CreateUserName = current.CreateUserName
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AgentWorkStatusRepository.Save(ctx.Tx, item); err != nil {
			return err
		}
		return repositories.AgentProfileRepository.Updates(ctx.Tx, profile.ID, map[string]any{
			"last_status_at":   now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		})
	}); err != nil {
		return nil, err
	}
	// Work status is a display/presence signal. Automatic dispatch uses the
	// enterprise calendar, approved leave, profile/member configuration,
	// capacity, and reachability, so changing this field must not mutate
	// assignments or trigger a dispatch scan.
	return item, nil
}

// MarkOffline 将工程师工作状态置为离线（登出时调用）。仅对有效工程师档案生效，
// 避免给非工程师建工作状态记录。置离线后该工程师不再参与自动派单。
func (s *agentWorkStatusService) MarkOffline(userID int64, now time.Time) error {
	if userID <= 0 {
		return nil
	}
	profile := AgentProfileService.GetByUserID(userID)
	if profile == nil || profile.Status != enums.StatusOk || profile.TenantID <= 0 {
		return nil
	}
	current := repositories.AgentWorkStatusRepository.GetByTenantAndUser(sqls.DB(), profile.TenantID, userID)
	if current != nil && current.Status == AgentWorkStatusOffline {
		s.recoverPendingAssignmentsForUser(profile.TenantID, userID, AgentWorkStatusOffline, now)
		return nil
	}
	item := &models.AgentWorkStatus{
		TenantID:        profile.TenantID,
		UserID:          userID,
		Status:          AgentWorkStatusOffline,
		ConfirmedAt:     now,
		StatusChangedAt: now,
		AuditFields:     utils.BuildAuditFields(systemDispatchPrincipal()),
	}
	if current != nil {
		item.ID = current.ID
		item.CreatedAt = current.CreatedAt
		item.CreateUserID = current.CreateUserID
		item.CreateUserName = current.CreateUserName
		item.Note = current.Note
	}
	if err := repositories.AgentWorkStatusRepository.Save(sqls.DB(), item); err != nil {
		return err
	}
	s.recoverPendingAssignmentsForUser(profile.TenantID, userID, AgentWorkStatusOffline, now)
	return nil
}

func (s *agentWorkStatusService) recoverPendingAssignmentsForUser(tenantID, userID int64, reason string, now time.Time) {
	if recovered, recoverErr := TicketDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now); recoverErr != nil {
		slog.Warn("recover pending ticket assignments after engineer status unavailable failed",
			"tenant_id", tenantID,
			"user_id", userID,
			"status", reason,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
	if recovered, recoverErr := ConversationDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now); recoverErr != nil {
		slog.Warn("recover pending conversation assignments after engineer status unavailable failed",
			"tenant_id", tenantID,
			"user_id", userID,
			"status", reason,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
}

func (s *agentWorkStatusService) IsDispatchAvailable(tenantID, userID int64) bool {
	if tenantID <= 0 || userID <= 0 {
		return false
	}
	return AgentScheduleExceptionService.IsUserAvailableDB(sqls.DB(), tenantID, userID, time.Now())
}

func (s *agentWorkStatusService) FindAvailableUserSet(tenantID int64, userIDs []int64) map[int64]bool {
	return AgentScheduleExceptionService.FindAvailableUserSetDB(sqls.DB(), tenantID, userIDs, time.Now())
}

func (s *agentWorkStatusService) ensureDefaultStatus(operator *dto.AuthPrincipal, profile *models.AgentProfile, now time.Time) (*models.AgentWorkStatus, error) {
	if item := repositories.AgentWorkStatusRepository.GetByTenantAndUser(sqls.DB(), profile.TenantID, profile.UserID); item != nil {
		return item, nil
	}
	status := AgentWorkStatusAvailable
	if profile.ServiceStatus != enums.ServiceStatusIdle {
		status = AgentWorkStatusBusy
	}
	item := &models.AgentWorkStatus{
		TenantID:        profile.TenantID,
		UserID:          profile.UserID,
		Status:          status,
		StatusChangedAt: now,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	if err := repositories.AgentWorkStatusRepository.Save(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func isValidAgentWorkStatus(value string) bool {
	switch value {
	case AgentWorkStatusAvailable, AgentWorkStatusBusy, AgentWorkStatusLeave, AgentWorkStatusOffline, AgentWorkStatusCustom:
		return true
	default:
		return false
	}
}

func equalAgentStatusTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
