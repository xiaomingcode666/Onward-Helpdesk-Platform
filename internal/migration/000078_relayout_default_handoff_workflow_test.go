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

func TestRelayoutPlatformDefaultHandoffWorkflowKeepsAIFallbackPathForward(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "default-handoff-layout.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	legacyDefinition := `{"schemaVersion":1}`
	workflow := &models.AIWorkflow{
		TenantID:        0,
		Code:            services.PlatformDefaultAfterSalesWorkflowCode,
		Scope:           models.AIWorkflowScopePlatform,
		Name:            "平台默认产品售后客服流程",
		DraftDefinition: legacyDefinition,
		Locked:          true,
		Status:          0,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatalf("create legacy workflow: %v", err)
	}
	legacyVersion := &models.AIWorkflowVersion{
		WorkflowID:      workflow.ID,
		Version:         1,
		Status:          0,
		Definition:      legacyDefinition,
		DefinitionHash:  "legacy-system-definition",
		ReleaseChannel:  models.AIWorkflowReleaseChannelStable,
		PublishedAt:     &now,
		PublishedByName: "system",
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(legacyVersion).Error; err != nil {
		t.Fatalf("create legacy workflow version: %v", err)
	}
	if err := db.Model(workflow).Updates(map[string]any{
		"current_stable_version_id": legacyVersion.ID,
		"published_version_id":      legacyVersion.ID,
	}).Error; err != nil {
		t.Fatalf("bind legacy stable version: %v", err)
	}

	if err := relayoutPlatformDefaultHandoffWorkflow(db); err != nil {
		t.Fatalf("relayoutPlatformDefaultHandoffWorkflow() error = %v", err)
	}
	if err := db.First(workflow, workflow.ID).Error; err != nil {
		t.Fatalf("reload workflow: %v", err)
	}
	if workflow.CurrentStableVersionID == legacyVersion.ID {
		t.Fatal("expected a new immutable stable version")
	}
	if workflow.Description != "通用咨询先查企业知识并支持自然对话、工单与人工接管；关联设备后进入完整 AI 诊断与人工协同" {
		t.Fatalf("unexpected workflow description: %q", workflow.Description)
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

	answerabilityRoute, ok := findWorkflowNode(definition, "answerability_route_1")
	if !ok {
		t.Fatal("answerability route node is missing")
	}
	handoff, ok := findWorkflowNode(definition, "handoff_1")
	if !ok {
		t.Fatal("handoff node is missing")
	}
	handoffEnd, ok := findWorkflowNode(definition, "handoff_end_1")
	if !ok {
		t.Fatal("handoff end node is missing")
	}
	safeReply, ok := findWorkflowNode(definition, "diagnostic_safe_reply_1")
	if !ok {
		t.Fatal("diagnostic safe reply node is missing")
	}
	policyRoute, ok := findWorkflowNode(definition, "policy_route_1")
	if !ok {
		t.Fatal("policy route node is missing")
	}
	if !(answerabilityRoute.Position.X < safeReply.Position.X && policyRoute.Position.X < handoff.Position.X && handoff.Position.X < handoffEnd.Position.X) {
		t.Fatalf("workflow fallback and handoff paths must move left-to-right: answerability=%v safeReply=%v policy=%v handoff=%v end=%v", answerabilityRoute.Position, safeReply.Position, policyRoute.Position, handoff.Position, handoffEnd.Position)
	}

	var answerabilityConfig dsl.ConditionConfig
	if err := json.Unmarshal(answerabilityRoute.Config, &answerabilityConfig); err != nil {
		t.Fatalf("unmarshal answerability route config: %v", err)
	}
	foundSafeFallback := false
	for _, branch := range answerabilityConfig.Branches {
		if branch.TargetNodeID == "handoff_1" {
			t.Fatal("unanswerable diagnosis must not automatically hand off")
		}
		if branch.Default && branch.TargetNodeID == "diagnostic_safe_reply_1" {
			foundSafeFallback = true
		}
	}
	if !foundSafeFallback {
		t.Fatal("answerability must use the safe AI fallback when knowledge is insufficient")
	}
	var policyConfig dsl.ConditionConfig
	if err := json.Unmarshal(policyRoute.Config, &policyConfig); err != nil {
		t.Fatalf("unmarshal policy route config: %v", err)
	}
	foundPolicyHandoff := false
	for _, branch := range policyConfig.Branches {
		if branch.TargetNodeID == "handoff_1" && branch.Condition != nil {
			foundPolicyHandoff = true
		}
	}
	if !foundPolicyHandoff {
		t.Fatal("policy route must keep explicit handoff as a governed customer action")
	}

	var versionCount int64
	if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&versionCount).Error; err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if err := relayoutPlatformDefaultHandoffWorkflow(db); err != nil {
		t.Fatalf("second relayoutPlatformDefaultHandoffWorkflow() error = %v", err)
	}
	var secondVersionCount int64
	if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", workflow.ID).Count(&secondVersionCount).Error; err != nil {
		t.Fatalf("count versions after second run: %v", err)
	}
	if secondVersionCount != versionCount {
		t.Fatalf("migration is not idempotent, version count changed from %d to %d", versionCount, secondVersionCount)
	}
}

func findWorkflowNode(definition dsl.Definition, nodeID string) (dsl.Node, bool) {
	for _, node := range definition.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return dsl.Node{}, false
}
