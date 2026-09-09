package migration

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(44, "materialize reusable AI workflow templates and stable versions", func() error {
		return migrateAIWorkflowTemplates(sqls.DB())
	})
}

func migrateAIWorkflowTemplates(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	now := time.Now()
	var workflows []models.AIWorkflow
	if err := db.Where("agent_id > 0").Find(&workflows).Error; err != nil {
		return err
	}
	for i := range workflows {
		workflow := workflows[i]
		tenantID := int64(0)
		if agent := repositoriesAIAgentForMigration(db, workflow.AgentID); agent != nil {
			tenantID = agent.TenantID
		}
		updates := map[string]any{
			"tenant_id":        tenantID,
			"scope":            models.AIWorkflowScopeTenant,
			"code":             fmt.Sprintf("legacy_agent_%d", workflow.AgentID),
			"updated_at":       now,
			"update_user_name": "system",
		}
		if workflow.PublishedVersionID > 0 {
			updates["current_stable_version_id"] = workflow.PublishedVersionID
		}
		if err := db.Model(&models.AIWorkflow{}).Where("id = ?", workflow.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&models.AIWorkflowVersion{}).
		Where("release_channel = '' OR release_channel IS NULL").
		Updates(map[string]any{"release_channel": models.AIWorkflowReleaseChannelStable, "schema_version": 1}).Error; err != nil {
		return err
	}
	if _, _, err := services.AIWorkflowService.EnsurePlatformDefaultWorkflowDB(db); err != nil {
		return err
	}
	workflowTable, err := migrationModelTableName(db, &models.AIWorkflow{})
	if err != nil {
		return err
	}
	workflowIndexSQL := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_workflows_scope_code ON %s (tenant_id, code) WHERE code <> '' AND status <> %d",
		quoteMigrationIdentifier(workflowTable),
		enums.StatusDeleted,
	)
	if err := db.Exec(workflowIndexSQL).Error; err != nil {
		return err
	}
	if db.Migrator().HasTable(&models.AIAgentRelease{}) {
		releaseTable, err := migrationModelTableName(db, &models.AIAgentRelease{})
		if err != nil {
			return err
		}
		releaseIndexSQL := fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_agent_releases_active ON %s (agent_id) WHERE deployment_status = 'active' AND status <> %d",
			quoteMigrationIdentifier(releaseTable),
			enums.StatusDeleted,
		)
		if err := db.Exec(releaseIndexSQL).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(&models.AIAgent{}) {
		return nil
	}
	var agents []models.AIAgent
	if err := db.Where("workflow_id = 0 AND workflow_version_id > 0").Find(&agents).Error; err != nil {
		return err
	}
	for i := range agents {
		version := &models.AIWorkflowVersion{}
		if err := db.First(version, "id = ?", agents[i].WorkflowVersionID).Error; err != nil {
			continue
		}
		if err := db.Model(&models.AIAgent{}).Where("id = ?", agents[i].ID).Update("workflow_id", version.WorkflowID).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrationModelTableName(db *gorm.DB, model any) (string, error) {
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		return "", err
	}
	if statement.Schema == nil || strings.TrimSpace(statement.Schema.Table) == "" {
		return "", fmt.Errorf("cannot resolve migration table name for %T", model)
	}
	return statement.Schema.Table, nil
}

func quoteMigrationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func repositoriesAIAgentForMigration(db *gorm.DB, id int64) *models.AIAgent {
	ret := &models.AIAgent{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}
