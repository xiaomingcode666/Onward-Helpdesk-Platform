package dashboard

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConversationAgentTeamFilterIncludesTeamPoolConversations(t *testing.T) {
	db := setupDashboardConversationHandlerTestDB(t)
	now := time.Now()
	if err := db.Create(&models.AgentTeam{ID: 10, TenantID: 1, Name: "产品维修组", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&models.User{ID: 101, Username: "engineer", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: 101, TeamID: 10, AgentCode: "ENG-101", DisplayName: "工程师",
		ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, MaxConcurrentCount: 3, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 10, UserID: 101, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	conversations := []models.Conversation{
		{ID: 1, TenantID: 1, CustomerName: "团队池", Status: enums.IMConversationStatusPending, CurrentTeamID: 10, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{ID: 2, TenantID: 1, CustomerName: "成员负责", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 101, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{ID: 3, TenantID: 1, CustomerName: "其他团队池", Status: enums.IMConversationStatusPending, CurrentTeamID: 20, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{ID: 4, TenantID: 1, CustomerName: "其他成员", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 202, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}

	cnd := sqls.NewCnd().Asc("id")
	applyConversationAgentTeamFilter(cnd, 10)
	got := services.ConversationService.Find(cnd)

	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("team filter should include team pool and assignee conversations, got %+v", got)
	}
}

func setupDashboardConversationHandlerTestDB(t *testing.T) *gorm.DB {
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
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentProfile{},
		&models.Conversation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}
