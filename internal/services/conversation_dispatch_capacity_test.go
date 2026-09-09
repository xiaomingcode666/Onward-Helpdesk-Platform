package services

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

func TestConversationDispatchIgnoresProductScheduleRows(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":         1,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatalf("enforce schedule: %v", err)
	}
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	rows := []models.AgentTeamSchedule{
		{
			TenantID: 1, TeamID: 1, UserID: 101, RepeatType: AgentTeamScheduleRepeatOnce,
			StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour),
			PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		},
		{
			TenantID: 1, TeamID: 1, UserID: 102,
			RepeatType: AgentTeamScheduleRepeatWeekly, DayType: AgentTeamScheduleDayTypeRest,
			Weekday: 1, Timezone: "UTC", PublishStatus: AgentTeamSchedulePublishPublished, Version: 1,
			StartAt: time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC),
			Status: enums.StatusOk,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create legacy schedule rows: %v", err)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(legacy schedules) error = %v", err)
	}
	if len(candidates) != 2 || report.Reason != "ok" {
		t.Fatalf("legacy product schedule rows must not restrict dispatch candidates: candidates=%+v report=%+v", candidates, report)
	}
	for _, userID := range []int64{101, 102} {
		found := false
		for _, candidate := range candidates {
			if candidate.profile.UserID == userID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidate user %d was unexpectedly removed by legacy schedule rows: %+v", userID, candidates)
		}
	}
	publicCandidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{TenantID: 1, TeamID: 1}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates(member availability) error = %v", err)
	}
	if len(publicCandidates) != 2 {
		t.Fatalf("public member-availability candidate count = %d, want 2", len(publicCandidates))
	}
	for _, publicCandidate := range publicCandidates {
		if !publicCandidate.TeamEligible || !publicCandidate.WorkStatusConfirmed || !publicCandidate.Reachable || !publicCandidate.CapacityAvailable {
			t.Fatalf("public candidate availability fields should stay explicit: %+v", publicCandidate)
		}
	}
}

func ensureEnterpriseTemplateCoversTime(t *testing.T, db *gorm.DB, tenantID int64, at time.Time) {
	t.Helper()
	local := at.In(engineerScheduleLocation())
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := max(0, minute-1)
	endMinute := min(24*60, minute+30)
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	createEnterpriseTemplate(t, db, tenantID, []int{weekday}, startMinute, endMinute)
}

func ensureEnterpriseTemplateExcludesTime(t *testing.T, db *gorm.DB, tenantID int64, at time.Time) {
	t.Helper()
	local := at.In(engineerScheduleLocation())
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	restWeekday := weekday%7 + 1
	createEnterpriseTemplate(t, db, tenantID, []int{restWeekday}, 0, 1)
}

func createEnterpriseTemplate(t *testing.T, db *gorm.DB, tenantID int64, workdays []int, startMinute, endMinute int) {
	t.Helper()
	workdaysJSON := "["
	for index, weekday := range workdays {
		if index > 0 {
			workdaysJSON += ","
		}
		workdaysJSON += strconv.Itoa(weekday)
	}
	workdaysJSON += "]"
	values := map[string]any{
		"workdays":     workdaysJSON,
		"start_minute": startMinute,
		"end_minute":   endMinute,
		"timezone":     EngineerScheduleTimezone,
		"status":       enums.StatusOk,
		"updated_at":   time.Now(),
	}
	result := db.Model(&models.AgentTeamScheduleTemplate{}).
		Where("tenant_id = ?", tenantID).
		Updates(values)
	if result.Error != nil {
		t.Fatalf("update enterprise template: %v", result.Error)
	}
	if result.RowsAffected > 0 {
		return
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    workdaysJSON,
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    EngineerScheduleTimezone,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}).Error; err != nil {
		t.Fatalf("create enterprise template: %v", err)
	}
}

func TestConversationDispatchPersonalRuleBlocksOutsideWorkdays(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Date(2026, time.August, 15, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	ensureEnterpriseTemplateExcludesTime(t, db, 1, now)

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(personal rule) error = %v", err)
	}
	if len(candidates) != 0 || report.Reason != "outside_personal_dispatch_rule" {
		t.Fatalf("personal workday rule should block dispatch outside workdays: candidates=%+v report=%+v", candidates, report)
	}
}

