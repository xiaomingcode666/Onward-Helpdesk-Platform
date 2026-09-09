package migration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFreezeLegacyWorkflowModelPoliciesUsesVersionThenDraft(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-policy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatal(err)
	}

	tenantOnly := policyDefinitionJSON(t, []string{dsl.ModelCredentialScopeTenantDefault})
	productFirst := policyDefinitionJSON(t, []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault})
	workflow := models.AIWorkflow{DraftDefinition: tenantOnly, Status: enums.StatusOk}
	if err := db.Create(&workflow).Error; err != nil {
		t.Fatal(err)
	}
	legacy := models.AIWorkflowVersion{WorkflowID: workflow.ID, Version: 1, Definition: `{"schemaVersion":1}`, Status: enums.StatusOk}
	explicit := models.AIWorkflowVersion{WorkflowID: workflow.ID, Version: 2, Definition: productFirst, Status: enums.StatusOk}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&explicit).Error; err != nil {
		t.Fatal(err)
	}

	if err := freezeLegacyWorkflowModelPolicies(db); err != nil {
		t.Fatal(err)
	}
	assertVersionPolicySnapshot(t, db, legacy.ID, []string{dsl.ModelCredentialScopeTenantDefault})
	assertVersionPolicySnapshot(t, db, explicit.ID, []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault})

	if err := db.Model(&models.AIWorkflow{}).Where("id = ?", workflow.ID).Update("draft_definition", productFirst).Error; err != nil {
		t.Fatal(err)
	}
	if err := freezeLegacyWorkflowModelPolicies(db); err != nil {
		t.Fatal(err)
	}
	assertVersionPolicySnapshot(t, db, legacy.ID, []string{dsl.ModelCredentialScopeTenantDefault})
}

func policyDefinitionJSON(t *testing.T, chain []string) string {
	t.Helper()
	raw, err := json.Marshal(dsl.Definition{SchemaVersion: 1, ModelPolicy: &dsl.ModelPolicy{CredentialChain: chain}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertVersionPolicySnapshot(t *testing.T, db *gorm.DB, versionID int64, want []string) {
	t.Helper()
	version := models.AIWorkflowVersion{}
	if err := db.First(&version, versionID).Error; err != nil {
		t.Fatal(err)
	}
	policy := workflowModelPolicyFromJSON(version.ModelPolicySnapshot)
	if policy == nil || !reflect.DeepEqual(policy.CredentialChain, want) {
		t.Fatalf("version %d policy = %#v, want %#v", versionID, policy, want)
	}
}
