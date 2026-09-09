package services

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AgentScheduleExceptionService = newAgentScheduleExceptionService()

const (
	EngineerScheduleTimezone = "Asia/Shanghai"

	AgentScheduleExceptionTypeLeave = "leave"

	AgentScheduleApprovalPending   = "pending"
	AgentScheduleApprovalApproved  = "approved"
	AgentScheduleApprovalRejected  = "rejected"
	AgentScheduleApprovalCancelled = "cancelled"
)

type agentScheduleExceptionService struct {
	writeMu sync.Mutex
}

func newAgentScheduleExceptionService() *agentScheduleExceptionService {
	return &agentScheduleExceptionService{}
}

func engineerScheduleLocation() *time.Location {
	location, err := time.LoadLocation(EngineerScheduleTimezone)
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func (s *agentScheduleExceptionService) BuildEnterpriseWeeklyScheduleDB(db *gorm.DB, tenantID, userID int64) []models.AgentTeamSchedule {
	template := AgentTeamScheduleService.resolveTemplateDB(db, tenantID)
	workdaySet := make(map[int]struct{}, len(template.Workdays))
	for _, weekday := range template.Workdays {
		workdaySet[weekday] = struct{}{}
	}
	location := engineerScheduleLocation()
	items := make([]models.AgentTeamSchedule, 0, 7)
	for weekday := 1; weekday <= 7; weekday++ {
		dayType := AgentTeamScheduleDayTypeRest
		startMinute := 0
		endMinute := 0
		if _, workday := workdaySet[weekday]; workday {
			dayType = AgentTeamScheduleDayTypeWork
			startMinute = template.StartMinute
			endMinute = template.EndMinute
		}
		startAt := weeklyScheduleAnchorInLocation(weekday, startMinute, location)
		endAt := weeklyScheduleAnchorInLocation(weekday, endMinute, location)
		items = append(items, models.AgentTeamSchedule{
			ID:            -int64(weekday),
			TenantID:      tenantID,
			TeamID:        0,
			UserID:        userID,
			RepeatType:    AgentTeamScheduleRepeatWeekly,
			DayType:       dayType,
			Weekday:       weekday,
			StartMinute:   startMinute,
			EndMinute:     endMinute,
			Timezone:      EngineerScheduleTimezone,
			PublishStatus: AgentTeamSchedulePublishPublished,
			Version:       1,
			StartAt:       startAt,
			EndAt:         endAt,
			Status:        enums.StatusOk,
		})
	}
	return items
}

func (s *agentScheduleExceptionService) IsWithinEnterpriseWorkTimeDB(db *gorm.DB, tenantID int64, at time.Time) bool {
	if tenantID <= 0 {
		return false
	}
	template := AgentTeamScheduleService.resolveTemplateDB(db, tenantID)
	local := at.In(engineerScheduleLocation())
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	workdaySet := make(map[int]struct{}, len(template.Workdays))
	for _, value := range template.Workdays {
		workdaySet[value] = struct{}{}
	}
	if template.EndMinute > template.StartMinute {
		_, workday := workdaySet[weekday]
		return workday && minute >= template.StartMinute && minute < template.EndMinute
	}
	if minute >= template.StartMinute {
		_, workday := workdaySet[weekday]
		return workday
	}
	previous := weekday - 1
	if previous == 0 {
		previous = 7
	}
	_, previousWorkday := workdaySet[previous]
	return previousWorkday && minute < template.EndMinute
}

func (s *agentScheduleExceptionService) IsWithinPersonalDispatchRuleDB(db *gorm.DB, tenantID int64, at time.Time) bool {
	return s.IsWithinEnterpriseWorkTimeDB(db, tenantID, at)
}

func (s *agentScheduleExceptionService) FindAvailableUserSetDB(db *gorm.DB, tenantID int64, userIDs []int64, at time.Time) map[int64]bool {
	available := make(map[int64]bool)
	if db == nil || tenantID <= 0 || len(userIDs) == 0 {
		return available
	}
	userIDs = uniquePositiveInt64s(userIDs)
	for _, userID := range userIDs {
		available[userID] = true
	}
	if !s.IsWithinPersonalDispatchRuleDB(db, tenantID, at) {
		return map[int64]bool{}
	}
	if !db.Migrator().HasTable(&models.AgentScheduleException{}) {
		return available
	}
	leaves, err := repositories.AgentScheduleExceptionRepository.FindVisible(
		db, tenantID, userIDs, at, at.Add(time.Nanosecond), []string{AgentScheduleApprovalApproved},
	)
	if err != nil {
		return map[int64]bool{}
	}
	for _, leave := range leaves {
		if !at.Before(leave.StartAt) && at.Before(leave.EndAt) {
			delete(available, leave.UserID)
		}
	}
	return available
}

func (s *agentScheduleExceptionService) IsUserAvailableDB(db *gorm.DB, tenantID, userID int64, at time.Time) bool {
	return s.isUserAvailableDB(db, tenantID, userID, at)
}

func (s *agentScheduleExceptionService) isUserAvailableDB(db *gorm.DB, tenantID, userID int64, at time.Time) bool {
	return s.FindAvailableUserSetDB(db, tenantID, []int64{userID}, at)[userID]
}

func (s *agentScheduleExceptionService) ListForUsersDB(db *gorm.DB, tenantID int64, userIDs []int64, startAt, endAt time.Time, approvalStatuses []string) ([]models.AgentScheduleException, error) {
	if db == nil || !db.Migrator().HasTable(&models.AgentScheduleException{}) {
		return []models.AgentScheduleException{}, nil
	}
	return repositories.AgentScheduleExceptionRepository.FindVisible(db, tenantID, userIDs, startAt, endAt, approvalStatuses)
}

func (s *agentScheduleExceptionService) ListForTeam(tenantID, teamID int64, approvalStatus string, now time.Time) ([]models.AgentScheduleException, error) {
	if tenantID <= 0 || teamID <= 0 {
		return nil, errorsx.InvalidParam("product team is required")
	}
	team := AgentTeamService.GetForTenant(teamID, tenantID)
	if team == nil || team.Status != enums.StatusOk || team.TeamType != AgentTeamTypeProductRepair {
		return nil, errorsx.InvalidParam("product repair team was not found")
	}
	members := repositories.AgentTeamMemberRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("team_id", teamID).
		Eq("status", enums.StatusOk))
	userIDs := make([]int64, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	statuses, err := normalizeScheduleApprovalFilter(approvalStatus)
	if err != nil {
		return nil, err
	}
	return s.ListForUsersDB(sqls.DB(), tenantID, userIDs, now.AddDate(0, -1, 0), now.AddDate(1, 0, 0), statuses)
}

func normalizeScheduleApprovalFilter(value string) ([]string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "all" {
		return []string{
			AgentScheduleApprovalPending,
			AgentScheduleApprovalApproved,
			AgentScheduleApprovalRejected,
			AgentScheduleApprovalCancelled,
		}, nil
	}
	for _, candidate := range []string{
		AgentScheduleApprovalPending,
		AgentScheduleApprovalApproved,
		AgentScheduleApprovalRejected,
		AgentScheduleApprovalCancelled,
	} {
		if value == candidate {
			return []string{value}, nil
		}
	}
	return nil, errorsx.InvalidParam("leave approval status is invalid")
}

func (s *agentScheduleExceptionService) CreateMyLeave(req request.CreateAgentScheduleLeaveRequest, operator *dto.AuthPrincipal, now time.Time) (*models.AgentScheduleException, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID <= 0 || operator.UserID <= 0 {
		return nil, errorsx.InvalidParam("engineer tenant and user are required")
	}
	startAt, err := parseEngineerScheduleDateTime(req.StartAt)
	if err != nil {
		return nil, errorsx.InvalidParam("leave start time is invalid")
	}
	endAt, err := parseEngineerScheduleDateTime(req.EndAt)
	if err != nil {
		return nil, errorsx.InvalidParam("leave end time is invalid")
	}
	reason := strings.TrimSpace(req.Reason)
	if !endAt.After(startAt) {
		return nil, errorsx.InvalidParam("leave end time must be after start time")
	}
	if startAt.Before(now.Add(-5 * time.Minute)) {
		return nil, errorsx.InvalidParam("leave start time cannot be in the past")
	}
	if endAt.Sub(startAt) > 90*24*time.Hour {
		return nil, errorsx.InvalidParam("a leave request cannot exceed 90 days")
	}
	if len([]rune(reason)) < 2 || len([]rune(reason)) > 255 {
		return nil, errorsx.InvalidParam("leave reason must contain 2 to 255 characters")
	}
	requestKey := strings.TrimSpace(req.RequestKey)
	if requestKey == "" {
		requestKey = uuid.NewString()
	}
	if len(requestKey) > 80 {
		return nil, errorsx.InvalidParam("leave request key is too long")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var result *models.AgentScheduleException
	err = sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		db := txCtx.Tx
		if existing := repositories.AgentScheduleExceptionRepository.FindByRequestKey(db, tenantID, operator.UserID, requestKey); existing != nil {
			result = existing
			return nil
		}
		if err := s.lockEngineerProfileDB(db, tenantID, operator.UserID); err != nil {
			return err
		}
		overlaps, overlapErr := repositories.AgentScheduleExceptionRepository.FindOverlapping(
			db, tenantID, operator.UserID, startAt, endAt,
			[]string{AgentScheduleApprovalPending, AgentScheduleApprovalApproved},
		)
		if overlapErr != nil {
			return overlapErr
		}
		if len(overlaps) > 0 {
			return errorsx.InvalidParam("an active leave request already overlaps this period")
		}
		item := &models.AgentScheduleException{
			TenantID:       tenantID,
			UserID:         operator.UserID,
			RequestKey:     requestKey,
			ExceptionType:  AgentScheduleExceptionTypeLeave,
			StartAt:        startAt,
			EndAt:          endAt,
			ApprovalStatus: AgentScheduleApprovalPending,
			Reason:         reason,
			RequestedAt:    now,
			Status:         enums.StatusOk,
			AuditFields:    utils.BuildAuditFields(operator),
		}
		if err := repositories.AgentScheduleExceptionRepository.Create(db, item); err != nil {
			return err
		}
		if err := s.recordAuditTx(txCtx, operator, item, "schedule.leave.requested", nil, item); err != nil {
			return err
		}
		result = item
		return nil
	})
	return result, err
}

