package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(94, "harden meeting transcript history and translation retries", func() error {
		return hardenMeetingTranscriptRuntime(sqls.DB())
	})
}

func hardenMeetingTranscriptRuntime(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) {
		return nil
	}
	for _, field := range []string{"TranslationRetryCount", "TranslationNextRetryAt", "TranslationLeaseUntil"} {
		if !db.Migrator().HasColumn(&models.MeetingTranscriptSegment{}, field) {
			if err := db.Migrator().AddColumn(&models.MeetingTranscriptSegment{}, field); err != nil {
				return err
			}
		}
	}
	table, err := migrationModelTableName(db, &models.MeetingTranscriptSegment{})
	if err != nil {
		return err
	}
	if err := db.Exec("DROP INDEX IF EXISTS uk_meeting_transcript_provider_event").Error; err != nil {
		return err
	}
	return db.Exec(fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_meeting_transcript_timeline ON %s (tenant_id, meeting_id, started_at_ms, created_at)",
		quoteMigrationIdentifier(table),
	)).Error
}