func TestConversationDispatchApprovedLeaveExcludesMember(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	if err := db.Create(&models.AgentScheduleException{
		TenantID: 1, UserID: 101, RequestKey: "approved-leave-101",
		ExceptionType: AgentScheduleExceptionTypeLeave,
		StartAt:       now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: AgentScheduleApprovalApproved,
		RequestedAt:    now.Add(-2 * time.Hour),
		Status:         enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create approved leave: %v", err)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(approved leave) error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].profile.UserID != 102 || report.Reason != "ok" {
		t.Fatalf("approved leave should remove only the leave member: candidates=%+v report=%+v", candidates, report)
	}
}

func TestConversationDispatchUnavailableReasonDistinguishesLeaveFromReachability(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	staleOnlineAt := now.Add(-24 * time.Hour)
	if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).Updates(map[string]any{
		"last_online_at":          staleOnlineAt,
		"receive_offline_message": false,
	}).Error; err != nil {
		t.Fatalf("make profile unreachable: %v", err)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(unreachable) error = %v", err)
	}
	if len(candidates) != 0 || report.Reason != "no_reachable_user" {
		t.Fatalf("unreachable engineer should not be reported as leave: candidates=%+v report=%+v", candidates, report)
	}
}

func TestConversationDispatchMemberScheduleNoLongerRestrictsCandidatePool(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":         1,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatalf("enforce schedule: %v", err)
	}
	now := time.Now()
	active := models.AgentTeamSchedule{
		TenantID: 1, TeamID: 1, UserID: 102, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active member schedule: %v", err)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(member schedule) error = %v", err)
	}
	if len(candidates) != 2 || report.Reason != "ok" {
		t.Fatalf("member-level schedule should not restrict dispatch to the scheduled member: candidates=%+v report=%+v", candidates, report)
	}
}

func TestConversationDispatchTransactionIgnoresScheduleRowChanges(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":         1,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatalf("enforce schedule: %v", err)
	}
	now := time.Now()
	active := models.AgentTeamSchedule{
		TenantID: 1, TeamID: 1, UserID: 101, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active member schedule: %v", err)
	}
	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(member schedule) error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].profile.UserID != 101 || report.Reason != "ok" {
		t.Fatalf("expected initial candidate from active schedule: candidates=%+v report=%+v", candidates, report)
	}

	if err := db.Model(&models.AgentTeamSchedule{}).Where("id = ?", active.ID).Update("user_id", int64(102)).Error; err != nil {
		t.Fatalf("move schedule to another engineer: %v", err)
	}
	_, err = ConversationDispatchService.lockAndValidateDispatchCandidate(db, candidates[0], 1, 0)
	if err != nil {
		t.Fatalf("schedule row change must not invalidate the candidate: %v", err)
	}
}

func TestConversationDispatchRecoversPurePendingAssigneeWhenLeaveExcludesEngineer(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	agent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":         1,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatalf("enforce schedule: %v", err)
	}
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	if err := db.Create(&models.AgentScheduleException{
		TenantID: 1, UserID: 101, RequestKey: "recover-leave-101",
		ExceptionType: AgentScheduleExceptionTypeLeave,
		StartAt:       now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: AgentScheduleApprovalApproved,
		RequestedAt:    now.Add(-2 * time.Hour),
		Status:         enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create active leave: %v", err)
	}
	conversation := models.Conversation{
		TenantID:          1,
		AIAgentID:         agent.ID,
		CustomerName:      "仅人工待接入客户",
		Status:            enums.IMConversationStatusPending,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create pure pending conversation: %v", err)
	}
	if err := db.Create(&models.ConversationAssignment{
		ConversationID: conversation.ID,
		FromUserID:     0,
		ToUserID:       101,
		AssignType:     string(enums.IMAssignmentTypeAssign),
		Status:         enums.IMAssignmentStatusActive,
		CreatedAt:      now.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("create active assignment: %v", err)
	}

	recovered, err := ConversationDispatchService.RecoverTeamIneligibleAssignments(1, 1, now)
	if err != nil {
		t.Fatalf("RecoverTeamIneligibleAssignments() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered conversations = %d, want 1", recovered)
	}
	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if current.CurrentAssigneeID != 102 || current.CurrentTeamID != 1 || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("conversation was not redispatched to on-duty engineer: %+v", current)
	}
	var ticketCount int64
	if err := db.Model(&models.Ticket{}).Where("conversation_id = ?", conversation.ID).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count linked tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("pure conversation recovery should not create a ticket, got %d", ticketCount)
	}
	var activeAssignments int64
	if err := db.Model(&models.ConversationAssignment{}).
		Where("conversation_id = ? AND status = ?", conversation.ID, enums.IMAssignmentStatusActive).
		Count(&activeAssignments).Error; err != nil {
		t.Fatalf("count active assignments: %v", err)
	}
	if activeAssignments != 1 {
		t.Fatalf("active assignments = %d, want exactly the new assignee active", activeAssignments)
	}
	var newAssignment models.ConversationAssignment
	if err := db.Where("conversation_id = ? AND status = ?", conversation.ID, enums.IMAssignmentStatusActive).
		First(&newAssignment).Error; err != nil {
		t.Fatalf("load active assignment: %v", err)
	}
	if newAssignment.ToUserID != 102 {
		t.Fatalf("active assignment user = %d, want 102", newAssignment.ToUserID)
	}
}

