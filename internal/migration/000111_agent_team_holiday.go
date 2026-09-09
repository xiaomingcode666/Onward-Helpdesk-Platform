package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(111, "add agent team holiday calendar", func() error {
		return migrateAgentTeamHoliday(sqls.DB())
	})
}

func migrateAgentTeamHoliday(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(&models.AgentTeamHoliday{})
}
