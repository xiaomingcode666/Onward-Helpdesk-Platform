package services

import (
	"encoding/json"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AgentTeamScheduleService = newAgentTeamScheduleService()

func newAgentTeamScheduleService() *agentTeamScheduleService {
	return &agentTeamScheduleService{}
}

type agentTeamScheduleService struct{}

const (
	AgentTeamScheduleRepeatOnce   = "once"
	AgentTeamScheduleRepeatWeekly = "weekly"
	AgentTeamScheduleDayTypeWork  = "work"
	AgentTeamScheduleDayTypeRest  = "rest"

	AgentTeamSchedulePublishDraft     = "draft"
	AgentTeamSchedulePublishPublished = "published"
	AgentTeamSchedulePublishArchived  = "archived"

	defaultAgentTeamScheduleStartMinute = 0
	defaultAgentTeamScheduleEndMinute   = 24 * 60

	legacyAgentTeamScheduleWriteDisabledMessage = "产品组排班已改为成员可接单规则和请假；旧排班写入已下线"
)

type AgentTeamScheduleDraftResult struct {
	TeamID  int64
	Version int
	Count   int
}

type AgentTeamSchedulePublishResult struct {
	TeamID          int64
	Version         int
	Published       int
	MissingWeekdays []int
}

type AgentTeamScheduleTemplateConfig struct {
	TenantID    int64
	Workdays    []int
	StartMinute int
	EndMinute   int
	Timezone    string
}

type AgentTeamScheduleBatchPreviewResult struct {
	Total    int
	Conflict bool
	Items    []AgentTeamScheduleBatchPreviewItem
}

type AgentTeamScheduleBatchPreviewItem struct {
	TeamID         int64
	TeamName       string
	Date           time.Time
	Weekday        int
	StartAt        time.Time
	EndAt          time.Time
	Remark         string
	Conflict       bool
	ConflictReason string
}

type AgentTeamScheduleBatchGenerateResult struct {
	Created int
}

type EngineerWorkScheduleResult struct {
	IsEngineer     bool
	Teams          []models.AgentTeam
	Schedules      []models.AgentTeamSchedule
	DraftSchedules []models.AgentTeamSchedule
	BaseSchedule   *AgentTeamScheduleTemplateConfig
	Timezone       string
	Exceptions     []models.AgentScheduleException
}

type AgentTeamMemberAvailability struct {
	Profile             models.AgentProfile
	Member              models.AgentTeamMember
	User                *models.User
	WorkStatus          *models.AgentWorkStatus
	ActiveLeave         *models.AgentScheduleException
	PendingLeave        *models.AgentScheduleException
	Workdays            []int
	StartMinute         int
	EndMinute           int
	Timezone            string
	AvailableNow        bool
	UnavailableReason   string
	WorkStatusConfirmed bool
}

func (s *agentTeamScheduleService) Get(id int64) *models.AgentTeamSchedule {
	return repositories.AgentTeamScheduleRepository.Get(sqls.DB(), id)
}

func (s *agentTeamScheduleService) Take(where ...interface{}) *models.AgentTeamSchedule {
	return repositories.AgentTeamScheduleRepository.Take(sqls.DB(), where...)
}

func (s *agentTeamScheduleService) Find(cnd *sqls.Cnd) []models.AgentTeamSchedule {
	return repositories.AgentTeamScheduleRepository.Find(sqls.DB(), cnd)
}

func (s *agentTeamScheduleService) FindOne(cnd *sqls.Cnd) *models.AgentTeamSchedule {
	return repositories.AgentTeamScheduleRepository.FindOne(sqls.DB(), cnd)
}

func (s *agentTeamScheduleService) FindPageByParams(params *params.QueryParams) (list []models.AgentTeamSchedule, paging *sqls.Paging) {
	return repositories.AgentTeamScheduleRepository.FindPageByParams(sqls.DB(), params)
}

func (s *agentTeamScheduleService) FindPageByCnd(cnd *sqls.Cnd) (list []models.AgentTeamSchedule, paging *sqls.Paging) {
	return repositories.AgentTeamScheduleRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *agentTeamScheduleService) Count(cnd *sqls.Cnd) int64 {
	return repositories.AgentTeamScheduleRepository.Count(sqls.DB(), cnd)
}

func (s *agentTeamScheduleService) GetTemplate(operator *dto.AuthPrincipal) (*AgentTeamScheduleTemplateConfig, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	return s.resolveTemplateDB(sqls.DB(), tenantID), nil
}

func (s *agentTeamScheduleService) UpdateTemplate(req request.UpdateAgentTeamScheduleTemplateRequest, operator *dto.AuthPrincipal) (*AgentTeamScheduleTemplateConfig, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	workdays, err := normalizeAgentTeamScheduleTemplateWorkdays(req.Workdays)
	if err != nil {
		return nil, err
	}
	timezone := EngineerScheduleTimezone
	location := engineerScheduleLocation()
	startClock, err := parseRequiredClockInLocation(req.StartTime, location, "error.agentTeamSchedule.startTimeInvalid", "error.agentTeamSchedule.startTimeInvalidWithFormat")
	if err != nil {
		return nil, err
	}
	endClock, err := parseRequiredClockInLocation(req.EndTime, location, "error.agentTeamSchedule.endTimeInvalid", "error.agentTeamSchedule.endTimeInvalidWithFormat")
	if err != nil {
		return nil, err
	}
	startMinute := scheduleClockMinute(req.StartTime, startClock)
	endMinute := scheduleClockMinute(req.EndTime, endClock)
	if startMinute == endMinute {
		return nil, errorsx.InvalidParam("default shift start and end times cannot be equal")
	}
	workdaysJSON, err := json.Marshal(workdays)
	if err != nil {
		return nil, err
	}
	item := &models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    string(workdaysJSON),
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    timezone,
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.AgentTeamScheduleTemplateRepository.Upsert(sqls.DB(), item); err != nil {
		return nil, err
	}
	return &AgentTeamScheduleTemplateConfig{
		TenantID: tenantID, Workdays: workdays, StartMinute: startMinute, EndMinute: endMinute, Timezone: timezone,
	}, nil
}

func (s *agentTeamScheduleService) resolveTemplateDB(db *gorm.DB, tenantID int64) *AgentTeamScheduleTemplateConfig {
	config := &AgentTeamScheduleTemplateConfig{
		TenantID: tenantID, Workdays: []int{1, 2, 3, 4, 5},
		StartMinute: defaultAgentTeamScheduleStartMinute,
		EndMinute:   defaultAgentTeamScheduleEndMinute,
		Timezone:    EngineerScheduleTimezone,
	}
	if db == nil || !db.Migrator().HasTable(&models.AgentTeamScheduleTemplate{}) {
		return config
	}
	item := repositories.AgentTeamScheduleTemplateRepository.FindByTenant(db, tenantID)
	if item == nil || item.Status != enums.StatusOk {
		return config
	}
	if workdays, err := parseAgentTeamScheduleTemplateWorkdays(item.Workdays); err == nil {
		config.Workdays = workdays
	}
	if isValidScheduleMinute(item.StartMinute) && isValidScheduleMinute(item.EndMinute) && item.StartMinute != item.EndMinute {
		config.StartMinute = item.StartMinute
		config.EndMinute = item.EndMinute
	}
	return config
}

func (s *agentTeamScheduleService) FindCalendarSchedules(req request.AgentTeamScheduleCalendarRequest) ([]models.AgentTeamSchedule, error) {
	startAtValue, err := parseRequiredDateTime(req.StartAt, "error.agentTeamSchedule.startAtInvalid", "error.agentTeamSchedule.startAtInvalidWithFormat")
	if err != nil {
		return nil, err
	}
	endAtValue, err := parseRequiredDateTime(req.EndAt, "error.agentTeamSchedule.endAtInvalid", "error.agentTeamSchedule.endAtInvalidWithFormat")
	if err != nil {
		return nil, err
	}
	if !endAtValue.After(startAtValue) {
		return nil, errorsx.InvalidParamI18n("error.e0296")
	}
	once := repositories.AgentTeamScheduleRepository.FindByTimeRange(sqls.DB(), startAtValue, endAtValue, req.TeamID, req.TenantID)
	result := make([]models.AgentTeamSchedule, 0, len(once))
	for _, item := range once {
		if NormalizeAgentTeamScheduleRepeatType(item.RepeatType) == AgentTeamScheduleRepeatWeekly || normalizeAgentTeamSchedulePublishStatus(item.PublishStatus) != AgentTeamSchedulePublishPublished {
			continue
		}
		result = append(result, item)
	}
	cnd := sqls.NewCnd().Eq("repeat_type", AgentTeamScheduleRepeatWeekly).
		Eq("publish_status", AgentTeamSchedulePublishPublished).
		Eq("status", enums.StatusOk)
	if req.TenantID > 0 {
		cnd.Eq("tenant_id", req.TenantID)
	}
	if req.TeamID > 0 {
		cnd.Eq("team_id", req.TeamID)
	}
	for _, template := range repositories.AgentTeamScheduleRepository.Find(sqls.DB(), cnd) {
		location := scheduleLocation(template.Timezone)
		firstLocalDay := startOfDayInLocation(startAtValue.AddDate(0, 0, -1), location)
		lastLocalDay := startOfDayInLocation(endAtValue, location)
		for date := firstLocalDay; !date.After(lastLocalDay); date = date.AddDate(0, 0, 1) {
			occurrenceStart, occurrenceEnd, ok := weeklyScheduleOccurrence(template, date)
			if !ok || !occurrenceStart.Before(endAtValue) || !occurrenceEnd.After(startAtValue) {
				continue
			}
			expanded := template
			expanded.StartAt = occurrenceStart
			expanded.EndAt = occurrenceEnd
			result = append(result, expanded)
		}
	}
	slices.SortFunc(result, func(a, b models.AgentTeamSchedule) int {
		if a.StartAt.Before(b.StartAt) {
			return -1
		}
		if a.StartAt.After(b.StartAt) {
			return 1
		}
		return int(a.ID - b.ID)
	})
	return result, nil
}

func (s *agentTeamScheduleService) GetMyWeeklySchedule(operator *dto.AuthPrincipal) (*EngineerWorkScheduleResult, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	result := &EngineerWorkScheduleResult{Timezone: EngineerScheduleTimezone}
	if tenantID <= 0 || operator.UserID <= 0 {
		return result, nil
	}

	teamIDsByUser := AgentTeamMemberService.FindTeamIDsByUserIDs(sqls.DB(), tenantID, []int64{operator.UserID})
	teamIDs := teamIDsByUser[operator.UserID]
	if len(teamIDs) == 0 {
		return result, nil
	}

	visibleTeams := make([]models.AgentTeam, 0, len(teamIDs))
	for _, team := range AgentTeamService.FindByIds(teamIDs) {
		if team.TenantID != tenantID || team.Status != enums.StatusOk || team.TeamType != AgentTeamTypeProductRepair || team.ProductID <= 0 {
			continue
		}
		visibleTeams = append(visibleTeams, team)
	}
	if len(visibleTeams) == 0 {
		return result, nil
	}
	slices.SortFunc(visibleTeams, func(left, right models.AgentTeam) int {
		if left.ProductID < right.ProductID {
			return -1
		}
		if left.ProductID > right.ProductID {
			return 1
		}
		if value := strings.Compare(left.Name, right.Name); value != 0 {
			return value
		}
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})

	result.IsEngineer = true
	result.Teams = visibleTeams
	result.BaseSchedule = s.resolveTemplateDB(sqls.DB(), tenantID)
	result.Timezone = result.BaseSchedule.Timezone
	result.Schedules = s.findEngineerTeamWeeklySchedulesDB(sqls.DB(), tenantID, operator.UserID, visibleTeams, AgentTeamSchedulePublishPublished)
	result.DraftSchedules = s.findEngineerTeamWeeklySchedulesDB(sqls.DB(), tenantID, operator.UserID, visibleTeams, AgentTeamSchedulePublishDraft)
	startAt := time.Now().AddDate(0, -1, 0)
	endAt := time.Now().AddDate(0, 6, 0)
	result.Exceptions, _ = AgentScheduleExceptionService.ListForUsersDB(
		sqls.DB(), tenantID, []int64{operator.UserID}, startAt, endAt,
		[]string{AgentScheduleApprovalPending, AgentScheduleApprovalApproved, AgentScheduleApprovalRejected, AgentScheduleApprovalCancelled},
	)
	return result, nil
}

func (s *agentTeamScheduleService) ListTeamMemberAvailability(teamID int64, operator *dto.AuthPrincipal, now time.Time) ([]AgentTeamMemberAvailability, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID <= 0 || teamID <= 0 {
		return nil, errorsx.InvalidParam("support team is required")
	}
	team := AgentTeamService.GetForTenant(teamID, tenantID)
	if team == nil || team.Status != enums.StatusOk ||
		(team.TeamType != AgentTeamTypeProductRepair && team.TeamType != AgentTeamTypeTechnicalRepair) {
		return nil, errorsx.InvalidParam("support team was not found")
	}
	db := sqls.DB()
	members := AgentTeamMemberService.FindActiveMembersByTeamID(db, tenantID, teamID)
	if len(members) == 0 {
		return []AgentTeamMemberAvailability{}, nil
	}
	userIDs := make([]int64, 0, len(members))
	for _, member := range members {
		if member.UserID > 0 {
			userIDs = append(userIDs, member.UserID)
		}
	}
	userIDs = uniqueServiceInt64s(userIDs)
	profileByUserID := make(map[int64]models.AgentProfile, len(userIDs))
	for _, profile := range repositories.AgentProfileRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		In("user_id", userIDs).
		Eq("status", enums.StatusOk).
		Asc("id")) {
		if _, exists := profileByUserID[profile.UserID]; !exists {
			profileByUserID[profile.UserID] = profile
		}
	}
	userByID := make(map[int64]*models.User, len(userIDs))
	for _, user := range UserService.FindByIds(userIDs) {
		item := user
		userByID[item.ID] = &item
	}
	statusByUserID := make(map[int64]models.AgentWorkStatus)
	for _, status := range repositories.AgentWorkStatusRepository.FindByTenantAndUserIDs(db, tenantID, userIDs) {
		statusByUserID[status.UserID] = status
	}
	leaveStart := now.AddDate(0, -1, 0)
	leaveEnd := now.AddDate(0, 6, 0)
	leaves, err := AgentScheduleExceptionService.ListForUsersDB(
		db, tenantID, userIDs, leaveStart, leaveEnd,
		[]string{AgentScheduleApprovalPending, AgentScheduleApprovalApproved},
	)
	if err != nil {
		return nil, err
	}
	activeLeaveByUserID := make(map[int64]*models.AgentScheduleException)
	pendingLeaveByUserID := make(map[int64]*models.AgentScheduleException)
	for i := range leaves {
		leave := leaves[i]
		switch leave.ApprovalStatus {
		case AgentScheduleApprovalApproved:
			if !now.Before(leave.StartAt) && now.Before(leave.EndAt) {
				item := leave
				activeLeaveByUserID[leave.UserID] = &item
			}
		case AgentScheduleApprovalPending:
			if _, exists := pendingLeaveByUserID[leave.UserID]; !exists {
				item := leave
				pendingLeaveByUserID[leave.UserID] = &item
			}
		}
	}
	template := s.resolveTemplateDB(db, tenantID)
	ret := make([]AgentTeamMemberAvailability, 0, len(members))
	for _, member := range members {
		profile := profileByUserID[member.UserID]
		if profile.UserID <= 0 {
			profile.TenantID = tenantID
			profile.UserID = member.UserID
		}
		statusValue := (*models.AgentWorkStatus)(nil)
		status, hasStatus := statusByUserID[profile.UserID]
		if hasStatus {
			statusValue = &status
		}
		// Dispatch availability is intentionally schedule-only. The old work-status
		// handshake made a member disappear from the roster until they explicitly
		// accepted work, which is not part of the product rule.
		availableNow := AgentScheduleExceptionService.isUserAvailableDB(db, tenantID, profile.UserID, now)
		confirmed := true
		reason := ""
		switch {
		case profile.ID <= 0:
			availableNow = false
			reason = "profile_missing"
		case activeLeaveByUserID[profile.UserID] != nil:
			availableNow = false
			reason = "approved_leave"
		case !availableNow:
			reason = "outside_personal_dispatch_rule"
		}
		ret = append(ret, AgentTeamMemberAvailability{
			Profile:             profile,
			Member:              member,
			User:                userByID[profile.UserID],
			WorkStatus:          statusValue,
			ActiveLeave:         activeLeaveByUserID[profile.UserID],
			PendingLeave:        pendingLeaveByUserID[profile.UserID],
			Workdays:            append([]int(nil), template.Workdays...),
			StartMinute:         template.StartMinute,
			EndMinute:           template.EndMinute,
			Timezone:            template.Timezone,
			AvailableNow:        availableNow,
			UnavailableReason:   reason,
			WorkStatusConfirmed: confirmed,
		})
	}
	return ret, nil
}

