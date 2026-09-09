package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(47, "enforce AI workflow version and agent release uniqueness", func() error {
		return enforceAIReleaseUniqueness(sqls.DB())
	})
}

func enforceAIReleaseUniqueness(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		table, err := migrationModelTableName(db, &models.AIWorkflowVersion{})
		if err != nil {
			return err
		}
		if err := db.Exec(fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS uk_ai_workflow_version_no ON %s (workflow_id, version)",
			quoteMigrationIdentifier(table),
		)).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	table, err := migrationModelTableName(db, &models.AIAgentRelease{})
	if err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_ai_agent_release_no ON %s (agent_id, release_no)",
		quoteMigrationIdentifier(table),
	)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_agent_release_active ON %s (agent_id) WHERE deployment_status = 'active' AND status <> %d",
		quoteMigrationIdentifier(table),
		enums.StatusDeleted,
	)).Error; err != nil {
		return err
	}
	return db.Exec("DROP INDEX IF EXISTS ux_ai_agent_releases_active").Error
}
