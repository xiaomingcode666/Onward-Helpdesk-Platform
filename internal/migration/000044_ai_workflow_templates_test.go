package migration

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMigrateAIWorkflowTemplatesUsesConfiguredTableNamingAndPartialSchema(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "workflow-template.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	if err := migrateAIWorkflowTemplates(db); err != nil {
		t.Fatalf("migrateAIWorkflowTemplates() error = %v", err)
	}
	if !db.Migrator().HasIndex(&models.AIWorkflow{}, "ux_ai_workflows_scope_code") {
		t.Fatalf("expected workflow scope/code index on configured table")
	}
	var workflow models.AIWorkflow
	if err := db.Where("scope = ? AND code = ?", models.AIWorkflowScopePlatform, services.PlatformDefaultAfterSalesWorkflowCode).First(&workflow).Error; err != nil {
		t.Fatalf("load platform workflow: %v", err)
	}
	if !workflow.Locked || workflow.CurrentStableVersionID <= 0 {
		t.Fatalf("unexpected platform workflow: %+v", workflow)
	}
}