func (s *agentScheduleExceptionService) CancelMyLeave(id int64, operator *dto.AuthPrincipal, now time.Time) (*models.AgentScheduleException, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var result *models.AgentScheduleException
	wasApprovedAndActive := false
	err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		item, err := s.lockExceptionDB(txCtx.Tx, tenantID, id)
		if err != nil {
			return err
		}
		if item.UserID != operator.UserID {
			return errorsx.Forbidden("leave request does not belong to current engineer")
		}
		if item.ApprovalStatus == AgentScheduleApprovalCancelled {
			result = item
			return nil
		}
		if item.ApprovalStatus != AgentScheduleApprovalPending && item.ApprovalStatus != AgentScheduleApprovalApproved {
			return errorsx.InvalidParam("only pending or approved leave can be cancelled")
		}
		before := *item
		wasApprovedAndActive = item.ApprovalStatus == AgentScheduleApprovalApproved && !now.Before(item.StartAt) && now.Before(item.EndAt)
		item.ApprovalStatus = AgentScheduleApprovalCancelled
		item.UpdatedAt = now
		item.UpdateUserID = operator.UserID
		item.UpdateUserName = operator.Username
		if err := txCtx.Tx.Save(item).Error; err != nil {
			return err
		}
		if err := s.recordAuditTx(txCtx, operator, item, "schedule.leave.cancelled", &before, item); err != nil {
			return err
		}
		result = item
		return nil
	})
	if err == nil && wasApprovedAndActive {
		s.dispatchAfterAvailabilityChange(tenantID, operator.UserID, now, false)
	}
	return result, err
}

