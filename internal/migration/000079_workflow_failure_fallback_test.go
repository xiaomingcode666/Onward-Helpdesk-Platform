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

func TestUpgradePlatformDefaultWorkflowAddsPortableFailureFallback(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-failure-fallback.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	workflow := &models.AIWorkflow{
		TenantID:        0,
		Code:            services.PlatformDefaultAfterSalesWorkflowCode,
		Scope:           models.AIWorkflowScopePlatform,
		Name:            "平台默认产品售后客服流程",
		DraftDefinition: `{"schemaVersion":1}`,
		Locked:          true,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	legacy := &models.AIWorkflowVersion{
		WorkflowID: workflow.ID, Version: 1, Definition: workflow.DraftDefinition,
		DefinitionHash: "legacy", ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		PublishedAt: &now, PublishedByName: "system",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create workflow version: %v", err)
	}
	if err := db.Model(workflow).Updates(map[string]any{
		"current_stable_version_id": legacy.ID,
		"published_version_id":      legacy.ID,
	}).Error; err != nil {
		t.Fatalf("bind workflow version: %v", err)
	}

	if err := upgradePlatformDefaultWorkflowFailureFallback(db); err != nil {
		t.Fatalf("upgradePlatformDefaultWorkflowFailureFallback() error = %v", err)
	}
	if err := db.First(workflow, workflow.ID).Error; err != nil {
		t.Fatalf("reload workflow: %v", err)
	}
	var upgraded models.AIWorkflowVersion
	if err := db.First(&upgraded, workflow.CurrentStableVersionID).Error; err != nil {
		t.Fatalf("load upgraded version: %v", err)
	}
	if upgraded.ChangeSummary != "通用咨询接入意图识别、企业知识、自然对话、工单与人工协同" {
		t.Fatalf("unexpected change summary: %q", upgraded.ChangeSummary)
	}
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(upgraded.Definition), &definition); err != nil {
		t.Fatalf("unmarshal upgraded definition: %v", err)
	}
	for _, nodeID := range []string{"retrieve_1", "reply_1"} {
		node, ok := findWorkflowNode(definition, nodeID)
		if !ok || node.ErrorTargetNodeID != "diagnosis_failure_route_1" {
			t.Fatalf("node %s error target = %q, want diagnosis_failure_route_1", nodeID, node.ErrorTargetNodeID)
		}
	}

	var versionCount int64
	if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&versionCount).Error; err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if err := upgradePlatformDefaultWorkflowFailureFallback(db); err != nil {
		t.Fatalf("second upgrade error = %v", err)
	}
	var secondVersionCount int64
	if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&secondVersionCount).Error; err != nil {
		t.Fatalf("count versions after second run: %v", err)
	}
	if secondVersionCount != versionCount {
		t.Fatalf("migration is not idempotent: %d -> %d versions", versionCount, secondVersionCount)
	}
}
