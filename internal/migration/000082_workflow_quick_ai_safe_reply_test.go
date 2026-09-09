package migration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestUpgradePlatformDefaultWorkflowMakesQuickAIFallbackDeterministic(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-quick-ai-safe-reply.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := upgradePlatformDefaultWorkflowQuickAISafeReply(db); err != nil {
		t.Fatalf("upgradePlatformDefaultWorkflowQuickAISafeReply() error = %v", err)
	}
	workflow := repositoriesWorkflowByCodeForTest(t, db, services.PlatformDefaultAfterSalesWorkflowCode)
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(workflow.DraftDefinition), &definition); err != nil {
		t.Fatalf("unmarshal workflow definition: %v", err)
	}
	fallback, ok := findWorkflowNode(definition, "quick_fallback_reply_1")
	if !ok {
		t.Fatal("upgraded workflow does not contain the quick AI fallback reply")
	}
	var config struct {
		StaticReply string `json:"staticReply"`
	}
	if err := json.Unmarshal(fallback.Config, &config); err != nil {
		t.Fatalf("unmarshal fallback reply config: %v", err)
	}
	if !strings.Contains(config.StaticReply, "人工支持") || !strings.Contains(config.StaticReply, "创建工单") || strings.Contains(config.StaticReply, "不会创建工单或转接人工") {
		t.Fatalf("fallback reply does not preserve the assisted-service options: %q", config.StaticReply)
	}
}