func TestAgentTeamDisableClearsPurePendingConversationTeamPool(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	agent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	now := time.Now()
	conversation := models.Conversation{
		TenantID:          1,
		AIAgentID:         agent.ID,
		CustomerName:      "禁用团队中的待接入客户",
		Status:            enums.IMConversationStatusPending,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
		AuditFields:       models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create pure pending conversation: %v", err)
	}
	if err := db.Create(&models.ConversationAssignment{
		ConversationID: conversation.ID,
		FromUserID:     0,
		ToUserID:       101,
		AssignType:     string(enums.IMAssignmentTypeAssign),
		Status:         enums.IMAssignmentStatusActive,
		CreatedAt:      now.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("create active assignment: %v", err)
	}
	poolConversation := models.Conversation{
		TenantID:      1,
		AIAgentID:     agent.ID,
		CustomerName:  "禁用团队池中的无人接入客户",
		Status:        enums.IMConversationStatusPending,
		CurrentTeamID: 1,
		AuditFields:   models.AuditFields{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)},
	}
	if err := db.Create(&poolConversation).Error; err != nil {
		t.Fatalf("create pool conversation: %v", err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := AgentTeamService.UpdateAgentTeam(request.UpdateAgentTeamRequest{
		ID:     1,
		Name:   "售后支持组",
		Status: int(enums.StatusDisabled),
	}, manager); err != nil {
		t.Fatalf("UpdateAgentTeam() error = %v", err)
	}
	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("disabled team should return pure conversation to the global pool: %+v", current)
	}
	var poolCurrent models.Conversation
	if err := db.First(&poolCurrent, poolConversation.ID).Error; err != nil {
		t.Fatalf("reload pool conversation: %v", err)
	}
	if poolCurrent.CurrentAssigneeID != 0 || poolCurrent.CurrentTeamID != 0 || poolCurrent.Status != enums.IMConversationStatusPending {
		t.Fatalf("disabled team should release existing team-pool conversation: %+v", poolCurrent)
	}
	var activeAssignments int64
	if err := db.Model(&models.ConversationAssignment{}).
		Where("conversation_id = ? AND status = ?", conversation.ID, enums.IMAssignmentStatusActive).
		Count(&activeAssignments).Error; err != nil {
		t.Fatalf("count active assignments: %v", err)
	}
	if activeAssignments != 0 {
		t.Fatalf("active assignments = %d, want none after disabled team recovery", activeAssignments)
	}
}

func TestAgentTeamDeleteClearsUnassignedWorkPools(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.Create(&models.AgentTeam{ID: 50, TenantID: 1, Name: "临时维修组", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "DELETE-TEAM-POOL", Title: "删除团队池内工单", TenantID: 1, CurrentTeamID: 50,
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	conversation := models.Conversation{
		TenantID:      1,
		CustomerName:  "删除团队池内会话",
		Status:        enums.IMConversationStatusPending,
		CurrentTeamID: 50,
		AuditFields:   models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := AgentTeamService.DeleteAgentTeam(50, manager); err != nil {
		t.Fatalf("DeleteAgentTeam() error = %v", err)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.CurrentTeamID != 0 || currentTicket.CurrentAssigneeID != 0 || currentTicket.LastDispatchFailureReason != "team_disabled_pool_released" {
		t.Fatalf("deleted team should release ticket pool item: %+v", currentTicket)
	}
	var currentConversation models.Conversation
	if err := db.First(&currentConversation, conversation.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if currentConversation.CurrentTeamID != 0 || currentConversation.CurrentAssigneeID != 0 || currentConversation.Status != enums.IMConversationStatusPending {
		t.Fatalf("deleted team should release conversation pool item: %+v", currentConversation)
	}
}

func TestConversationDispatchRequiresExplicitTeamMembershipWhenMemberTableExists(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       101,
		Username: "legacy-profile-only",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create legacy profile user: %v", err)
	}
	profile := &models.AgentProfile{
		TenantID:           1,
		UserID:             101,
		TeamID:             1,
		AgentCode:          "LEGACY-101",
		DisplayName:        "旧档案工程师",
		ServiceStatus:      enums.ServiceStatusIdle,
		MaxConcurrentCount: 3,
		AutoAssignEnabled:  true,
		LastOnlineAt:       &now,
		Status:             enums.StatusOk,
	}
	if err := db.Create(profile).Error; err != nil {
		t.Fatalf("create legacy profile: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: 101, Status: AgentWorkStatusAvailable,
		ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(profile-only) error = %v", err)
	}
	if len(candidates) != 0 || report.Reason != "no_matched_profile" {
		t.Fatalf("profile-only engineer should not be dispatchable: candidates=%+v report=%+v", candidates, report)
	}
	_, err = ConversationDispatchService.lockAndValidateDispatchCandidate(db, dispatchCandidate{profile: *profile, teamID: 1}, 1, 0)
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) {
		t.Fatalf("profile-only transaction validation err = %v, want candidate unavailable", err)
	}
}

func TestConversationDispatchWeightedModeUsesProjectedLoad(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	if err := db.Model(&models.AgentProfile{}).Where("user_id IN ?", []int64{101, 102}).
		Update("max_concurrent_count", 10).Error; err != nil {
		t.Fatalf("raise weighted test capacity: %v", err)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("assignment_mode", AgentTeamAssignmentModeWeighted).Error; err != nil {
		t.Fatalf("set weighted assignment mode: %v", err)
	}
	if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ? AND user_id = ?", 1, 101).
		Update("dispatch_weight", 1).Error; err != nil {
		t.Fatalf("set low weight: %v", err)
	}
	if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ? AND user_id = ?", 1, 102).
		Update("dispatch_weight", 3).Error; err != nil {
		t.Fatalf("set high weight: %v", err)
	}

	now := time.Now()
	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(initial) error = %v", err)
	}
	if len(candidates) < 2 || report.Reason != "ok" || candidates[0].profile.UserID != 102 {
		t.Fatalf("empty weighted pool should choose high-weight candidate first: candidates=%+v report=%+v", candidates, report)
	}

	if err := db.Create(&[]models.Ticket{
		{TicketNo: "W-102-1", Title: "高权重第 1 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 102, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "W-102-2", Title: "高权重第 2 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 102, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}).Error; err != nil {
		t.Fatalf("create high-weight workload: %v", err)
	}
	candidates, report, err = ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(weight boundary) error = %v", err)
	}
	if len(candidates) < 2 || report.Reason != "ok" || candidates[0].profile.UserID != 102 {
		t.Fatalf("weighted 3:1 boundary should still choose high-weight candidate: candidates=%+v report=%+v", candidates, report)
	}

	if err := db.Create(&models.Ticket{
		TicketNo: "W-102-3", Title: "高权重第 3 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 102,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create third high-weight workload: %v", err)
	}
	candidates, report, err = ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pickDispatchCandidates(after ratio) error = %v", err)
	}
	if len(candidates) < 2 || report.Reason != "ok" || candidates[0].profile.UserID != 101 {
		t.Fatalf("after high-weight reaches ratio, low-weight candidate should receive the next ticket: candidates=%+v report=%+v", candidates, report)
	}
}

func TestConversationDispatchRechecksCandidateCapacityInsideTransaction(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope engineer team: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Updates(map[string]any{
		"tenant_id":            1,
		"max_concurrent_count": 1,
	}).Error; err != nil {
		t.Fatalf("limit engineer capacity: %v", err)
	}
	conversation := models.Conversation{
		TenantID:     1,
		CustomerName: "等待派单的客户",
		Status:       enums.IMConversationStatusPending,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create pending conversation: %v", err)
	}

	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatalf("load dispatch profile: %v", err)
	}
	candidate := dispatchCandidate{profile: profile, teamID: 1}

	// The candidate was selected while idle, but another request consumed the
	// final slot before this assignment transaction started.
	if err := db.Create(&models.Ticket{
		TicketNo:          "CAPACITY-RACE-1",
		Title:             "并发请求已占用容量",
		Status:            enums.TicketStatusProcessing,
		CurrentAssigneeID: profile.UserID,
	}).Error; err != nil {
		t.Fatalf("create concurrent workload: %v", err)
	}

	dispatched, err := ConversationDispatchService.tryAssignConversation(conversation.ID, candidate, "自动分配")
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
		t.Fatalf("stale candidate assignment = (%+v, %v), want capacity rejection", dispatched, err)
	}
	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("capacity rejection mutated conversation: %+v", current)
	}
	var assignmentCount int64
	if err := db.Model(&models.ConversationAssignment{}).Where("conversation_id = ?", conversation.ID).Count(&assignmentCount).Error; err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("capacity rejection created %d assignments", assignmentCount)
	}
}

