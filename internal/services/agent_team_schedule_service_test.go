package services_test

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAgentTeamScheduleServiceFindCalendarSchedulesReturnsIntersectingSchedules(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	createAgentTeamScheduleTestData(t, db)

	list, err := services.AgentTeamScheduleService.FindCalendarSchedules(request.AgentTeamScheduleCalendarRequest{
		StartAt: "2026-04-27 00:00:00",
		EndAt:   "2026-05-04 00:00:00",
	})
	if err != nil {
		t.Fatalf("FindCalendarSchedules() error = %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 intersecting schedules, got %d: %+v", len(list), list)
	}
	gotIDs := make([]int64, 0, len(list))
	for _, item := range list {
		gotIDs = append(gotIDs, item.ID)
	}
	wantIDs := []int64{1, 2, 3}
	for i, want := range wantIDs {
		if gotIDs[i] != want {
			t.Fatalf("expected ids %v, got %v", wantIDs, gotIDs)
		}
	}
}

func TestAgentTeamScheduleServiceFindCalendarSchedulesFiltersTeamID(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	createAgentTeamScheduleTestData(t, db)

	list, err := services.AgentTeamScheduleService.FindCalendarSchedules(request.AgentTeamScheduleCalendarRequest{
		StartAt: "2026-04-27 00:00:00",
		EndAt:   "2026-05-04 00:00:00",
		TeamID:  2,
	})
	if err != nil {
		t.Fatalf("FindCalendarSchedules() error = %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 schedule for team 2, got %d: %+v", len(list), list)
	}
	if list[0].ID != 3 || list[0].TeamID != 2 {
		t.Fatalf("unexpected schedule: %+v", list[0])
	}
}

func TestAgentTeamScheduleServiceFindCalendarSchedulesValidatesTimeRange(t *testing.T) {
	setupAgentTeamScheduleTestDB(t)

	_, err := services.AgentTeamScheduleService.FindCalendarSchedules(request.AgentTeamScheduleCalendarRequest{
		StartAt: "2026-05-04 00:00:00",
		EndAt:   "2026-04-27 00:00:00",
	})
	if err == nil {
		t.Fatalf("expected invalid time range to fail")
	}
}

