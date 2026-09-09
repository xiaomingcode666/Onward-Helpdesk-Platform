package third

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/providers"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestJitsiWebhookRejectsReplayAndPersistsIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jitsi-handler?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.WebhookEventInbox{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	const secret = "jitsi-webhook-test-secret"
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{WebhookSecret: secret})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().Unix()
	staleBody := []byte(fmt.Sprintf(`{"event_id":"stale-1","action":"unknown","timestamp":%d}`, now-jitsiWebhookMaxAge-1))
	if status := performSignedJitsiWebhook(staleBody, secret); status != http.StatusUnauthorized {
		t.Fatalf("stale callback status = %d, want 401", status)
	}
	missingTimestamp := []byte(`{"event_id":"missing-time","action":"unknown"}`)
	if status := performSignedJitsiWebhook(missingTimestamp, secret); status != http.StatusUnauthorized {
		t.Fatalf("missing timestamp status = %d, want 401", status)
	}
	freshBody := []byte(fmt.Sprintf(`{"event_id":"fresh-1","action":"unknown","timestamp":%d}`, now))
	if status := performSignedJitsiWebhook(freshBody, "wrong-secret"); status != http.StatusUnauthorized {
		t.Fatalf("invalid signature status = %d, want 401", status)
	}
	if status := performSignedJitsiWebhook(freshBody, secret); status != http.StatusOK {
		t.Fatalf("fresh callback status = %d, want 200", status)
	}
	if status := performSignedJitsiWebhook(freshBody, secret); status != http.StatusOK {
		t.Fatalf("duplicate callback status = %d, want 200", status)
	}
	var count int64
	if err := db.Model(&models.WebhookEventInbox{}).Where("provider = ? AND event_id = ?", "jitsi", "fresh-1").Count(&count).Error; err != nil {
		t.Fatalf("count inbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("inbox count = %d, want 1", count)
	}
}

func performSignedJitsiWebhook(body []byte, secret string) int {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/third/jitsi/webhook", bytes.NewReader(body))
	ctx.Request.Header.Set("X-Jitsi-Webhook-Signature", signature)
	JitsiPostWebhook(ctx)
	return recorder.Code
}
