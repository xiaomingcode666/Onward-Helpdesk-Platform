package dashboard

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestBuildChannelResponseExposesOnlySecretStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "handler.db")), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.AIAgent{}, &models.Channel{}, &models.ChannelSecret{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldCfg := sqls.DB(), config.CurrentOrDefault()
	sqls.SetDB(db)
	config.SetCurrent(&config.Config{EncryptionKey: "handler-secret-test-key-0123456789abcd"})
	t.Cleanup(func() {
		config.SetCurrent(&oldCfg)
		sqls.SetDB(oldDB)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	agent := models.AIAgent{Name: "a", Status: enums.StatusOk, WorkflowVersionID: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "wa-h", AIAgentID: agent.ID, Name: "WA", Status: enums.StatusOk, ConfigJSON: `{}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	changes, err := services.NormalizeWhatsAppSecretsInput(&request.WhatsAppSecretsInput{MetaAccessToken: "very-secret-token"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		return services.ChannelSecretService.ApplyTx(txCtx, &channel, changes, &dto.AuthPrincipal{UserID: 1, Username: "admin"})
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	raw, err := json.Marshal(buildChannelResponse(&channel))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "very-secret-token") || strings.Contains(body, "enc:v1:") {
		t.Fatalf("response leaked secret: %s", body)
	}
	if !strings.Contains(body, `"metaAccessToken":{"configured":true,"fingerprintSuffix":"`) || !strings.Contains(body, `"metaAppSecret":{"configured":false}`) {
		t.Fatalf("missing status: %s", body)
	}

	web := models.Channel{ChannelType: enums.ChannelTypeWeb, ChannelID: "web-h", AIAgentID: agent.ID, Name: "Web", Status: enums.StatusOk}
	if err := db.Create(&web).Error; err != nil {
		t.Fatalf("create web channel: %v", err)
	}
	webRaw, err := json.Marshal(buildChannelResponse(&web))
	if err != nil {
		t.Fatalf("marshal web: %v", err)
	}
	if strings.Contains(string(webRaw), "whatsappSecrets") {
		t.Fatalf("non-WhatsApp channel exposes whatsappSecrets: %s", webRaw)
	}
}
