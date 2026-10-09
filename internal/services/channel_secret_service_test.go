package services

import (
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const channelSecretTestKey = "channel-secret-test-key-0123456789abcdef"

func setupChannelSecretTestDB(t *testing.T, extra ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channel-secret.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	tables := append([]any{&models.AIAgent{}, &models.Channel{}, &models.ChannelSecret{}, &models.AuditLog{}, &models.DomainEvent{}, &models.OutboxRecord{}}, extra...)
	if err := db.AutoMigrate(tables...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldCfg := sqls.DB(), config.CurrentOrDefault()
	sqls.SetDB(db)
	config.SetCurrent(&config.Config{EncryptionKey: channelSecretTestKey})
	t.Cleanup(func() {
		config.SetCurrent(&oldCfg)
		sqls.SetDB(oldDB)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createChannelSecretTestChannel(t *testing.T, db *gorm.DB, tenantID int64, configJSON string) *models.Channel {
	t.Helper()
	agent := models.AIAgent{TenantID: tenantID, Name: "secret agent", Status: enums.StatusOk, WorkflowVersionID: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-" + t.Name(), AIAgentID: agent.ID, Name: "WA", Status: enums.StatusOk, ConfigJSON: configJSON}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return &channel
}

func channelSecretTestOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: 7, Username: "secret-admin", DomainType: "enterprise", TenantID: 5}
}

func applyChannelSecretsForTest(t *testing.T, channel *models.Channel, in *request.WhatsAppSecretsInput) {
	t.Helper()
	changes, err := NormalizeWhatsAppSecretsInput(in)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		return ChannelSecretService.ApplyTx(txCtx, channel, changes, channelSecretTestOperator())
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func TestNormalizeWhatsAppSecretsInput(t *testing.T) {
	changes, err := NormalizeWhatsAppSecretsInput(&request.WhatsAppSecretsInput{MetaAccessToken: "  EAAG-token\n", Clear: []string{"metaAppSecret"}})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if changes.Set[ChannelSecretMetaAccessToken] != "EAAG-token" || !changes.Clear[ChannelSecretMetaAppSecret] || len(changes.Set) != 1 {
		t.Fatalf("unexpected changes: %+v", changes)
	}
	empty, err := NormalizeWhatsAppSecretsInput(nil)
	if err != nil || !empty.Empty() {
		t.Fatalf("nil input should be empty: %+v %v", empty, err)
	}
	for name, in := range map[string]*request.WhatsAppSecretsInput{
		"inner whitespace": {MetaAppSecret: "abc def"},
		"secret reference": {WebhookVerifyToken: "Secret://meta-whatsapp-verify-token"},
		"too long":         {MetaAccessToken: strings.Repeat("a", 4097)},
		"unknown clear":    {Clear: []string{"metaPassword"}},
		"set and clear":    {MetaAppSecret: "abc", Clear: []string{"metaAppSecret"}},
	} {
		if _, err := NormalizeWhatsAppSecretsInput(in); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestChannelSecretServiceStoresEncryptedAndResolves(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	channel := createChannelSecretTestChannel(t, db, 3, `{}`)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAccessToken: "plain-access-token", MetaAppSecret: "plain-app-secret"})

	var stored []models.ChannelSecret
	if err := db.Find(&stored).Error; err != nil || len(stored) != 2 {
		t.Fatalf("stored rows = %d err=%v", len(stored), err)
	}
	for _, row := range stored {
		if !strings.HasPrefix(row.Ciphertext, "enc:v1:") || strings.Contains(row.Ciphertext, "plain-") || len(row.Fingerprint) != 64 {
			t.Fatalf("row not encrypted: %+v", row)
		}
	}
	got, err := ChannelSecretService.Resolve(channel, ChannelSecretMetaAccessToken, "")
	if err != nil || got != "plain-access-token" {
		t.Fatalf("Resolve() = %q, %v", got, err)
	}
	if _, err := ChannelSecretService.Resolve(channel, ChannelSecretWebhookVerifyToken, ""); err == nil {
		t.Fatal("unconfigured secret without reference resolved")
	}
	if !ChannelSecretService.Available(channel, ChannelSecretMetaAppSecret, "") || ChannelSecretService.Available(channel, ChannelSecretWebhookVerifyToken, "") {
		t.Fatal("Available() mismatch")
	}
	if !ChannelSecretService.Available(channel, ChannelSecretWebhookVerifyToken, "secret://verify") {
		t.Fatal("reference should make secret available")
	}
}

func TestChannelSecretServiceKeepOverwriteClearAndStatus(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	channel := createChannelSecretTestChannel(t, db, 3, `{}`)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAccessToken: "first-token", WebhookVerifyToken: "verify-1"})
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{}) // keep
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAccessToken: "second-token", Clear: []string{"webhookVerifyToken"}})

	if got, _ := ChannelSecretService.Resolve(channel, ChannelSecretMetaAccessToken, ""); got != "second-token" {
		t.Fatalf("overwrite failed: %q", got)
	}
	statuses := ChannelSecretService.Statuses(channel.ID)
	if !statuses["metaAccessToken"].Configured || len(statuses["metaAccessToken"].FingerprintSuffix) != 4 || statuses["metaAccessToken"].UpdatedAt == "" {
		t.Fatalf("access token status = %+v", statuses["metaAccessToken"])
	}
	if statuses["webhookVerifyToken"].Configured || statuses["metaAppSecret"].Configured {
		t.Fatalf("cleared/unset statuses = %+v", statuses)
	}
	var audits []models.AuditLog
	if err := db.Where("resource_type = ?", "channel_secret").Order("id").Find(&audits).Error; err != nil {
		t.Fatalf("load audits: %v", err)
	}
	// set access, set verify, set access again, clear verify
	if len(audits) != 4 || audits[3].Action != "cleared" || audits[0].TenantID != 3 {
		t.Fatalf("audits = %+v", audits)
	}
	for _, a := range audits {
		for _, leaked := range []string{"first-token", "second-token", "verify-1", "enc:v1:"} {
			if strings.Contains(a.AfterState, leaked) || strings.Contains(a.BeforeState, leaked) {
				t.Fatalf("audit leaked %q: %s", leaked, a.AfterState)
			}
		}
	}
}

func TestChannelSecretServiceDeleteAll(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	channel := createChannelSecretTestChannel(t, db, 3, `{}`)
	applyChannelSecretsForTest(t, channel, &request.WhatsAppSecretsInput{MetaAccessToken: "token"})
	if err := ChannelSecretService.DeleteAllTx(db, channel.ID); err != nil {
		t.Fatalf("DeleteAllTx: %v", err)
	}
	if len(ChannelSecretService.ConfiguredNames(channel.ID)) != 0 {
		t.Fatal("secrets remain after DeleteAllTx")
	}
}
