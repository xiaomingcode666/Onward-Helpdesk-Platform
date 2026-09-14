package migration

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"remotehelpdesk/internal/models"
	"testing"
)

func TestProjectConfigurationMigrationPreservesLegacyTickets(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE t_ticket (id integer PRIMARY KEY, ticket_no text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO t_ticket (id,ticket_no) VALUES (1,'LEGACY-1')").Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := migrateProjectConfiguration(db); err != nil {
			t.Fatal(err)
		}
		if err := migrateProjectRuntimeConfiguration(db); err != nil {
			t.Fatal(err)
		}
	}
	var ticket models.Ticket
	if err := db.First(&ticket, 1).Error; err != nil {
		t.Fatal(err)
	}
	if ticket.TicketNo != "LEGACY-1" || ticket.IntakeConfigVersionID != 0 || ticket.ProjectConfigVersionID != 0 {
		t.Fatal("historical version fabricated or ticket changed")
	}
	if !db.Migrator().HasIndex(&models.Ticket{}, "IntakeConfigVersionID") {
		t.Fatal("version lookup index missing")
	}
	if !db.Migrator().HasIndex(&models.Ticket{}, "ProjectConfigVersionID") {
		t.Fatal("runtime version lookup index missing")
	}
}
