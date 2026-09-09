package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(110, "allow supplier collaboration timeout terminal state", func() error {
		return migrateSupplierCollaborationTimeoutIndex(sqls.DB())
	})
}

func migrateSupplierCollaborationTimeoutIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.TicketSupplierCollaboration{}) {
		return nil
	}
	table, err := migrationModelTableName(db, &models.TicketSupplierCollaboration{})
	if err != nil {
		return err
	}
	if err := db.Exec("DROP INDEX IF EXISTS uk_ticket_supplier_active").Error; err != nil {
		return err
	}
	return db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX uk_ticket_supplier_active ON %s (tenant_id, ticket_id, product_module_id, partner_company_id) WHERE record_status <> %d AND status NOT IN ('resolved','timeout')",
		quoteMigrationIdentifier(table),
		enums.StatusDeleted,
	)).Error
}