func (s *agentScheduleExceptionService) ReviewLeave(req request.ReviewAgentScheduleLeaveRequest, operator *dto.AuthPrincipal, now time.Time) (*models.AgentScheduleException, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != AgentScheduleApprovalApproved && decision != AgentScheduleApprovalRejected {
		return nil, errorsx.InvalidParam("leave decision must be approved or rejected")
	}
	reviewNote := strings.TrimSpace(req.ReviewNote)
	if len([]rune(reviewNote)) > 255 {
		return nil, errorsx.InvalidParam("leave review note is too long")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var result *models.AgentScheduleException
	approvedAndActive := false
	err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		item, err := s.lockExceptionDB(txCtx.Tx, tenantID, req.ID)
		if err != nil {
			return err
		}
		if item.ApprovalStatus == decision {
			result = item
			return nil
		}
		if item.ApprovalStatus != AgentScheduleApprovalPending {
			return errorsx.InvalidParam("leave request has already been reviewed or cancelled")
		}
		if decision == AgentScheduleApprovalApproved {
			overlaps, overlapErr := repositories.AgentScheduleExceptionRepository.FindOverlapping(
				txCtx.Tx, tenantID, item.UserID, item.StartAt, item.EndAt, []string{AgentScheduleApprovalApproved},
			)
			if overlapErr != nil {
				return overlapErr
			}
			for _, overlap := range overlaps {
				if overlap.ID != item.ID {
					return errorsx.InvalidParam("approved leave already overlaps this period")
				}
			}
		}
		before := *item
		item.ApprovalStatus = decision
		item.ReviewNote = reviewNote
		item.ReviewedAt = &now
		item.ReviewerUserID = operator.UserID
		item.ReviewerUserName = operator.Username
		item.UpdatedAt = now
		item.UpdateUserID = operator.UserID
		item.UpdateUserName = operator.Username
		if err := txCtx.Tx.Save(item).Error; err != nil {
			return err
		}
		if err := s.recordAuditTx(txCtx, operator, item, "schedule.leave."+decision, &before, item); err != nil {
			return err
		}
		approvedAndActive = decision == AgentScheduleApprovalApproved && !now.Before(item.StartAt) && now.Before(item.EndAt)
		result = item
		return nil
	})
	if err == nil && approvedAndActive && result != nil {
		s.dispatchAfterAvailabilityChange(tenantID, result.UserID, now, true)
	}
	return result, err
}

