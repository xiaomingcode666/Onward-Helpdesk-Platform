package migration

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLocalizeDefaultProductAgentPromptsPreservesCustomPrompts(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.AIAgent{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	product := &models.Product{
		TenantID: 17,
		Code:     "PUMP-01",
		Name:     "测试泵站",
		Status:   enums.StatusOk,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	untouched := &models.AIAgent{
		TenantID:     product.TenantID,
		ProductID:    product.ID,
		Name:         "默认机器人",
		SystemPrompt: legacyDefaultProductAgentSystemPrompt(product),
		Status:       enums.StatusOk,
	}
	custom := &models.AIAgent{
		TenantID:     product.TenantID,
		ProductID:    product.ID,
		Name:         "自定义机器人",
		SystemPrompt: "保留企业自定义提示词",
		Status:       enums.StatusOk,
	}
	if err := db.Create(untouched).Error; err != nil {
		t.Fatalf("create untouched agent: %v", err)
	}
	if err := db.Create(custom).Error; err != nil {
		t.Fatalf("create custom agent: %v", err)
	}

	if err := localizeDefaultProductAgentPrompts(db); err != nil {
		t.Fatalf("localizeDefaultProductAgentPrompts() error = %v", err)
	}
	if err := db.First(untouched, untouched.ID).Error; err != nil {
		t.Fatalf("reload untouched agent: %v", err)
	}
	if untouched.SystemPrompt != localizedDefaultProductAgentSystemPrompt(product) {
		t.Fatalf("untouched prompt was not localized: %q", untouched.SystemPrompt)
	}
	if err := db.First(custom, custom.ID).Error; err != nil {
		t.Fatalf("reload custom agent: %v", err)
	}
	if custom.SystemPrompt != "保留企业自定义提示词" {
		t.Fatalf("custom prompt changed: %q", custom.SystemPrompt)
	}
}
