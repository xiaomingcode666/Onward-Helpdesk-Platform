package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestValidateWhatsAppLocalInboundRequiresStableIdentifiers(t *testing.T) {
	base := WhatsAppLocalInbound{
		EventID:   "event-1",
		ChannelID: "channel-1",
		From:      "8613800000000",
		MessageID: "wamid-1",
		Text:      "你好",
	}

	for name, mutate := range map[string]func(*WhatsAppLocalInbound){
		"event id":   func(v *WhatsAppLocalInbound) { v.EventID = "" },
		"channel id": func(v *WhatsAppLocalInbound) { v.ChannelID = "" },
		"sender":     func(v *WhatsAppLocalInbound) { v.From = "" },
		"message id": func(v *WhatsAppLocalInbound) { v.MessageID = "" },
		"text":       func(v *WhatsAppLocalInbound) { v.Text = "" },
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			mutate(&input)
			if err := ValidateWhatsAppLocalInbound(input); err == nil {
				t.Fatalf("ValidateWhatsAppLocalInbound() accepted missing %s", name)
			}
		})
	}
}

func TestBuildWhatsAppLocalClientMessageIDPrefixesProvider(t *testing.T) {
	if got, want := BuildWhatsAppLocalClientMessageID("wamid-1"), "whatsapp-local:wamid-1"; got != want {
		t.Fatalf("BuildWhatsAppLocalClientMessageID() = %q, want %q", got, want)
	}
}

func TestWhatsAppLocalServiceCreatesConversationAndMessageIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:whatsapp-local-process?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Channel{}, &models.AIAgent{}, &models.WebhookEventInbox{}, &models.Customer{}, &models.CustomerIdentity{},
		&models.Conversation{}, &models.ConversationParticipant{}, &models.ConversationEventLog{}, &models.Message{}, &models.ConversationReadState{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previousEnqueue := EnqueueAIReplyJobHook
	previousTrigger := TriggerAIReplyAsyncHook
	EnqueueAIReplyJobHook = nil
	TriggerAIReplyAsyncHook = nil
	t.Cleanup(func() {
		EnqueueAIReplyJobHook = previousEnqueue
		TriggerAIReplyAsyncHook = previousTrigger
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	agent := models.AIAgent{Name: "本地 WhatsApp 测试 Agent", Status: enums.StatusOk, ServiceMode: enums.IMConversationServiceModeAIOnly}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-process", AIAgentID: agent.ID, Status: enums.StatusOk}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	input := WhatsAppLocalInbound{EventID: "event-process-1", ChannelID: channel.ChannelID, From: "8613800000000", ProfileName: "本地访客", MessageID: "wamid-process-1", Text: "测试消息"}
	first, err := WhatsAppLocalService.ProcessInbound(nil, input)
	if err != nil {
		t.Fatalf("process first inbound: %v", err)
	}
	if first.Duplicate || first.ConversationID == 0 || first.MessageID == 0 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := WhatsAppLocalService.ProcessInbound(nil, input)
	if err != nil {
		t.Fatalf("process duplicate inbound: %v", err)
	}
	if !second.Duplicate {
		t.Fatalf("duplicate result = %+v, want duplicate", second)
	}
	var messages int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ? AND client_msg_id = ?", first.ConversationID, BuildWhatsAppLocalClientMessageID(input.MessageID)).Count(&messages).Error; err != nil {
		t.Fatalf("count inbound messages: %v", err)
	}
	if messages != 1 {
		t.Fatalf("inbound message count = %d, want 1", messages)
	}
}
