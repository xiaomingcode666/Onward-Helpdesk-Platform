package migration

import (
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
)

func init() {
	register(131, "add immutable project configuration versions and intake version binding", func() error { return migrateProjectConfiguration(sqls.DB()) })
}

func migrateProjectConfiguration(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.ProjectConfigurationVersion{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationActivation{}); err != nil {
		return err
	}
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasColumn(&models.Ticket{}, "IntakeConfigVersionID") {
		if err := db.Migrator().AddColumn(&models.Ticket{}, "IntakeConfigVersionID"); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasIndex(&models.Ticket{}, "IntakeConfigVersionID") {
		return db.Migrator().CreateIndex(&models.Ticket{}, "IntakeConfigVersionID")
	}
	return nil
}
