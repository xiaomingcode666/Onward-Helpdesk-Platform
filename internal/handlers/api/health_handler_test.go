package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/web"
	"gorm.io/gorm"
)

func TestWriteReadinessResponseUsesServiceUnavailableForFailedDependency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeReadinessResponse(ctx, map[string]checkInfo{
		"postgres": {Status: "ok"},
		"redis":    {Status: "error", Detail: "connection refused"},
	})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var result web.JsonResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["status"] != "unhealthy" {
		t.Fatalf("unexpected readiness payload: %+v", result.Data)
	}
}

func TestWriteReadinessResponseUsesOKWhenAllDependenciesPass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeReadinessResponse(ctx, map[string]checkInfo{
		"postgres": {Status: "ok"},
		"redis":    {Status: "ok"},
		"qdrant":   {Status: "ok"},
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestCheckOutboxReportsDeadLettersAndStaleDeliveries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatalf("migrate outbox: %v", err)
	}
	now := time.Now()
	if check := checkOutboxDB(db, now); check.Status != "ok" {
		t.Fatalf("empty outbox check = %+v, want ok", check)
	}

	dead := models.OutboxRecord{EventID: 1, EventType: "test.dead", Status: models.OutboxStatusDead, RetryCount: 3, MaxRetries: 3, CreatedAt: now}
	if err := db.Create(&dead).Error; err != nil {
		t.Fatalf("create dead letter: %v", err)
	}
	if check := checkOutboxDB(db, now); check.Status != "error" || check.Detail != "dead_letter_count=1" {
		t.Fatalf("dead letter check = %+v", check)
	}
	if err := db.Delete(&dead).Error; err != nil {
		t.Fatalf("delete dead letter: %v", err)
	}

	stale := models.OutboxRecord{EventID: 2, EventType: "test.stale", Status: models.OutboxStatusPending, MaxRetries: 3, CreatedAt: now.Add(-10 * time.Minute)}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("create stale delivery: %v", err)
	}
	if check := checkOutboxDB(db, now); check.Status != "error" || check.Detail != "stale_delivery_count=1" {
		t.Fatalf("stale delivery check = %+v", check)
	}
}

func TestCheckStorageUsesConfiguredLocalProvider(t *testing.T) {
	root := t.TempDir()
	previous := config.Current()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local:   config.LocalStorageConfig{Root: root},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })

	if check := checkStorage(); check.Status != "ok" {
		t.Fatalf("storage check = %+v, want ok", check)
	}
}
