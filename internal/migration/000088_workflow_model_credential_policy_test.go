package migration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAddWorkflowModelCredentialPolicies(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-model-credentials.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := addWorkflowModelCredentialPolicies(db); err != nil {
		t.Fatal(err)
	}

	wants := map[string][]string{
		services.PlatformDefaultAfterSalesWorkflowCode: {
			dsl.ModelCredentialScopeProduct,
			dsl.ModelCredentialScopeTenantDefault,
		},
		services.PlatformDeviceAIOnlyWorkflowCode: {
			dsl.ModelCredentialScopeProduct,
			dsl.ModelCredentialScopeTenantDefault,
		},
	}
	for code, want := range wants {
		workflow := repositoriesWorkflowByCodeForTest(t, db, code)
		var definition dsl.Definition
		if err := json.Unmarshal([]byte(workflow.DraftDefinition), &definition); err != nil {
			t.Fatal(err)
		}
		if definition.ModelPolicy == nil || !reflect.DeepEqual(definition.ModelPolicy.CredentialChain, want) {
			t.Fatalf("workflow %s model policy = %#v, want %#v", code, definition.ModelPolicy, want)
		}
		if strings.Contains(workflow.DraftDefinition, "credentialScope") {
			t.Fatalf("workflow %s still contains legacy credentialScope: %s", code, workflow.DraftDefinition)
		}
		if workflow.CurrentStableVersionID <= 0 {
			t.Fatalf("workflow %s has no stable version", code)
		}
		var version models.AIWorkflowVersion
		if err := db.First(&version, workflow.CurrentStableVersionID).Error; err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
			t.Fatal(err)
		}
		if definition.ModelPolicy == nil || !reflect.DeepEqual(definition.ModelPolicy.CredentialChain, want) {
			t.Fatalf("workflow %s stable version policy = %#v, want %#v", code, definition.ModelPolicy, want)
		}
	}
}
