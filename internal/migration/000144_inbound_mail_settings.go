package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(144, "add MVP IMAP receive settings", func() error {
		db := sqls.DB()
		for _, column := range []string{"IMAPHost", "IMAPPort", "IMAPUsername", "IMAPPassword", "IMAPUseTLS", "IMAPEnabled"} {
			if !db.Migrator().HasColumn(&models.TenantMailSetting{}, column) {
				if err := db.Migrator().AddColumn(&models.TenantMailSetting{}, column); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