func TestConversationDispatchRechecksZeroCapacityInsideTransaction(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope engineer team: %v", err)
	}
	conversation := models.Conversation{
		TenantID:     1,
		CustomerName: "等待派单的客户",
		Status:       enums.IMConversationStatusPending,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create pending conversation: %v", err)
	}

	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatalf("load dispatch profile: %v", err)
	}
	candidate := dispatchCandidate{profile: profile, teamID: 1}
	if err := db.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).Update("max_concurrent_count", 0).Error; err != nil {
		t.Fatalf("disable candidate capacity: %v", err)
	}

	dispatched, err := ConversationDispatchService.tryAssignConversation(conversation.ID, candidate, "自动分配")
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
		t.Fatalf("zero-capacity stale candidate assignment = (%+v, %v), want candidate unavailable", dispatched, err)
	}
	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("zero-capacity rejection mutated conversation: %+v", current)
	}
}

func TestConversationDispatchRechecksApprovedLeaveInsideTransaction(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.AgentWorkStatus{}); err != nil {
		t.Fatalf("migrate engineer work status: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope engineer team: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope engineer profile: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.AgentScheduleException{
		TenantID: 1, UserID: 101, RequestKey: "transaction-approved-leave-101",
		ExceptionType: AgentScheduleExceptionTypeLeave,
		StartAt:       now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: AgentScheduleApprovalApproved, RequestedAt: now.Add(-2 * time.Hour), Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create approved leave: %v", err)
	}
	conversation := models.Conversation{TenantID: 1, CustomerName: "请假期间客户", Status: enums.IMConversationStatusPending}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create pending conversation: %v", err)
	}
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatalf("load dispatch profile: %v", err)
	}
	dispatched, err := ConversationDispatchService.tryAssignConversation(conversation.ID, dispatchCandidate{profile: profile, teamID: 1}, "自动分配")
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
		t.Fatalf("leave candidate assignment = (%+v, %v), want rejection", dispatched, err)
	}
	assertDispatchConversationUnassigned(t, db, conversation.ID)
}

