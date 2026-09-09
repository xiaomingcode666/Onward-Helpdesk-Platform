package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestTicketDispatchDeferMigrationAddsColumnsAndIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	for _, index := range []string{"DispatchDeferredUntil", "LastDispatchFailureReason"} {
		if err := db.Migrator().DropIndex(&models.Ticket{}, index); err != nil {
			t.Fatalf("drop dispatch defer index %s: %v", index, err)
		}
	}

	if err := migrateTicketDispatchDefer(db); err != nil {
		t.Fatalf("migrateTicketDispatchDefer() error = %v", err)
	}
	for _, column := range []string{"dispatch_deferred_until", "last_dispatch_failure_reason"} {
		if !db.Migrator().HasColumn(&models.Ticket{}, column) {
			t.Fatalf("missing ticket dispatch defer column %s", column)
		}
	}
	for _, index := range []string{"DispatchDeferredUntil", "LastDispatchFailureReason"} {
		if !db.Migrator().HasIndex(&models.Ticket{}, index) {
			t.Fatalf("missing ticket dispatch defer index %s", index)
		}
	}
}
