package migration

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestUpgradePlatformDefaultWorkflowAddsQuickAIFallback(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-quick-ai-fallback.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := upgradePlatformDefaultWorkflowQuickAIFallback(db); err != nil {
		t.Fatalf("upgradePlatformDefaultWorkflowQuickAIFallback() error = %v", err)
	}
	workflow := repositoriesWorkflowByCodeForTest(t, db, services.PlatformDefaultAfterSalesWorkflowCode)
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(workflow.DraftDefinition), &definition); err != nil {
		t.Fatalf("unmarshal workflow definition: %v", err)
	}
	retrieve, ok := findWorkflowNode(definition, "quick_retrieve_1")
	if !ok || retrieve.Type != workflowregistry.NodeTypeKnowledgeRetrieve {
		t.Fatal("upgraded workflow does not contain the quick knowledge retrieve node")
	}
	if retrieve.ErrorTargetNodeID != "quick_fallback_reply_1" {
		t.Fatalf("quick retrieve error target = %q", retrieve.ErrorTargetNodeID)
	}
	fallback, ok := findWorkflowNode(definition, retrieve.ErrorTargetNodeID)
	if !ok || fallback.Type != workflowregistry.NodeTypeLLMReply {
		t.Fatal("quick knowledge failure does not route to a portable AI fallback node")
	}
}
