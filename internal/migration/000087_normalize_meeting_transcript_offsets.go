package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dbtime"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const legacyTranscriptAbsoluteThresholdMS = int64((7 * 24 * time.Hour) / time.Millisecond)

func init() {
	register(87, "harden meeting transcript persistence and normalize timeline offsets", func() error {
		return hardenMeetingTranscriptPersistence(sqls.DB())
	})
}

func hardenMeetingTranscriptPersistence(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) {
		return nil
	}
	for _, field := range []string{
		"TranslatedLanguage", "TranslatedText", "TranslationProvider", "TranslationStatus",
		"TranslationError", "TranslationUpdatedAt",
	} {
		if !db.Migrator().HasColumn(&models.MeetingTranscriptSegment{}, field) {
			if err := db.Migrator().AddColumn(&models.MeetingTranscriptSegment{}, field); err != nil {
				return err
			}
		}
	}
	if err := deduplicateMeetingTranscriptEvents(db); err != nil {
		return err
	}
	table, err := migrationModelTableName(db, &models.MeetingTranscriptSegment{})
	if err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_meeting_transcript_event_v2 ON %s (meeting_id, provider, provider_event_id)",
		quoteMigrationIdentifier(table),
	)).Error; err != nil {
		return err
	}
	if db.Migrator().HasIndex(&models.MeetingTranscriptSegment{}, "uk_meeting_transcript_provider_event") {
		if err := db.Migrator().DropIndex(&models.MeetingTranscriptSegment{}, "uk_meeting_transcript_provider_event"); err != nil {
			return err
		}
	}
	return normalizeMeetingTranscriptOffsets(db)
}

func deduplicateMeetingTranscriptEvents(db *gorm.DB) error {
	var segments []models.MeetingTranscriptSegment
	if err := db.Order("meeting_id ASC, provider ASC, provider_event_id ASC").
		Order("CASE WHEN translation_status = 'completed' THEN 0 ELSE 1 END ASC").
		Order("CASE WHEN is_final THEN 0 ELSE 1 END ASC").
		Order("created_at ASC, id ASC").Find(&segments).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(segments))
	duplicateIDs := make([]string, 0)
	for i := range segments {
		key := segments[i].MeetingID + "\x00" + segments[i].Provider + "\x00" + segments[i].ProviderEventID
		if _, exists := seen[key]; exists {
			duplicateIDs = append(duplicateIDs, segments[i].ID)
			continue
		}
		seen[key] = struct{}{}
	}
	if len(duplicateIDs) == 0 {
		return nil
	}
	return db.Unscoped().Where("id IN ?", duplicateIDs).Delete(&models.MeetingTranscriptSegment{}).Error
}

func normalizeMeetingTranscriptOffsets(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) || !db.Migrator().HasTable(&models.MeetingRoomJitsi{}) {
		return nil
	}
	var segments []models.MeetingTranscriptSegment
	if err := db.Where("started_at_ms > ? OR ended_at_ms > ?", legacyTranscriptAbsoluteThresholdMS, legacyTranscriptAbsoluteThresholdMS).
		Find(&segments).Error; err != nil {
		return err
	}
	meetings := make(map[string]models.MeetingRoomJitsi)
	for i := range segments {
		meeting, ok := meetings[segments[i].MeetingID]
		if !ok {
			if err := db.First(&meeting, "id = ?", segments[i].MeetingID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					continue
				}
				return err
			}
			meetings[meeting.ID] = meeting
		}
		origin := meeting.CreatedAt
		if meeting.StartedAt != nil && !meeting.StartedAt.IsZero() {
			origin = *meeting.StartedAt
		}
		if origin.IsZero() {
			continue
		}
		originMS := dbtime.WallClockUnixMilli(db, origin)
		startedAtMS, startChanged := normalizeStoredTranscriptOffset(segments[i].StartedAtMS, originMS)
		endedAtMS, endChanged := normalizeStoredTranscriptOffset(segments[i].EndedAtMS, originMS)
		if !startChanged && !endChanged {
			continue
		}
		if endedAtMS < startedAtMS {
			endedAtMS = startedAtMS
		}
		if err := db.Model(&models.MeetingTranscriptSegment{}).
			Where("id = ? AND tenant_id = ?", segments[i].ID, segments[i].TenantID).
			Updates(map[string]any{"started_at_ms": startedAtMS, "ended_at_ms": endedAtMS}).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeStoredTranscriptOffset(value, originMS int64) (int64, bool) {
	if value <= legacyTranscriptAbsoluteThresholdMS {
		return value, false
	}
	offset := value - originMS
	if offset < 0 || offset > legacyTranscriptAbsoluteThresholdMS {
		return value, false
	}
	return offset, true
}