func (s *agentTeamScheduleService) findEngineerTeamWeeklySchedulesDB(db *gorm.DB, tenantID, userID int64, teams []models.AgentTeam, publishStatus string) []models.AgentTeamSchedule {
	if db == nil || tenantID <= 0 || userID <= 0 || len(teams) == 0 || !db.Migrator().HasTable(&models.AgentTeamSchedule{}) {
		return []models.AgentTeamSchedule{}
	}
	teamIDs := make([]int64, 0, len(teams))
	teamOrder := make(map[int64]int, len(teams))
	for index, team := range teams {
		if team.ID <= 0 {
			continue
		}
		teamIDs = append(teamIDs, team.ID)
		teamOrder[team.ID] = index
	}
	if len(teamIDs) == 0 {
		return []models.AgentTeamSchedule{}
	}
	items := make([]models.AgentTeamSchedule, 0)
	db.Where(
		"tenant_id = ? AND team_id IN ? AND user_id IN ? AND repeat_type = ? AND publish_status = ? AND status = ?",
		tenantID, teamIDs, []int64{0, userID}, AgentTeamScheduleRepeatWeekly, publishStatus, enums.StatusOk,
	).Find(&items)
	slices.SortFunc(items, func(left, right models.AgentTeamSchedule) int {
		if teamOrder[left.TeamID] != teamOrder[right.TeamID] {
			return teamOrder[left.TeamID] - teamOrder[right.TeamID]
		}
		if left.Weekday != right.Weekday {
			return left.Weekday - right.Weekday
		}
		if left.StartMinute != right.StartMinute {
			return left.StartMinute - right.StartMinute
		}
		if left.UserID != right.UserID {
			if left.UserID == userID {
				return -1
			}
			if right.UserID == userID {
				return 1
			}
			return int(left.UserID - right.UserID)
		}
		return int(left.ID - right.ID)
	})
	return items
}

