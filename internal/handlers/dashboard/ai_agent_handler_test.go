package dashboard

import (
	"testing"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestBuildAIAgentResponseExposesWorkflowPublishState(t *testing.T) {
	setupAIAgentHandlerTestDB(t)

	draft := builders.BuildAIAgent(&models.AIAgent{})
	if draft.WorkflowPublished {
		t.Fatalf("draft.WorkflowPublished = true, want false")
	}
	if draft.WorkflowState != "draft" {
		t.Fatalf("draft.WorkflowState = %q, want draft", draft.WorkflowState)
	}
	if draft.WorkflowStateText == "" {
		t.Fatalf("expected draft workflow state text")
	}

	published := builders.BuildAIAgent(&models.AIAgent{WorkflowVersionID: 12, LLMModelName: "gpt-5.6"})
	if !published.WorkflowPublished {
		t.Fatalf("published.WorkflowPublished = false, want true")
	}
	if published.WorkflowState != "published" {
		t.Fatalf("published.WorkflowState = %q, want published", published.WorkflowState)
	}
	if published.WorkflowStateText == "" {
		t.Fatalf("expected published workflow state text")
	}
	if published.LLMModelName != "gpt-5.6" {
		t.Fatalf("published.LLMModelName = %q", published.LLMModelName)
	}
}

func setupAIAgentHandlerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sqlite db: %v", err)
		}
	})
	if err := db.AutoMigrate(
		&models.AIConfig{},
		&models.AgentTeam{},
		&models.KnowledgeBase{},
		&models.SkillDefinition{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
}
