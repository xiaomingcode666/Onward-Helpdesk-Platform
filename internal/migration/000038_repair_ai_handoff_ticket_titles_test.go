package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRepairAIHandoffTicketTitles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Message{}, &models.Ticket{}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)
	customerAt := now.Add(-2 * time.Minute)
	if err := db.Create(&models.Message{
		ConversationID: 11,
		ClientMsgID:    "customer-message",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "设备显示故障码 RHD-FLOW-ALPHA-7742，复位后仍然红色",
		SendStatus:     enums.IMMessageStatusSent,
		AuditFields:    models.AuditFields{CreatedAt: customerAt, UpdatedAt: customerAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Message{
		ConversationID: 11,
		ClientMsgID:    "ai-message",
		SenderType:     enums.IMSenderTypeAI,
		MessageType:    enums.IMMessageTypeText,
		Content:        "请补充具体的产品、场景和报错信息。",
		SendStatus:     enums.IMMessageStatusSent,
		AuditFields:    models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Message{
		ConversationID: 11,
		ClientMsgID:    "customer-follow-up-after-ticket",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "收到，设备已经保持断电，请安排视频确认。",
		SendStatus:     enums.IMMessageStatusSent,
		AuditFields:    models.AuditFields{CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
	}).Error; err != nil {
		t.Fatal(err)
	}

	generatedKey := "ai-handoff:11"
	previousMigrationKey := "ai-handoff:12"
	customKey := "ai-handoff:13"
	generated := models.Ticket{
		TenantID: 1, TicketNo: "TK-HANDOFF-1", ConversationID: 11,
		Title: "请补充具体的产品、场景和报错信息。", Source: enums.TicketSourceConversation,
		Description:    "客户请求人工支持\n\n请补充具体的产品、场景和报错信息。",
		IdempotencyKey: &generatedKey, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	previousMigration := models.Ticket{
		TenantID: 1, TicketNo: "TK-HANDOFF-2", ConversationID: 11,
		Title: "收到，设备已经保持断电，请安排视频确认。", Source: enums.TicketSourceConversation,
		IdempotencyKey: &previousMigrationKey, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	custom := models.Ticket{
		TenantID: 1, TicketNo: "TK-HANDOFF-3", ConversationID: 11,
		Title: "工程师人工整理标题", Source: enums.TicketSourceConversation,
		IdempotencyKey: &customKey, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&generated).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&previousMigration).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&custom).Error; err != nil {
		t.Fatal(err)
	}

	if err := repairAIHandoffTicketTitles(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&generated, generated.ID).Error; err != nil || generated.Title != "设备显示故障码 RHD-FLOW-ALPHA-7742，复位后仍然红色" {
		t.Fatalf("generated title was not repaired: title=%q err=%v", generated.Title, err)
	}
	if generated.Description != "客户请求人工支持\n\n设备显示故障码 RHD-FLOW-ALPHA-7742，复位后仍然红色" {
		t.Fatalf("generated description was not repaired: description=%q", generated.Description)
	}
	if err := db.First(&previousMigration, previousMigration.ID).Error; err != nil || previousMigration.Title != "设备显示故障码 RHD-FLOW-ALPHA-7742，复位后仍然红色" {
		t.Fatalf("title from the previous migration was not repaired: title=%q err=%v", previousMigration.Title, err)
	}
	if err := db.First(&custom, custom.ID).Error; err != nil || custom.Title != "工程师人工整理标题" {
		t.Fatalf("custom title should be preserved: title=%q err=%v", custom.Title, err)
	}
}
