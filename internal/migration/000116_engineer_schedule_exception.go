package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(116, "add engineer global schedule exceptions", func() error {
		return migrateEngineerScheduleException(sqls.DB())
	})
}

func migrateEngineerScheduleException(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.AutoMigrate(&models.AgentScheduleException{}); err != nil {
		return err
	}
	if db.Migrator().HasTable(&models.AgentTeamScheduleTemplate{}) {
		return db.Model(&models.AgentTeamScheduleTemplate{}).
			Where("timezone = ? OR timezone = ''", "UTC").
			Update("timezone", "Asia/Shanghai").Error
	}
	return nil
}
