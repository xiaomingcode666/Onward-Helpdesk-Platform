package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMetaWhatsAppServiceDownloadsScansAndStoresInboundImageOnce(t *testing.T) {
	imageData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode test image: %v", err)
	}
	imageHash := sha256.Sum256(imageData)
	var downloads atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v25.0/media-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"url":%q,"mime_type":"image/png","sha256":%q,"file_size":%d}`, server.URL+"/media-download", hex.EncodeToString(imageHash[:]), len(imageData))
		case "/media-download":
			downloads.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "meta-media.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Channel{}, &models.AIAgent{}, &models.WebhookEventInbox{}, &models.Customer{}, &models.CustomerIdentity{},
		&models.Conversation{}, &models.ConversationParticipant{}, &models.ConversationEventLog{}, &models.Message{}, &models.ConversationReadState{},
		&models.ChannelMessageOutbox{}, &models.Asset{}, &models.AssetScanAttempt{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldCfg := sqls.DB(), config.CurrentOrDefault()
	sqls.SetDB(db)
	storageRoot := t.TempDir()
	var clamReply atomic.Value
	clamReply.Store("stream: OK\x00")
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal, MaxUploadSizeMB: 2, MaxReceiveSizeMB: 2,
		Local:          config.LocalStorageConfig{Root: filepath.Join(storageRoot, "public"), BaseURL: "/storage"},
		QuarantineRoot: filepath.Join(storageRoot, "quarantine"),
		UploadSecurity: config.UploadSecurityConfig{ClamAV: config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &clamReply)}},
	}})
	previousEnqueue, previousTrigger := EnqueueAIReplyJobHook, TriggerAIReplyAsyncHook
	EnqueueAIReplyJobHook, TriggerAIReplyAsyncHook = nil, nil
	t.Cleanup(func() {
		EnqueueAIReplyJobHook, TriggerAIReplyAsyncHook = previousEnqueue, previousTrigger
		config.SetCurrent(&oldCfg)
		sqls.SetDB(oldDB)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	secretRoot := t.TempDir()
	secretDir := filepath.Join(secretRoot, "0", "development")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		t.Fatalf("create secret dir: %v", err)
	}
	for name, value := range map[string]string{"meta-app": "app-secret", "wa-token": "access-token"} {
		if err := os.WriteFile(filepath.Join(secretDir, name), []byte(value), 0o600); err != nil {
			t.Fatalf("write secret %s: %v", name, err)
		}
	}
	t.Setenv("RHD_PROJECT_SECRET_DIR", secretRoot)
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_META_GRAPH_API_BASE_URL", server.URL)
	t.Setenv("RHD_META_GRAPH_API_VERSION", "v25.0")

	agent := models.AIAgent{TenantID: 0, Name: "Meta media agent", Status: enums.StatusOk, ServiceMode: enums.IMConversationServiceModeAIOnly}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "meta-media-channel", AIAgentID: agent.ID, Status: enums.StatusOk,
		ConfigJSON: `{"metaPhoneNumberId":"phone-1","metaBusinessAccountId":"waba-1","metaAccessTokenSecretRef":"secret://wa-token","metaAppSecretSecretRef":"secret://meta-app"}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	body := []byte(fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone-1"},"contacts":[{"profile":{"name":"Media Customer"},"wa_id":"8613800000000"}],"messages":[{"from":"8613800000000","id":"wamid.image-1","type":"image","image":{"id":"media-1","mime_type":"image/png","sha256":%q,"caption":"damage"}}]}}]}]}`, hex.EncodeToString(imageHash[:])))
	signature := signMetaWhatsAppTestPayload("app-secret", body)
	first, err := MetaWhatsAppService.ProcessWebhook(nil, body, signature)
	if err != nil {
		var failed models.Asset
		_ = db.Order("id DESC").First(&failed).Error
		t.Fatalf("process image webhook: %v (asset status=%s reason=%s mime=%s size=%d)", err, failed.ScanStatus, failed.ScanReason, failed.MimeType, failed.FileSize)
	}
	second, err := MetaWhatsAppService.ProcessWebhook(nil, body, signature)
	if err != nil {
		t.Fatalf("process duplicate image webhook: %v", err)
	}
	if first.ProcessedMessages != 1 || second.DuplicateMessages != 1 || downloads.Load() != 1 {
		t.Fatalf("results first=%+v second=%+v downloads=%d", first, second, downloads.Load())
	}
	var message models.Message
	if err := db.Where("client_msg_id = ?", "meta-whatsapp:wamid.image-1").First(&message).Error; err != nil {
		t.Fatalf("load media message: %v", err)
	}
	if message.MessageType != enums.IMMessageTypeImage {
		t.Fatalf("message type = %q", message.MessageType)
	}
	payload, err := parseIMMessageAssetPayload(message.Payload)
	if err != nil {
		t.Fatalf("parse asset payload: %v", err)
	}
	asset := AssetService.GetByAssetID(payload.AssetID)
	if asset == nil || asset.TenantID != 0 || asset.ConversationID != message.ConversationID || asset.MimeType != "image/png" || asset.SHA256 != hex.EncodeToString(imageHash[:]) || !asset.Usable() {
		t.Fatalf("unexpected inbound asset: %+v", asset)
	}
}

