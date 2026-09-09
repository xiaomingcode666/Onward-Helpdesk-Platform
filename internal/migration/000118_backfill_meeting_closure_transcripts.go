package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(118, "backfill empty ended meeting transcript archives", func() error {
		return backfillEmptyEndedMeetingTranscriptArchives(sqls.DB())
	})
}

func backfillEmptyEndedMeetingTranscriptArchives(db *gorm.DB) error {
	for {
		backfilled, err := services.MeetingIntelligenceService.BackfillEndedMeetingClosureArchivesDB(db, 2000)
		if err != nil {
			return err
		}
		if backfilled == 0 {
			return nil
		}
	}
}