func TestConversationDispatchRechecksConfigurationAndReachabilityInsideTransaction(t *testing.T) {
	previousWS := WsService
	WsService = newWsService()
	t.Cleanup(func() { WsService = previousWS })

	cases := []struct {
		name   string
		mutate func(t *testing.T, db *gorm.DB, profile models.AgentProfile)
	}{
		{
			name: "global automatic dispatch disabled",
			mutate: func(t *testing.T, db *gorm.DB, profile models.AgentProfile) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).Update("auto_assign_enabled", false).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "team automatic dispatch disabled",
			mutate: func(t *testing.T, db *gorm.DB, profile models.AgentProfile) {
				t.Helper()
				if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ? AND user_id = ?", 1, profile.UserID).Update("dispatch_enabled", false).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "busy service status",
			mutate: func(t *testing.T, db *gorm.DB, profile models.AgentProfile) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).Update("service_status", enums.ServiceStatusBusy).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "stale unreachable profile",
			mutate: func(t *testing.T, db *gorm.DB, profile models.AgentProfile) {
				t.Helper()
				stale := time.Now().Add(-48 * time.Hour)
				if err := db.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).Updates(map[string]any{
					"last_online_at": stale, "receive_offline_message": false,
				}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupHumanDispatchRealtimeTestDB(t)
			createHumanDispatchRealtimeTeam(t, db, 1)
			createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
			conversation := models.Conversation{TenantID: 1, CustomerName: "事务资格复核", Status: enums.IMConversationStatusPending}
			if err := db.Create(&conversation).Error; err != nil {
				t.Fatal(err)
			}
			var profile models.AgentProfile
			if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
				t.Fatal(err)
			}
			tc.mutate(t, db, profile)

			dispatched, err := ConversationDispatchService.tryAssignConversation(conversation.ID, dispatchCandidate{
				profile: profile, teamID: 1, teamDispatchEnabled: true,
			}, "自动分配")
			if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
				t.Fatalf("stale %s candidate assignment = (%+v, %v), want candidate unavailable", tc.name, dispatched, err)
			}
			assertDispatchConversationUnassigned(t, db, conversation.ID)
		})
	}
}

