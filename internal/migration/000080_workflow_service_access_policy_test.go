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

func TestUpgradePlatformDefaultWorkflowAddsServiceAccessPolicy(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-service-access.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := upgradePlatformDefaultWorkflowServiceAccessPolicy(db); err != nil {
		t.Fatalf("upgradePlatformDefaultWorkflowServiceAccessPolicy() error = %v", err)
	}
	workflow := repositoriesWorkflowByCodeForTest(t, db, services.PlatformDefaultAfterSalesWorkflowCode)
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(workflow.DraftDefinition), &definition); err != nil {
		t.Fatalf("unmarshal workflow definition: %v", err)
	}
	for _, nodeType := range []string{workflowregistry.NodeTypeEntryContext, workflowregistry.NodeTypeServiceAccessPolicy} {
		if !definition.HasNodeType(nodeType) {
			t.Fatalf("upgraded workflow does not contain %s", nodeType)
		}
	}
	route, ok := findWorkflowNode(definition, "service_access_route_1")
	if !ok {
		t.Fatal("upgraded workflow does not contain the service access route")
	}
	var config dsl.ConditionConfig
	if err := json.Unmarshal(route.Config, &config); err != nil {
		t.Fatalf("unmarshal service access route: %v", err)
	}
	targets := make(map[string]bool)
	for _, branch := range config.Branches {
		targets[branch.TargetNodeID] = true
	}
	if !targets["understanding_1"] || !targets["handoff_1"] {
		t.Fatalf("service access route does not expose intent-aware AI service and human-only paths: %#v", targets)
	}
}

func repositoriesWorkflowByCodeForTest(t *testing.T, db *gorm.DB, code string) *models.AIWorkflow {
	t.Helper()
	var workflow models.AIWorkflow
	if err := db.Where("code = ?", code).First(&workflow).Error; err != nil {
		t.Fatalf("load workflow by code: %v", err)
	}
	return &workflow
}