func (s *agentTeamScheduleService) CreateAgentTeamSchedule(req request.CreateAgentTeamScheduleRequest, operator *dto.AuthPrincipal) (*models.AgentTeamSchedule, error) {
	_ = req
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) UpdateAgentTeamSchedule(req request.UpdateAgentTeamScheduleRequest, operator *dto.AuthPrincipal) error {
	_ = req
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) DeleteAgentTeamSchedule(id int64, operator *dto.AuthPrincipal) error {
	_ = id
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) PrepareDraft(teamID int64, operator *dto.AuthPrincipal) (*AgentTeamScheduleDraftResult, error) {
	_ = teamID
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) PublishDraft(teamID int64, allowCoverageGap bool, operator *dto.AuthPrincipal) (*AgentTeamSchedulePublishResult, error) {
	_ = teamID
	_ = allowCoverageGap
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) Rollback(teamID int64, operator *dto.AuthPrincipal) (*AgentTeamSchedulePublishResult, error) {
	_ = teamID
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) DisableEnforcement(teamID int64, operator *dto.AuthPrincipal) error {
	_ = teamID
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) BatchPreview(req request.AgentTeamScheduleBatchRequest, operator *dto.AuthPrincipal) (*AgentTeamScheduleBatchPreviewResult, error) {
	_ = req
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func (s *agentTeamScheduleService) BatchGenerate(req request.AgentTeamScheduleBatchRequest, operator *dto.AuthPrincipal) (*AgentTeamScheduleBatchGenerateResult, error) {
	_ = req
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return nil, errorsx.InvalidParam(legacyAgentTeamScheduleWriteDisabledMessage)
}

func NormalizeAgentTeamScheduleRepeatType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AgentTeamScheduleRepeatOnce:
		return AgentTeamScheduleRepeatOnce
	case AgentTeamScheduleRepeatWeekly:
		return AgentTeamScheduleRepeatWeekly
	default:
		return ""
	}
}

