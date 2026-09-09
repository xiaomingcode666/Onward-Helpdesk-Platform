package migration

import (
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNormalizeWorkflowModelCredentialChainsPublishesRegisteredTemplates(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-model-credential-chain.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := normalizeWorkflowModelCredentialChains(db); err != nil {
		t.Fatal(err)
	}

	for _, code := range services.PlatformBuiltInWorkflowCodes() {
		workflow := repositoriesWorkflowByCodeForTest(t, db, code)
		if workflow.CurrentStableVersionID <= 0 || strings.Contains(workflow.DraftDefinition, "credentialScope") {
			t.Fatalf("workflow %s was not normalized: %#v", code, workflow)
		}
		if code != services.PlatformDispatchOnlyWorkflowCode && !strings.Contains(workflow.DraftDefinition, "credentialChain") {
			t.Fatalf("model-backed workflow %s has no credential chain: %#v", code, workflow)
		}
	}
}