func TestAgentTeamScheduleServiceGetMyWeeklyScheduleScopesToEngineer(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	now := time.Now()
	teams := []models.AgentTeam{
		{ID: 1, TenantID: 1, ProductID: 100, Name: "泵维修组", TeamType: services.AgentTeamTypeProductRepair, Status: enums.StatusOk},
		{ID: 2, TenantID: 1, ProductID: 101, Name: "阀维修组", TeamType: services.AgentTeamTypeProductRepair, Status: enums.StatusOk},
		{ID: 3, TenantID: 1, ProductID: 0, Name: "自定义组", TeamType: "custom", Status: enums.StatusOk},
		{ID: 4, TenantID: 2, ProductID: 200, Name: "其他租户组", TeamType: services.AgentTeamTypeProductRepair, Status: enums.StatusOk},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams error = %v", err)
	}
	members := []models.AgentTeamMember{
		{TenantID: 1, TeamID: 1, UserID: 101, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 2, UserID: 101, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 3, UserID: 101, Status: enums.StatusOk},
		{TenantID: 2, TeamID: 4, UserID: 101, Status: enums.StatusOk},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("create members error = %v", err)
	}
	schedules := []models.AgentTeamSchedule{
		{ID: 11, TenantID: 1, TeamID: 1, UserID: 101, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 1, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 12, TenantID: 1, TeamID: 2, UserID: 101, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 2, StartMinute: 600, EndMinute: 1080, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 13, TenantID: 1, TeamID: 1, UserID: 102, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 1, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 14, TenantID: 1, TeamID: 1, UserID: 101, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 3, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishDraft, Version: 2, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 15, TenantID: 1, TeamID: 3, UserID: 101, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 4, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 16, TenantID: 2, TeamID: 4, UserID: 101, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 5, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 17, TenantID: 1, TeamID: 1, UserID: 0, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 2, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 18, TenantID: 1, TeamID: 2, UserID: 0, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 4, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishDraft, Version: 2, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{ID: 19, TenantID: 1, TeamID: 2, UserID: 102, RepeatType: services.AgentTeamScheduleRepeatWeekly, Weekday: 5, StartMinute: 540, EndMinute: 1020, Timezone: "Asia/Shanghai", PublishStatus: services.AgentTeamSchedulePublishDraft, Version: 2, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("create schedules error = %v", err)
	}

	result, err := services.AgentTeamScheduleService.GetMyWeeklySchedule(&dto.AuthPrincipal{TenantID: 1, UserID: 101})
	if err != nil {
		t.Fatalf("GetMyWeeklySchedule() error = %v", err)
	}
	if !result.IsEngineer {
		t.Fatalf("expected current member to be treated as engineer")
	}
	if len(result.Teams) != 2 {
		t.Fatalf("expected two product repair teams, got %d: %+v", len(result.Teams), result.Teams)
	}
	if result.BaseSchedule == nil || result.BaseSchedule.TenantID != 1 || result.BaseSchedule.StartMinute != 0 || result.BaseSchedule.EndMinute != 24*60 {
		t.Fatalf("expected enterprise base schedule in personal response, got %+v", result.BaseSchedule)
	}
	if len(result.BaseSchedule.Workdays) != 5 {
		t.Fatalf("expected five base workdays, got %v", result.BaseSchedule.Workdays)
	}
	for index, weekday := range []int{1, 2, 3, 4, 5} {
		if result.BaseSchedule.Workdays[index] != weekday {
			t.Fatalf("expected weekday base schedule, got %v", result.BaseSchedule.Workdays)
		}
	}
	gotIDs := make([]int64, 0, len(result.Schedules))
	for _, item := range result.Schedules {
		gotIDs = append(gotIDs, item.ID)
	}
	wantIDs := []int64{11, 17, 12}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("expected schedule ids %v, got %v", wantIDs, gotIDs)
	}
	for index, want := range wantIDs {
		if gotIDs[index] != want {
			t.Fatalf("expected schedule ids %v, got %v", wantIDs, gotIDs)
		}
	}
	draftIDs := make([]int64, 0, len(result.DraftSchedules))
	for _, item := range result.DraftSchedules {
		draftIDs = append(draftIDs, item.ID)
	}
	wantDraftIDs := []int64{14, 18}
	if len(draftIDs) != len(wantDraftIDs) {
		t.Fatalf("expected draft schedule ids %v, got %v", wantDraftIDs, draftIDs)
	}
	for index, want := range wantDraftIDs {
		if draftIDs[index] != want {
			t.Fatalf("expected draft schedule ids %v, got %v", wantDraftIDs, draftIDs)
		}
	}
}

func TestAgentTeamScheduleServiceLegacyWriteMethodsAreDisabled(t *testing.T) {
	operator := testOperator()
	assertDisabled := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "旧排班写入已下线") {
			t.Fatalf("%s error = %v, want legacy write disabled", name, err)
		}
	}

	_, err := services.AgentTeamScheduleService.CreateAgentTeamSchedule(request.CreateAgentTeamScheduleRequest{TeamID: 1}, operator)
	assertDisabled("create", err)
	assertDisabled("update", services.AgentTeamScheduleService.UpdateAgentTeamSchedule(request.UpdateAgentTeamScheduleRequest{ID: 1}, operator))
	assertDisabled("delete", services.AgentTeamScheduleService.DeleteAgentTeamSchedule(1, operator))
	_, err = services.AgentTeamScheduleService.PrepareDraft(1, operator)
	assertDisabled("prepare draft", err)
	_, err = services.AgentTeamScheduleService.PublishDraft(1, false, operator)
	assertDisabled("publish", err)
	_, err = services.AgentTeamScheduleService.Rollback(1, operator)
	assertDisabled("rollback", err)
	assertDisabled("disable", services.AgentTeamScheduleService.DisableEnforcement(1, operator))
	_, err = services.AgentTeamScheduleService.BatchPreview(request.AgentTeamScheduleBatchRequest{TeamIDs: []int64{1}}, operator)
	assertDisabled("batch preview", err)
	_, err = services.AgentTeamScheduleService.BatchGenerate(request.AgentTeamScheduleBatchRequest{TeamIDs: []int64{1}}, operator)
	assertDisabled("batch generate", err)
}

