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

// TestLogoutMarksEngineerOfflineWhenLastSession 验证工程师登出最后一个会话后被置离线，
// 从而不再参与自动派单（工单响应闭环 M4）。
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

	if err := newAuthService().Logout("Bearer ak_only"); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	status := repositories.AgentWorkStatusRepository.GetByTenantAndUser(db, 1, 11)
	if status == nil {
		t.Fatal("expected engineer work status to exist after logout")
	}
	if status.Status != AgentWorkStatusOffline {
		t.Fatalf("expected offline after last session logout, got %q", status.Status)
	}
	if AgentWorkStatusService.IsDispatchAvailable(1, 11) {
		t.Fatal("offline engineer must not be dispatchable")
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

	if err := newAuthService().Logout("Bearer ak_a"); err != nil {
		t.Fatalf("logout failed: %v", err)
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
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(nil) })
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
