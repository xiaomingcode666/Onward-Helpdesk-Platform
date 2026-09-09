package services_test

import (
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNotificationServiceCreateAndUnreadCount(t *testing.T) {
	setupNotificationTestDB(t)
	seedNotificationTestUser(t, 1, 101)
	seedNotificationTestUser(t, 1, 102)

	item, err := services.NotificationService.Create(request.CreateNotificationRequest{
		TenantID:         1,
		RecipientUserID:  101,
		Title:            "工单指派提醒",
		Content:          "工单 TK-1 已指派给你",
		NotificationType: "ticket_assigned",
		BizType:          "system",
		BizID:            1,
		ActionURL:        "/dashboard/tickets/1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if item.ID == 0 {
		t.Fatalf("expected notification id to be assigned")
	}
	if item.RecipientUserID != 101 || item.ReadAt != nil {
		t.Fatalf("unexpected notification: %+v", item)
	}
	if got := services.NotificationService.CountUnread(101); got != 1 {
		t.Fatalf("expected unread count 1, got %d", got)
	}
	if got := services.NotificationService.CountUnread(102); got != 0 {
		t.Fatalf("expected unread count 0 for another user, got %d", got)
	}
}

func TestNotificationServiceMarkReadRequiresOwner(t *testing.T) {
	setupNotificationTestDB(t)
	seedNotificationTestUser(t, 1, 201)
	seedNotificationTestUser(t, 1, 202)

	item, err := services.NotificationService.Create(request.CreateNotificationRequest{
		TenantID:         1,
		RecipientUserID:  201,
		Title:            "会话分配提醒",
		Content:          "会话 #9 已分配给你",
		NotificationType: "conversation_assigned",
		BizType:          "system",
		BizID:            9,
		ActionURL:        "/dashboard/conversations?conversationId=9",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := services.NotificationService.MarkRead(item.ID, 202); err == nil {
		t.Fatalf("expected foreign user mark read to fail")
	}
	if got := services.NotificationService.CountUnread(201); got != 1 {
		t.Fatalf("expected notification to remain unread, got %d", got)
	}
	if err := services.NotificationService.MarkRead(item.ID, 201); err != nil {
		t.Fatalf("MarkRead() owner error = %v", err)
	}
	if got := services.NotificationService.CountUnread(201); got != 0 {
		t.Fatalf("expected unread count 0 after mark read, got %d", got)
	}
}

func TestNotificationServiceMarkAllReadOnlyCurrentUser(t *testing.T) {
	setupNotificationTestDB(t)
	seedNotificationTestUser(t, 1, 301)
	seedNotificationTestUser(t, 1, 302)

	for _, userID := range []int64{301, 301, 302} {
		if _, err := services.NotificationService.Create(request.CreateNotificationRequest{
			TenantID:         1,
			RecipientUserID:  userID,
			Title:            "工单指派提醒",
			Content:          "工单已指派给你",
			NotificationType: "ticket_assigned",
			BizType:          "system",
			BizID:            userID,
			ActionURL:        "/dashboard/tickets/1",
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	if err := services.NotificationService.MarkAllRead(301); err != nil {
		t.Fatalf("MarkAllRead() error = %v", err)
	}
	if got := services.NotificationService.CountUnread(301); got != 0 {
		t.Fatalf("expected user 301 unread count 0, got %d", got)
	}
	if got := services.NotificationService.CountUnread(302); got != 1 {
		t.Fatalf("expected user 302 unread count 1, got %d", got)
	}
}

func TestNotificationServiceCreateIsIdempotentForQueuedJob(t *testing.T) {
	setupNotificationTestDB(t)
	seedNotificationTestUser(t, 1, 401)
	req := request.CreateNotificationRequest{
		TenantID:         1,
		RecipientUserID:  401,
		Title:            "队列通知",
		NotificationType: "ticket_created",
		BizType:          "system",
		IdempotencyKey:   "notification-job-1",
	}
	first, err := services.NotificationService.Create(req)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	second, err := services.NotificationService.Create(req)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("notification ids = %d and %d, want same id", first.ID, second.ID)
	}
	var count int64
	if err := sqls.DB().Model(&models.Notification{}).Count(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("notification count = %d, want 1", count)
	}
}

func TestNotificationServiceAllowsSameIdempotencyKeyAcrossTenants(t *testing.T) {
	setupNotificationTestDB(t)
	seedNotificationTestUser(t, 1, 501)
	seedNotificationTestUser(t, 2, 502)

	var ids []int64
	for _, fixture := range []struct {
		tenantID int64
		userID   int64
	}{{tenantID: 1, userID: 501}, {tenantID: 2, userID: 502}} {
		item, err := services.NotificationService.Create(request.CreateNotificationRequest{
			TenantID:         fixture.tenantID,
			RecipientUserID:  fixture.userID,
			Title:            "租户内幂等通知",
			NotificationType: "ticket_created",
			BizType:          "system",
			IdempotencyKey:   "ticket:42:created",
		})
		if err != nil {
			t.Fatalf("tenant %d Create() error = %v", fixture.tenantID, err)
		}
		ids = append(ids, item.ID)
	}
	if ids[0] == ids[1] {
		t.Fatalf("cross-tenant notifications unexpectedly reused id %d", ids[0])
	}
	var count int64
	if err := sqls.DB().Model(&models.Notification{}).Count(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 2 {
		t.Fatalf("notification count = %d, want 2", count)
	}
}

func setupNotificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
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
	if err := db.AutoMigrate(&models.User{}, &models.TenantMember{}, &models.Notification{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func seedNotificationTestUser(t *testing.T, tenantID, userID int64) {
	t.Helper()
	now := time.Now()
	user := &models.User{
		ID:       userID,
		Username: fmt.Sprintf("notification-user-%d", userID),
		Nickname: fmt.Sprintf("User %d", userID),
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(user).Error; err != nil {
		t.Fatalf("create notification user: %v", err)
	}
	member := &models.TenantMember{
		TenantID:    tenantID,
		UserID:      userID,
		DisplayName: user.Nickname,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(member).Error; err != nil {
		t.Fatalf("create notification member: %v", err)
	}
}
