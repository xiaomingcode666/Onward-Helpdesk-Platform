package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(150, "add knowledge access grants and ticket support readiness", func() error {
		if err := sqls.DB().AutoMigrate(&models.KnowledgeAccessGrant{}); err != nil {
			return err
		}
		for _, column := range []string{"KnowledgeBaseID", "SupportStatus", "SupportReasonCode", "SupportReason", "SupportCheckedAt"} {
			if !sqls.DB().Migrator().HasColumn(&models.Ticket{}, column) {
				if err := sqls.DB().Migrator().AddColumn(&models.Ticket{}, column); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