func TestConversationDispatchConcurrentCapacityDoesNotOversell(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	rawDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// SQLite does not implement PostgreSQL row locks. One connection still lets
	// this test prove that each transaction re-reads workload before committing.
	rawDB.SetMaxOpenConns(1)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope engineer team: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Updates(map[string]any{
		"tenant_id":            1,
		"max_concurrent_count": 1,
	}).Error; err != nil {
		t.Fatalf("scope engineer capacity: %v", err)
	}
	conversations := []models.Conversation{
		{TenantID: 1, CustomerName: "并发客户一", Status: enums.IMConversationStatusPending},
		{TenantID: 1, CustomerName: "并发客户二", Status: enums.IMConversationStatusPending},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create concurrent conversations: %v", err)
	}
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatalf("load dispatch profile: %v", err)
	}
	candidate := dispatchCandidate{profile: profile, teamID: 1}

	start := make(chan struct{})
	results := make(chan *models.Conversation, len(conversations))
	errorsFound := make(chan error, len(conversations))
	var wait sync.WaitGroup
	for index := range conversations {
		conversationID := conversations[index].ID
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			assigned, assignErr := ConversationDispatchService.tryAssignConversation(conversationID, candidate, "并发自动分配")
			results <- assigned
			errorsFound <- assignErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)

	assignedCount := 0
	for assigned := range results {
		if assigned != nil {
			assignedCount++
		}
	}
	capacityRejected := 0
	for assignErr := range errorsFound {
		switch {
		case assignErr == nil:
		case errors.Is(assignErr, errConversationDispatchCandidateUnavailable):
			capacityRejected++
		default:
			t.Fatalf("unexpected concurrent dispatch error: %v", assignErr)
		}
	}
	if assignedCount != 1 || capacityRejected != 1 {
		t.Fatalf("concurrent dispatch = assigned:%d capacity_rejected:%d, want 1/1", assignedCount, capacityRejected)
	}
	var persistedAssignments int64
	if err := db.Model(&models.Conversation{}).Where("current_assignee_id = ?", 101).Count(&persistedAssignments).Error; err != nil {
		t.Fatalf("count persisted assignments: %v", err)
	}
	if persistedAssignments != 1 {
		t.Fatalf("persisted assignments = %d, want 1", persistedAssignments)
	}
	var durableEventCount, outboxCount int64
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventConversationAssigned).Count(&durableEventCount).Error; err != nil {
		t.Fatalf("count durable assignment events: %v", err)
	}
	if err := db.Model(&models.OutboxRecord{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count assignment outbox records: %v", err)
	}
	if durableEventCount != 1 || outboxCount != 1 {
		t.Fatalf("durable assignment events=%d outbox=%d, want 1/1", durableEventCount, outboxCount)
	}
}

