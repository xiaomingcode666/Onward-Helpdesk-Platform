package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
)

const runtimeWAConfig = `{"projectKey":"p1","metaPhoneNumberId":"phone-1","metaBusinessAccountId":"waba-1"}`

func writeSecretFileForTest(t *testing.T, tenantID, name, value string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, tenantID, "development")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
}

func TestVerifyTokenUsesDatabaseSecretWithoutSecretDir(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	channel := createChannelSecretTestChannel(t, db, 1, runtimeWAConfig)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{WebhookVerifyToken: "db-verify"})
	if !MetaWhatsAppService.VerifyToken("db-verify") {
		t.Fatal("database verify token rejected")
	}
	if MetaWhatsAppService.VerifyToken("other") {
		t.Fatal("wrong verify token accepted")
	}
}

func TestVerifyTokenPrefersDatabaseOverFileAndFallsBackToFile(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	writeSecretFileForTest(t, "1", "verify", "file-verify")
	channel := createChannelSecretTestChannel(t, db, 1, `{"projectKey":"p1","metaPhoneNumberId":"phone-1","metaBusinessAccountId":"waba-1","webhookVerifyTokenSecretRef":"secret://verify"}`)
	if !MetaWhatsAppService.VerifyToken("file-verify") {
		t.Fatal("file fallback rejected")
	}
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{WebhookVerifyToken: "db-verify"})
	if !MetaWhatsAppService.VerifyToken("db-verify") || MetaWhatsAppService.VerifyToken("file-verify") {
		t.Fatal("database secret must take precedence over file")
	}
}

func TestVerifyTokenMatchesAnyChannelDatabaseSecret(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	first := createChannelSecretTestChannel(t, db, 1, runtimeWAConfig)
	second := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-second", AIAgentID: first.AIAgentID, Name: "WA2", Status: enums.StatusOk, ConfigJSON: runtimeWAConfig}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second: %v", err)
	}
	applyChannelSecretsForTest(t, first, &request.WhatsAppSecretsInput{WebhookVerifyToken: "verify-first"})
	applyChannelSecretsForTest(t, &second, &request.WhatsAppSecretsInput{WebhookVerifyToken: "verify-second"})
	if !MetaWhatsAppService.VerifyToken("verify-first") || !MetaWhatsAppService.VerifyToken("verify-second") {
		t.Fatal("each channel verify token must be accepted")
	}
}

func TestVerifyTokenFailsClosedWhenSecretCannotBeDecrypted(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	channel := createChannelSecretTestChannel(t, db, 1, runtimeWAConfig)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{WebhookVerifyToken: "db-verify"})
	config.SetCurrent(&config.Config{EncryptionKey: "rotated-key-without-fallback-0123456789"})
	if MetaWhatsAppService.VerifyToken("db-verify") {
		t.Fatal("undecryptable secret accepted")
	}
}

func TestProcessWebhookValidatesSignatureWithDatabaseAppSecret(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	channel := createChannelSecretTestChannel(t, db, 1, runtimeWAConfig)
	// Disable the agent: a correct signature then fails on the later agent
	// check, proving the signature step passed without creating conversations.
	if err := db.Model(&models.AIAgent{}).Where("id = ?", channel.AIAgentID).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatalf("disable agent: %v", err)
	}
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAppSecret: "db-app-secret"})
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone-1"},"contacts":[{"profile":{"name":"C"},"wa_id":"8613800000000"}],"messages":[{"from":"8613800000000","id":"wamid.sig-1","type":"text","text":{"body":"hi"}}]}}]}]}`)
	if _, err := MetaWhatsAppService.ProcessWebhook(nil, body, signMetaWhatsAppTestPayload("wrong-secret", body)); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("wrong signature error = %v", err)
	}
	_, err := MetaWhatsAppService.ProcessWebhook(nil, body, signMetaWhatsAppTestPayload("db-app-secret", body))
	if err == nil || !strings.Contains(err.Error(), "AI agent is not enabled") {
		t.Fatalf("valid signature should reach agent check, got %v", err)
	}
}

func TestMetaWhatsAppOutboundUsesDatabaseAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer db-access-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.db-1"}]}`))
	}))
	defer server.Close()
	db := setupChannelSecretTestDB(t, &models.CustomerIdentity{}, &models.Conversation{}, &models.Message{}, &models.ChannelMessageOutbox{})
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	t.Setenv("RHD_META_GRAPH_API_BASE_URL", server.URL)
	t.Setenv("RHD_META_GRAPH_API_VERSION", "v25.0")
	channel := createChannelSecretTestChannel(t, db, 0, runtimeWAConfig)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAccessToken: "db-access-token"})
	identity := models.CustomerIdentity{CustomerID: 1, ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: "8613800000000", Status: enums.StatusOk}
	conversation := models.Conversation{ChannelID: channel.ID, CustomerID: 1}
	for _, row := range []any{&identity, &conversation} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	message := models.Message{ConversationID: conversation.ID, ClientMsgID: "agent-db-1", SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "reply"}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	outbox := models.ChannelMessageOutbox{ChannelType: enums.ChannelTypeWhatsApp, ConversationID: conversation.ID, MessageID: message.ID, SendStatus: string(enums.ChannelMessageOutboxStatusPending)}
	if err := db.Create(&outbox).Error; err != nil {
		t.Fatalf("create outbox: %v", err)
	}
	if count := MetaWhatsAppOutboundService.DispatchPendingOutbox(); count != 1 {
		t.Fatalf("dispatch count = %d", count)
	}
	var got models.ChannelMessageOutbox
	if err := db.First(&got, outbox.ID).Error; err != nil || got.ExternalMessageID != "wamid.db-1" {
		t.Fatalf("outbox = %+v err=%v", got, err)
	}
}