func NormalizeAgentTeamScheduleDayType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AgentTeamScheduleDayTypeWork:
		return AgentTeamScheduleDayTypeWork
	case AgentTeamScheduleDayTypeRest:
		return AgentTeamScheduleDayTypeRest
	default:
		return ""
	}
}

func weeklyScheduleAnchor(weekday, minute int) time.Time {
	return weeklyScheduleAnchorInLocation(weekday, minute, time.Local)
}

func weeklyScheduleAnchorInLocation(weekday, minute int, location *time.Location) time.Time {
	monday := time.Date(2000, time.January, 3, 0, 0, 0, 0, location)
	return monday.AddDate(0, 0, weekday-1).Add(time.Duration(minute) * time.Minute)
}

func parseRequiredClockInLocation(value string, location *time.Location, emptyKey, formatKey string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errorsx.InvalidParamI18n(emptyKey)
	}
	if isScheduleClockEndOfDay(value) {
		return time.Date(0, time.January, 1, 0, 0, 0, 0, location), nil
	}
	layouts := []string{"15:04", "15:04:05"}
	for _, layout := range layouts {
		if ret, err := time.ParseInLocation(layout, value, location); err == nil {
			return ret, nil
		}
	}
	return time.Time{}, errorsx.InvalidParamI18n(formatKey)
}