func TestConversationDispatchFiltersTeamsByTenantAndProduct(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	teams := []models.AgentTeam{
		{ID: 1, TenantID: 1, ProductID: 11, Name: "当前产品组", Status: enums.StatusOk},
		{ID: 2, TenantID: 2, ProductID: 11, Name: "其他租户产品组", Status: enums.StatusOk},
		{ID: 3, TenantID: 1, ProductID: 33, Name: "同租户其他产品组", Status: enums.StatusOk},
		{ID: 4, TenantID: 1, ProductID: 0, Name: "租户公共支持组", Status: enums.StatusOk},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create scoped teams: %v", err)
	}

	got := ConversationDispatchService.findEligibleTeamIDs(1, 11, []int64{2, 3, 1, 4, 1})
	if len(got) != 2 || got[0] != 1 || got[1] != 4 {
		t.Fatalf("eligible teams = %v, want current product team and tenant-wide support team", got)
	}
}

func TestConversationDispatchTransactionRejectsForeignTenantCandidate(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 2)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 2)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 2).Updates(map[string]any{
		"tenant_id":  2,
		"product_id": 22,
	}).Error; err != nil {
		t.Fatalf("scope foreign team: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 202).Update("tenant_id", 2).Error; err != nil {
		t.Fatalf("scope foreign profile: %v", err)
	}
	conversation := models.Conversation{
		TenantID:     1,
		ProductID:    11,
		CustomerName: "租户一客户",
		Status:       enums.IMConversationStatusPending,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create tenant conversation: %v", err)
	}
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 202).First(&profile).Error; err != nil {
		t.Fatalf("load foreign profile: %v", err)
	}

	dispatched, err := ConversationDispatchService.tryAssignConversation(
		conversation.ID,
		dispatchCandidate{profile: profile, teamID: 2},
		"错误跨租户候选",
	)
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
		t.Fatalf("foreign tenant assignment = (%+v, %v), want rejection", dispatched, err)
	}
	assertDispatchConversationUnassigned(t, db, conversation.ID)
}

func TestConversationDispatchTransactionRejectsOtherProductCandidate(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 3)
	createHumanDispatchRealtimeAgentProfile(t, db, 303, 3)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 3).Updates(map[string]any{
		"tenant_id":  1,
		"product_id": 33,
	}).Error; err != nil {
		t.Fatalf("scope other product team: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 303).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope profile: %v", err)
	}
	conversation := models.Conversation{
		TenantID:     1,
		ProductID:    11,
		CustomerName: "产品十一客户",
		Status:       enums.IMConversationStatusPending,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create product conversation: %v", err)
	}
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 303).First(&profile).Error; err != nil {
		t.Fatalf("load other product profile: %v", err)
	}

	dispatched, err := ConversationDispatchService.tryAssignConversation(
		conversation.ID,
		dispatchCandidate{profile: profile, teamID: 3},
		"错误跨产品候选",
	)
	if !errors.Is(err, errConversationDispatchCandidateUnavailable) || dispatched != nil {
		t.Fatalf("other product assignment = (%+v, %v), want rejection", dispatched, err)
	}
	assertDispatchConversationUnassigned(t, db, conversation.ID)
}

func TestConversationDispatchRejectsAgentScopeMismatch(t *testing.T) {
	conversation := &models.Conversation{TenantID: 1, ProductID: 11}
	if _, _, ok := resolveConversationDispatchScope(conversation, &models.AIAgent{TenantID: 2, ProductID: 11}); ok {
		t.Fatal("agent from another tenant must not dispatch the conversation")
	}
	if _, _, ok := resolveConversationDispatchScope(conversation, &models.AIAgent{TenantID: 1, ProductID: 22}); ok {
		t.Fatal("agent from another product must not dispatch the conversation")
	}
	tenantID, productID, ok := resolveConversationDispatchScope(conversation, &models.AIAgent{TenantID: 1, ProductID: 11})
	if !ok || tenantID != 1 || productID != 11 {
		t.Fatalf("matching dispatch scope = (%d, %d, %v)", tenantID, productID, ok)
	}
}

func assertDispatchConversationUnassigned(t *testing.T, db *gorm.DB, conversationID int64) {
	t.Helper()
	var conversation models.Conversation
	if err := db.First(&conversation, conversationID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if conversation.CurrentAssigneeID != 0 || conversation.CurrentTeamID != 0 || conversation.Status != enums.IMConversationStatusPending {
		t.Fatalf("rejected dispatch mutated conversation: %+v", conversation)
	}
	var assignmentCount int64
	if err := db.Model(&models.ConversationAssignment{}).Where("conversation_id = ?", conversationID).Count(&assignmentCount).Error; err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("rejected dispatch created %d assignments", assignmentCount)
	}
}
