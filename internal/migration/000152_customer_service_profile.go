package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/projectconfig"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(152, "add customer and ticket service profiles", func() error {
		db := sqls.DB()
		for _, column := range []string{"ServiceProfile"} {
			if !db.Migrator().HasColumn(&models.Customer{}, column) {
				if err := db.Migrator().AddColumn(&models.Customer{}, column); err != nil {
					return err
				}
			}
			if !db.Migrator().HasColumn(&models.Ticket{}, column) {
				if err := db.Migrator().AddColumn(&models.Ticket{}, column); err != nil {
					return err
				}
			}
		}
		return db.Model(&models.Customer{}).
			Where("service_profile = ''").
			Update("service_profile", projectconfig.DefaultServiceProfile).Error
	})
}
