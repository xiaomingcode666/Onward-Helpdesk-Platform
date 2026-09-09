package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(90, "backfill meeting transcript ingest source", func() error {
		return backfillMeetingTranscriptIngestSource(sqls.DB())
	})
}

func backfillMeetingTranscriptIngestSource(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.MeetingTranscriptSegment{}, "IngestSource") {
		if err := db.Migrator().AddColumn(&models.MeetingTranscriptSegment{}, "IngestSource"); err != nil {
			return err
		}
	}
	return db.Model(&models.MeetingTranscriptSegment{}).
		Where("provider = ? AND ingest_source = ? AND raw_json <> '' AND raw_json <> '{}'", "jitsi_caption", "provider").
		Update("ingest_source", "client").Error
}
