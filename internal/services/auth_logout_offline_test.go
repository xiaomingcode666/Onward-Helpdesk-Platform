package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TestLogoutMarksEngineerOfflineWhenLastSession 验证最后一个会话登出后令牌失效，
// 工程师状态置离线，并保持由日历和已审批请假决定的排班可用性。
func TestLogoutMarksEngineerOfflineWhenLastSession(t *testing.T) {
	db := setupLogoutOfflineTestDB(t)
	now := time.Now()
	createLogoutOfflineEngineer(t, db, 11)
	if err := db.Create(&models.LoginSession{
		UserID: 11, Token: "ak_only", ClientType: "admin_web", ExpiredAt: now.Add(time.Hour),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}
	calendarAvailable := AgentWorkStatusService.IsDispatchAvailable(1, 11)

	auth := newAuthService()
	if err := auth.Logout("Bearer ak_only"); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	session := LoginSessionService.FindOne(sqls.NewCnd().Eq("token", "ak_only"))
	if session == nil || session.RevokedAt == nil {
		t.Fatal("logout must persist the session revocation")
	}
	if _, err := auth.validateSessionToken("ak_only"); err == nil {
		t.Fatal("logged-out token must not authenticate")
	}

	status := repositories.AgentWorkStatusRepository.GetByTenantAndUser(db, 1, 11)
	if status == nil {
		t.Fatal("expected engineer work status to exist after logout")
	}
	if status.Status != AgentWorkStatusOffline {
		t.Fatalf("expected offline after last session logout, got %q", status.Status)
	}
	if AgentWorkStatusService.IsDispatchAvailable(1, 11) != calendarAvailable {
		t.Fatal("logout presence must not change calendar availability")
	}
}

// TestLogoutKeepsEngineerAvailableWithOtherActiveSession 验证仍有其他活跃会话时登出不置离线，
// 避免多端登录场景下误下线。
func TestLogoutKeepsEngineerAvailableWithOtherActiveSession(t *testing.T) {
	db := setupLogoutOfflineTestDB(t)
	now := time.Now()
	createLogoutOfflineEngineer(t, db, 12)
	for _, token := range []string{"ak_a", "ak_b"} {
		if err := db.Create(&models.LoginSession{
			UserID: 12, Token: token, ClientType: "admin_web", ExpiredAt: now.Add(time.Hour),
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}).Error; err != nil {
			t.Fatalf("seed session %s: %v", token, err)
		}
	}

	auth := newAuthService()
	if err := auth.Logout("Bearer ak_a"); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if _, err := auth.validateSessionToken("ak_a"); err == nil {
		t.Fatal("logged-out token must not authenticate")
	}
	if _, err := auth.validateSessionToken("ak_b"); err != nil {
		t.Fatalf("other active session must remain valid: %v", err)
	}

	status := repositories.AgentWorkStatusRepository.GetByTenantAndUser(db, 1, 12)
	if status == nil || status.Status != AgentWorkStatusAvailable {
		got := ""
		if status != nil {
			got = status.Status
		}
		t.Fatalf("expected engineer to stay available with another active session, got %q", got)
	}
}

func setupLogoutOfflineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.LoginSession{},
		&models.AgentProfile{},
		&models.AgentWorkStatus{},
		&models.Ticket{},
		&models.Conversation{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(nil)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createLogoutOfflineEngineer(t *testing.T, db *gorm.DB, userID int64) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID: userID, Username: "engineer-" + time.Now().Format("150405.000000000"), Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: userID, TeamID: 1, AgentCode: "ENG" + string(rune('A'+userID%26)),
		DisplayName: "工程师", ServiceStatus: enums.ServiceStatusIdle,
		AutoAssignEnabled: true, Status: enums.StatusOk, LastOnlineAt: &now,
	}).Error; err != nil {
		t.Fatalf("create agent profile: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: userID, Status: AgentWorkStatusAvailable,
		ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}
}
