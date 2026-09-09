package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(114, "add service outcome, industry solution, and MQTT ingestion schema", func() error {
		return migrateServiceOutcomeIndustryMQTT(sqls.DB())
	})
}

func migrateServiceOutcomeIndustryMQTT(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(
		&models.AccessConnector{},
		&models.TicketServiceOutcomeFact{},
		&models.IndustrySolutionPack{},
		&models.IndustrySolutionPackResource{},
		&models.IndustrySolutionPackApplication{},
		&models.IndustrySolutionPackApplicationItem{},
		&models.DiagnosisEvalCase{},
		&models.DiagnosisEvalRun{},
		&models.ARWorkInstruction{},
		&models.ARWorkStep{},
		&models.ConnectorSyncCursor{},
		&models.ConnectorWorkerLease{},
		&models.ConnectorEventInbox{},
		&models.DeviceTelemetryEvent{},
		&models.DeviceTelemetrySnapshot{},
		&models.DeviceAlarmEvent{},
		&models.ConnectorDeadLetter{},
	)
}
