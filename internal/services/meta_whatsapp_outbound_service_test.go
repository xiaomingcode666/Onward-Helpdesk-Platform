package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestBuildMetaWhatsAppMediaPayloadRoutesSupportedMedia(t *testing.T) {
	tests := []struct {
		name, mimeType, filename, caption, wantType string
	}{
		{name: "image", mimeType: "image/jpeg", filename: "damage.jpg", caption: "damage", wantType: "image"},
		{name: "document", mimeType: "application/pdf", filename: "report.pdf", caption: "report", wantType: "document"},
		{name: "audio", mimeType: "audio/ogg", filename: "voice.ogg", wantType: "audio"},
		{name: "video", mimeType: "video/mp4", filename: "fault.mp4", caption: "fault", wantType: "video"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := buildMetaWhatsAppMediaPayload("+86 138-0000-0000", "media-1", tt.mimeType, tt.filename, tt.caption)
			if err != nil {
				t.Fatalf("build media payload: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(payload, &got); err != nil {
				t.Fatalf("decode media payload: %v", err)
			}
			if got["to"] != "8613800000000" || got["type"] != tt.wantType {
				t.Fatalf("unexpected envelope: %#v", got)
			}
			media, ok := got[tt.wantType].(map[string]any)
			if !ok || media["id"] != "media-1" {
				t.Fatalf("unexpected media object: %#v", got[tt.wantType])
			}
			if tt.wantType == "document" && media["filename"] != tt.filename {
				t.Fatalf("document filename = %#v", media["filename"])
			}
			if tt.caption != "" && media["caption"] != tt.caption {
				t.Fatalf("caption = %#v", media["caption"])
			}
			if tt.wantType == "audio" {
				if _, exists := media["caption"]; exists {
					t.Fatalf("audio payload must not contain caption: %#v", media)
				}
			}
		})
	}
}

func TestMetaWhatsAppOutboundUploadsMediaBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v25.0/phone-1/media" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if got := r.FormValue("messaging_product"); got != "whatsapp" {
			t.Fatalf("messaging_product = %q", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("read file part: %v", err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "damage.jpg" || header.Header.Get("Content-Type") != "image/jpeg" || !bytes.Equal(data, []byte("jpeg-data")) {
			t.Fatalf("unexpected upload: name=%q type=%q data=%q", header.Filename, header.Header.Get("Content-Type"), data)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"media-uploaded-1"}`))
	}))
	defer server.Close()

	mediaID, err := MetaWhatsAppOutboundService.uploadMedia(metaWhatsAppRuntimeConfig{
		PhoneNumberID: "phone-1", AccessToken: "access-token", APIBaseURL: server.URL, APIVersion: "v25.0",
	}, bytes.NewReader([]byte("jpeg-data")), "damage.jpg", "image/jpeg")
	if err != nil {
		t.Fatalf("uploadMedia() error = %v", err)
	}
	if mediaID != "media-uploaded-1" {
		t.Fatalf("media ID = %q", mediaID)
	}
}

func TestBuildMetaWhatsAppTextPayloadNormalizesRecipient(t *testing.T) {
	payload, err := buildMetaWhatsAppTextPayload("+86 138-0000-0000", "客服回复")
	if err != nil {
		t.Fatalf("buildMetaWhatsAppTextPayload() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got["messaging_product"] != "whatsapp" || got["recipient_type"] != "individual" || got["to"] != "8613800000000" || got["type"] != "text" {
		t.Fatalf("unexpected Meta payload: %#v", got)
	}
	text, ok := got["text"].(map[string]any)
	if !ok || text["body"] != "客服回复" || text["preview_url"] != false {
		t.Fatalf("unexpected text payload: %#v", got["text"])
	}
}

func TestMetaWhatsAppOutboundDispatchesPendingOutbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.dispatched-1"}]}`))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file:meta-whatsapp-dispatch?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIAgent{}, &models.Channel{}, &models.CustomerIdentity{}, &models.Conversation{}, &models.Message{}, &models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	root := t.TempDir()
	secretDir := filepath.Join(root, "0", "development")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		t.Fatalf("create secret dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "wa-token"), []byte("access-token"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_META_GRAPH_API_BASE_URL", server.URL)
	t.Setenv("RHD_META_GRAPH_API_VERSION", "v25.0")

	agent := models.AIAgent{Name: "Meta agent", Status: enums.StatusOk}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "meta-channel", AIAgentID: agent.ID, Status: enums.StatusOk, ConfigJSON: `{"metaPhoneNumberId":"phone-1","metaAccessTokenSecretRef":"secret://wa-token"}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	identity := models.CustomerIdentity{CustomerID: 1, ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: "8613800000000", Status: enums.StatusOk}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create identity: %v", err)
	}
	conversation := models.Conversation{ChannelID: channel.ID, CustomerID: identity.CustomerID}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{ConversationID: conversation.ID, ClientMsgID: "agent-1", SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "客服回复"}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	outbox := models.ChannelMessageOutbox{ChannelType: enums.ChannelTypeWhatsApp, ConversationID: conversation.ID, MessageID: message.ID, SendStatus: string(enums.ChannelMessageOutboxStatusPending)}
	if err := db.Create(&outbox).Error; err != nil {
		t.Fatalf("create outbox: %v", err)
	}

	if count := MetaWhatsAppOutboundService.DispatchPendingOutbox(); count != 1 {
		t.Fatalf("dispatch count = %d, want 1", count)
	}
	var got models.ChannelMessageOutbox
	if err := db.First(&got, outbox.ID).Error; err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if got.SendStatus != string(enums.ChannelMessageOutboxStatusSent) || got.ExternalMessageID != "wamid.dispatched-1" || got.ProviderStatus != "sent" || got.SentAt == nil {
		t.Fatalf("unexpected dispatched outbox: %+v", got)
	}
}

func TestBuildMetaWhatsAppTextPayloadRejectsMissingFields(t *testing.T) {
	if _, err := buildMetaWhatsAppTextPayload("", "reply"); err == nil {
		t.Fatal("missing recipient was accepted")
	}
	if _, err := buildMetaWhatsAppTextPayload("8613800000000", ""); err == nil {
		t.Fatal("missing body was accepted")
	}
}

func TestMetaWhatsAppOutboundSendsGraphAPIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v25.0/phone-1/messages" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["to"] != "8613800000000" {
			t.Fatalf("recipient = %#v", payload["to"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messaging_product":"whatsapp","contacts":[{"input":"8613800000000","wa_id":"8613800000000"}],"messages":[{"id":"wamid.outbound-1"}]}`))
	}))
	defer server.Close()

	payload, err := buildMetaWhatsAppTextPayload("8613800000000", "客服回复")
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}
	messageID, err := MetaWhatsAppOutboundService.sendMessage(metaWhatsAppRuntimeConfig{
		PhoneNumberID: "phone-1", AccessToken: "access-token", APIBaseURL: server.URL, APIVersion: "v25.0",
	}, payload)
	if err != nil {
		t.Fatalf("sendMessage() error = %v", err)
	}
	if messageID != "wamid.outbound-1" {
		t.Fatalf("message ID = %q, want wamid.outbound-1", messageID)
	}
}
