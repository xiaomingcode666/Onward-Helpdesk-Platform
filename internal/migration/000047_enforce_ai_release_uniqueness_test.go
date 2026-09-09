package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnforceAIReleaseUniquenessUsesConfiguredTableNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflowVersion{}, &models.AIAgentRelease{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := enforceAIReleaseUniqueness(db); err != nil {
		t.Fatalf("enforceAIReleaseUniqueness() error = %v", err)
	}
	if !db.Migrator().HasIndex(&models.AIWorkflowVersion{}, "uk_ai_workflow_version_no") {
		t.Fatal("workflow version unique index was not created")
	}
	if !db.Migrator().HasIndex(&models.AIAgentRelease{}, "uk_ai_agent_release_no") {
		t.Fatal("release number unique index was not created")
	}
	if !db.Migrator().HasIndex(&models.AIAgentRelease{}, "ux_ai_agent_release_active") {
		t.Fatal("active release partial unique index was not created")
	}
}
