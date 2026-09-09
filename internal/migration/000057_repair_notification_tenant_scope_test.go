package migration

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRepairNotificationTenantScope(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "notification-scope.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TenantMember{}, &models.Conversation{}, &models.Notification{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	user := &models.User{
		Username: "notification-owner",
		Nickname: "Notification Owner",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    7,
		UserID:      user.ID,
		DisplayName: "值班工程师",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	conversation := &models.Conversation{
		TenantID: 7,
		Status:   enums.IMConversationStatusActive,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	notification := &models.Notification{
		TenantID:         0,
		RecipientUserID:  user.ID,
		Title:            "会话分配提醒",
		NotificationType: "conversation_assigned",
		BizType:          "conversation",
		BizID:            conversation.ID,
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}
	if err := db.Create(notification).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	if err := repairNotificationTenantScope(db); err != nil {
		t.Fatalf("repairNotificationTenantScope() error = %v", err)
	}
	if err := repairNotificationTenantScope(db); err != nil {
		t.Fatalf("idempotent repairNotificationTenantScope() error = %v", err)
	}
	var repaired models.Notification
	if err := db.First(&repaired, notification.ID).Error; err != nil {
		t.Fatalf("load repaired notification: %v", err)
	}
	if repaired.TenantID != 7 || repaired.Category != "ticket" || repaired.RecipientName != "值班工程师" {
		t.Fatalf("unexpected repaired notification: %+v", repaired)
	}
	if repaired.ActionURL != "/enterprise/ticket-workbench?conversationId=1" {
		t.Fatalf("action url = %q", repaired.ActionURL)
	}
}
