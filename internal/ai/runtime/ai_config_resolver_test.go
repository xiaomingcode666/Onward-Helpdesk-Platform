package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestApplyAgentLLMModelOverridesCopy(t *testing.T) {
	original := &models.AIConfig{ModelName: "gpt-default"}
	resolved := applyAgentLLMModel(original, "  gpt-agent  ")
	if resolved == original {
		t.Fatal("applyAgentLLMModel() must not mutate the shared AI config")
	}
	if resolved.ModelName != "gpt-agent" {
		t.Fatalf("resolved model = %q", resolved.ModelName)
	}
	if original.ModelName != "gpt-default" {
		t.Fatalf("original model was mutated: %q", original.ModelName)
	}
}

func TestApplyAgentLLMModelKeepsDefaultForBlankSelection(t *testing.T) {
	original := &models.AIConfig{ModelName: "gpt-default"}
	if resolved := applyAgentLLMModel(original, "  "); resolved != original {
		t.Fatal("blank model selection should keep the resolved default config")
	}
}

func TestResolveAgentWorkflowCredentialChainUsesImmutableVersionPolicy(t *testing.T) {
	db := setupAIConfigResolverTestDB(t)
	workflow := createCredentialPolicyWorkflow(t, db, []string{dsl.ModelCredentialScopeTenantDefault})
	want := []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault}
	version := createCredentialPolicyVersion(t, db, workflow.ID, want)

	if got := resolveAgentWorkflowCredentialChain(models.AIAgent{WorkflowVersionID: version.ID}); !reflect.DeepEqual(got, want) {
		t.Fatalf("credential chain = %#v, want %#v", got, want)
	}
}

func TestResolveAgentWorkflowCredentialChainUsesFrozenLegacySnapshot(t *testing.T) {
	db := setupAIConfigResolverTestDB(t)
	want := []string{dsl.ModelCredentialScopeTenantDefault}
	workflow := createCredentialPolicyWorkflow(t, db, want)
	version := createCredentialPolicyVersion(t, db, workflow.ID, nil)
	version.ModelPolicySnapshot = `{"credentialChain":["tenant_default"]}`
	if err := db.Save(version).Error; err != nil {
		t.Fatal(err)
	}
	workflow.DraftDefinition = credentialPolicyDefinition(t, []string{dsl.ModelCredentialScopeProduct})
	if err := db.Save(workflow).Error; err != nil {
		t.Fatal(err)
	}

	if got := resolveAgentWorkflowCredentialChain(models.AIAgent{WorkflowVersionID: version.ID}); !reflect.DeepEqual(got, want) {
		t.Fatalf("credential chain = %#v, want frozen version snapshot %#v", got, want)
	}
}

func TestResolveAgentAIConfigWorkflowPolicyOverridesLegacyAgentConfig(t *testing.T) {
	db := setupAIConfigResolverTestDB(t)
	t.Setenv("LLM_SOURCE", "database")
	workflow := createCredentialPolicyWorkflow(t, db, []string{dsl.ModelCredentialScopeProduct})
	version := createCredentialPolicyVersion(t, db, workflow.ID, []string{dsl.ModelCredentialScopeProduct})
	config := models.AIConfig{ModelType: enums.AIModelTypeLLM, ModelName: "legacy-model", Status: enums.StatusOk}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}

	_, resolved, err := resolveAgentAIConfig(context.Background(), models.AIAgent{
		AIConfigID:        config.ID,
		WorkflowVersionID: version.ID,
		TenantID:          7,
		ProductID:         42,
	}, 7, 42, nil)
	if resolved != nil || err == nil || !strings.Contains(err.Error(), "credential policy conflicts") {
		t.Fatalf("resolveAgentAIConfig() = (%#v, %v), want workflow-policy conflict", resolved, err)
	}
}

func setupAIConfigResolverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "ai-config-resolver.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}, &models.AIConfig{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	return db
}

func createCredentialPolicyWorkflow(t *testing.T, db *gorm.DB, credentialChain []string) *models.AIWorkflow {
	t.Helper()
	definition := credentialPolicyDefinition(t, credentialChain)
	workflow := &models.AIWorkflow{
		Code:            "credential-policy-test",
		Scope:           models.AIWorkflowScopePlatform,
		DraftDefinition: definition,
		Status:          enums.StatusOk,
	}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatal(err)
	}
	return workflow
}

func createCredentialPolicyVersion(t *testing.T, db *gorm.DB, workflowID int64, credentialChain []string) *models.AIWorkflowVersion {
	t.Helper()
	version := &models.AIWorkflowVersion{
		WorkflowID: workflowID,
		Version:    1,
		Definition: credentialPolicyDefinition(t, credentialChain),
		Status:     enums.StatusOk,
	}
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
	return version
}

func credentialPolicyDefinition(t *testing.T, credentialChain []string) string {
	t.Helper()
	definition := dsl.Definition{SchemaVersion: 1, EntryNodeID: "start_1"}
	if len(credentialChain) > 0 {
		definition.ModelPolicy = &dsl.ModelPolicy{CredentialChain: credentialChain}
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
