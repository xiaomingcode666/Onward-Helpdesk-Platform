package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAgentTeamHolidayMigrationCreatesUniqueCalendarDate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := migrateAgentTeamHoliday(db); err != nil {
		t.Fatalf("migrate agent team holiday: %v", err)
	}
	if !db.Migrator().HasTable(&models.AgentTeamHoliday{}) {
		t.Fatal("agent team holiday table was not created")
	}
	holiday := models.AgentTeamHoliday{
		TenantID: 1, TeamID: 2, HolidayDate: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC", Name: "holiday", Status: enums.StatusOk,
	}
	if err := db.Create(&holiday).Error; err != nil {
		t.Fatalf("create holiday: %v", err)
	}
	duplicate := models.AgentTeamHoliday{
		TenantID: 1, TeamID: 2, HolidayDate: holiday.HolidayDate,
		Timezone: "UTC", Name: "duplicate holiday", Status: enums.StatusOk,
	}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate team holiday date should violate the unique index")
	}
}
