package services

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupAgentTeamMemberTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Tenant{},
		&models.TenantMember{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentTeamSchedule{},
		&models.AgentTeamScheduleTemplate{},
		&models.AgentProfile{},
		&models.AgentWorkStatus{},
		&models.Conversation{},
		&models.Ticket{},
		&models.AuditLog{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func ensureAgentTeamMemberEnterpriseWorkTime(t *testing.T, db *gorm.DB, tenantID int64, at time.Time) {
	t.Helper()
	local := at.In(engineerScheduleLocation())
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := minute - 1
	if startMinute < 0 {
		startMinute = 0
	}
	endMinute := minute + 30
	if endMinute > 24*60 {
		endMinute = 24 * 60
	}
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    fmt.Sprintf("[%d]", weekday),
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
}

func TestEnsureProductRepairMemberDoesNotCreateDefaultScheduleDraft(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	if err := db.Create(&models.Tenant{ID: 1, Name: "测试企业", Timezone: "Asia/Shanghai", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &models.User{Username: "weekly.draft.member", Nickname: "排班成员", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &models.AgentTeam{
		ID: 30, TenantID: 1, ProductID: 300, TeamType: AgentTeamTypeProductRepair,
		Name: "产品 C 维修组", Status: enums.StatusOk,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	member := &models.TenantMember{TenantID: 1, UserID: user.ID, DisplayName: "排班成员", Status: enums.StatusOk}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		created, err := AgentTeamMemberService.EnsureMember(1, team.ID, user.ID, member.ID, 0, true, operator)
		if err != nil {
			t.Fatalf("ensure member attempt %d: %v", attempt, err)
		}
		if created == nil || created.DispatchWeight != 1 || !created.DispatchEnabled {
			t.Fatalf("unexpected membership on attempt %d: %+v", attempt, created)
		}
	}

	var schedules []models.AgentTeamSchedule
	if err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ? AND publish_status = ?", 1, team.ID, user.ID, AgentTeamSchedulePublishDraft).
		Order("weekday ASC").Find(&schedules).Error; err != nil {
		t.Fatalf("load draft schedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Fatalf("membership should not generate product schedule rows, got %d: %+v", len(schedules), schedules)
	}
	reloaded := repositories.AgentTeamRepository.Get(db, team.ID)
	if reloaded == nil || reloaded.ScheduleEnforced || reloaded.ScheduleVersion != 0 {
		t.Fatalf("membership must not change production schedule: %+v", reloaded)
	}
}

func TestEnsureNonProductTeamMemberDoesNotCreateScheduleDraft(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{Username: "administrative.member", Nickname: "行政成员", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	team := &models.AgentTeam{ID: 31, TenantID: 1, TeamType: "custom", Name: "行政组", Status: enums.StatusOk}
	if err := db.Create(team).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AgentTeamMemberService.EnsureMember(1, team.ID, user.ID, 0, 1, true, operator); err != nil {
		t.Fatalf("ensure custom team member: %v", err)
	}
	var count int64
	if err := db.Model(&models.AgentTeamSchedule{}).Where("team_id = ?", team.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("non-product team generated %d schedule rows", count)
	}
}

func setupAgentTeamMemberLegacyFallbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.AgentTeam{},
		&models.AgentProfile{},
	); err != nil {
		t.Fatalf("auto migrate legacy fallback DB: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestAgentProfileTeamIDDoesNotGrantRuntimeMembership(t *testing.T) {
	db := setupAgentTeamMemberLegacyFallbackTestDB(t)
	if err := db.Create(&models.AgentTeam{
		ID:       10,
		TenantID: 1,
		Name:     "旧档案团队",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&models.User{ID: 101, Username: "legacy-profile-only", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           1,
		UserID:             101,
		TeamID:             10,
		AgentCode:          "ENG-LEGACY",
		DisplayName:        "旧档案工程师",
		ServiceStatus:      enums.ServiceStatusIdle,
		MaxConcurrentCount: 3,
		AutoAssignEnabled:  true,
		Status:             enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create legacy profile: %v", err)
	}

	if got := AgentTeamMemberService.FindTeamIDsByUserID(db, 1, 101); len(got) != 0 {
		t.Fatalf("profile TeamID leaked into user team IDs: %v", got)
	}
	if got := AgentTeamMemberService.FindTeamIDsByUserIDs(db, 1, []int64{101}); len(got[101]) != 0 {
		t.Fatalf("profile TeamID leaked into user team ID map: %v", got[101])
	}
	if AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, 1, 10, 101) {
		t.Fatal("profile TeamID must not grant active team membership")
	}
	if AgentTeamMemberService.IsUserDispatchEnabledMemberOfTeamDB(db, 1, 10, 101) {
		t.Fatal("profile TeamID must not grant dispatch-enabled team membership")
	}
	if got := AgentTeamMemberService.FindProfilesByTeamID(db, 1, 10); len(got) != 0 {
		t.Fatalf("profile TeamID leaked into team profile list: %+v", got)
	}
}

func TestFindTeamIDsByUserIDsUsesExplicitMembershipWhenTableExists(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	teams := []models.AgentTeam{
		{ID: 10, TenantID: 1, ProductID: 100, Name: "产品 A", Status: enums.StatusOk},
		{ID: 11, TenantID: 1, ProductID: 101, Name: "产品 B", Status: enums.StatusOk},
		{ID: 12, TenantID: 1, ProductID: 102, Name: "已删除产品", Status: enums.StatusDeleted},
		{ID: 13, TenantID: 1, ProductID: 103, Name: "已停用产品", Status: enums.StatusDisabled},
		{ID: 20, TenantID: 2, ProductID: 200, Name: "其他租户产品", Status: enums.StatusOk},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	profiles := []models.AgentProfile{
		{TenantID: 1, UserID: 101, TeamID: 10, AgentCode: "ENG-101", DisplayName: "工程师 A", Status: enums.StatusOk},
		{TenantID: 1, UserID: 102, TeamID: 11, AgentCode: "ENG-102", DisplayName: "工程师 B", Status: enums.StatusOk},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatalf("create profiles: %v", err)
	}
	memberships := []models.AgentTeamMember{
		{TenantID: 1, TeamID: 11, UserID: 101, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 12, UserID: 101, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 13, UserID: 101, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 20, UserID: 101, Status: enums.StatusOk},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	got := AgentTeamMemberService.FindTeamIDsByUserIDs(db, 1, []int64{101, 102})
	if !slices.Equal(got[101], []int64{11}) {
		t.Fatalf("user 101 team IDs = %v, want explicit active membership [11]", got[101])
	}
	if len(got[102]) != 0 {
		t.Fatalf("user 102 profile-only team IDs = %v, want no explicit memberships", got[102])
	}
}

func TestFindDispatchProfilesByTeamIDsRequiresExplicitMembership(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	now := time.Now()
	if err := db.Create(&models.AgentTeam{
		ID:        10,
		TenantID:  1,
		ProductID: 100,
		Name:      "产品 A",
		Status:    enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&models.User{ID: 101, Username: "profile-only", Nickname: "仅档案", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           1,
		UserID:             101,
		TeamID:             10,
		AgentCode:          "ENG-101",
		DisplayName:        "仅档案",
		ServiceStatus:      enums.ServiceStatusIdle,
		AutoAssignEnabled:  true,
		MaxConcurrentCount: 2,
		LastOnlineAt:       &now,
		Status:             enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}

	candidates := AgentTeamMemberService.FindDispatchProfilesByTeamIDs(db, 1, []int64{10})
	if len(candidates) != 0 {
		t.Fatalf("profile-only team fallback leaked into dispatch candidates: %+v", candidates)
	}
}

func TestEnsureAgentTeamMemberCreatesDefaultDispatchProfileAndGrantsTicketScope(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{Username: "repair.member", Nickname: "维修成员", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &models.AgentTeam{
		ID:        10,
		TenantID:  1,
		ProductID: 100,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "产品 A 维修组",
		Status:    enums.StatusOk,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	member := &models.TenantMember{
		TenantID:    1,
		UserID:      user.ID,
		DisplayName: "张工",
		MemberType:  "employee",
		Status:      enums.StatusOk,
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}

	if _, err := AgentTeamMemberService.EnsureMemberDB(db, 1, team.ID, user.ID, member.ID, 1, true, operator); err != nil {
		t.Fatalf("ensure member: %v", err)
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("tenant_id", int64(1)).Eq("user_id", user.ID))
	if profile == nil || profile.TeamID != team.ID || profile.DisplayName != "张工" || !profile.AutoAssignEnabled {
		t.Fatalf("default dispatch profile not created correctly: %+v", profile)
	}
	teamIDs := AgentTeamMemberService.FindTeamIDsByUserID(db, 1, user.ID)
	if len(teamIDs) != 1 || teamIDs[0] != team.ID {
		t.Fatalf("teamIDs = %+v, want [%d]", teamIDs, team.ID)
	}

	now := time.Now()
	if err := db.Create(&models.Ticket{
		TenantID:      1,
		ProductID:     100,
		CurrentTeamID: team.ID,
		TicketNo:      "TK-PRODUCT-GROUP",
		Title:         "产品组未分配工单",
		Status:        enums.TicketStatusPending,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	rows, err := EnterpriseTicketService.ListForOperator(1, EnterpriseTicketQuery{Page: 1, PageSize: 10}, &dto.AuthPrincipal{
		TenantID: 1, UserID: user.ID, Username: user.Username, Roles: []string{EnterpriseRoleEngineer},
	})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if rows.Total != 1 || len(rows.Items) != 1 || rows.Items[0].TicketNo != "TK-PRODUCT-GROUP" {
		t.Fatalf("visible tickets = total %d items %+v, want product group ticket", rows.Total, rows.Items)
	}
}

func TestEnsureAgentTeamMemberRejectsDisabledTeam(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{Username: "disabled.team.member", Nickname: "停用团队成员", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &models.AgentTeam{
		ID:        11,
		TenantID:  1,
		ProductID: 100,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "已停用产品维修组",
		Status:    enums.StatusDisabled,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}

	if _, err := AgentTeamMemberService.EnsureMemberDB(db, 1, team.ID, user.ID, 0, 1, true, operator); err == nil {
		t.Fatal("expected disabled team member upsert to fail")
	}
	var members int64
	if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ?", team.ID).Count(&members).Error; err != nil {
		t.Fatalf("count members: %v", err)
	}
	if members != 0 {
		t.Fatalf("disabled team should not get dispatch members, got %d", members)
	}
}

func TestRemoveAgentTeamMemberPreservesAccountHistoryAndOtherMemberships(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	now := time.Now()
	user := &models.User{ID: 101, Username: "multi.team.engineer", Nickname: "多组工程师", Status: enums.StatusOk}
	team := &models.AgentTeam{ID: 11, TenantID: 1, ProductID: 100, TeamType: AgentTeamTypeProductRepair, Name: "产品 A 维修组", Status: enums.StatusOk}
	otherTeam := &models.AgentTeam{ID: 12, TenantID: 1, ProductID: 200, TeamType: AgentTeamTypeProductRepair, Name: "产品 B 维修组", Status: enums.StatusOk}
	profile := &models.AgentProfile{
		ID: 21, TenantID: 1, UserID: user.ID, TeamID: team.ID, AgentCode: "ENG-MULTI", DisplayName: "多组工程师",
		ServiceStatus: enums.ServiceStatusIdle, MaxConcurrentCount: 5, AutoAssignEnabled: true, Status: enums.StatusOk,
	}
	memberships := []models.AgentTeamMember{
		{ID: 31, TenantID: 1, TeamID: team.ID, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk},
		{ID: 32, TenantID: 1, TeamID: otherTeam.ID, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 2, Status: enums.StatusOk},
	}
	schedules := []models.AgentTeamSchedule{
		{TenantID: 1, TeamID: team.ID, UserID: user.ID, RepeatType: AgentTeamScheduleRepeatWeekly, DayType: AgentTeamScheduleDayTypeWork, Weekday: 1, PublishStatus: AgentTeamSchedulePublishDraft, Version: 2, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: user.ID, RepeatType: AgentTeamScheduleRepeatWeekly, DayType: AgentTeamScheduleDayTypeWork, Weekday: 1, PublishStatus: AgentTeamSchedulePublishPublished, Version: 1, StartAt: now, EndAt: now.Add(time.Hour), Status: enums.StatusOk},
	}
	for _, value := range []any{user, team, otherTeam, profile} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("create removal fixture: %v", err)
		}
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("create schedules: %v", err)
	}

	result, err := AgentTeamMemberService.RemoveMember(1, team.ID, user.ID, operator)
	if err != nil {
		t.Fatalf("RemoveMember() error = %v", err)
	}
	if result.AlreadyRemoved {
		t.Fatalf("removal result = %+v, want active removal", result)
	}
	var removedMembership models.AgentTeamMember
	if err := db.First(&removedMembership, memberships[0].ID).Error; err != nil {
		t.Fatalf("reload removed membership: %v", err)
	}
	if removedMembership.Status != enums.StatusDeleted || removedMembership.DispatchEnabled {
		t.Fatalf("removed membership = %+v, want soft-deleted and dispatch disabled", removedMembership)
	}
	if !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, 1, otherTeam.ID, user.ID) {
		t.Fatal("other product team membership should remain active")
	}
	refreshedProfile := repositories.AgentProfileRepository.Get(db, profile.ID)
	if refreshedProfile == nil || refreshedProfile.Status != enums.StatusOk || refreshedProfile.TeamID != otherTeam.ID {
		t.Fatalf("profile = %+v, want active profile repointed to remaining team", refreshedProfile)
	}
	var draft, published models.AgentTeamSchedule
	if err := db.First(&draft, schedules[0].ID).Error; err != nil {
		t.Fatalf("reload draft schedule: %v", err)
	}
	if err := db.First(&published, schedules[1].ID).Error; err != nil {
		t.Fatalf("reload published schedule: %v", err)
	}
	if draft.Status != enums.StatusOk || published.Status != enums.StatusOk {
		t.Fatalf("legacy schedule statuses = draft %d published %d, want unchanged", draft.Status, published.Status)
	}
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("tenant_id = ? AND resource_type = ? AND resource_id = ? AND action = ?", int64(1), "agent_team_member", fmt.Sprint(memberships[0].ID), "agent_team_member.removed").
		Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("removal audit count = %d, error = %v, want 1", auditCount, err)
	}
	var refreshedUser models.User
	if err := db.First(&refreshedUser, user.ID).Error; err != nil || refreshedUser.Status != enums.StatusOk {
		t.Fatalf("enterprise account should remain active: user=%+v error=%v", refreshedUser, err)
	}

	repeated, err := AgentTeamMemberService.RemoveMember(1, team.ID, user.ID, operator)
	if err != nil || !repeated.AlreadyRemoved {
		t.Fatalf("repeated RemoveMember() = %+v, %v, want idempotent success", repeated, err)
	}
	if _, err := AgentTeamMemberService.RemoveMember(1, otherTeam.ID, user.ID, operator); err != nil {
		t.Fatalf("remove last membership: %v", err)
	}
	if refreshedProfile := repositories.AgentProfileRepository.Get(db, profile.ID); refreshedProfile == nil || refreshedProfile.TeamID != 0 {
		t.Fatalf("profile after last membership removal = %+v, want team 0", refreshedProfile)
	}
	if _, err := AgentTeamMemberService.EnsureMemberDB(db, 1, team.ID, user.ID, 0, 1, true, operator); err != nil {
		t.Fatalf("rejoin member: %v", err)
	}
	if refreshedProfile := repositories.AgentProfileRepository.Get(db, profile.ID); refreshedProfile == nil || refreshedProfile.TeamID != team.ID {
		t.Fatalf("profile after rejoin = %+v, want restored primary team %d", refreshedProfile, team.ID)
	}
}

func TestRemoveAgentTeamMemberRejectsCurrentLeader(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{ID: 201, Username: "team.leader", Status: enums.StatusOk}
	team := &models.AgentTeam{ID: 21, TenantID: 1, ProductID: 100, TeamType: AgentTeamTypeProductRepair, Name: "主管组", LeaderUserID: user.ID, Status: enums.StatusOk}
	membership := &models.AgentTeamMember{TenantID: 1, TeamID: team.ID, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk}
	for _, value := range []any{user, team, membership} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("create leader fixture: %v", err)
		}
	}

	if _, err := AgentTeamMemberService.RemoveMember(1, team.ID, user.ID, operator); err == nil {
		t.Fatal("expected current leader removal to fail")
	}
	if refreshed := repositories.AgentTeamMemberRepository.Get(db, membership.ID); refreshed == nil || refreshed.Status != enums.StatusOk {
		t.Fatalf("leader membership changed after rejected removal: %+v", refreshed)
	}
}

func TestRemoveAgentTeamMemberRejectsActiveAcceptedWork(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	now := time.Now()
	user := &models.User{ID: 301, Username: "active.engineer", Status: enums.StatusOk}
	team := &models.AgentTeam{ID: 31, TenantID: 1, ProductID: 100, TeamType: AgentTeamTypeProductRepair, Name: "在途工单组", Status: enums.StatusOk}
	membership := &models.AgentTeamMember{TenantID: 1, TeamID: team.ID, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk}
	ticket := &models.Ticket{
		TicketNo: "TK-ACTIVE-MEMBER", TenantID: 1, ProductID: team.ProductID, CurrentTeamID: team.ID,
		CurrentAssigneeID: user.ID, Status: enums.TicketStatusProcessing, AcceptedAt: &now,
	}
	for _, value := range []any{user, team, membership, ticket} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("create active work fixture: %v", err)
		}
	}

	if _, err := AgentTeamMemberService.RemoveMember(1, team.ID, user.ID, operator); err == nil {
		t.Fatal("expected active accepted work to block member removal")
	}
	if refreshed := repositories.AgentTeamMemberRepository.Get(db, membership.ID); refreshed == nil || refreshed.Status != enums.StatusOk {
		t.Fatalf("membership changed after active-work rejection: %+v", refreshed)
	}
}

func TestCreateAgentProfileRejectsDisabledTeam(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{Username: "disabled.profile.member", Nickname: "停用档案成员", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{TenantID: 1, UserID: user.ID, Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	team := &models.AgentTeam{
		ID:        12,
		TenantID:  1,
		ProductID: 100,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "停用档案维修组",
		Status:    enums.StatusDisabled,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}

	_, err := AgentProfileService.CreateAgentProfile(request.CreateAgentProfileRequest{
		UserID:                user.ID,
		TeamID:                team.ID,
		AgentCode:             "ENG-DISABLED-TEAM",
		DisplayName:           "停用团队工程师",
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    2,
		AutoAssignEnabled:     true,
		ReceiveOfflineMessage: true,
	}, operator)
	if err == nil {
		t.Fatal("expected disabled team agent profile creation to fail")
	}
	if profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", user.ID)); profile != nil {
		t.Fatalf("disabled team should not receive an agent profile: %+v", profile)
	}
}

func TestUpdateAgentProfilePreservesExistingTeamDispatchSettings(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "tenant-admin"}
	user := &models.User{Username: "repair.profile.edit", Nickname: "档案编辑工程师", Status: enums.StatusOk}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &models.AgentTeam{
		ID:        20,
		TenantID:  1,
		ProductID: 200,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "产品 B 维修组",
		Status:    enums.StatusOk,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID: 1,
		UserID:   user.ID,
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	now := time.Now()
	profile := &models.AgentProfile{
		TenantID:              1,
		UserID:                user.ID,
		TeamID:                team.ID,
		AgentCode:             "ENG-PRESERVE",
		DisplayName:           "档案编辑工程师",
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    2,
		AutoAssignEnabled:     false,
		ReceiveOfflineMessage: true,
		Status:                enums.StatusOk,
		AuditFields:           models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        1,
		TeamID:          team.ID,
		UserID:          user.ID,
		DispatchEnabled: false,
		DispatchWeight:  7,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create disabled membership: %v", err)
	}

	err := AgentProfileService.UpdateAgentProfile(request.UpdateAgentProfileRequest{
		ID: profile.ID,
		CreateAgentProfileRequest: request.CreateAgentProfileRequest{
			UserID:                user.ID,
			TeamID:                team.ID,
			AgentCode:             "ENG-PRESERVE",
			DisplayName:           "档案编辑工程师更新",
			ServiceStatus:         enums.ServiceStatusIdle,
			MaxConcurrentCount:    5,
			PriorityLevel:         20,
			AutoAssignEnabled:     true,
			ReceiveOfflineMessage: true,
			Remark:                "仅更新档案，不应覆盖团队派单设置",
		},
	}, operator)
	if err != nil {
		t.Fatalf("UpdateAgentProfile() error = %v", err)
	}

	var member models.AgentTeamMember
	if err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ?", int64(1), team.ID, user.ID).First(&member).Error; err != nil {
		t.Fatalf("reload membership: %v", err)
	}
	if member.DispatchEnabled || member.DispatchWeight != 7 {
		t.Fatalf("profile update clobbered team dispatch settings: %+v", member)
	}
	updated := repositories.AgentProfileRepository.Get(db, profile.ID)
	if updated == nil || !updated.AutoAssignEnabled || updated.MaxConcurrentCount != 5 {
		t.Fatalf("profile itself was not updated correctly: %+v", updated)
	}
}

func TestFindDispatchCandidatesFiltersByProductGradeEligibility(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	now := time.Now()
	ensureAgentTeamMemberEnterpriseWorkTime(t, db, 1, now)
	staleOnlineAt := now.Add(-24 * time.Hour)
	team := &models.AgentTeam{
		ID:        30,
		TenantID:  1,
		ProductID: 300,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "产品 C 维修组",
		Status:    enums.StatusOk,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	users := []models.User{
		{ID: 101, Username: "eligible", Nickname: "可派", Status: enums.StatusOk},
		{ID: 102, Username: "team-disabled", Nickname: "本组停派", Status: enums.StatusOk},
		{ID: 103, Username: "busy", Nickname: "忙碌", Status: enums.StatusOk},
		{ID: 104, Username: "capacity", Nickname: "容量满", Status: enums.StatusOk},
		{ID: 105, Username: "stale", Nickname: "不在线", Status: enums.StatusOk},
		{ID: 106, Username: "no-capacity", Nickname: "容量未配置", Status: enums.StatusOk},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	profiles := []models.AgentProfile{
		{TenantID: 1, UserID: 101, TeamID: team.ID, AgentCode: "ENG-101", DisplayName: "可派", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &now, Status: enums.StatusOk},
		{TenantID: 1, UserID: 102, TeamID: team.ID, AgentCode: "ENG-102", DisplayName: "本组停派", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &now, Status: enums.StatusOk},
		{TenantID: 1, UserID: 103, TeamID: team.ID, AgentCode: "ENG-103", DisplayName: "忙碌", ServiceStatus: enums.ServiceStatusBusy, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &now, Status: enums.StatusOk},
		{TenantID: 1, UserID: 104, TeamID: team.ID, AgentCode: "ENG-104", DisplayName: "容量满", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 1, LastOnlineAt: &now, Status: enums.StatusOk},
		{TenantID: 1, UserID: 105, TeamID: team.ID, AgentCode: "ENG-105", DisplayName: "不在线", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &staleOnlineAt, Status: enums.StatusOk},
		{TenantID: 1, UserID: 106, TeamID: team.ID, AgentCode: "ENG-106", DisplayName: "容量未配置", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 0, LastOnlineAt: &now, Status: enums.StatusOk},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatalf("create profiles: %v", err)
	}
	memberships := []models.AgentTeamMember{
		{TenantID: 1, TeamID: team.ID, UserID: 101, DispatchEnabled: true, DispatchWeight: 5, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 102, DispatchEnabled: false, DispatchWeight: 9, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 103, DispatchEnabled: true, DispatchWeight: 3, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 104, DispatchEnabled: true, DispatchWeight: 4, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 105, DispatchEnabled: true, DispatchWeight: 2, Status: enums.StatusOk},
		{TenantID: 1, TeamID: team.ID, UserID: 106, DispatchEnabled: true, DispatchWeight: 10, Status: enums.StatusOk},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}
	statuses := []models.AgentWorkStatus{
		{TenantID: 1, UserID: 101, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
		{TenantID: 1, UserID: 102, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
		{TenantID: 1, UserID: 103, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
		{TenantID: 1, UserID: 104, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
		{TenantID: 1, UserID: 105, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
		{TenantID: 1, UserID: 106, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now},
	}
	if err := db.Create(&statuses).Error; err != nil {
		t.Fatalf("create work statuses: %v", err)
	}
	if err := db.Create(&models.Conversation{
		TenantID:          1,
		ProductID:         team.ProductID,
		Status:            enums.IMConversationStatusActive,
		CurrentTeamID:     team.ID,
		CurrentAssigneeID: 101,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create active conversation: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:          1,
		ProductID:         team.ProductID,
		CurrentTeamID:     team.ID,
		CurrentAssigneeID: 104,
		TicketNo:          "TK-CAPACITY",
		Title:             "容量占用",
		Status:            enums.TicketStatusAssigned,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create capacity ticket: %v", err)
	}

	candidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{
		TenantID: 1,
		TeamID:   team.ID,
	}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1: %+v", len(candidates), candidates)
	}
	candidate := candidates[0]
	if candidate.Profile.UserID != 101 || candidate.TeamID != team.ID {
		t.Fatalf("candidate = %+v, want user 101 on team %d", candidate, team.ID)
	}
	if candidate.DispatchWeight != 5 || candidate.ActiveConversationCount != 1 || candidate.Workload != 1 {
		t.Fatalf("candidate metrics = %+v, want weight 5 active conversations 1 workload 1", candidate)
	}
	if !candidate.TeamDispatchEnabled || !candidate.Profile.AutoAssignEnabled {
		t.Fatalf("automatic candidate lost global or team dispatch configuration: %+v", candidate)
	}
}

func TestFindDispatchProfilesByTeamIDsRejectsDisabledTeam(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	now := time.Now()
	team := &models.AgentTeam{
		ID:        31,
		TenantID:  1,
		ProductID: 300,
		TeamType:  AgentTeamTypeProductRepair,
		Name:      "已停用维修组",
		Status:    enums.StatusDisabled,
	}
	if err := db.Create(team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&models.User{ID: 201, Username: "disabled-team-engineer", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: 201, TeamID: team.ID, AgentCode: "ENG-201", DisplayName: "旧组工程师",
		ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &now, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: team.ID, UserID: 201, DispatchEnabled: true, DispatchWeight: 10, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}

	profiles := AgentTeamMemberService.FindDispatchProfilesByTeamIDs(db, 1, []int64{team.ID})
	if len(profiles) != 0 {
		t.Fatalf("disabled team must not expose dispatch profiles: %+v", profiles)
	}
}

func TestFindDispatchCandidatesFallsBackFromDisabledConversationTeamToProductRepairTeam(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	now := time.Now()
	ensureAgentTeamMemberEnterpriseWorkTime(t, db, 1, now)
	disabledTeam := models.AgentTeam{
		ID: 31, TenantID: 1, ProductID: 300, TeamType: AgentTeamTypeProductRepair,
		Name: "旧产品维修组", Status: enums.StatusDisabled,
	}
	activeTeam := models.AgentTeam{
		ID: 32, TenantID: 1, ProductID: 300, TeamType: AgentTeamTypeProductRepair,
		Name: "当前产品维修组", Status: enums.StatusOk,
	}
	if err := db.Create(&[]models.AgentTeam{disabledTeam, activeTeam}).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	if err := db.Create(&models.User{ID: 201, Username: "fallback", Nickname: "兜底候选", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: 201, TeamID: activeTeam.ID, AgentCode: "ENG-201", DisplayName: "兜底候选",
		ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 2, LastOnlineAt: &now, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: activeTeam.ID, UserID: 201, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: 201, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 1, ProductID: 300, Status: enums.IMConversationStatusActive, CurrentTeamID: disabledTeam.ID,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	candidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{
		TenantID:       1,
		ConversationID: conversation.ID,
	}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].TeamID != activeTeam.ID || candidates[0].Profile.UserID != 201 {
		t.Fatalf("conversation dispatch candidates should use active product repair team, got %+v", candidates)
	}
}

func TestFindManualTransferCandidatesUsesProductTeamAndKeepsFullCapacityPeer(t *testing.T) {
	db := setupAgentTeamMemberTestDB(t)
	now := time.Now()
	legacyTeam := models.AgentTeam{
		ID: 41, TenantID: 1, Name: "旧通用维修组", TeamType: AgentTeamTypeTechnicalRepair, Status: enums.StatusOk,
	}
	productTeam := models.AgentTeam{
		ID: 42, TenantID: 1, ProductID: 300, Name: "产品 C 维修组", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk,
	}
	if err := db.Create(&[]models.AgentTeam{legacyTeam, productTeam}).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	users := []models.User{
		{ID: 301, Username: "current-handler", Nickname: "当前处理人", Status: enums.StatusOk},
		{ID: 302, Username: "product-peer", Nickname: "同产品组成员", Status: enums.StatusOk},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	profiles := []models.AgentProfile{
		{TenantID: 1, UserID: 301, TeamID: legacyTeam.ID, AgentCode: "ENG-301", DisplayName: "当前处理人", MaxConcurrentCount: 2, Status: enums.StatusOk},
		{TenantID: 1, UserID: 302, TeamID: productTeam.ID, AgentCode: "ENG-302", DisplayName: "同产品组成员", MaxConcurrentCount: 1, Status: enums.StatusOk},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatalf("create profiles: %v", err)
	}
	memberships := []models.AgentTeamMember{
		{TenantID: 1, TeamID: productTeam.ID, UserID: 301, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk},
		{TenantID: 1, TeamID: productTeam.ID, UserID: 302, DispatchEnabled: false, DispatchWeight: 1, Status: enums.StatusOk},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}
	if err := db.AutoMigrate(&models.EngineerProfile{}); err != nil {
		t.Fatalf("migrate engineer profiles: %v", err)
	}
	targetMember := models.TenantMember{
		TenantID: 1, UserID: 302, MemberNo: "M302", DisplayName: "同产品组成员",
		MemberType: "employee", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&targetMember).Error; err != nil {
		t.Fatalf("create target tenant member: %v", err)
	}
	if err := db.Create(&models.EngineerProfile{
		TenantID: 1, MemberID: targetMember.ID, ServiceRegionsJSON: `["emea"]`,
		LanguagesJSON: "[]", Timezone: "UTC", MaxTicketLoad: 1,
		DispatchEnabled: false, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create target engineer profile: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID: 1, ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID, CurrentAssigneeID: 302,
		TicketNo: "TK-MANUAL-TRANSFER-CAPACITY", Title: "占满人工转接目标容量", Status: enums.TicketStatusProcessing,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create capacity ticket: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 1, ProductID: productTeam.ProductID, Status: enums.IMConversationStatusActive,
		CurrentTeamID: legacyTeam.ID, CurrentAssigneeID: 301,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID: 1, ProductID: productTeam.ProductID, ConversationID: conversation.ID,
		TicketNo: "TK-MANUAL-TRANSFER-CAPABILITY", Title: "人工转接不按区域过滤",
		ServiceRegion: "apac", Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create linked capability ticket: %v", err)
	}
	if err := db.Model(&models.AgentProfile{}).
		Where("tenant_id = ? AND user_id = ?", 1, 302).
		Update("auto_assign_enabled", false).Error; err != nil {
		t.Fatalf("disable target profile auto assignment: %v", err)
	}

	candidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{
		TenantID:       1,
		ConversationID: conversation.ID,
		ManualTransfer: true,
	}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Profile.UserID != 302 || candidates[0].TeamID != productTeam.ID {
		t.Fatalf("manual transfer candidates should contain every other active product-team member, got %+v", candidates)
	}
	if candidates[0].CapacityAvailable || candidates[0].Workload != 1 {
		t.Fatalf("full-capacity peer should remain selectable with accurate workload metadata, got %+v", candidates[0])
	}
	if candidates[0].TeamDispatchEnabled || candidates[0].Profile.AutoAssignEnabled {
		t.Fatalf("manual candidate must expose disabled automatic-dispatch configuration: %+v", candidates[0])
	}
}
