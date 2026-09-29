package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnqueueWhatsAppMessageStoresAgentReplyForLocalInspection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:whatsapp-outbox?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Channel{}, &models.Conversation{}, &models.Message{}, &models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-local", Status: enums.StatusOk}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	conversation := models.Conversation{ChannelID: channel.ID}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{ConversationID: conversation.ID, SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "本地回复", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := ChannelMessageOutboxService.EnqueueWhatsAppMessage(&conversation, &message); err != nil {
		t.Fatalf("enqueue whatsapp message: %v", err)
	}
	var got models.ChannelMessageOutbox
	if err := db.Where("channel_type = ? AND message_id = ?", enums.ChannelTypeWhatsApp, message.ID).First(&got).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if got.SendStatus != string(enums.ChannelMessageOutboxStatusPending) || got.Payload == "" {
		t.Fatalf("unexpected outbox: %+v", got)
	}
}

func TestEnqueueWhatsAppMessageStoresSupportedMediaReplies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:whatsapp-media-outbox?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Channel{}, &models.Conversation{}, &models.Message{}, &models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := sqls.DB()
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(oldDB)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-media", Status: enums.StatusOk,
		ConfigJSON: `{"metaPhoneNumberId":"phone-1","metaAccessTokenSecretRef":"secret://wa-token"}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	conversation := models.Conversation{ChannelID: channel.ID}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	for _, messageType := range []enums.IMMessageType{enums.IMMessageTypeImage, enums.IMMessageTypeAudio, enums.IMMessageTypeAttachment} {
		message := models.Message{ConversationID: conversation.ID, ClientMsgID: "media-" + string(messageType), SenderType: enums.IMSenderTypeAgent, MessageType: messageType, Content: "media", Payload: `{"assetId":"asset-1"}`}
		if err := db.Create(&message).Error; err != nil {
			t.Fatalf("create %s message: %v", messageType, err)
		}
		if err := ChannelMessageOutboxService.EnqueueWhatsAppMessage(&conversation, &message); err != nil {
			t.Fatalf("enqueue %s message: %v", messageType, err)
		}
		if got := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeWhatsApp, message.ID); got == nil {
			t.Fatalf("%s message was not enqueued", messageType)
		}
	}
	twilioChannel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-twilio", Status: enums.StatusOk}
	if err := db.Create(&twilioChannel).Error; err != nil {
		t.Fatalf("create Twilio channel: %v", err)
	}
	twilioConversation := models.Conversation{ChannelID: twilioChannel.ID}
	if err := db.Create(&twilioConversation).Error; err != nil {
		t.Fatalf("create Twilio conversation: %v", err)
	}
	twilioMedia := models.Message{ConversationID: twilioConversation.ID, ClientMsgID: "twilio-media", SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeImage, Content: "image.jpg", Payload: `{"assetId":"asset-2"}`}
	if err := db.Create(&twilioMedia).Error; err != nil {
		t.Fatalf("create Twilio media message: %v", err)
	}
	if err := ChannelMessageOutboxService.EnqueueWhatsAppMessage(&twilioConversation, &twilioMedia); err != nil {
		t.Fatalf("enqueue Twilio media: %v", err)
	}
	if got := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeWhatsApp, twilioMedia.ID); got != nil {
		t.Fatalf("Twilio media must not enter the Meta outbox: %+v", got)
	}
}
