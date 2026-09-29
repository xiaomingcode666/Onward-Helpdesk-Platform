package services

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestApplyWhatsAppProviderStatusRecordsReadReceipt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:whatsapp-provider-status?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	outbox := models.ChannelMessageOutbox{ChannelType: "whatsapp", ConversationID: 1, MessageID: 2, SendStatus: "sent", ExternalMessageID: "wamid.outbound-1"}
	if err := db.Create(&outbox).Error; err != nil {
		t.Fatalf("create outbox: %v", err)
	}

	if !ChannelMessageOutboxService.ApplyWhatsAppProviderStatus("wamid.outbound-1", "read") {
		t.Fatal("read status did not match outbox")
	}
	var got models.ChannelMessageOutbox
	if err := db.First(&got, outbox.ID).Error; err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if got.ProviderStatus != "read" || got.ReadAt == nil || got.DeliveredAt == nil {
		t.Fatalf("read receipt was not recorded: %+v", got)
	}
}