func TestMetaWhatsAppInboundMessageTypeMapsMedia(t *testing.T) {
	wants := map[string]enums.IMMessageType{
		"text": enums.IMMessageTypeText, "image": enums.IMMessageTypeImage, "audio": enums.IMMessageTypeAudio,
		"document": enums.IMMessageTypeAttachment, "video": enums.IMMessageTypeAttachment,
	}
	for input, want := range wants {
		got, err := metaWhatsAppInboundMessageType(input)
		if err != nil || got != want {
			t.Fatalf("type %q = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := metaWhatsAppInboundMessageType("location"); err == nil {
		t.Fatal("unsupported media type was accepted")
	}
}

func TestMetaWhatsAppServiceVerifiesTokenAndProcessesSignedMessageIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:meta-whatsapp-process?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Channel{}, &models.AIAgent{}, &models.WebhookEventInbox{}, &models.Customer{}, &models.CustomerIdentity{},
		&models.Conversation{}, &models.ConversationParticipant{}, &models.ConversationEventLog{}, &models.Message{}, &models.ConversationReadState{},
		&models.ChannelMessageOutbox{},
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

	root := t.TempDir()
	secretDir := filepath.Join(root, "0", "development")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		t.Fatalf("create secret dir: %v", err)
	}
	for name, value := range map[string]string{"meta-app": "app-secret", "verify": "verify-token", "wa-token": "access-token"} {
		if err := os.WriteFile(filepath.Join(secretDir, name), []byte(value), 0o600); err != nil {
			t.Fatalf("write secret %s: %v", name, err)
		}
	}
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")

	agent := models.AIAgent{TenantID: 0, Name: "Meta WhatsApp Agent", Status: enums.StatusOk, ServiceMode: enums.IMConversationServiceModeAIOnly}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{
		ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "meta-channel", AIAgentID: agent.ID, Status: enums.StatusOk,
		ConfigJSON: `{"projectKey":"support","metaPhoneNumberId":"phone-1","metaBusinessAccountId":"waba-1","metaAccessTokenSecretRef":"secret://wa-token","metaAppSecretSecretRef":"secret://meta-app","webhookVerifyTokenSecretRef":"secret://verify"}`,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}

	if !MetaWhatsAppService.VerifyToken("verify-token") {
		t.Fatal("configured webhook verify token was rejected")
	}
	if MetaWhatsAppService.VerifyToken("wrong-token") {
		t.Fatal("wrong webhook verify token was accepted")
	}

	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone-1"},"contacts":[{"profile":{"name":"Test Customer"},"wa_id":"8613800000000"}],"messages":[{"from":"8613800000000","id":"wamid.inbound-1","type":"text","text":{"body":"hello helpdesk"}}]}}]}]}`)
	signature := signMetaWhatsAppTestPayload("app-secret", body)
	first, err := MetaWhatsAppService.ProcessWebhook(nil, body, signature)
	if err != nil {
		t.Fatalf("process first webhook: %v", err)
	}
	if first.ProcessedMessages != 1 || first.DuplicateMessages != 0 {
		t.Fatalf("first result = %+v", first)
	}
	second, err := MetaWhatsAppService.ProcessWebhook(nil, body, signature)
	if err != nil {
		t.Fatalf("process duplicate webhook: %v", err)
	}
	if second.ProcessedMessages != 0 || second.DuplicateMessages != 1 {
		t.Fatalf("duplicate result = %+v", second)
	}
	var messages int64
	if err := db.Model(&models.Message{}).Where("client_msg_id = ?", "meta-whatsapp:wamid.inbound-1").Count(&messages).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messages != 1 {
		t.Fatalf("message count = %d, want 1", messages)
	}
	if _, err := MetaWhatsAppService.ProcessWebhook(nil, body, "sha256=bad"); err == nil {
		t.Fatal("invalid Meta signature was accepted")
	}
}

func signMetaWhatsAppTestPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
