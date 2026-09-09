package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(21, "seed default platform tenant partner and customer IAM roles", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := services.EnsurePlatformDefaultIAMRolesDB(ctx.Tx, nil); err != nil {
				return err
			}
			var tenants []models.Tenant
			if err := ctx.Tx.Find(&tenants).Error; err != nil {
				return err
			}
			for _, tenant := range tenants {
				if err := services.EnsureTenantDefaultIAMRolesDB(ctx.Tx, tenant.ID, nil); err != nil {
					return err
				}
			}
			return nil
		})
	})
}
