package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const legacyTicketIdempotencyIndex = "idx_t_ticket_idempotency_key"

func init() {
	register(72, "scope ticket idempotency by tenant", func() error {
		return ensureTicketTenantIdempotencyIndex(sqls.DB())
	})
}

func ensureTicketTenantIdempotencyIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	if db.Migrator().HasIndex(&models.Ticket{}, legacyTicketIdempotencyIndex) {
		if err := db.Migrator().DropIndex(&models.Ticket{}, legacyTicketIdempotencyIndex); err != nil {
			return err
		}
	}
	if db.Migrator().HasIndex(&models.Ticket{}, "uk_ticket_tenant_idempotency") {
		return nil
	}
	return db.Migrator().CreateIndex(&models.Ticket{}, "uk_ticket_tenant_idempotency")
}
