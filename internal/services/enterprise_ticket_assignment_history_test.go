package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestTicketAssignmentHistoryKeepsEveryAssignedEngineer(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentTeam{}, &models.TicketDispatchAttempt{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	first := models.User{ID: 51, Username: "first.engineer", Nickname: "首位工程师", Status: enums.StatusOk}
	second := models.User{ID: 52, Username: "second.engineer", Nickname: "接手工程师", Status: enums.StatusOk}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first engineer: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second engineer: %v", err)
	}
	team := models.AgentTeam{ID: 61, TenantID: 9, Name: "技术支持组", Status: enums.StatusOk}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	attempts := []models.TicketDispatchAttempt{
		{TenantID: 9, TicketID: 88, TeamID: team.ID, AssigneeID: first.ID, AttemptNo: 1, Outcome: "timed_out", AssignedAt: now.Add(-30 * time.Minute), EndedAt: testTimePtr(now.Add(-20 * time.Minute)), AuditFields: models.AuditFields{CreateUserID: 0, CreateUserName: "system"}},
		{TenantID: 9, TicketID: 88, TeamID: team.ID, AssigneeID: second.ID, AttemptNo: 2, Outcome: "accepted", AssignedAt: now.Add(-10 * time.Minute), AcceptedAt: testTimePtr(now.Add(-9 * time.Minute)), AuditFields: models.AuditFields{CreateUserID: 99, CreateUserName: "manager"}},
		{TenantID: 10, TicketID: 88, TeamID: team.ID, AssigneeID: first.ID, AttemptNo: 1, Outcome: "accepted", AssignedAt: now},
	}
	if err := db.Create(&attempts).Error; err != nil {
		t.Fatalf("create dispatch attempts: %v", err)
	}

	history := EnterpriseTicketService.buildAssignmentHistory(&models.Ticket{ID: 88, TenantID: 9})
	if len(history) != 2 {
		t.Fatalf("assignment history count = %d, want 2: %+v", len(history), history)
	}
	if history[0].AssigneeName != first.Nickname || history[0].Outcome != "timed_out" {
		t.Fatalf("first assignment = %+v, want timed-out first engineer", history[0])
	}
	if history[0].Source != "automatic" {
		t.Fatalf("first assignment source = %q, want automatic", history[0].Source)
	}
	if history[1].AssigneeName != second.Nickname || history[1].Outcome != "accepted" {
		t.Fatalf("second assignment = %+v, want accepted second engineer", history[1])
	}
	if history[1].Source != "manual" {
		t.Fatalf("second assignment source = %q, want manual", history[1].Source)
	}
	if history[0].TeamName != team.Name || history[1].TeamName != team.Name {
		t.Fatalf("assignment history team names = %q, %q; want %q", history[0].TeamName, history[1].TeamName, team.Name)
	}
}

func testTimePtr(value time.Time) *time.Time {
	return &value
}
