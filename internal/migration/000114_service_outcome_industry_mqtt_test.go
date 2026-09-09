package migration

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMigrateServiceOutcomeIndustryMQTTUsesProductionNaming(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "service-industry-mqtt.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := migrateServiceOutcomeIndustryMQTT(db); err != nil {
		t.Fatalf("migrateServiceOutcomeIndustryMQTT() error = %v", err)
	}

	for _, table := range []string{
		"t_ticket_service_outcome_fact",
		"t_industry_solution_pack",
		"t_industry_solution_pack_resource",
		"t_industry_solution_pack_application",
		"t_industry_solution_pack_application_item",
		"t_diagnosis_eval_case",
		"t_diagnosis_eval_run",
		"t_ar_work_instruction",
		"t_ar_work_step",
		"t_connector_sync_cursor",
		"t_connector_worker_lease",
		"t_connector_event_inbox",
		"t_device_telemetry_event",
		"t_device_telemetry_snapshot",
		"t_device_alarm_event",
		"t_connector_dead_letter",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected production table %s", table)
		}
	}
	if !db.Migrator().HasColumn(&models.AccessConnector{}, "last_message_at") {
		t.Fatal("expected access connector last_message_at column")
	}
}