func TestAgentTeamScheduleServiceListTeamMemberAvailabilityUsesPersonalWorkRuleAndLeave(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	now := time.Date(2026, time.August, 10, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	tenant := models.Tenant{ID: 1, Name: "测试企业", Status: enums.StatusOk}
	team := models.AgentTeam{ID: 22, TenantID: 1, ProductID: 220, TeamType: services.AgentTeamTypeProductRepair, Name: "产品可用性组", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	users := []models.User{
		{ID: 401, Username: "available.engineer", Nickname: "可接单工程师", Status: enums.StatusOk},
		{ID: 402, Username: "leave.engineer", Nickname: "请假工程师", Status: enums.StatusOk},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	profiles := []models.AgentProfile{
		{TenantID: 1, UserID: 401, TeamID: team.ID, AgentCode: "ENG-401", DisplayName: "旧名 A", ServiceStatus: enums.ServiceStatusIdle, MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk},
		{TenantID: 1, UserID: 402, TeamID: team.ID, AgentCode: "ENG-402", DisplayName: "旧名 B", ServiceStatus: enums.ServiceStatusIdle, MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatal(err)
	}
	members := []models.AgentTeamMember{
		{TenantID: 1, TeamID: team.ID, UserID: 401, DispatchEnabled: true, DispatchWeight: 2, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 402, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatal(err)
	}
	statuses := []models.AgentWorkStatus{
		{TenantID: 1, UserID: 401, Status: services.AgentWorkStatusAvailable, ConfirmedAt: now.Add(-time.Minute), StatusChangedAt: now.Add(-time.Minute)},
		{TenantID: 1, UserID: 402, Status: services.AgentWorkStatusAvailable, ConfirmedAt: now.Add(-time.Minute), StatusChangedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&statuses).Error; err != nil {
		t.Fatal(err)
	}
	leaves := []models.AgentScheduleException{
		{TenantID: 1, UserID: 402, RequestKey: "approved-leave", ExceptionType: services.AgentScheduleExceptionTypeLeave, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour), ApprovalStatus: services.AgentScheduleApprovalApproved, RequestedAt: now.Add(-2 * time.Hour), Status: enums.StatusOk},
		{TenantID: 1, UserID: 401, RequestKey: "pending-leave", ExceptionType: services.AgentScheduleExceptionTypeLeave, StartAt: now.Add(24 * time.Hour), EndAt: now.Add(48 * time.Hour), ApprovalStatus: services.AgentScheduleApprovalPending, RequestedAt: now.Add(-time.Hour), Status: enums.StatusOk},
	}
	if err := db.Create(&leaves).Error; err != nil {
		t.Fatal(err)
	}

	result, err := services.AgentTeamScheduleService.ListTeamMemberAvailability(team.ID, &dto.AuthPrincipal{TenantID: 1, UserID: 1, Username: "tester"}, now)
	if err != nil {
		t.Fatalf("ListTeamMemberAvailability() error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("availability count = %d, want 2: %+v", len(result), result)
	}
	byUserID := map[int64]services.AgentTeamMemberAvailability{}
	for _, item := range result {
		byUserID[item.Profile.UserID] = item
	}
	available := byUserID[401]
	if !available.AvailableNow || available.UnavailableReason != "" || available.StartMinute != 0 || available.EndMinute != 24*60 {
		t.Fatalf("available member should inherit default 24-hour workday rule: %+v", available)
	}
	if available.PendingLeave == nil || available.PendingLeave.RequestKey != "pending-leave" {
		t.Fatalf("pending leave was not exposed: %+v", available.PendingLeave)
	}
	onLeave := byUserID[402]
	if onLeave.AvailableNow || onLeave.UnavailableReason != "approved_leave" || onLeave.ActiveLeave == nil {
		t.Fatalf("approved leave should block member availability: %+v", onLeave)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", team.ID).Updates(map[string]any{
		"product_id": 0,
		"team_type":  services.AgentTeamTypeTechnicalRepair,
	}).Error; err != nil {
		t.Fatalf("convert fixture to technical maintenance team: %v", err)
	}
	result, err = services.AgentTeamScheduleService.ListTeamMemberAvailability(team.ID, &dto.AuthPrincipal{TenantID: 1, UserID: 1, Username: "tester"}, now)
	if err != nil || len(result) != 2 {
		t.Fatalf("technical maintenance team availability = %+v, error = %v", result, err)
	}
}

func setupAgentTeamScheduleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.User{}, &models.Tenant{}, &models.Customer{}, &models.Product{}, &models.Device{}, &models.AgentProfile{}, &models.AgentTeam{}, &models.AgentTeamMember{}, &models.AgentTeamSchedule{}, &models.AgentTeamScheduleTemplate{}, &models.AgentScheduleException{}, &models.AgentWorkStatus{}, &models.Conversation{}, &models.Ticket{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createAgentTeamScheduleTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	createAgentTeamScheduleTestTeams(t, db)

	parse := func(value string) time.Time {
		t.Helper()
		ret, err := time.ParseInLocation(time.DateTime, value, time.Local)
		if err != nil {
			t.Fatalf("parse time %q error = %v", value, err)
		}
		return ret
	}
	schedules := []models.AgentTeamSchedule{
		{ID: 1, TeamID: 1, StartAt: parse("2026-04-26 20:00:00"), EndAt: parse("2026-04-27 10:00:00"), Status: enums.StatusOk},
		{ID: 2, TeamID: 1, StartAt: parse("2026-04-28 09:00:00"), EndAt: parse("2026-04-28 18:00:00"), Status: enums.StatusOk},
		{ID: 3, TeamID: 2, StartAt: parse("2026-05-03 20:00:00"), EndAt: parse("2026-05-04 08:00:00"), Status: enums.StatusOk},
		{ID: 4, TeamID: 1, StartAt: parse("2026-04-20 09:00:00"), EndAt: parse("2026-04-20 18:00:00"), Status: enums.StatusOk},
		{ID: 5, TeamID: 2, StartAt: parse("2026-05-04 09:00:00"), EndAt: parse("2026-05-04 18:00:00"), Status: enums.StatusOk},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("create schedules error = %v", err)
	}
}

func createAgentTeamScheduleTestTeams(t *testing.T, db *gorm.DB) {
	t.Helper()
	teams := []models.AgentTeam{
		{ID: 1, Name: "售前组", Status: enums.StatusOk},
		{ID: 2, Name: "售后组", Status: enums.StatusOk},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams error = %v", err)
	}
}

func formatTestDateTime(date time.Time, clock string) string {
	return date.Format(time.DateOnly) + " " + clock
}

func nextTestWeekday(target time.Weekday) time.Time {
	ret := startOfTestDay(time.Now()).AddDate(0, 0, 1)
	for ret.Weekday() != target {
		ret = ret.AddDate(0, 0, 1)
	}
	return ret
}

func startOfTestDay(value time.Time) time.Time {
	year, month, day := value.In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func weekdayForRequest(value time.Time) int {
	if value.Weekday() == time.Sunday {
		return 7
	}
	return int(value.Weekday())
}

func createFutureAgentTeamSchedule(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	tomorrow := time.Now().AddDate(0, 0, 1)
	item := models.AgentTeamSchedule{
		TeamID:  1,
		StartAt: parseTestDateTime(t, formatTestDateTime(tomorrow, "09:00:00")),
		EndAt:   parseTestDateTime(t, formatTestDateTime(tomorrow, "18:00:00")),
		Status:  enums.StatusOk,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create future schedule error = %v", err)
	}
	return item.ID
}

func parseTestDateTime(t *testing.T, value string) time.Time {
	t.Helper()
	ret, err := time.ParseInLocation(time.DateTime, value, time.Local)
	if err != nil {
		t.Fatalf("parse time %q error = %v", value, err)
	}
	return ret
}

func testOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: 1, Username: "tester", Status: enums.StatusOk}
}