func scheduleClockMinute(raw string, clock time.Time) int {
	if isScheduleClockEndOfDay(raw) {
		return 24 * 60
	}
	return clock.Hour()*60 + clock.Minute()
}

func isScheduleClockEndOfDay(value string) bool {
	value = strings.TrimSpace(value)
	return value == "24:00" || value == "24:00:00"
}

func weekdayForBatchRequest(value time.Time) int {
	if value.Weekday() == time.Sunday {
		return 7
	}
	return int(value.Weekday())
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	ret := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}

func parseRequiredDateTime(value, emptyKey, formatKey string) (time.Time, error) {
	return parseRequiredDateTimeInLocation(value, time.Local, emptyKey, formatKey)
}

func parseRequiredDateTimeInLocation(value string, location *time.Location, emptyKey, formatKey string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errorsx.InvalidParamI18n(emptyKey)
	}
	ret, err := parseDateTimeValueInLocation(value, location)
	if err != nil {
		return time.Time{}, errorsx.InvalidParamI18n(formatKey)
	}
	return ret, nil
}

func parseDateTimeValue(value string) (time.Time, error) {
	return parseDateTimeValueInLocation(value, time.Local)
}

func parseDateTimeValueInLocation(value string, location *time.Location) (time.Time, error) {
	layouts := []string{
		time.DateTime,
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if ret, err := time.ParseInLocation(layout, value, location); err == nil {
			return ret, nil
		}
	}
	return time.Time{}, errorsx.InvalidParamI18n("error.e0227")
}

