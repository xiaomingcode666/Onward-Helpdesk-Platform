package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(9, "init remote helpdesk default tenant", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			now := time.Now()
			var count int64
			if err := ctx.Tx.Model(&models.Tenant{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			return ctx.Tx.Create(&models.Tenant{
				Name:          "Default Tenant",
				Industry:      "machinery",
				CountryRegion: "global",
				DefaultLocale: "en",
				Timezone:      "UTC",
				DataRegion:    "global",
				Status:        enums.StatusOk,
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserID:   constants.SystemAuditUserID,
					CreateUserName: constants.SystemAuditUserName,
					UpdatedAt:      now,
					UpdateUserID:   constants.SystemAuditUserID,
					UpdateUserName: constants.SystemAuditUserName,
				},
			}).Error
		})
	})
}
