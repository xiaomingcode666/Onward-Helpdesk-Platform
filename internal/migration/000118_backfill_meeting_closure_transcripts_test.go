package migration

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBackfillEmptyEndedMeetingTranscriptArchives(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.MeetingParticipant{}, &models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX uk_meeting_transcript_event_v2 ON meeting_transcript_segments (meeting_id, provider, provider_event_id)").Error; err != nil {
		t.Fatalf("create transcript unique index: %v", err)
	}

	startedAt := time.Now().Add(-10 * time.Minute)
	endedAt := startedAt.Add(6 * time.Minute)
	meeting := models.MeetingRoomJitsi{
		ID: "migration-empty-ended", TenantID: 7, TicketID: "481", RoomName: "migration-room",
		Status: "ended", StartedAt: &startedAt, EndedAt: &endedAt,
		BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: endedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	if err := backfillEmptyEndedMeetingTranscriptArchives(db); err != nil {
		t.Fatalf("backfill empty transcript archives: %v", err)
	}
	var segment models.MeetingTranscriptSegment
	if err := db.Where("meeting_id = ?", meeting.ID).First(&segment).Error; err != nil {
		t.Fatalf("load backfilled transcript: %v", err)
	}
	if segment.Provider != "system" || segment.IngestSource != "system" || !strings.Contains(segment.Text, "未收到实时语音转写片段") {
		t.Fatalf("unexpected backfilled transcript: %#v", segment)
	}
}

func TestBackfillEmptyEndedMeetingTranscriptArchivesRunsUntilExhausted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX uk_meeting_transcript_event_v2 ON meeting_transcript_segments (meeting_id, provider, provider_event_id)").Error; err != nil {
		t.Fatalf("create transcript unique index: %v", err)
	}

	endedAt := time.Now().Add(-time.Minute)
	meetings := make([]models.MeetingRoomJitsi, 0, 2001)
	for i := 0; i < 2001; i++ {
		meetings = append(meetings, models.MeetingRoomJitsi{
			ID: "migration-ended-batch-" + strconv.Itoa(i), TenantID: 7, TicketID: "481", RoomName: "migration-room-" + strconv.Itoa(i),
			Status: "ended", EndedAt: &endedAt,
			BaseModel: models.BaseModel{CreatedAt: endedAt.Add(-time.Minute), UpdatedAt: endedAt},
		})
	}
	if err := db.CreateInBatches(meetings, 500).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}

	if err := backfillEmptyEndedMeetingTranscriptArchives(db); err != nil {
		t.Fatalf("backfill empty transcript archives: %v", err)
	}
	var count int64
	if err := db.Model(&models.MeetingTranscriptSegment{}).Where("provider = ?", "system").Count(&count).Error; err != nil || count != int64(len(meetings)) {
		t.Fatalf("backfilled transcript count = %d, want %d, err %v", count, len(meetings), err)
	}
}
