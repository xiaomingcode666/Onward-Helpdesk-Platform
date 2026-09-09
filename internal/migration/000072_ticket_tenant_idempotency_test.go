package migration

import (
	"strconv"
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestTicketIdempotencyMigrationScopesKeysByTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Migrator().DropIndex(&models.Ticket{}, "uk_ticket_tenant_idempotency"); err != nil {
		t.Fatalf("drop current ticket index: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX " + legacyTicketIdempotencyIndex + " ON t_ticket (idempotency_key)").Error; err != nil {
		t.Fatalf("create legacy ticket index: %v", err)
	}

	if err := ensureTicketTenantIdempotencyIndex(db); err != nil {
		t.Fatalf("ensureTicketTenantIdempotencyIndex() error = %v", err)
	}
	if db.Migrator().HasIndex(&models.Ticket{}, legacyTicketIdempotencyIndex) {
		t.Fatal("legacy global ticket idempotency index still exists")
	}
	if !db.Migrator().HasIndex(&models.Ticket{}, "uk_ticket_tenant_idempotency") {
		t.Fatal("tenant-scoped ticket idempotency index was not created")
	}

	key := "crm-create:42"
	for _, tenantID := range []int64{1, 2} {
		item := ticketIdempotencyFixture(tenantID, &key)
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create ticket for tenant %d: %v", tenantID, err)
		}
	}
	duplicate := ticketIdempotencyFixture(1, &key)
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("tenant-scoped ticket idempotency index accepted a duplicate within one tenant")
	}
}

func ticketIdempotencyFixture(tenantID int64, key *string) models.Ticket {
	return models.Ticket{
		TenantID:       tenantID,
		TicketNo:       "TK-IDEMPOTENCY-" + strconv.FormatInt(tenantID, 10),
		IdempotencyKey: key,
		Title:          "idempotency test",
	}
}
