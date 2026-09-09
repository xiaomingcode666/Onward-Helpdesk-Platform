package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(121, "create platform system intro docs and sync systemIntro permissions", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := ctx.Tx.AutoMigrate(&models.SystemIntroDoc{}); err != nil {
				return err
			}
			if _, err := ensurePermissions(ctx.Tx); err != nil {
				return err
			}
			return services.EnsurePlatformDefaultIAMRolesDB(ctx.Tx, nil)
		})
	})
}
