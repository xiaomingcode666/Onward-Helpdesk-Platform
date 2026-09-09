package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAgentTeamScheduleResponseUsesCurrentEngineerName(t *testing.T) {
	db := setupDashboardAgentTeamScheduleHandlerTestDB(t)
	now := time.Now()
	if err := db.Create(&models.User{
		ID:          501,
		Username:    "current.engineer",
		Nickname:    "当前工程师",
		Avatar:      "https://example.invalid/current.png",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    1,
		UserID:      501,
		DisplayName: "IAM 当前工程师",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:    1,
		UserID:      501,
		TeamID:      5,
		AgentCode:   "ENG-501",
		DisplayName: "旧派单档案名",
		Avatar:      "https://example.invalid/stale.png",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create stale agent profile: %v", err)
	}
	if err := db.Create(&models.AgentTeam{
		ID:          5,
		TenantID:    1,
		Name:        "产品维修组",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}

	ret := buildAgentTeamScheduleResponse(&models.AgentTeamSchedule{
		ID:            9001,
		TenantID:      1,
		TeamID:        5,
		UserID:        501,
		RepeatType:    services.AgentTeamScheduleRepeatWeekly,
		DayType:       services.AgentTeamScheduleDayTypeWork,
		Weekday:       1,
		StartMinute:   9 * 60,
		EndMinute:     18 * 60,
		Timezone:      services.EngineerScheduleTimezone,
		PublishStatus: services.AgentTeamSchedulePublishPublished,
		Version:       1,
		StartAt:       now,
		EndAt:         now.Add(9 * time.Hour),
		Status:        enums.StatusOk,
	})

	if ret.UserName != "IAM 当前工程师" {
		t.Fatalf("schedule user name = %q, want current IAM name", ret.UserName)
	}
	if ret.UserAvatar != "https://example.invalid/current.png" {
		t.Fatalf("schedule user avatar = %q, want current user avatar", ret.UserAvatar)
	}
}

func TestAgentTeamMemberAvailabilityResponseFormatsWorkdayRuleAndLeave(t *testing.T) {
	db := setupDashboardAgentTeamScheduleHandlerTestDB(t)
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       601,
		Username: "availability.engineer",
		Nickname: "可用性工程师",
		Avatar:   "https://example.invalid/availability.png",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    1,
		UserID:      601,
		DisplayName: "人员表可用性工程师",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	leave := &models.AgentScheduleException{
		ID: 7001, TenantID: 1, UserID: 601, RequestKey: "leave-601",
		ExceptionType: services.AgentScheduleExceptionTypeLeave,
		StartAt:       now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: services.AgentScheduleApprovalApproved,
		RequestedAt:    now.Add(-2 * time.Hour),
		Status:         enums.StatusOk,
	}
	ret := buildAgentTeamMemberAvailabilityResponse(services.AgentTeamMemberAvailability{
		Profile: models.AgentProfile{
			ID: 9001, TenantID: 1, UserID: 601, AgentCode: "ENG-601",
			DisplayName: "旧可用性名称", ServiceStatus: enums.ServiceStatusIdle,
			MaxConcurrentCount: 3, AutoAssignEnabled: true, Status: enums.StatusOk,
		},
		Member: models.AgentTeamMember{
			MemberID: 8001, UserID: 601, DispatchEnabled: true, DispatchWeight: 2,
		},
		User:                &models.User{ID: 601, Username: "availability.engineer", Nickname: "可用性工程师", Avatar: "https://example.invalid/availability.png"},
		WorkStatus:          &models.AgentWorkStatus{Status: services.AgentWorkStatusAvailable},
		ActiveLeave:         leave,
		Workdays:            []int{1, 2, 3, 4, 5},
		StartMinute:         0,
		EndMinute:           24 * 60,
		Timezone:            services.EngineerScheduleTimezone,
		AvailableNow:        false,
		UnavailableReason:   "approved_leave",
		WorkStatusConfirmed: true,
	})

	if ret.DisplayName != "人员表可用性工程师" || ret.Nickname != "可用性工程师" {
		t.Fatalf("display identity should expose current personnel name: %+v", ret)
	}
	if ret.StartTime != "00:00" || ret.EndTime != "24:00" {
		t.Fatalf("workday full-day rule not formatted: %+v", ret)
	}
	if ret.ActiveLeave == nil || ret.ActiveLeave.ID != leave.ID || ret.UnavailableReason != "approved_leave" {
		t.Fatalf("active leave was not exposed: %+v", ret)
	}
}

func TestAgentProfileResponsePrefersCurrentTenantMemberName(t *testing.T) {
	db := setupDashboardAgentTeamScheduleHandlerTestDB(t)
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       701,
		Username: "profile.engineer",
		Nickname: "当前人员昵称",
		Avatar:   "https://example.invalid/profile.png",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    1,
		UserID:      701,
		DisplayName: "人员表当前姓名",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:    1,
		UserID:      701,
		TeamID:      5,
		AgentCode:   "ENG-701",
		DisplayName: "旧档案姓名",
		Avatar:      "https://example.invalid/profile-stale.png",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create agent profile: %v", err)
	}
	if err := db.Create(&models.AgentTeam{
		ID:          5,
		TenantID:    1,
		Name:        "产品维修组",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}

	ret := builders.BuildAgentProfileResponse(&models.AgentProfile{
		TenantID:    1,
		UserID:      701,
		TeamID:      5,
		AgentCode:   "ENG-701",
		DisplayName: "旧档案姓名",
		Avatar:      "https://example.invalid/profile-stale.png",
		Status:      enums.StatusOk,
	})
	if ret == nil {
		t.Fatal("build agent profile response returned nil")
	}
	if ret.DisplayName != "人员表当前姓名" {
		t.Fatalf("profile display name = %q, want current tenant member name", ret.DisplayName)
	}
	if ret.Avatar != "https://example.invalid/profile.png" {
		t.Fatalf("profile avatar = %q, want current user avatar", ret.Avatar)
	}
}

func TestAgentTeamScheduleLegacyWriteHandlersAreGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		permission constants.Permission
		handler    gin.HandlerFunc
	}{
		{name: "batch preview", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostBatch_preview},
		{name: "batch generate", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostBatch_generate},
		{name: "create", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostCreate},
		{name: "update", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostUpdate},
		{name: "delete", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostDelete},
		{name: "prepare draft", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostPrepare_draft},
		{name: "publish", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostPublish},
		{name: "rollback", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostRollback},
		{name: "disable", permission: constants.PermissionAgentTeamScheduleUpdate, handler: AgentTeamSchedulePostDisable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/dashboard/agent-team-schedule/deprecated", strings.NewReader("{}"))
			ctx.Set("authPrincipal", &dto.AuthPrincipal{Permissions: []string{tt.permission.Code}})

			tt.handler(ctx)

			if rec.Code != http.StatusGone {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusGone, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), agentTeamScheduleDeprecatedWriteMessage) {
				t.Fatalf("response should explain deprecated schedule writes, body=%s", rec.Body.String())
			}
		})
	}
}

func setupDashboardAgentTeamScheduleHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.TenantMember{},
		&models.AgentProfile{},
		&models.AgentTeam{},
		&models.AgentScheduleException{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}
