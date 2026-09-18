package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(151, "migrate module access support to knowledge bases", func() error {
		db := sqls.DB()
		if err := db.AutoMigrate(&models.KnowledgeAccessGrant{}); err != nil {
			return err
		}
		if !db.Migrator().HasColumn(&models.Ticket{}, "KnowledgeBaseID") {
			if err := db.Migrator().AddColumn(&models.Ticket{}, "KnowledgeBaseID"); err != nil {
				return err
			}
		}
		return db.Exec("DROP TABLE IF EXISTS t_module_access_grant").Error
	})
}
