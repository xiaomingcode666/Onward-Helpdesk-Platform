package migration

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"remotehelpdesk/internal/models"
	"testing"
)

func TestTicketIntakeMigrationLegacyAndIdempotent(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE t_ticket (id integer PRIMARY KEY, tenant_id integer NOT NULL DEFAULT 0, ticket_no text)",
		"CREATE TABLE t_tenant (id integer PRIMARY KEY)",
		"INSERT INTO t_ticket (id, tenant_id, ticket_no) VALUES (1, 7, 'LEGACY-1'), (2, 7, 'LEGACY-2')",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := migrateTicketIntake(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, field := range []string{"SourceRecordID", "SourceRecordKey", "ProjectKey", "TicketType", "CallerName", "CallerPhone", "ReceivedAt", "ContextStatus", "MissingContextJSON"} {
		if !db.Migrator().HasColumn(&models.Ticket{}, field) {
			t.Fatalf("missing %s", field)
		}
	}
	if !db.Migrator().HasColumn(&models.Tenant{}, "TicketIntakePolicyJSON") || !db.Migrator().HasIndex(&models.Ticket{}, "uk_ticket_intake_source") {
		t.Fatal("policy or unique index missing")
	}
	var legacy []models.Ticket
	if err := db.Find(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	for _, ticket := range legacy {
		if ticket.SourceRecordID != "" || ticket.SourceRecordKey != nil || ticket.ReceivedAt != nil || ticket.ContextStatus != "not_evaluated" {
			t.Fatalf("fabricated historical metadata %+v", ticket)
		}
	}
	if err := db.Model(&models.Ticket{}).Where("id = ?", 1).UpdateColumn("source_record_key", "key").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.Ticket{}).Where("id = ?", 2).UpdateColumn("source_record_key", "key").Error; err == nil {
		t.Fatal("duplicate source key accepted")
	}
}