func (s *agentScheduleExceptionService) lockEngineerProfileDB(db *gorm.DB, tenantID, userID int64) error {
	var profile models.AgentProfile
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, enums.StatusOk).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errorsx.Forbidden("current user is not an active engineer")
	}
	return err
}

func (s *agentScheduleExceptionService) lockExceptionDB(db *gorm.DB, tenantID, id int64) (*models.AgentScheduleException, error) {
	if tenantID <= 0 || id <= 0 {
		return nil, errorsx.InvalidParam("leave request is required")
	}
	item := &models.AgentScheduleException{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, enums.StatusOk).
		First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorsx.InvalidParam("leave request was not found")
	}
	return item, err
}

func (s *agentScheduleExceptionService) recordAuditTx(txCtx *sqls.TxContext, operator *dto.AuthPrincipal, item *models.AgentScheduleException, action string, before, after any) error {
	if txCtx == nil || txCtx.Tx == nil || !txCtx.Tx.Migrator().HasTable(&models.AuditLog{}) {
		return nil
	}
	return AuditService.RecordAuditTx(txCtx, RecordAuditInput{
		TenantID:     item.TenantID,
		ActorID:      strconv.FormatInt(operator.UserID, 10),
		ActorType:    "user",
		Domain:       "schedule",
		ResourceType: "agent_schedule_exception",
		ResourceID:   strconv.FormatInt(item.ID, 10),
		Action:       action,
		BeforeState:  before,
		AfterState:   after,
		RiskLevel:    models.RiskLevelMedium,
	})
}

func (s *agentScheduleExceptionService) dispatchAfterAvailabilityChange(tenantID, userID int64, now time.Time, recover bool) {
	if recover {
		_, _ = ConversationDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now)
		_, _ = TicketDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now)
	}
	_, _ = ConversationDispatchService.DispatchPendingConversations(0)
	_, _ = TicketDispatchService.DispatchPendingTickets(0)
}

func parseEngineerScheduleDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("time is required")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.In(engineerScheduleLocation()), nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", time.DateTime} {
		if parsed, err := time.ParseInLocation(layout, value, engineerScheduleLocation()); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("unsupported schedule time format")
}
