package migration

import (
	"fmt"
	"strings"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(109, "make agent team member dispatch opt-in by default", func() error {
		return migrateAgentTeamMemberDispatchDefault(sqls.DB())
	})
}

func migrateAgentTeamMemberDispatchDefault(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AgentTeamMember{}) {
		return nil
	}
	switch db.Dialector.Name() {
	case "postgres":
		tableName, err := schemaTableName(db, &models.AgentTeamMember{})
		if err != nil {
			return err
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN dispatch_enabled SET DEFAULT false", quotePostgresIdentifier(tableName))).Error
	case "sqlite":
		return nil
	default:
		return db.Migrator().AlterColumn(&models.AgentTeamMember{}, "DispatchEnabled")
	}
}

func schemaTableName(db *gorm.DB, value any) (string, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(value); err != nil {
		return "", err
	}
	return stmt.Schema.Table, nil
}

func quotePostgresIdentifier(name string) string {
	parts := strings.Split(name, ".")
	for i := range parts {
		parts[i] = `"` + strings.ReplaceAll(parts[i], `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}
