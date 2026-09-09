package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/eventbus"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupMeteringServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "metering_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ProductAIUsageEvent{}, &models.QuotaLimit{}, &models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatalf("migrate metering models: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		eventbus.WaitAsync[events.QuotaExceededEvent]()
		_ = sqlDB.Close()
	})
	sqls.SetDB(db)
	return db
}

func TestMeteringRecordUsageIsIdempotent(t *testing.T) {
	db := setupMeteringServiceTestDB(t)
	for range 2 {
		if err := MeteringService.RecordUsage(context.Background(), 1, 2, "key-1", "chat_completion", "ai_reply:99", 10, 5, 0); err != nil {
			t.Fatalf("record usage: %v", err)
		}
	}
	var count int64
	if err := db.Model(&models.ProductAIUsageEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count usage: %v", err)
	}
	if count != 1 {
		t.Fatalf("usage count = %d, want 1", count)
	}
}

func TestMeteringQuotaPrefersProductLimitAndUsesIdempotentUsageTable(t *testing.T) {
	db := setupMeteringServiceTestDB(t)
	now := time.Now()
	limits := []models.QuotaLimit{
		{TenantID: 1, ProductID: 0, ResourceType: models.QuotaResourceRequests, LimitValue: 100, Period: models.QuotaPeriodDaily, NotifyThreshold: 0.8, Status: 0},
		{TenantID: 1, ProductID: 2, ResourceType: models.QuotaResourceRequests, LimitValue: 1, Period: models.QuotaPeriodDaily, NotifyThreshold: 0.8, Status: 0},
	}
	if err := db.Create(&limits).Error; err != nil {
		t.Fatalf("create quota limits: %v", err)
	}
	if err := db.Create(&models.ProductAIUsageEvent{
		TenantID: 1, ProductID: 2, UsageType: "chat_completion", RequestID: "ai_reply:100", OccurredAt: now, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create usage event: %v", err)
	}

	remaining, exceeded, limitValue, err := MeteringService.CheckQuotaWithLimit(1, 2, models.QuotaResourceRequests)
	if err != nil {
		t.Fatalf("check product quota: %v", err)
	}
	if !exceeded || remaining != 0 || limitValue != 1 {
		t.Fatalf("product quota = remaining %d exceeded %v limit %d", remaining, exceeded, limitValue)
	}
	if _, _, _, err := MeteringService.CheckQuotaWithLimit(1, 2, models.QuotaResourceRequests); err != nil {
		t.Fatalf("repeat product quota check: %v", err)
	}
	var quotaEventCount int64
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventQuotaExceeded).Count(&quotaEventCount).Error; err != nil {
		t.Fatalf("count quota events: %v", err)
	}
	if quotaEventCount != 1 {
		t.Fatalf("quota threshold events = %d, want one per quota period", quotaEventCount)
	}

	remaining, exceeded, limitValue, err = MeteringService.CheckQuotaWithLimit(1, 3, models.QuotaResourceRequests)
	if err != nil {
		t.Fatalf("check tenant quota: %v", err)
	}
	if exceeded || remaining != 99 || limitValue != 100 {
		t.Fatalf("tenant quota = remaining %d exceeded %v limit %d", remaining, exceeded, limitValue)
	}
}
