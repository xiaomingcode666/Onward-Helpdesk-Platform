package migration

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestHardenPlatformWorkflowFailureRecoveryCreatesImmutableVersions(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-failure-recovery.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	tests := []struct {
		code       string
		name       string
		definition dsl.Definition
		targets    map[string]string
	}{
		{
			code:       services.PlatformDefaultAfterSalesWorkflowCode,
			name:       "legacy collaboration workflow",
			definition: services.AIWorkflowService.DefaultAgentWorkflowDefinition(),
			targets: map[string]string{
				"entry_context_1":  "quick_fallback_reply_1",
				"service_access_1": "quick_fallback_reply_1",
				"quick_reply_1":    "quick_fallback_reply_1",
				"understanding_1":  "diagnosis_failure_route_1",
				"policy_1":         "diagnosis_failure_route_1",
				"answerability_1":  "diagnosis_failure_route_1",
				"handoff_1":        "service_failure_reply_1",
				"draft_ticket_1":   "service_failure_reply_1",
				"create_ticket_1":  "service_failure_reply_1",
			},
		},
		{
			code:       services.PlatformDeviceAIOnlyWorkflowCode,
			name:       "legacy device AI workflow",
			definition: services.AIWorkflowService.DeviceAIOnlyWorkflowDefinition(),
			targets: map[string]string{
				"entry_context_1": "quick_safe_reply_1",
				"quick_reply_1":   "quick_safe_reply_1",
				"answerability_1": "diagnostic_safe_reply_1",
			},
		},
	}

	legacyVersionIDs := make(map[string]int64, len(tests))
	now := time.Now()
	for _, test := range tests {
		legacyDefinition := test.definition
		for index := range legacyDefinition.Nodes {
			legacyDefinition.Nodes[index].ErrorTargetNodeID = ""
		}
		definitionJSON, err := json.Marshal(legacyDefinition)
		if err != nil {
			t.Fatalf("marshal %s legacy definition: %v", test.code, err)
		}
		workflow := &models.AIWorkflow{
			Code:            test.code,
			Scope:           models.AIWorkflowScopePlatform,
			Name:            test.name,
			DraftDefinition: string(definitionJSON),
			Locked:          true,
			AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(workflow).Error; err != nil {
			t.Fatalf("create %s legacy workflow: %v", test.code, err)
		}
		version := &models.AIWorkflowVersion{
			WorkflowID:      workflow.ID,
			Version:         1,
			Definition:      string(definitionJSON),
			DefinitionHash:  "legacy-" + test.code,
			ReleaseChannel:  models.AIWorkflowReleaseChannelStable,
			PublishedAt:     &now,
			PublishedByName: "system",
			AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(version).Error; err != nil {
			t.Fatalf("create %s legacy version: %v", test.code, err)
		}
		if err := db.Model(workflow).Updates(map[string]any{
			"current_stable_version_id": version.ID,
			"published_version_id":      version.ID,
		}).Error; err != nil {
			t.Fatalf("bind %s legacy stable version: %v", test.code, err)
		}
		legacyVersionIDs[test.code] = version.ID
	}

	if err := hardenPlatformWorkflowFailureRecovery(db); err != nil {
		t.Fatalf("hardenPlatformWorkflowFailureRecovery() error = %v", err)
	}
	for _, test := range tests {
		workflow := repositoriesWorkflowByCodeForTest(t, db, test.code)
		if workflow.CurrentStableVersionID == legacyVersionIDs[test.code] {
			t.Fatalf("%s stable version was not upgraded", test.code)
		}
		var version models.AIWorkflowVersion
		if err := db.First(&version, workflow.CurrentStableVersionID).Error; err != nil {
			t.Fatalf("load %s upgraded version: %v", test.code, err)
		}
		var definition dsl.Definition
		if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
			t.Fatalf("unmarshal %s upgraded definition: %v", test.code, err)
		}
		for nodeID, targetID := range test.targets {
			node, ok := findWorkflowNode(definition, nodeID)
			if !ok || node.ErrorTargetNodeID != targetID {
				t.Fatalf("%s node %s failure target = %q, want %q", test.code, nodeID, node.ErrorTargetNodeID, targetID)
			}
		}
		var versionCount int64
		if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&versionCount).Error; err != nil {
			t.Fatalf("count %s versions: %v", test.code, err)
		}
		if versionCount != 2 {
			t.Fatalf("%s version count = %d, want 2", test.code, versionCount)
		}
	}

	if err := hardenPlatformWorkflowFailureRecovery(db); err != nil {
		t.Fatalf("second hardenPlatformWorkflowFailureRecovery() error = %v", err)
	}
	for _, test := range tests {
		workflow := repositoriesWorkflowByCodeForTest(t, db, test.code)
		var versionCount int64
		if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&versionCount).Error; err != nil {
			t.Fatalf("count %s versions after second run: %v", test.code, err)
		}
		if versionCount != 2 {
			t.Fatalf("%s migration is not idempotent: got %d versions", test.code, versionCount)
		}
	}
}
