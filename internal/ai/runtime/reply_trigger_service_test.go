package runtime

import (
	"context"
	"path/filepath"
	"testing"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRecordAIReplyUsageUsesResolvedRuntimeKey(t *testing.T) {
	db := setupReplyUsageTestDB(t)
	service := newAIReplyService()
	err := service.recordAIReplyUsage(context.Background(), aiReplyContext{
		Conversation: models.Conversation{TenantID: 7, ProductID: 42},
		Message:      models.Message{ID: 99},
	}, &applicationruntime.Summary{
		PromptTokens:         12,
		CompletionTokens:     4,
		ModelAPIKeyID:        "tenant-default-key",
		ModelCredentialScope: "tenant_default",
	})
	if err != nil {
		t.Fatal(err)
	}
	var event models.ProductAIUsageEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.APIKeyID != "tenant-default-key" || event.ProductID != 42 || event.InputTokens != 12 || event.OutputTokens != 4 {
		t.Fatalf("usage event does not match resolved runtime credential: %#v", event)
	}
}

func TestRecordAIReplyUsageSkipsRunsWithoutModelTokens(t *testing.T) {
	db := setupReplyUsageTestDB(t)
	service := newAIReplyService()
	if err := service.recordAIReplyUsage(context.Background(), aiReplyContext{
		Conversation: models.Conversation{TenantID: 7, ProductID: 42},
		Message:      models.Message{ID: 100},
	}, &applicationruntime.Summary{ModelAPIKeyID: "product-key"}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.ProductAIUsageEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("usage event count = %d, want 0", count)
	}
}

func TestRecordAIReplyUsageSkipsRunsWithoutResolvedKeyID(t *testing.T) {
	db := setupReplyUsageTestDB(t)
	service := newAIReplyService()
	if err := service.recordAIReplyUsage(context.Background(), aiReplyContext{
		Conversation: models.Conversation{TenantID: 7, ProductID: 42},
		Message:      models.Message{ID: 101},
	}, &applicationruntime.Summary{PromptTokens: 12, CompletionTokens: 4}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.ProductAIUsageEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("usage event count = %d, want 0 for an unresolved key", count)
	}
}

func setupReplyUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "reply-usage.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ProductAIUsageEvent{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	return db
}
