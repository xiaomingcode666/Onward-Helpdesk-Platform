package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(58, "allow repeated resolved supplier collaborations while enforcing one active collaboration", func() error {
		return repairSupplierCollaborationActiveUniqueness(sqls.DB())
	})
}

func repairSupplierCollaborationActiveUniqueness(db *gorm.DB) error {
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
		"CREATE UNIQUE INDEX uk_ticket_supplier_active ON %s (tenant_id, ticket_id, product_module_id, partner_company_id) WHERE record_status <> %d AND status <> 'resolved'",
		quoteMigrationIdentifier(table),
		enums.StatusDeleted,
	)).Error
}
