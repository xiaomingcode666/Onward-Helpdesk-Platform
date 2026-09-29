package third

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMetaWhatsAppGetWebhookReturnsChallengeForConfiguredToken(t *testing.T) {
	setupMetaWhatsAppHandlerConfig(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/third/meta/whatsapp/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=challenge-1", nil)

	MetaWhatsAppGetWebhook(ctx)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "challenge-1" {
		t.Fatalf("verification response = %d %q, want 200 challenge-1", recorder.Code, recorder.Body.String())
	}
}

func TestMetaWhatsAppPostWebhookRejectsMissingSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/third/meta/whatsapp/webhook", strings.NewReader(`{"object":"whatsapp_business_account"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	MetaWhatsAppPostWebhook(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature status = %d, want 401", recorder.Code)
	}
}

func setupMetaWhatsAppHandlerConfig(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Channel{}, &models.AIAgent{}); err != nil {
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
	if err := os.WriteFile(filepath.Join(secretDir, "verify"), []byte("verify-token"), 0o600); err != nil {
		t.Fatalf("write verify secret: %v", err)
	}
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	agent := models.AIAgent{Name: "Meta agent", Status: enums.StatusOk}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWhatsApp, ChannelID: "meta-channel", AIAgentID: agent.ID, Status: enums.StatusDisabled, ConfigJSON: `{"webhookVerifyTokenSecretRef":"secret://verify"}`}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
}
