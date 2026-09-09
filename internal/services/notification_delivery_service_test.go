package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeNotificationEmailTransport struct {
	err   error
	calls int
}

func (f *fakeNotificationEmailTransport) SendNotificationEmail(int64, string, string, string, map[string]interface{}) error {
	f.calls++
	return f.err
}

type fakeNotificationPushTransport struct {
	err      error
	calls    int
	token    string
	platform string
	message  providers.MobilePushMessage
}

func (f *fakeNotificationPushTransport) SendNotificationPush(_ context.Context, platform, token string, message providers.MobilePushMessage) (string, error) {
	f.calls++
	f.platform = platform
	f.token = token
	f.message = message
	if f.err != nil {
		return "", f.err
	}
	return "provider-message-1", nil
}

func TestNotificationDeliverySchedulesOnceAndMarksEmailSent(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	email := "engineer@example.com"
	seedNotificationDeliveryUser(t, db, 1, 101, &email)
	item := createEmailNotificationForDeliveryTest(t, 1, 101, "ticket.closed:1:notification:recipient:101")

	transport := &fakeNotificationEmailTransport{}
	service := newNotificationDeliveryService(transport)
	baseTime := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return baseTime }
	if err := service.Schedule(item); err != nil {
		t.Fatalf("schedule email delivery: %v", err)
	}
	if err := service.Schedule(item); err != nil {
		t.Fatalf("reschedule email delivery: %v", err)
	}
	var count int64
	if err := db.Model(&models.DeliveryLog{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("delivery task count = %d, err = %v", count, err)
	}
	if processed := service.ProcessDue(context.Background(), 20); processed != 1 {
		t.Fatalf("processed deliveries = %d, want 1", processed)
	}
	if transport.calls != 1 {
		t.Fatalf("email transport calls = %d, want 1", transport.calls)
	}
	var delivery models.DeliveryLog
	if err := db.First(&delivery).Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if delivery.Status != notificationDeliveryStatusSent || delivery.SentAt == nil || delivery.NextAttemptAt != nil {
		t.Fatalf("unexpected completed delivery: %+v", delivery)
	}
	updated := repositoriesNotificationForTest(t, db, item.ID)
	if updated.ExternalChannelStatus != notificationEmailStatusSent || updated.DeliveryStatus != "sent" {
		t.Fatalf("unexpected notification delivery state: %+v", updated)
	}
}

func TestNotificationDeliveryHonorsRetryPolicyAndStops(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	email := "manager@example.com"
	seedNotificationDeliveryUser(t, db, 2, 201, &email)
	if err := db.Create(&models.TenantMailSetting{
		TenantID: 2, RetryPolicy: "retry_1_5m", Status: int(enums.StatusOk), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create mail setting: %v", err)
	}
	item := createEmailNotificationForDeliveryTest(t, 2, 201, "sla.warning:2:notification:recipient:201")
	transport := &fakeNotificationEmailTransport{err: errors.New("smtp unavailable")}
	service := newNotificationDeliveryService(transport)
	clock := time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	if err := service.Schedule(item); err != nil {
		t.Fatalf("schedule email delivery: %v", err)
	}
	if processed := service.ProcessDue(context.Background(), 20); processed != 1 {
		t.Fatalf("first processed count = %d", processed)
	}
	var delivery models.DeliveryLog
	if err := db.First(&delivery).Error; err != nil {
		t.Fatalf("load retry delivery: %v", err)
	}
	if delivery.Status != notificationDeliveryStatusWaitingRetry || delivery.RetryCount != 1 || delivery.NextAttemptAt == nil || !delivery.NextAttemptAt.Equal(clock.Add(5*time.Minute)) {
		t.Fatalf("unexpected retry state: %+v", delivery)
	}
	clock = clock.Add(5 * time.Minute)
	if processed := service.ProcessDue(context.Background(), 20); processed != 1 {
		t.Fatalf("second processed count = %d", processed)
	}
	var terminal models.DeliveryLog
	if err := db.First(&terminal, delivery.ID).Error; err != nil {
		t.Fatalf("reload failed delivery: %v", err)
	}
	if terminal.Status != notificationDeliveryStatusFailed || terminal.RetryCount != 2 || terminal.NextAttemptAt != nil {
		t.Fatalf("unexpected terminal delivery: %+v", terminal)
	}
	updated := repositoriesNotificationForTest(t, db, item.ID)
	if updated.ExternalChannelStatus != notificationEmailStatusFailed || transport.calls != 2 {
		t.Fatalf("unexpected terminal notification state: notification=%+v calls=%d", updated, transport.calls)
	}
}

func TestNotificationDeliveryStopsWhenRecipientHasNoEmail(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	seedNotificationDeliveryUser(t, db, 3, 301, nil)
	item := createEmailNotificationForDeliveryTest(t, 3, 301, "quota.exceeded:3:notification:recipient:301")
	transport := &fakeNotificationEmailTransport{}
	service := newNotificationDeliveryService(transport)
	service.now = func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) }
	if err := service.Schedule(item); err != nil {
		t.Fatalf("schedule email delivery: %v", err)
	}
	service.ProcessDue(context.Background(), 20)
	var delivery models.DeliveryLog
	if err := db.First(&delivery).Error; err != nil {
		t.Fatalf("load unavailable delivery: %v", err)
	}
	if delivery.Status != notificationDeliveryStatusFailed || transport.calls != 0 {
		t.Fatalf("unexpected unavailable recipient state: delivery=%+v calls=%d", delivery, transport.calls)
	}
	if updated := repositoriesNotificationForTest(t, db, item.ID); updated.ExternalChannelStatus != notificationEmailStatusUnavailable {
		t.Fatalf("email status = %q, want unavailable", updated.ExternalChannelStatus)
	}
}

