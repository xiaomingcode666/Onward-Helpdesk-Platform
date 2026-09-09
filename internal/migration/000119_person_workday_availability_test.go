package migration

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigratePersonWorkdayAvailabilityDeletesOnlyGeneratedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentTeam{}, &models.AgentTeamSchedule{}, &models.AgentTeamScheduleTemplate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	template := models.AgentTeamScheduleTemplate{
		TenantID: 1, Workdays: "[1,2,3,4,5]", StartMinute: 9 * 60, EndMinute: 18 * 60,
		Timezone: "UTC", Status: enums.StatusOk,
	}
	if err := db.Create(&template).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	teams := []models.AgentTeam{
		{ID: 10, TenantID: 1, ProductID: 100, TeamType: services.AgentTeamTypeProductRepair, Name: "only generated", ScheduleEnforced: true, ScheduleVersion: 1, Status: enums.StatusOk},
		{ID: 11, TenantID: 1, ProductID: 101, TeamType: services.AgentTeamTypeProductRepair, Name: "custom coverage", ScheduleEnforced: true, ScheduleVersion: 1, Status: enums.StatusOk},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	baseStart := time.Date(2000, time.January, 3, 9, 0, 0, 0, time.UTC)
	schedules := []models.AgentTeamSchedule{
		{TenantID: 1, TeamID: 10, UserID: 501, RepeatType: services.AgentTeamScheduleRepeatWeekly, DayType: services.AgentTeamScheduleDayTypeWork, Weekday: 1, StartMinute: 9 * 60, EndMinute: 18 * 60, Timezone: services.EngineerScheduleTimezone, PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: baseStart, EndAt: baseStart.Add(9 * time.Hour), Remark: "企业默认模板：值班", Status: enums.StatusOk},
		{TenantID: 1, TeamID: 10, UserID: 501, RepeatType: services.AgentTeamScheduleRepeatWeekly, DayType: services.AgentTeamScheduleDayTypeRest, Weekday: 6, Timezone: services.EngineerScheduleTimezone, PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: baseStart, EndAt: baseStart, Remark: "企业默认模板：休息", Status: enums.StatusOk},
		{TenantID: 1, TeamID: 11, UserID: 502, RepeatType: services.AgentTeamScheduleRepeatWeekly, DayType: services.AgentTeamScheduleDayTypeWork, Weekday: 1, StartMinute: 9 * 60, EndMinute: 18 * 60, Timezone: services.EngineerScheduleTimezone, PublishStatus: services.AgentTeamSchedulePublishPublished, Version: 1, StartAt: baseStart, EndAt: baseStart.Add(9 * time.Hour), Remark: "人工周一覆盖", Status: enums.StatusOk},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("create schedules: %v", err)
	}

	if err := migratePersonWorkdayAvailability(db); err != nil {
		t.Fatalf("migrate person workday availability: %v", err)
	}

	var reloadedTemplate models.AgentTeamScheduleTemplate
	if err := db.First(&reloadedTemplate, "tenant_id = ?", int64(1)).Error; err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if reloadedTemplate.StartMinute != 0 || reloadedTemplate.EndMinute != 24*60 || reloadedTemplate.Timezone != services.EngineerScheduleTimezone {
		t.Fatalf("template was not converted to workday full-day: %+v", reloadedTemplate)
	}
	var generatedCount int64
	if err := db.Model(&models.AgentTeamSchedule{}).
		Where("team_id = ? AND status = ?", int64(10), enums.StatusOk).
		Count(&generatedCount).Error; err != nil {
		t.Fatalf("count generated team schedules: %v", err)
	}
	if generatedCount != 0 {
		t.Fatalf("generated default schedules still active: %d", generatedCount)
	}
	var custom models.AgentTeamSchedule
	if err := db.First(&custom, "team_id = ? AND remark = ?", int64(11), "人工周一覆盖").Error; err != nil {
		t.Fatalf("reload custom coverage: %v", err)
	}
	if custom.Status != enums.StatusOk {
		t.Fatalf("custom coverage should remain active: %+v", custom)
	}
	var generatedOnlyTeam models.AgentTeam
	if err := db.First(&generatedOnlyTeam, "id = ?", int64(10)).Error; err != nil {
		t.Fatalf("reload generated-only team: %v", err)
	}
	if generatedOnlyTeam.ScheduleEnforced {
		t.Fatalf("generated-only team should fall back to personal work rules: %+v", generatedOnlyTeam)
	}
	var customTeam models.AgentTeam
	if err := db.First(&customTeam, "id = ?", int64(11)).Error; err != nil {
		t.Fatalf("reload custom team: %v", err)
	}
	if !customTeam.ScheduleEnforced {
		t.Fatalf("team with custom coverage should keep schedule enforcement: %+v", customTeam)
	}
}
