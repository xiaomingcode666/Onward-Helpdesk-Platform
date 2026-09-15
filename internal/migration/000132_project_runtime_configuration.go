package migration

import (
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
)

func init() {
	register(132, "bind tickets to runtime project configuration", func() error {
		return migrateProjectRuntimeConfiguration(sqls.DB())
	})
}
func migrateProjectRuntimeConfiguration(db *gorm.DB) error {
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasColumn(&models.Ticket{}, "ProjectConfigVersionID") {
		if err := db.Migrator().AddColumn(&models.Ticket{}, "ProjectConfigVersionID"); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.Ticket{}) && !db.Migrator().HasIndex(&models.Ticket{}, "ProjectConfigVersionID") {
		return db.Migrator().CreateIndex(&models.Ticket{}, "ProjectConfigVersionID")
	}
	return nil
}