func TestNotificationDeliverySchedulesCustomerPushOnceAndRevokesInvalidToken(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	seedNotificationDeliveryUser(t, db, 4, 401, nil)
	idempotencyKey := "conversation:9001:message:1:customer-push:401"
	item := &models.Notification{
		TenantID: 4, RecipientUserID: 401, Title: "新回复", Content: "工程师已回复", NotificationType: "conversation_message",
		BizType: "conversation", BizID: 9001, ActionURL: "/mobile?state=chat&conversationId=9001", Channels: "push",
		IdempotencyKey: &idempotencyKey, DeliveryStatus: "sent", Status: int(enums.StatusOk), CreatedAt: time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create push notification: %v", err)
	}
	ciphertext, err := secretstore.Encrypt("native-device-token")
	if err != nil {
		t.Fatalf("encrypt push token: %v", err)
	}
	now := time.Now()
	token := &models.MobilePushToken{
		TenantID: 4, UserID: 401, Platform: "ios", TokenFingerprint: "fingerprint", TokenCiphertext: ciphertext,
		LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create push token: %v", err)
	}

	transport := &fakeNotificationPushTransport{}
	service := newNotificationDeliveryService(&fakeNotificationEmailTransport{}, transport)
	baseTime := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return baseTime }
	// Do not call Schedule here: ProcessDue must recover the durable push task
	// if a process stopped after the notification row was committed.
	if processed := service.ProcessDue(context.Background(), 20); processed != 1 {
		t.Fatalf("processed push deliveries = %d, want 1", processed)
	}
	if transport.calls != 1 || transport.platform != "ios" || transport.token != "native-device-token" {
		t.Fatalf("push transport = calls:%d platform:%q token:%q", transport.calls, transport.platform, transport.token)
	}
	if transport.message.Data["conversationId"] != "9001" || transport.message.Data["actionUrl"] != item.ActionURL {
		t.Fatalf("push deep-link data = %+v", transport.message.Data)
	}
	var delivery models.DeliveryLog
	if err := db.Where("channel = ?", "push").First(&delivery).Error; err != nil {
		t.Fatalf("load push delivery: %v", err)
	}
	if delivery.Status != notificationDeliveryStatusSent || delivery.ProviderMsgID != "provider-message-1" || delivery.SentAt == nil {
		t.Fatalf("push delivery state = %+v", delivery)
	}

	invalidItem := &models.Notification{
		TenantID: 4, RecipientUserID: 401, Title: "新回复", Content: "另一条消息", NotificationType: "conversation_message",
		BizType: "conversation", BizID: 9002, ActionURL: "/mobile?state=chat&conversationId=9002", Channels: "push",
		DeliveryStatus: "sent", Status: int(enums.StatusOk), CreatedAt: baseTime,
	}
	if err := db.Create(invalidItem).Error; err != nil {
		t.Fatalf("create invalid-token notification: %v", err)
	}
	transport.err = &providers.MobilePushProviderError{Platform: "ios", StatusCode: http.StatusGone, Reason: "Unregistered", InvalidToken: true, Permanent: true}
	if err := service.Schedule(invalidItem); err != nil {
		t.Fatalf("schedule invalid-token push: %v", err)
	}
	if processed := service.ProcessDue(context.Background(), 20); processed != 1 {
		t.Fatalf("processed invalid-token deliveries = %d, want 1", processed)
	}
	var revoked models.MobilePushToken
	if err := db.First(&revoked, token.ID).Error; err != nil || revoked.RevokedAt == nil {
		t.Fatalf("invalid push token not revoked: %+v, %v", revoked, err)
	}
}

func setupNotificationDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TenantMember{}, &models.Notification{}, &models.DeliveryLog{}, &models.TenantMailSetting{}, &models.NotificationRecipientSetting{}, &models.MobilePushToken{}); err != nil {
		t.Fatalf("migrate notification delivery models: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func seedNotificationDeliveryUser(t *testing.T, db *gorm.DB, tenantID, userID int64, email *string) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID: userID, Username: fmt.Sprintf("delivery-user-%d", userID), Nickname: fmt.Sprintf("User %d", userID), Email: email, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create delivery user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID: tenantID, UserID: userID, DisplayName: fmt.Sprintf("User %d", userID), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create delivery member: %v", err)
	}
}

func createEmailNotificationForDeliveryTest(t *testing.T, tenantID, userID int64, key string) *models.Notification {
	t.Helper()
	item, err := NotificationService.Create(request.CreateNotificationRequest{
		TenantID: tenantID, RecipientUserID: userID, Title: "邮件通知", NotificationType: "sla_warning", BizType: "system", Channels: "in_app,email", IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	return item
}
