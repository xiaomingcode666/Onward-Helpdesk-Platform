package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type fakeMobileConversationNotificationScheduler struct {
	calls int
	item  *models.Notification
}

func (f *fakeMobileConversationNotificationScheduler) Schedule(item *models.Notification) error {
	f.calls++
	f.item = item
	return nil
}

func TestMobileConversationPushTargetsOnlyFormalConversationCustomer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CustomerUser{}, &models.ConversationParticipant{}, &models.Notification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(nil)
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	now := time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)
	customer := &models.CustomerUser{
		TenantID: 7, CustomerOrgID: 70, UserID: 701, DisplayName: "Customer 701", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	participant := &models.ConversationParticipant{
		ConversationID: 8001, ParticipantType: string(enums.IMParticipantTypeCustomer), ExternalParticipantID: "user:701",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(participant).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	scheduler := &fakeMobileConversationNotificationScheduler{}
	service := newMobileConversationPushService(scheduler)
	service.now = func() time.Time { return now }
	conversation := &models.Conversation{ID: 8001, TenantID: 7}
	message := &models.Message{ID: 9001, ConversationID: 8001, SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "工程师回复内容"}
	if err := service.ScheduleReply(conversation, message); err != nil {
		t.Fatalf("ScheduleReply(): %v", err)
	}
	if err := service.ScheduleReply(conversation, message); err != nil {
		t.Fatalf("idempotent ScheduleReply(): %v", err)
	}
	if scheduler.calls != 2 || scheduler.item == nil {
		t.Fatalf("scheduler calls/item = %d/%+v", scheduler.calls, scheduler.item)
	}
	if scheduler.item.RecipientUserID != 701 || scheduler.item.BizID != 8001 || scheduler.item.Channels != "push" {
		t.Fatalf("push notification target = %+v", scheduler.item)
	}
	if scheduler.item.ActionURL != "/mobile?state=chat&conversationId=8001" {
		t.Fatalf("push action URL = %q", scheduler.item.ActionURL)
	}
	var notificationCount int64
	if err := db.Model(&models.Notification{}).Count(&notificationCount).Error; err != nil || notificationCount != 1 {
		t.Fatalf("notification count = %d, %v", notificationCount, err)
	}

	customerMessage := &models.Message{ID: 9002, ConversationID: 8001, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "客户消息"}
	if err := service.ScheduleReply(conversation, customerMessage); err != nil {
		t.Fatalf("customer ScheduleReply(): %v", err)
	}
	if scheduler.calls != 2 {
		t.Fatalf("customer message scheduled a push; calls = %d", scheduler.calls)
	}

	crossTenantConversation := &models.Conversation{ID: 8001, TenantID: 8}
	crossTenantMessage := &models.Message{ID: 9003, ConversationID: 8001, SenderType: enums.IMSenderTypeAI, MessageType: enums.IMMessageTypeText, Content: "AI reply"}
	if err := service.ScheduleReply(crossTenantConversation, crossTenantMessage); err != nil {
		t.Fatalf("cross-tenant ScheduleReply(): %v", err)
	}
	if scheduler.calls != 2 {
		t.Fatalf("cross-tenant message scheduled a push; calls = %d", scheduler.calls)
	}
}