func startOfDayInLocation(value time.Time, location *time.Location) time.Time {
	year, month, day := value.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func normalizeAgentTeamSchedulePublishStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AgentTeamSchedulePublishPublished:
		return AgentTeamSchedulePublishPublished
	case AgentTeamSchedulePublishDraft:
		return AgentTeamSchedulePublishDraft
	case AgentTeamSchedulePublishArchived:
		return AgentTeamSchedulePublishArchived
	default:
		return ""
	}
}

func NormalizeAgentTeamSchedulePublishStatus(value string) string {
	return normalizeAgentTeamSchedulePublishStatus(value)
}

func scheduleLocation(value string) *time.Location {
	if location, err := time.LoadLocation(strings.TrimSpace(value)); err == nil {
		return location
	}
	return time.Local
}

func weeklyScheduleOccurrence(item models.AgentTeamSchedule, localDate time.Time) (time.Time, time.Time, bool) {
	if NormalizeAgentTeamScheduleDayType(item.DayType) == AgentTeamScheduleDayTypeRest {
		return time.Time{}, time.Time{}, false
	}
	location := scheduleLocation(item.Timezone)
	date := startOfDayInLocation(localDate, location)
	if weekdayForBatchRequest(date) != item.Weekday {
		return time.Time{}, time.Time{}, false
	}
	if item.EffectiveFrom != nil && date.Before(startOfDayInLocation(*item.EffectiveFrom, location)) {
		return time.Time{}, time.Time{}, false
	}
	if item.EffectiveUntil != nil && date.After(startOfDayInLocation(*item.EffectiveUntil, location)) {
		return time.Time{}, time.Time{}, false
	}
	year, month, day := date.Date()
	startAt := time.Date(year, month, day, item.StartMinute/60, item.StartMinute%60, 0, 0, location)
	endMinute := item.EndMinute
	if endMinute > 24*60 {
		endMinute = 24 * 60
	}
	endAt := time.Date(year, month, day, endMinute/60, endMinute%60, 0, 0, location)
	if item.EndMinute < item.StartMinute {
		endAt = endAt.AddDate(0, 0, 1)
	}
	return startAt, endAt, true
}

func isValidScheduleMinute(value int) bool {
	return value >= 0 && value <= 24*60
}

func normalizeAgentTeamScheduleTemplateWorkdays(values []int) ([]int, error) {
	seen := make(map[int]struct{}, len(values))
	workdays := make([]int, 0, len(values))
	for _, weekday := range values {
		if weekday < 1 || weekday > 7 {
			return nil, errorsx.InvalidParamI18n("error.agentTeamSchedule.weekdayInvalid")
		}
		if _, ok := seen[weekday]; ok {
			continue
		}
		seen[weekday] = struct{}{}
		workdays = append(workdays, weekday)
	}
	if len(workdays) == 0 {
		return nil, errorsx.InvalidParamI18n("error.agentTeamSchedule.templateWorkdaysRequired")
	}
	slices.Sort(workdays)
	return workdays, nil
}

func parseAgentTeamScheduleTemplateWorkdays(value string) ([]int, error) {
	var workdays []int
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &workdays); err != nil {
		return nil, err
	}
	return normalizeAgentTeamScheduleTemplateWorkdays(workdays)
}
