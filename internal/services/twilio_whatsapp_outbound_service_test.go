package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/twilio"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestTwilioWhatsAppOutboundSkipsMetaChannelOutbox(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:twilio-skip-meta?mode=memory&cache=shared"), &gorm.Config{
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
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "meta", Status: enums.StatusOk,
		ConfigJSON: `{"metaPhoneNumberId":"phone-1","metaAccessTokenSecretRef":"secret://wa-token"}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	conversation := models.Conversation{ChannelID: channel.ID}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{ConversationID: conversation.ID, ClientMsgID: "reply-1", SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "reply"}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	outbox := models.ChannelMessageOutbox{ChannelType: enums.ChannelTypeWhatsApp, ConversationID: conversation.ID, MessageID: message.ID, SendStatus: string(enums.ChannelMessageOutboxStatusPending)}
	if err := db.Create(&outbox).Error; err != nil {
		t.Fatalf("create outbox: %v", err)
	}
	if err := TwilioWhatsAppOutboundService.processOutbox(outbox.ID, twilio.WhatsAppConfig{AccountSID: "AC123", AuthToken: "token", From: "whatsapp:+10000000000"}); err != nil {
		t.Fatalf("process Meta outbox with Twilio dispatcher: %v", err)
	}
	var got models.ChannelMessageOutbox
	if err := db.First(&got, outbox.ID).Error; err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if got.SendStatus != string(enums.ChannelMessageOutboxStatusPending) {
		t.Fatalf("Meta outbox status = %q, want pending", got.SendStatus)
	}
}

func TestBuildTwilioWhatsAppMessageFormNormalizesRecipient(t *testing.T) {
	form, err := buildTwilioWhatsAppMessageForm("whatsapp:+17372508034", "8613800000000", "客服回复")
	if err != nil {
		t.Fatalf("buildTwilioWhatsAppMessageForm() error = %v", err)
	}
	if got, want := form.Get("From"), "whatsapp:+17372508034"; got != want {
		t.Fatalf("From = %q, want %q", got, want)
	}
	if got, want := form.Get("To"), "whatsapp:+8613800000000"; got != want {
		t.Fatalf("To = %q, want %q", got, want)
	}
	if got, want := form.Get("Body"), "客服回复"; got != want {
		t.Fatalf("Body = %q, want %q", got, want)
	}
}

func TestBuildTwilioWhatsAppMessageFormRejectsMissingFields(t *testing.T) {
	if _, err := buildTwilioWhatsAppMessageForm("", "8613800000000", "reply"); err == nil {
		t.Fatal("missing sender was accepted")
	}
	if _, err := buildTwilioWhatsAppMessageForm("whatsapp:+17372508034", "", "reply"); err == nil {
		t.Fatal("missing recipient was accepted")
	}
	if _, err := buildTwilioWhatsAppMessageForm("whatsapp:+17372508034", "8613800000000", ""); err == nil {
		t.Fatal("missing body was accepted")
	}
}

func TestTwilioWhatsAppOutboundSendsFormToMessagesAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "AC123" || pass != "auth-token" {
			t.Fatalf("unexpected basic auth: %q/%q/%v", user, pass, ok)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("To") != "whatsapp:+8613800000000" || r.Form.Get("Body") != "客服回复" {
			t.Fatalf("unexpected Twilio form: %#v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sid":"SM999"}`))
	}))
	defer server.Close()
	form, err := buildTwilioWhatsAppMessageForm("whatsapp:+17372508034", "8613800000000", "客服回复")
	if err != nil {
		t.Fatalf("build form: %v", err)
	}
	sid, err := TwilioWhatsAppOutboundService.sendMessage(twilio.WhatsAppConfig{AccountSID: "AC123", AuthToken: "auth-token", APIBaseURL: server.URL}, form)
	if err != nil {
		t.Fatalf("sendMessage() error = %v", err)
	}
	if sid != "SM999" {
		t.Fatalf("sid = %q, want SM999", sid)
	}
}
