package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(115, "add tenant language and timezone preferences", func() error {
		return migrateTenantLanguageTimezonePreferences(sqls.DB())
	})
}

func migrateTenantLanguageTimezonePreferences(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) {
		return nil
	}
	for _, column := range []struct {
		name  string
		field string
	}{
		{name: "supported_locales_json", field: "SupportedLocalesJSON"},
		{name: "supported_timezones_json", field: "SupportedTimezonesJSON"},
	} {
		if db.Migrator().HasColumn(&models.Tenant{}, column.name) {
			continue
		}
		if err := db.Migrator().AddColumn(&models.Tenant{}, column.field); err != nil {
			return err
		}
	}
	return nil
}
