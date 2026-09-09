package repositories

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestFindSupplierConversationMessagesIncludesAIReplies(t *testing.T) {
	dbName := "message_repository_" + strings.NewReplacer("/", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Message{}); err != nil {
		t.Fatalf("auto migrate messages: %v", err)
	}
	now := time.Now()
	messages := []models.Message{
		{ConversationID: 9, ClientMsgID: "customer", SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "客户描述故障", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{ConversationID: 9, ClientMsgID: "ai", SenderType: enums.IMSenderTypeAI, MessageType: enums.IMMessageTypeText, Content: "服务助手诊断", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(time.Millisecond), UpdatedAt: now.Add(time.Millisecond)}},
		{ConversationID: 9, ClientMsgID: "agent", SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "工程师回复", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(2 * time.Millisecond), UpdatedAt: now.Add(2 * time.Millisecond)}},
		{ConversationID: 9, ClientMsgID: "partner", SenderType: enums.IMSenderTypePartner, MessageType: enums.IMMessageTypeText, Content: "供应商回复", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(3 * time.Millisecond), UpdatedAt: now.Add(3 * time.Millisecond)}},
		{ConversationID: 9, ClientMsgID: "system", SenderType: enums.IMSenderTypeSystem, MessageType: enums.IMMessageTypeText, Content: "系统事件", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(4 * time.Millisecond), UpdatedAt: now.Add(4 * time.Millisecond)}},
		{ConversationID: 9, ClientMsgID: "failed-ai", SenderType: enums.IMSenderTypeAI, MessageType: enums.IMMessageTypeText, Content: "失败消息", SendStatus: enums.IMMessageStatusFailed, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(5 * time.Millisecond), UpdatedAt: now.Add(5 * time.Millisecond)}},
		{ConversationID: 10, ClientMsgID: "other-ai", SenderType: enums.IMSenderTypeAI, MessageType: enums.IMMessageTypeText, Content: "其他会话", SendStatus: enums.IMMessageStatusSent, SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now.Add(6 * time.Millisecond), UpdatedAt: now.Add(6 * time.Millisecond)}},
	}
	if err := db.Create(&messages).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}

	got := MessageRepository.FindSupplierConversationMessages(db, 9, now.Add(-time.Minute), nil)
	if len(got) != 5 {
		t.Fatalf("supplier-visible messages = %d, want 5: %#v", len(got), got)
	}
	seen := make(map[enums.IMSenderType]bool, len(got))
	for _, item := range got {
		seen[item.SenderType] = true
		if item.SendStatus == enums.IMMessageStatusFailed || item.ConversationID != 9 {
			t.Fatalf("unexpected message leaked into supplier view: %+v", item)
		}
	}
	for _, senderType := range []enums.IMSenderType{
		enums.IMSenderTypeCustomer,
		enums.IMSenderTypeAI,
		enums.IMSenderTypeAgent,
		enums.IMSenderTypePartner,
		enums.IMSenderTypeSystem,
	} {
		if !seen[senderType] {
			t.Fatalf("sender type %q missing from supplier-visible messages: %#v", senderType, got)
		}
	}
}
