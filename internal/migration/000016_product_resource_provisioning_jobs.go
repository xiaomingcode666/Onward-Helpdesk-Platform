package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(16, "product resource provisioning jobs and product ai credential fields", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := ensureProductResourceProvisioningSchema(ctx.Tx); err != nil {
				return err
			}
			return nil
		})
	})
}

func ensureProductResourceProvisioningSchema(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&models.ProductResourceProvisioningJob{}) {
		if err := migrator.CreateTable(&models.ProductResourceProvisioningJob{}); err != nil {
			return err
		}
	}
	for _, column := range []string{
		"Sub2APIKeyID",
		"KeyName",
		"QuotaLimit",
		"QuotaUsed",
		"Currency",
		"ProvisionStatus",
		"ProvisionError",
	} {
		if !migrator.HasColumn(&models.ProductAIUsageCredential{}, column) {
			if err := migrator.AddColumn(&models.ProductAIUsageCredential{}, column); err != nil {
				return err
			}
		}
	}
	if db.Dialector != nil && db.Dialector.Name() == "postgres" {
		if err := migrator.AlterColumn(&models.ProductAIUsageCredential{}, "APIKeyRef"); err != nil {
			return err
		}
	}
	return nil
}
